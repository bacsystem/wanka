package jobs

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// pingArgs es el job de prueba de tenant.
type pingArgs struct {
	TenantID pgtype.UUID `json:"tenant_id"`
	Key      string      `json:"key"`
}

func (pingArgs) Kind() string          { return "test.ping" }
func (a pingArgs) Tenant() pgtype.UUID { return a.TenantID }

// noTenantArgs no implementa TenantArgs: el job es de plataforma.
type noTenantArgs struct {
	Key string `json:"key"`
}

func (noTenantArgs) Kind() string { return "test.notenant" }

func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func uuidOf(t testing.TB, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatalf("parsing uuid %q: %v", s, err)
	}
	return u
}

// migrated devuelve una base con las migraciones del kernel.
func migrated(t testing.TB) *dbtest.DB {
	t.Helper()
	d := dbtest.New(t)
	d.Migrate(context.Background(), t)
	return d
}

func openPool(t testing.TB, d *dbtest.DB, role string, maxConns int32) *db.Pool {
	t.Helper()
	p, err := db.Open(context.Background(), d.URL(role), db.Options{MaxConns: maxConns, AppName: "jobs-test-" + role})
	if err != nil {
		t.Fatalf("opening the %s pool: %v", role, err)
	}
	t.Cleanup(p.Close)
	return p
}

// eventually reintenta cond cada 20 ms hasta el plazo.
func eventually(t testing.TB, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// jobRow es una fila de ops.river_job, leída como superusuario.
type jobRow struct {
	ID       int64
	State    string
	Attempt  int
	Max      int
	Queue    string
	Errors   int
	LastErr  string
	Finished bool
}

func jobsOfKind(ctx context.Context, t testing.TB, conn *pgx.Conn, kind string) []jobRow {
	t.Helper()
	rows, err := conn.Query(ctx, `
		SELECT id, state::text, attempt, max_attempts, queue, coalesce(array_length(errors, 1), 0),
		       coalesce(errors[array_length(errors, 1)]->>'error', ''), finalized_at IS NOT NULL
		FROM ops.river_job WHERE kind = $1 ORDER BY id`, kind)
	if err != nil {
		t.Fatalf("listing jobs: %v", err)
	}
	defer rows.Close()
	var out []jobRow
	for rows.Next() {
		var r jobRow
		if err := rows.Scan(&r.ID, &r.State, &r.Attempt, &r.Max, &r.Queue, &r.Errors, &r.LastErr, &r.Finished); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// enqueueTx encola args en una transacción propia de tenant (wanka_app) y la confirma.
func enqueueTx(ctx context.Context, t testing.TB, pool *db.Pool, enq Enqueuer, s dbtest.Seeded, args Args, opts ...Option) {
	t.Helper()
	err := pool.InTenantTx(ctx, db.Actor{TenantID: uuidOf(t, s.TenantID), UserID: uuidOf(t, s.UserID), Kind: db.KindUser},
		func(tx pgx.Tx) error { return enq.InsertTx(ctx, tx, args, opts...) })
	if err != nil {
		t.Fatalf("enqueueing %s: %v", args.Kind(), err)
	}
}

// startWorker arranca un worker con los handlers que registre setup y devuelve
// la función que lo detiene y espera su Run.
func startWorker(t testing.TB, pool *db.Pool, cfg Config, setup func(*Registry)) (stop func() error) {
	t.Helper()
	reg := NewRegistry(pool)
	setup(reg)
	if cfg.Concurrency == nil {
		cfg.Concurrency = DefaultConcurrency
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	if cfg.RetryBase == 0 {
		cfg.RetryBase, cfg.RetryMax = 20*time.Millisecond, 200*time.Millisecond
	}
	w, err := NewWorker(pool, reg, cfg, discardLog())
	if err != nil {
		t.Fatalf("creating the worker: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		err := w.Run(ctx)
		if err != nil {
			t.Logf("worker Run: %v", err)
		}
		errc <- err
	}()
	var stopped bool
	stop = func() error {
		if stopped {
			return nil
		}
		stopped = true
		cancel()
		select {
		case err := <-errc:
			return err
		case <-time.After(30 * time.Second):
			return context.DeadlineExceeded
		}
	}
	t.Cleanup(func() { _ = stop() })
	return stop
}

func scanStrings(t testing.TB, rows pgx.Rows) []string {
	t.Helper()
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func onlyRiver(in []string) []string {
	var out []string
	for _, s := range in {
		if strings.Contains(s, "river_") {
			out = append(out, s)
		}
	}
	return out
}

// openSuper abre un pool con el superusuario de la base de prueba, que no cae bajo ningún rol de la aplicación.
func openSuper(t testing.TB, d *dbtest.DB) (*db.Pool, error) {
	t.Helper()
	p, err := db.Open(context.Background(), d.SuperuserURL(), db.Options{MaxConns: 2})
	if err == nil {
		t.Cleanup(p.Close)
	}
	return p, err
}
