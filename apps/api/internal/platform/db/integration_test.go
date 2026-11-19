package db_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

func discardLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func open(t *testing.T, url string, o db.Options) *db.Pool {
	t.Helper()
	p, err := db.Open(context.Background(), url, o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func TestPoolConnectsWithServiceRoleAndPingSucceeds(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	p := open(t, d.URL(dbtest.App), db.Options{MaxConns: 3, AppName: "wanka-test"})
	if err := p.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	conn, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	var user, app, timeout string
	err = conn.QueryRow(ctx, `SELECT current_user, current_setting('application_name'), current_setting('statement_timeout')`).Scan(&user, &app, &timeout)
	if err != nil {
		t.Fatal(err)
	}
	if user != "wanka_app" || app != "wanka-test" || timeout != "5s" {
		t.Fatalf("user=%q application_name=%q statement_timeout=%q", user, app, timeout)
	}
}

func TestPoolNeverExceedsMaxConns(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	const max = 3
	p := open(t, d.URL(dbtest.App), db.Options{MaxConns: max, AcquireTimeout: 2 * time.Second})

	var inUse, peak atomic.Int32
	var wg sync.WaitGroup
	for range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := p.Acquire(ctx)
			if err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			defer conn.Release()
			n := inUse.Add(1)
			for {
				old := peak.Load()
				if n <= old || peak.CompareAndSwap(old, n) {
					break
				}
			}
			_, _ = conn.Exec(ctx, `SELECT pg_sleep(0.01)`)
			inUse.Add(-1)
		}()
	}
	wg.Wait()
	if peak.Load() > max {
		t.Fatalf("peak %d connections in use, pool size %d", peak.Load(), max)
	}
}

// Pool de tamaño 1 con su única conexión retenida: un segundo llamador espera el
// plazo de adquisición y recibe ErrOverloaded, que la capa HTTP convierte en
// 503 overloaded con Retry-After: 1 (conventions.md §4, capacity.md).
func TestExhaustedPoolIsOverloaded503(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	p := open(t, d.URL(dbtest.App), db.Options{MaxConns: 1})

	held, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			held.Release()
		}
	}
	defer release()

	handler := httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := p.Acquire(r.Context())
		if err != nil {
			if !httpx.WriteDBError(w, r, err) {
				http.Error(w, "unexpected", http.StatusInternalServerError)
			}
			return
		}
		defer conn.Release()
		w.WriteHeader(http.StatusNoContent)
	}), discardLog())

	start := time.Now()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/needs-db", nil))
	elapsed := time.Since(start)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q", got)
	}
	var problem map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil || problem["code"] != "overloaded" {
		t.Fatalf("problem = %s (%v)", rec.Body, err)
	}
	if elapsed < db.DefaultAcquireTimeout || elapsed > 2*time.Second {
		t.Fatalf("answered after %v, want about %v", elapsed, db.DefaultAcquireTimeout)
	}

	// Liberar la conexión restablece el servicio: la sobrecarga no es permanente.
	release()
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/needs-db", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("after release: status %d, body %s", rec.Code, rec.Body)
	}
}

func TestReadyzIsHealthyAndFast(t *testing.T) {
	d := dbtest.New(t)
	p := open(t, d.URL(dbtest.App), db.Options{MaxConns: 5})
	h := httpx.NewMux(p.Ping)

	const n = 200
	times := make([]time.Duration, 0, n)
	for range n {
		rec := httptest.NewRecorder()
		start := time.Now()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		times = append(times, time.Since(start))
		if rec.Code != http.StatusOK {
			t.Fatalf("readyz %d: %s", rec.Code, rec.Body)
		}
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	if p95 := times[n*95/100]; p95 >= 50*time.Millisecond {
		t.Fatalf("readyz p95 = %v, want < 50ms", p95)
	}
}

func TestReadyzReportsDependencyUnavailableWhenDatabaseDown(t *testing.T) {
	p := open(t, closedPortURL(t), db.Options{MaxConns: 2})
	h := httpx.Chain(httpx.NewMux(p.Ping), discardLog())

	for range 3 { // el proceso sigue respondiendo, siempre
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		var problem map[string]any
		if rec.Code != http.StatusServiceUnavailable || json.Unmarshal(rec.Body.Bytes(), &problem) != nil || problem["code"] != "dependency_unavailable" {
			t.Fatalf("readyz = %d %s", rec.Code, rec.Body)
		}
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("healthz = %d: liveness must not depend on the database", rec.Code)
		}
	}
}

func TestDatabaseThatStopsRespondingIsUnavailable(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	p := open(t, d.URL(dbtest.App), db.Options{MaxConns: 2})
	if err := p.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	// Se mata la conexión del pool desde el servidor: el siguiente uso debe
	// recuperarse o fallar como no disponible, nunca entrar en pánico ni colgarse.
	admin := d.Admin(ctx, t)
	if _, err := admin.Exec(ctx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE usename = 'wanka_app' AND datname = $1`, d.Name); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := p.Ping(ctx); err == nil {
			return // reconnected
		} else if !errors.Is(err, db.ErrUnavailable) {
			t.Fatalf("Ping = %v, want nil or ErrUnavailable", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("pool did not recover after its connection was terminated")
}

func TestApplicationRolesCannotCreateObjects(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	d.Migrate(ctx, t)
	for _, role := range []string{dbtest.App, dbtest.Worker, dbtest.Webhooks, dbtest.Platform} {
		t.Run(role, func(t *testing.T) {
			conn, err := pgx.Connect(ctx, d.URL(role))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close(ctx)
			for _, stmt := range []string{
				`CREATE TABLE public.nope (id int)`,
				`CREATE TABLE app.nope (id int)`,
				`CREATE SCHEMA nope`,
			} {
				_, err := conn.Exec(ctx, stmt)
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
					t.Fatalf("%s: err = %v, want SQLSTATE 42501", stmt, err)
				}
			}
		})
	}
}

// Con el pool lleno y la base sana, la readiness no espera 2 s ni culpa a la
// base: responde 503 overloaded como cualquier petición sin conexión libre
// (conventions.md §4).
func TestReadyzWithAFullPoolIsOverloadedAndFast(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	p := open(t, d.URL(dbtest.App), db.Options{MaxConns: 1})
	held, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	h := httpx.Chain(httpx.NewMux(p.Ping), discardLog())
	start := time.Now()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("readyz took %v with a full pool", elapsed)
	}
	var problem map[string]any
	if rec.Code != http.StatusServiceUnavailable || json.Unmarshal(rec.Body.Bytes(), &problem) != nil || problem["code"] != "overloaded" {
		t.Fatalf("readyz = %d %s, want 503 overloaded", rec.Code, rec.Body)
	}
	if rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
}

// slowConnectProxy reenvía a upstream y demora cada conexión nueva: simula una
// red lenta al abrir conexiones, con la base sana.
func slowConnectProxy(t *testing.T, upstream string, delay time.Duration) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				time.Sleep(delay)
				u, err := net.Dial("tcp", upstream)
				if err != nil {
					return
				}
				defer u.Close()
				go func() { _, _ = io.Copy(u, c) }()
				_, _ = io.Copy(c, u)
			}()
		}
	}()
	return ln.Addr().String()
}

// Con la base sana pero una conexión nueva lenta (un proceso sin tráfico se
// queda sin conexiones), la readiness espera la conexión y responde 200: no la
// confunde con un pool lleno ni con una base caída.
func TestReadyzWithASlowNewConnectionIsStillReady(t *testing.T) {
	d := dbtest.New(t)
	u, err := url.Parse(d.URL(dbtest.App))
	if err != nil {
		t.Fatal(err)
	}
	u.Host = slowConnectProxy(t, u.Host, 3*db.DefaultAcquireTimeout)
	p := open(t, u.String(), db.Options{MaxConns: 3})

	h := httpx.Chain(httpx.NewMux(p.Ping), discardLog())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("first readyz with a slow connection = %d %s, want 200", rec.Code, rec.Body)
	}
}
