package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

// schema es el esquema de las tablas de River (no de tenant).
const schema = "ops"

// uniqueViolation es el SQLSTATE de river_job_unique_idx.
const uniqueViolation = "23505"

// uniqueStates son los estados en los que un job «está en curso» para WithUnique.
// River exige available, pending, running y scheduled; retryable se suma para
// que un job que espera su reintento tampoco se duplique.
var uniqueStates = []rivertype.JobState{
	rivertype.JobStateAvailable,
	rivertype.JobStatePending,
	rivertype.JobStateRetryable,
	rivertype.JobStateRunning,
	rivertype.JobStateScheduled,
}

type riverEnqueuer struct {
	client *river.Client[pgx.Tx]
}

// NewEnqueuer construye el Enqueuer de River para el pool de un servicio (api,
// webhooks o platform-api). No arranca nada: solo inserta filas, y con el rol
// del servicio basta el INSERT sobre ops.river_job.
func NewEnqueuer(pool *db.Pool) (Enqueuer, error) {
	// Sin Queues ni Workers, River crea un cliente solo de inserción.
	c, err := river.NewClient(pool.RiverDriver(), &river.Config{Schema: schema})
	if err != nil {
		return nil, fmt.Errorf("creating the River insert client: %w", err)
	}
	return &riverEnqueuer{client: c}, nil
}

func (e *riverEnqueuer) InsertTx(ctx context.Context, tx pgx.Tx, args Args, opts ...Option) error {
	o, err := resolveOptions(opts)
	if err != nil {
		return err
	}
	io := &river.InsertOpts{Queue: o.queue, MaxAttempts: o.maxAttempts, ScheduledAt: o.scheduledAt}
	if o.unique {
		io.UniqueOpts = river.UniqueOpts{ByArgs: true, ByState: uniqueStates}
	}
	params := []river.InsertManyParams{{Args: args, InsertOpts: io}}

	if !o.unique {
		// COPY: solo necesita INSERT y no lee la fila de vuelta.
		if _, err := e.client.InsertManyFastTx(ctx, tx, params); err != nil {
			return fmt.Errorf("inserting job %s: %w", args.Kind(), err)
		}
		return nil
	}
	// COPY no resuelve un conflicto de unicidad: falla con 23505 y aborta la
	// transacción. Un savepoint lo contiene, y un duplicado en curso no es un
	// error del llamador. (La vía de River que lo resuelve con ON CONFLICT lee
	// la fila de vuelta y exigiría SELECT y UPDATE a los roles que encolan.)
	sp, err := tx.Begin(ctx)
	if err != nil {
		return fmt.Errorf("opening savepoint for job %s: %w", args.Kind(), err)
	}
	if _, err := e.client.InsertManyFastTx(ctx, sp, params); err != nil {
		_ = sp.Rollback(context.WithoutCancel(ctx))
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return nil
		}
		return fmt.Errorf("inserting job %s: %w", args.Kind(), err)
	}
	if err := sp.Commit(ctx); err != nil {
		return fmt.Errorf("releasing savepoint for job %s: %w", args.Kind(), err)
	}
	return nil
}
