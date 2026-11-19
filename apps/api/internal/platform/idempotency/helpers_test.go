package idempotency_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/idempotency"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

const (
	keyA = "0190c1a2-1111-7000-8000-00000000000a"
	keyB = "0190c1a2-2222-7000-8000-00000000000b"
)

// itemReq es el cuerpo de las rutas de prueba. Controla qué hace el handler.
type itemReq struct {
	Name    string `json:"name"`
	SleepMS int    `json:"sleep_ms,omitempty"`
	// Fail: "" (éxito), "shift" (409 shift_not_open mientras el turno esté cerrado), "validation" (422),
	// "internal" (500), "panic", "redirect" (302) y "created" (201 sin cuerpo).
	Fail string `json:"fail,omitempty"`
}

// env es una API de prueba con dos rutas de tenant sobre rlstest.items:
//
//	POST /v1/items  con Idempotency-Key obligatoria
//	POST /v1/notes  con clave opcional
//
// El handler hace su efecto con idempotency.InTx, como lo haría un módulo.
type env struct {
	t     *testing.T
	d     *dbtest.DB
	pool  *db.Pool
	admin *pgx.Conn
	h     http.Handler
	log   *logBuf

	calls     atomic.Int32 // veces que corrió el handler
	shiftOpen atomic.Bool
	// onTx, si está, corre dentro del handler con la transacción de la petición.
	onTx func(ctx context.Context, tx pgx.Tx)

	tenants map[string]dbtest.Seeded // "a" y "b"
}

type logBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *logBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

type envOpts struct {
	lockTimeout    time.Duration
	maxConns       int32
	handlerTimeout time.Duration
}

func newEnv(t *testing.T, o envOpts) *env {
	t.Helper()
	if o.maxConns == 0 {
		o.maxConns = 8
	}
	probe, err := os.ReadFile("../db/testdata/99990101000000_tenant_probe.sql")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	d := dbtest.New(t)
	d.MigrateWith(ctx, t, fstest.MapFS{"99990101000000_tenant_probe.sql": {Data: probe}})
	pool, err := db.Open(ctx, d.URL(dbtest.App), db.Options{MaxConns: o.maxConns, AcquireTimeout: 10 * time.Second, AppName: "idempotency-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	e := &env{t: t, d: d, pool: pool, admin: d.Admin(ctx, t), log: &logBuf{},
		tenants: map[string]dbtest.Seeded{"a": d.SeedSession(t, dbtest.SessionSpec{}), "b": d.SeedSession(t, dbtest.SessionSpec{})}}
	e.shiftOpen.Store(false)

	mw := idempotency.New(pool, idempotency.Config{LockTimeout: o.lockTimeout})
	rt := httpx.NewRouter(httpx.RouterOptions{HandlerTimeout: o.handlerTimeout})
	rt.HandleFunc("POST /v1/items", e.handler(), httpx.Use(e.session, mw.Required()))
	rt.HandleFunc("POST /v1/notes", e.handler(), httpx.Use(e.session, mw.Optional()))
	// Sin sesión: error de cableado.
	rt.HandleFunc("POST /v1/nosession", e.handler(), httpx.Use(mw.Required()))
	e.h = httpx.Chain(rt, logging.New(e.log, slog.LevelDebug), nil)
	return e
}

func uuidOf(t testing.TB, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatal(err)
	}
	return u
}

// session fija la sesión del tenant que pide la cabecera X-Test-Tenant ("a" o "b").
func (e *env) session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := e.tenants[r.Header.Get("X-Test-Tenant")]
		next.ServeHTTP(w, r.WithContext(auth.WithSession(r.Context(), auth.Session{
			ID:    uuidOf(e.t, s.SessionID),
			Actor: db.Actor{TenantID: uuidOf(e.t, s.TenantID), UserID: uuidOf(e.t, s.UserID), Kind: db.KindUser},
		})))
	})
}

func (e *env) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		e.calls.Add(1)
		var in itemReq
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		s, _ := auth.FromContext(r.Context())
		var id int64
		err := idempotency.InTx(r.Context(), e.pool, s.Actor, func(tx pgx.Tx) error {
			if e.onTx != nil {
				e.onTx(r.Context(), tx)
			}
			if err := tx.QueryRow(r.Context(), `INSERT INTO rlstest.items (tenant_id, name) VALUES (app.current_tenant(), $1) RETURNING id`, in.Name).Scan(&id); err != nil {
				return err
			}
			if in.SleepMS > 0 {
				_, err := tx.Exec(r.Context(), `SELECT pg_sleep($1)`, float64(in.SleepMS)/1000)
				return err
			}
			return nil
		})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		switch in.Fail {
		case "shift":
			if !e.shiftOpen.Load() {
				httpx.WriteProblem(w, r, "shift_not_open", "no open shift")
				return
			}
		case "validation":
			httpx.WriteProblem(w, r, "validation_failed", "bad input")
			return
		case "internal":
			httpx.WriteProblem(w, r, "internal", "")
			return
		case "panic":
			panic("handler bug")
		case "redirect":
			w.Header().Set("Location", "/elsewhere")
			w.WriteHeader(http.StatusFound)
			return
		case "created":
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/v1/items/"+in.Name)
		w.Header().Set("X-Internal", "not stored")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "name": in.Name})
	}
}

// post envía un POST JSON; key vacía no manda la cabecera.
func (e *env) post(path, tenant, key string, body any) *httptest.ResponseRecorder {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case string:
		rd = bytes.NewBufferString(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(http.MethodPost, path, rd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Tenant", tenant)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func (e *env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.admin.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *env) items(tenant string) int {
	return e.count(`SELECT count(*) FROM rlstest.items WHERE tenant_id = $1`, e.tenants[tenant].TenantID)
}

func (e *env) keys() int { return e.count(`SELECT count(*) FROM ops.idempotency_keys`) }

type problem struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
	Errors []struct {
		Field string `json:"field"`
		Code  string `json:"code"`
	} `json:"errors"`
}

func mustProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) problem {
	t.Helper()
	var p problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("not a problem body (status %d): %q", rec.Code, rec.Body.String())
	}
	if rec.Code != status || p.Code != code {
		t.Fatalf("got %d %q, want %d %q: %s", rec.Code, p.Code, status, code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	return p
}
