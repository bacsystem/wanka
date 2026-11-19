package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

// Retención de los jobs terminados (data-model §5): completados 7 días,
// descartados 30. Los cancelados (errores definitivos) se guardan 7 días.
const (
	CompletedRetention = 7 * 24 * time.Hour
	CancelledRetention = 7 * 24 * time.Hour
	DiscardedRetention = 30 * 24 * time.Hour
)

// stopGrace es lo que se espera, tras cancelar el contexto de los jobs que no
// terminaron a tiempo, a que devuelvan.
const stopGrace = 5 * time.Second

// Registry reúne los handlers y los jobs periódicos que ejecuta un worker.
type Registry struct {
	workers  *river.Workers
	periodic []*river.PeriodicJob
	db       txRunner
	kinds    int
}

// noopArgs es el job que no hace nada. River no arranca con un Workers vacío, y
// el comando worker corre sin handlers hasta que lleguen los de cada módulo, así
// que todo Registry lo trae registrado.
type noopArgs struct{}

func (noopArgs) Kind() string { return "kernel.noop" }

// NewRegistry crea un Registry cuyos handlers abren sus transacciones con pool.
func NewRegistry(pool *db.Pool) *Registry {
	r := &Registry{workers: river.NewWorkers(), db: pool}
	river.AddWorker(r.workers, river.WorkFunc(func(context.Context, *river.Job[noopArgs]) error { return nil }))
	return r
}

// Register agrega el handler de los jobs A. Falla si el Kind ya tiene handler.
// El handler devuelve nil, un error transitorio, Cancel(err) o Snooze(d).
func Register[A Args](r *Registry, handler func(ctx context.Context, job Job[A]) error) error {
	work := river.WorkFunc(func(ctx context.Context, rj *river.Job[A]) error {
		err := handler(ctx, Job[A]{
			ID:          rj.ID,
			Attempt:     rj.Attempt,
			MaxAttempts: rj.MaxAttempts,
			Args:        rj.Args,
			db:          r.db,
		})
		return toRiverError(err)
	})
	if err := river.AddWorkerSafely(r.workers, work); err != nil {
		return fmt.Errorf("registering job handler: %w", err)
	}
	r.kinds++
	return nil
}

// toRiverError traduce Cancel y Snooze a los resultados de River.
func toRiverError(err error) error {
	var c *cancelError
	var s *snoozeError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &c):
		return river.JobCancel(err)
	case errors.As(err, &s):
		return river.JobSnooze(s.d)
	default:
		return err
	}
}

// AddPeriodic encola un job cada every. Solo el líder elegido entre las
// réplicas del worker lo hace, así que corre una vez por ciclo aunque haya N
// réplicas. Si el líder cambia, el ciclo se reinicia: un handler periódico
// también debe ser idempotente (WithUnique evita encimar ciclos).
func (r *Registry) AddPeriodic(every time.Duration, newArgs func() Args, opts ...Option) error {
	if every <= 0 {
		return fmt.Errorf("%w: periodic interval must be positive", ErrInvalidOption)
	}
	o, err := resolveOptions(opts)
	if err != nil {
		return err
	}
	probe := newArgs()
	r.periodic = append(r.periodic, river.NewPeriodicJob(
		river.PeriodicInterval(every),
		func() (river.JobArgs, *river.InsertOpts) {
			io := &river.InsertOpts{Queue: o.queue, MaxAttempts: o.maxAttempts}
			if o.unique {
				io.UniqueOpts = river.UniqueOpts{ByArgs: true, ByState: uniqueStates}
			}
			return newArgs(), io
		},
		&river.PeriodicJobOpts{ID: probe.Kind()},
	))
	return nil
}

// Worker ejecuta los jobs: las colas con su concurrencia, los reintentos, los
// periódicos y la limpieza de terminados.
type Worker struct {
	client  *river.Client[pgx.Tx]
	log     *slog.Logger
	timeout time.Duration
}

// NewWorker construye el worker sobre el pool de wanka_worker. No arranca nada.
func NewWorker(pool *db.Pool, reg *Registry, cfg Config, log *slog.Logger) (*Worker, error) {
	queues := make(map[string]river.QueueConfig, len(cfg.Concurrency))
	for name, n := range cfg.Concurrency {
		queues[name] = river.QueueConfig{MaxWorkers: n}
	}
	rc := &river.Config{
		Schema:       schema,
		Queues:       queues,
		Workers:      reg.workers,
		PeriodicJobs: reg.periodic,
		Logger:       log.With("component", "river"),
		MaxAttempts:  DefaultMaxAttempts,
		RetryPolicy:  newRetryPolicy(orDefault(cfg.RetryBase, DefaultRetryBase), orDefault(cfg.RetryMax, DefaultRetryMax)),

		CompletedJobRetentionPeriod: CompletedRetention,
		CancelledJobRetentionPeriod: CancelledRetention,
		DiscardedJobRetentionPeriod: DiscardedRetention,
		// El reindexado de River exige ser dueño del índice, y el dueño es wanka_migrator.
		ReindexerIndexNames: []string{},
	}
	c, err := river.NewClient(pool.RiverDriver(), rc)
	if err != nil {
		return nil, fmt.Errorf("creating the River worker client: %w", err)
	}
	return &Worker{client: c, log: log, timeout: cfg.ShutdownTimeout}, nil
}

func orDefault(v, def time.Duration) time.Duration {
	if v > 0 {
		return v
	}
	return def
}

// Run arranca las colas y bloquea hasta que ctx termina; entonces deja de tomar
// jobs y espera a los que están en curso hasta ShutdownTimeout. Pasado el plazo
// cancela el contexto de los que siguen (River los reintenta después) y espera
// unos segundos más. Devuelve error si no pudo arrancar o si algún job no
// devolvió ni cancelado.
func (w *Worker) Run(ctx context.Context) error {
	// El arranque no depende de ctx: la parada la manejamos nosotros, abajo.
	if err := w.client.Start(context.WithoutCancel(ctx)); err != nil {
		return fmt.Errorf("starting the job queues: %w", err)
	}
	w.log.Info("started")
	select {
	case <-ctx.Done():
	case <-w.client.Stopped():
		return errors.New("the job queues stopped on their own")
	}

	w.log.Info("stopping, waiting for running jobs", "timeout", w.timeout.String())
	soft, cancel := context.WithTimeout(context.WithoutCancel(ctx), w.timeout)
	defer cancel()
	err := w.client.Stop(soft)
	if err == nil {
		return nil
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("stopping the job queues: %w", err)
	}
	w.log.Warn("jobs still running after the shutdown timeout, cancelling them")
	hard, cancelHard := context.WithTimeout(context.WithoutCancel(ctx), stopGrace)
	defer cancelHard()
	if err := w.client.StopAndCancel(hard); err != nil {
		return fmt.Errorf("cancelling the running jobs: %w", err)
	}
	return nil
}
