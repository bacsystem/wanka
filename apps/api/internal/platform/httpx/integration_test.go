package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

const (
	itTenant = "aaaaaaaa-0000-0000-0000-00000000000a"
	itUser   = "11111111-1111-1111-1111-111111111111"
)

// httpDB devuelve un pool de wanka_app sobre una base con la tabla rlstest.items
// (la migración de prueba de db) y una conexión de superusuario para mirar.
func httpDB(t *testing.T, maxConns int32) (*db.Pool, *pgx.Conn, actorFn) {
	t.Helper()
	probe, err := os.ReadFile("../db/testdata/99990101000000_tenant_probe.sql")
	if err != nil {
		t.Fatal(err)
	}
	d := dbtest.New(t)
	ctx := context.Background()
	d.MigrateWith(ctx, t, fstest.MapFS{"99990101000000_tenant_probe.sql": {Data: probe}})
	p, err := db.Open(ctx, d.URL(dbtest.App), db.Options{MaxConns: maxConns, AcquireTimeout: 2 * time.Second, AppName: "httpx-it"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p, d.Admin(ctx, t), func() db.Actor {
		var tenant, user pgtype.UUID
		if err := tenant.Scan(itTenant); err != nil {
			t.Fatal(err)
		}
		if err := user.Scan(itUser); err != nil {
			t.Fatal(err)
		}
		return db.Actor{TenantID: tenant, UserID: user, Kind: db.KindUser}
	}
}

type actorFn func() db.Actor

func rowCount(t *testing.T, admin *pgx.Conn) int {
	t.Helper()
	var n int
	if err := admin.QueryRow(context.Background(), `SELECT count(*) FROM rlstest.items`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// El plazo del handler cancela el contexto: pgx cancela la sentencia, InTenantTx
// hace ROLLBACK y la conexión vuelve al pool en menos de 1 s.
func TestHandlerDeadlineRollsBackAndReturnsTheConnection(t *testing.T) {
	p, admin, actor := httpDB(t, 2)
	handlerErr := make(chan error, 1)
	h, buf, _ := chainWith(t, httpx.RouterOptions{HandlerTimeout: 300 * time.Millisecond}, func(rt *httpx.Router) {
		rt.HandleFunc("POST /slow", func(w http.ResponseWriter, r *http.Request) {
			err := p.InTenantTx(r.Context(), actor(), func(tx pgx.Tx) error {
				if _, err := tx.Exec(r.Context(), `INSERT INTO rlstest.items (tenant_id, name) VALUES ($1, 'will be rolled back')`, itTenant); err != nil {
					return err
				}
				_, err := tx.Exec(r.Context(), `SELECT pg_sleep(30)`)
				return err
			})
			handlerErr <- err
			httpx.WriteError(w, r, err)
		})
	})
	start := time.Now()
	rec := do(h, httptest.NewRequest(http.MethodPost, "/slow", nil))
	mustProblem(t, rec, http.StatusServiceUnavailable, "overloaded")
	if rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("the response took %v: the deadline did not cut the 30 s sleep", took)
	}
	responded := time.Now()

	// En menos de 1 s desde el 503: sin conexión adquirida ni sentencia activa.
	deadline := responded.Add(time.Second)
	for {
		var sleeping int
		if err := admin.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE application_name = 'httpx-it' AND state <> 'idle' AND pid <> pg_backend_pid()`).Scan(&sleeping); err != nil {
			t.Fatal(err)
		}
		if p.AcquiredConns() == 0 && sleeping == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("1 s after the 503: AcquiredConns = %d, active statements = %d", p.AcquiredConns(), sleeping)
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case err := <-handlerErr:
		if err == nil {
			t.Fatal("the cancelled statement must return an error")
		}
	case <-time.After(time.Second):
		t.Fatal("the handler is still inside InTenantTx")
	}
	if n := rowCount(t, admin); n != 0 {
		t.Fatalf("%d rows written: the transaction was not rolled back", n)
	}
	if strings.Contains(buf.String(), `"level":"ERROR"`) {
		t.Fatalf("a deadline must not log an error: %s", buf.String())
	}
	// El pool sirve la siguiente petición.
	if err := p.InTenantReadTx(context.Background(), actor(), func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `SELECT 1`)
		return err
	}); err != nil {
		t.Fatalf("pool unusable after the deadline: %v", err)
	}
}

// Una sentencia que supera el statement_timeout llega a WriteDBError tipada y
// responde 503 dependency_unavailable sin nada de PostgreSQL.
func TestStatementTimeoutEndToEnd(t *testing.T) {
	p, admin, actor := httpDB(t, 2)
	h, buf, m := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("POST /items", func(w http.ResponseWriter, r *http.Request) {
			err := p.InTenantTx(r.Context(), actor(), func(tx pgx.Tx) error {
				if _, err := tx.Exec(r.Context(), `SET LOCAL statement_timeout = '50ms'`); err != nil {
					return err
				}
				if _, err := tx.Exec(r.Context(), `INSERT INTO rlstest.items (tenant_id, name) VALUES ($1, 'x')`, itTenant); err != nil {
					return err
				}
				_, err := tx.Exec(r.Context(), `SELECT pg_sleep(1)`)
				return err
			})
			httpx.WriteError(w, r, err)
		})
	})
	rec := do(h, httptest.NewRequest(http.MethodPost, "/items", nil))
	mustProblem(t, rec, http.StatusServiceUnavailable, "dependency_unavailable")
	if rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	body := strings.ToLower(rec.Body.String())
	for _, banned := range []string{"57014", "statement", "timeout", "canceling", "pg_sleep"} {
		if strings.Contains(body, banned) {
			t.Fatalf("body contains %q: %s", banned, body)
		}
	}
	if n := rowCount(t, admin); n != 0 {
		t.Fatalf("%d rows written", n)
	}
	warns := buf.lines(t, "statement timeout")
	if len(warns) != 1 || warns[0]["sqlstate"] != "57014" || warns[0]["phase"] != "handler" || warns[0]["route"] != "POST /items" {
		t.Fatalf("warn = %v", warns)
	}
	if strings.Contains(buf.String(), "pg_sleep") || strings.Contains(buf.String(), "INSERT") || strings.Contains(buf.String(), "canceling") {
		t.Fatalf("the log carries SQL or the PostgreSQL message: %s", buf.String())
	}
	if m.get("db_statement_timeouts_total{phase,handler,route,POST /items}") != 1 {
		t.Fatalf("counter = %v", m.counts)
	}
	if lines := buf.lines(t, "request"); len(lines) != 1 || lines[0]["status"] != float64(503) {
		t.Fatalf("access log = %v", lines)
	}
}

// Con un pool caliente cuyas sesiones mueren (reinicio, failover,
// pg_terminate_backend), cada petición da 503 dependency_unavailable y nunca 500.
func TestDeadPoolConnectionsAre503NotInternalError(t *testing.T) {
	p, admin, actor := httpDB(t, 3)
	ctx := context.Background()
	h, buf, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /items", func(w http.ResponseWriter, r *http.Request) {
			err := p.InTenantReadTx(r.Context(), actor(), func(tx pgx.Tx) error {
				var n int
				return tx.QueryRow(r.Context(), `SELECT count(*) FROM rlstest.items`).Scan(&n)
			})
			if err != nil {
				httpx.WriteError(w, r, err)
			}
		})
	})
	// Calienta las 3 conexiones: tres transacciones abiertas a la vez.
	var txs []pgx.Tx
	release := make(chan struct{})
	started := make(chan struct{}, 3)
	done := make(chan struct{}, 3)
	for i := 0; i < 3; i++ {
		go func() {
			_ = p.InTenantReadTx(ctx, actor(), func(tx pgx.Tx) error {
				started <- struct{}{}
				<-release
				return nil
			})
			done <- struct{}{}
		}()
	}
	for i := 0; i < 3; i++ {
		<-started
	}
	_ = txs
	close(release)
	for i := 0; i < 3; i++ {
		<-done
	}
	if _, err := admin.Exec(ctx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity
		WHERE usename = 'wanka_app' AND datname = current_database() AND pid <> pg_backend_pid()`); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	var unavailable int
	for i := 0; i < 5; i++ {
		rec := do(h, httptest.NewRequest(http.MethodGet, "/items", nil))
		switch rec.Code {
		case http.StatusServiceUnavailable:
			mustProblem(t, rec, http.StatusServiceUnavailable, "dependency_unavailable")
			if rec.Header().Get("Retry-After") != "1" {
				t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
			}
			if strings.Contains(strings.ToLower(rec.Body.String()), "terminating") || strings.Contains(rec.Body.String(), "57P01") {
				t.Fatalf("body leaks the PostgreSQL message: %s", rec.Body.String())
			}
			unavailable++
		case http.StatusOK:
		default:
			t.Fatalf("request %d: %d (a dead connection must never be a 500): %s %s", i, rec.Code, rec.Body.String(), buf.String())
		}
	}
	if unavailable == 0 {
		t.Fatal("no request hit a dead connection: the test did not exercise the failure")
	}
}
