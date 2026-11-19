package jobs

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// workerEnv es una base migrada con un tenant, el pool del worker y el del encolador.
type workerEnv struct {
	d      *dbtest.DB
	s      dbtest.Seeded
	worker *db.Pool
	app    *db.Pool
	enq    Enqueuer
	admin  *pgx.Conn
}

func newWorkerEnv(t *testing.T, workerConns int32) *workerEnv {
	t.Helper()
	ctx := ctx5(t)
	d := migrated(t)
	e := &workerEnv{d: d, s: d.SeedSession(t, dbtest.SessionSpec{})}
	e.worker = openPool(t, d, dbtest.Worker, workerConns)
	e.app = openPool(t, d, dbtest.App, 5)
	e.enq = newEnqueuer(t, e.app)
	e.admin = d.Admin(ctx, t)
	return e
}

func (e *workerEnv) ping(t *testing.T, key string, opts ...Option) {
	t.Helper()
	if len(opts) == 0 {
		opts = []Option{WithQueue(QueueMaintenance)}
	}
	enqueueTx(context.Background(), t, e.app, e.enq, e.s, pingArgs{TenantID: uuidOf(t, e.s.TenantID), Key: key}, opts...)
}

func (e *workerEnv) rows(t *testing.T, kind string) []jobRow {
	t.Helper()
	return jobsOfKind(context.Background(), t, e.admin, kind)
}

func registerPing(t *testing.T, h func(context.Context, Job[pingArgs]) error) func(*Registry) {
	return func(r *Registry) {
		if err := Register(r, h); err != nil {
			t.Fatal(err)
		}
	}
}

// Criterio 4 (error transitorio): se reintenta con backoff hasta el máximo de intentos y queda discarded.
func TestTransientErrorRetriesUntilMaxAttemptsThenIsDiscarded(t *testing.T) {
	e := newWorkerEnv(t, 5)
	var calls atomic.Int32
	startWorker(t, e.worker, Config{}, registerPing(t, func(ctx context.Context, j Job[pingArgs]) error {
		calls.Add(1)
		return errors.New("the provider is down")
	}))
	e.ping(t, "retry", WithQueue(QueueMaintenance), WithMaxAttempts(4))

	eventually(t, 20*time.Second, "the job to be discarded", func() bool {
		r := e.rows(t, "test.ping")
		return len(r) == 1 && r[0].State == "discarded"
	})
	r := e.rows(t, "test.ping")[0]
	if r.Attempt != 4 || r.Errors != 4 || !r.Finished || calls.Load() != 4 {
		t.Fatalf("attempt %d, errors %d, finalized %v, handler calls %d; want 4, 4, true, 4", r.Attempt, r.Errors, r.Finished, calls.Load())
	}
	if r.LastErr != "the provider is down" {
		t.Fatalf("last error %q", r.LastErr)
	}
}

// Criterio 4 (error definitivo): Cancel no reintenta.
func TestCancelDoesNotRetry(t *testing.T) {
	e := newWorkerEnv(t, 5)
	var calls atomic.Int32
	startWorker(t, e.worker, Config{}, registerPing(t, func(ctx context.Context, j Job[pingArgs]) error {
		calls.Add(1)
		return Cancel(errors.New("the document was rejected"))
	}))
	e.ping(t, "cancel")

	eventually(t, 10*time.Second, "the job to be cancelled", func() bool {
		r := e.rows(t, "test.ping")
		return len(r) == 1 && r[0].State == "cancelled"
	})
	time.Sleep(500 * time.Millisecond) // un reintento erróneo ya habría vuelto a llamar al handler
	r := e.rows(t, "test.ping")[0]
	if r.Attempt != 1 || calls.Load() != 1 || r.State != "cancelled" {
		t.Fatalf("state %s, attempt %d, calls %d; want cancelled, 1, 1", r.State, r.Attempt, calls.Load())
	}
}

// Snooze vuelve a correr el job sin gastar un intento: con un solo intento permitido, completa.
func TestSnoozeRunsAgainWithoutSpendingAnAttempt(t *testing.T) {
	e := newWorkerEnv(t, 5)
	var calls atomic.Int32
	startWorker(t, e.worker, Config{}, registerPing(t, func(ctx context.Context, j Job[pingArgs]) error {
		if calls.Add(1) == 1 {
			return Snooze(100 * time.Millisecond)
		}
		return nil
	}))
	e.ping(t, "snooze", WithQueue(QueueMaintenance), WithMaxAttempts(1))

	eventually(t, 10*time.Second, "the job to complete", func() bool {
		r := e.rows(t, "test.ping")
		return len(r) == 1 && r[0].State == "completed"
	})
	if r := e.rows(t, "test.ping")[0]; calls.Load() != 2 || r.Attempt != 1 {
		t.Fatalf("calls %d, attempt %d; want 2 and 1", calls.Load(), r.Attempt)
	}
}

// Un job que no es de tenant corre sin transacción de tenant, y el handler recibe sus datos.
func TestHandlerReceivesTheJobAndAttempt(t *testing.T) {
	e := newWorkerEnv(t, 5)
	got := make(chan Job[pingArgs], 4)
	startWorker(t, e.worker, Config{}, registerPing(t, func(ctx context.Context, j Job[pingArgs]) error {
		got <- j
		if j.Attempt == 1 {
			return errors.New("first attempt fails")
		}
		return nil
	}))
	e.ping(t, "ctx", WithQueue(QueueMaintenance), WithMaxAttempts(5))
	j1, j2 := <-got, <-got
	if j1.Attempt != 1 || j2.Attempt != 2 || j1.MaxAttempts != 5 || j1.ID == 0 || j1.Args.Key != "ctx" || j1.Args.TenantID != uuidOf(t, e.s.TenantID) {
		t.Fatalf("unexpected jobs: %+v, %+v", j1, j2)
	}
}

// Criterio 5: el handler corre en una transacción de tenant de sistema, sin usuario, solo con el tenant del job.
func TestHandlerRunsAsSystemOfTheJobsTenantOnly(t *testing.T) {
	ctx := ctx5(t)
	e := newWorkerEnv(t, 5)
	other := e.d.SeedSession(t, dbtest.SessionSpec{}) // otro tenant con su propio rol
	type seen struct {
		kind, user, tenant string
		userNull           bool
		tenants            []string
		roles              int
	}
	res := make(chan seen, 1)
	startWorker(t, e.worker, Config{}, registerPing(t, func(ctx context.Context, j Job[pingArgs]) error {
		var s seen
		err := j.InTenantTx(ctx, func(tx pgx.Tx) error {
			if err := tx.QueryRow(ctx, `SELECT current_setting('app.session_kind'), coalesce(current_setting('app.user_id'), '<unset>'),
				app.current_user_id() IS NULL, app.current_tenant()::text`).Scan(&s.kind, &s.user, &s.userNull, &s.tenant); err != nil {
				return err
			}
			rows, err := tx.Query(ctx, `SELECT DISTINCT tenant_id::text FROM iam.roles`)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					return err
				}
				s.tenants = append(s.tenants, id)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return tx.QueryRow(ctx, `SELECT count(*) FROM iam.roles`).Scan(&s.roles)
		})
		res <- s
		return err
	}))
	e.ping(t, "isolation")

	select {
	case s := <-res:
		if s.kind != "system" || !s.userNull || s.user != "" || s.tenant != e.s.TenantID {
			t.Fatalf("session: kind %q, user %q (NULL %v), tenant %q; want system, NULL, %s", s.kind, s.user, s.userNull, s.tenant, e.s.TenantID)
		}
		if len(s.tenants) != 1 || s.tenants[0] != e.s.TenantID || s.roles != 1 {
			t.Fatalf("the handler sees roles of tenants %v (%d rows); want only %s (1 row), not %s", s.tenants, s.roles, e.s.TenantID, other.TenantID)
		}
	case <-ctx.Done():
		t.Fatal("the handler never ran")
	}
}

// Un job sin tenant_id recibe ErrNoTenant y no se reintenta (cancelado al primer intento).
func TestJobWithoutTenantGetsErrNoTenantAndIsCancelled(t *testing.T) {
	e := newWorkerEnv(t, 5)
	var gotErr atomic.Value
	var calls atomic.Int32
	startWorker(t, e.worker, Config{}, func(r *Registry) {
		if err := Register(r, func(ctx context.Context, j Job[pingArgs]) error {
			calls.Add(1)
			err := j.InTenantTx(ctx, func(tx pgx.Tx) error {
				t.Error("the transaction must not open without a tenant")
				return nil
			})
			gotErr.Store(&err)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if err := Register(r, func(ctx context.Context, j Job[noTenantArgs]) error {
			calls.Add(1)
			err := j.InTenantTx(ctx, func(pgx.Tx) error { t.Error("no tenant args must not open a transaction"); return nil })
			gotErr.Store(&err)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	})
	// tenant_id ausente (UUID vacío) y argumentos que ni siquiera lo declaran.
	e.ping(t, "empty-tenant") // el helper pone el tenant; lo vaciamos en la fila
	if _, err := e.admin.Exec(context.Background(), `UPDATE ops.river_job SET args = args - 'tenant_id' WHERE kind = 'test.ping'`); err != nil {
		t.Fatal(err)
	}
	enqueueTx(context.Background(), t, e.app, e.enq, e.s, noTenantArgs{Key: "platform"}, WithQueue(QueueMaintenance))

	for _, kind := range []string{"test.ping", "test.notenant"} {
		eventually(t, 10*time.Second, kind+" to be cancelled", func() bool {
			r := e.rows(t, kind)
			return len(r) == 1 && r[0].State == "cancelled"
		})
		if r := e.rows(t, kind)[0]; r.Attempt != 1 {
			t.Fatalf("%s: attempt %d, want 1 (no retries)", kind, r.Attempt)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("handler calls %d, want 2", calls.Load())
	}
	if p, ok := gotErr.Load().(*error); !ok || !errors.Is(*p, db.ErrNoTenant) {
		t.Fatalf("handler error %v, want db.ErrNoTenant", gotErr.Load())
	}
}

// Criterio 3: las cinco colas con su concurrencia, ajustable. Con 6 jobs bloqueados, una cola de 2 corre 2 a la vez.
func TestQueueConcurrencyIsEnforcedPerQueue(t *testing.T) {
	e := newWorkerEnv(t, 5)
	var running, peak atomic.Int32
	release := make(chan struct{})
	cfg := Config{Concurrency: map[string]int{QueueCPE: 20, QueueWebhooks: 10, QueueNotifications: 10, QueueReports: 2, QueueMaintenance: 3}}
	startWorker(t, e.worker, cfg, registerPing(t, func(ctx context.Context, j Job[pingArgs]) error {
		n := running.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
		running.Add(-1)
		return nil
	}))
	for i := range 8 {
		e.ping(t, string(rune('a'+i)), WithQueue(QueueMaintenance))
	}
	eventually(t, 10*time.Second, "3 jobs to run at once", func() bool { return running.Load() == 3 })
	time.Sleep(500 * time.Millisecond) // si el límite fallara, ya habría más
	if running.Load() != 3 || peak.Load() != 3 {
		t.Fatalf("running %d, peak %d; want 3 on the maintenance queue", running.Load(), peak.Load())
	}
	close(release)
	eventually(t, 10*time.Second, "all jobs to complete", func() bool {
		for _, r := range e.rows(t, "test.ping") {
			if r.State != "completed" {
				return false
			}
		}
		return len(e.rows(t, "test.ping")) == 8
	})

	var names []string
	rows, err := e.admin.Query(context.Background(), `SELECT name FROM ops.river_queue ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	want := []string{"cpe", "maintenance", "notifications", "reports", "webhooks"}
	if len(names) != len(want) {
		t.Fatalf("queues started: %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("queues started: %v, want %v", names, want)
		}
	}
}

// Criterio 3 (apagado ordenado): al recibir la señal el worker espera al job en curso, que termina y se completa.
func TestShutdownWaitsForRunningJobs(t *testing.T) {
	e := newWorkerEnv(t, 5)
	started := make(chan struct{})
	var finished atomic.Bool
	var cancelled atomic.Bool
	stop := startWorker(t, e.worker, Config{ShutdownTimeout: 10 * time.Second}, registerPing(t, func(ctx context.Context, j Job[pingArgs]) error {
		close(started)
		select {
		case <-time.After(1500 * time.Millisecond):
			finished.Store(true)
		case <-ctx.Done():
			cancelled.Store(true)
		}
		return nil
	}))
	e.ping(t, "slow")
	<-started

	t0 := time.Now()
	if err := stop(); err != nil {
		t.Fatalf("Run returned %v", err)
	}
	if !finished.Load() || cancelled.Load() {
		t.Fatalf("the running job was interrupted: finished %v, cancelled %v", finished.Load(), cancelled.Load())
	}
	if time.Since(t0) < time.Second {
		t.Fatalf("Run returned after %s, before the job could finish", time.Since(t0))
	}
	if r := e.rows(t, "test.ping"); len(r) != 1 || r[0].State != "completed" {
		t.Fatalf("the job after the shutdown: %+v, want completed", r)
	}
}

// Vencido el plazo, el worker cancela el contexto del job en curso y devuelve; el job queda para reintento.
func TestShutdownCancelsJobsThatOutliveTheTimeout(t *testing.T) {
	e := newWorkerEnv(t, 5)
	started := make(chan struct{})
	var sawCancel atomic.Bool
	stop := startWorker(t, e.worker, Config{ShutdownTimeout: time.Second}, registerPing(t, func(ctx context.Context, j Job[pingArgs]) error {
		close(started)
		<-ctx.Done()
		sawCancel.Store(true)
		return ctx.Err()
	}))
	e.ping(t, "stuck")
	<-started

	t0 := time.Now()
	if err := stop(); err != nil {
		t.Fatalf("Run returned %v", err)
	}
	if d := time.Since(t0); d < 900*time.Millisecond || d > 8*time.Second {
		t.Fatalf("Run returned after %s, want about the 1 s timeout", d)
	}
	if !sawCancel.Load() {
		t.Fatal("the handler context was not cancelled")
	}
	if r := e.rows(t, "test.ping"); len(r) != 1 || r[0].State == "completed" || r[0].State == "discarded" {
		t.Fatalf("a cancelled job must stay retryable, got %+v", r)
	}
}

// Criterio 9: sin conexión retenida durante la llamada de red. Pool de 2, 10 jobs de 500 ms: no se agota.
func TestJobsDoNotHoldAPoolConnectionDuringTheNetworkCall(t *testing.T) {
	e := newWorkerEnv(t, 2)
	var overloaded, failed atomic.Int32
	startWorker(t, e.worker, Config{}, registerPing(t, func(ctx context.Context, j Job[pingArgs]) error {
		short := func() error {
			return j.InTenantTx(ctx, func(tx pgx.Tx) error { _, err := tx.Exec(ctx, `SELECT 1`); return err })
		}
		if err := short(); err != nil { // lee
			if errors.Is(err, db.ErrOverloaded) {
				overloaded.Add(1)
			}
			failed.Add(1)
			return err
		}
		time.Sleep(500 * time.Millisecond) // la «llamada de red»: sin conexión tomada
		if err := short(); err != nil {    // guarda
			if errors.Is(err, db.ErrOverloaded) {
				overloaded.Add(1)
			}
			failed.Add(1)
			return err
		}
		return nil
	}))
	for i := range 10 {
		e.ping(t, string(rune('a'+i)), WithQueue(QueueCPE))
	}
	t0 := time.Now()
	eventually(t, 30*time.Second, "10 jobs to complete", func() bool {
		n := 0
		for _, r := range e.rows(t, "test.ping") {
			if r.State == "completed" {
				n++
			}
		}
		return n == 10
	})
	// Hechos en paralelo tardan ~1 s; con una conexión retenida en la espera tardarían 10 x 0,5 s o más.
	if d := time.Since(t0); d > 8*time.Second {
		t.Errorf("10 jobs took %s with a pool of 2", d)
	}
	for _, r := range e.rows(t, "test.ping") {
		if r.Attempt != 1 {
			t.Errorf("job %d needed %d attempts", r.ID, r.Attempt)
		}
	}
	if overloaded.Load() != 0 || failed.Load() != 0 {
		t.Errorf("%d transactions failed, %d of them with ErrOverloaded", failed.Load(), overloaded.Load())
	}
}

// Criterio no funcional: 44 jobs concurrentes en un pool de 10; la espera por conexión p99 < 50 ms.
func TestPoolWaitStaysUnder50msWith44ConcurrentJobs(t *testing.T) {
	e := newWorkerEnv(t, 10)
	var mu sync.Mutex
	var waits []time.Duration
	startWorker(t, e.worker, Config{}, registerPing(t, func(ctx context.Context, j Job[pingArgs]) error {
		for range 2 {
			t0 := time.Now()
			// La transacción completa (conexión, BEGIN, SET, SELECT, COMMIT) es una cota superior de la espera.
			if err := j.InTenantTx(ctx, func(tx pgx.Tx) error { _, err := tx.Exec(ctx, `SELECT 1`); return err }); err != nil {
				return err
			}
			mu.Lock()
			waits = append(waits, time.Since(t0))
			mu.Unlock()
			time.Sleep(300 * time.Millisecond)
		}
		return nil
	}))
	for i := range 44 {
		e.ping(t, string(rune('A'+i)), WithQueue(QueueCPE))
	}
	eventually(t, 60*time.Second, "44 jobs to complete", func() bool {
		n := 0
		for _, r := range e.rows(t, "test.ping") {
			if r.State == "completed" {
				n++
			}
		}
		return n == 44
	})
	mu.Lock()
	defer mu.Unlock()
	slices.Sort(waits)
	p99 := waits[len(waits)*99/100]
	t.Logf("InTenantTx latency over %d transactions: p50 %s, p99 %s, max %s", len(waits), waits[len(waits)/2], p99, waits[len(waits)-1])
	if p99 >= 50*time.Millisecond {
		t.Errorf("p99 %s, want under 50ms", p99)
	}
}
