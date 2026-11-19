package httpx_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// pgTimeout es lo que devuelve InTenantTx: ErrStatementTimeout envuelve el error
// de PostgreSQL, cuyo mensaje (traducible) nunca debe llegar al cliente.
func pgTimeout() error {
	pg := &pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout", Hint: "SELECT pg_sleep(1) FROM secret_table"}
	return fmt.Errorf("listing items: %w", fmt.Errorf("%w: %w", db.ErrStatementTimeout, pg))
}

// serveDBError responde err con WriteDBError dentro de Chain, bajo la ruta GET /items/{id}.
func serveDBError(t *testing.T, mutate func(*http.Request) *http.Request, err error) (*httptest.ResponseRecorder, bool, *logBuf, *fakeMetrics) {
	t.Helper()
	var wrote bool
	h, buf, m := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
			if mutate != nil {
				r = mutate(r)
			}
			wrote = httpx.WriteDBError(w, r, err)
		})
	})
	return do(h, httptest.NewRequest(http.MethodGet, "/items/42?token=SECRET-Q", nil)), wrote, buf, m
}

func TestWriteDBError(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		wrote bool
		code  string
	}{
		{"pool exhausted", db.ErrOverloaded, true, "overloaded"},
		{"wrapped pool exhausted", fmt.Errorf("acquiring: %w", db.ErrOverloaded), true, "overloaded"},
		{"database unavailable", fmt.Errorf("%w: connection refused", db.ErrUnavailable), true, "dependency_unavailable"},
		{"statement timeout", pgTimeout(), true, "dependency_unavailable"},
		{"other error", errors.New("boom"), false, ""},
		{"domain error", httpx.NewError("conflict", ""), false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, wrote, _, _ := serveDBError(t, nil, tc.err)
			if wrote != tc.wrote {
				t.Fatalf("wrote = %v, want %v", wrote, tc.wrote)
			}
			if !tc.wrote {
				if rec.Body.Len() != 0 {
					t.Fatalf("nothing expected, got %s", rec.Body.String())
				}
				return
			}
			mustProblem(t, rec, http.StatusServiceUnavailable, tc.code)
			if rec.Header().Get("Retry-After") != "1" {
				t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
			}
			if strings.Contains(rec.Body.String(), "connection refused") {
				t.Fatalf("detail leaks the cause: %s", rec.Body.String())
			}
		})
	}
}

func TestStatementTimeoutHasTheSameBodyAsUnavailableAndLeaksNothing(t *testing.T) {
	timeoutRec, _, _, _ := serveDBError(t, nil, pgTimeout())
	downRec, _, _, _ := serveDBError(t, nil, db.ErrUnavailable)
	a := mustProblem(t, timeoutRec, http.StatusServiceUnavailable, "dependency_unavailable")
	b := mustProblem(t, downRec, http.StatusServiceUnavailable, "dependency_unavailable")
	if a.Title != b.Title || a.Detail != b.Detail || a.Type != b.Type {
		t.Fatalf("statement timeout %+v differs from unavailable %+v", a, b)
	}
	body := strings.ToLower(timeoutRec.Body.String())
	for _, banned := range []string{"57014", "statement", "timeout", "canceling", "pg_sleep", "secret_table"} {
		if strings.Contains(body, banned) {
			t.Fatalf("body contains %q: %s", banned, body)
		}
	}
}

func TestStatementTimeoutLogsOneWarnAndCountsByPhaseAndRoute(t *testing.T) {
	_, _, buf, m := serveDBError(t, nil, pgTimeout())
	warns := buf.lines(t, "statement timeout")
	if len(warns) != 1 {
		t.Fatalf("warn lines = %d: %s", len(warns), buf.String())
	}
	w := warns[0]
	if w["level"] != "WARN" || w["route"] != "GET /items/{id}" || w["phase"] != "handler" || w["sqlstate"] != "57014" ||
		w["request_id"] == nil || w["elapsed_ms"] == nil {
		t.Fatalf("warn = %v", w)
	}
	if _, has := w["module"]; has {
		t.Fatalf("module must be absent when not set: %v", w)
	}
	if m.get("db_statement_timeouts_total{phase,handler,route,GET /items/{id}}") != 1 {
		t.Fatalf("counter = %v", m.counts)
	}
	// Nada del SQL, de sus parámetros, del mensaje de PostgreSQL ni de la query de la URL.
	for _, banned := range []string{"pg_sleep", "secret_table", "canceling", "SECRET-Q", "/items/42"} {
		for _, ln := range strings.Split(buf.String(), "\n") {
			if strings.Contains(ln, "statement timeout") && strings.Contains(ln, banned) {
				t.Fatalf("warn line contains %q: %s", banned, ln)
			}
		}
	}
}

func TestStatementTimeoutUsesPhaseAndModuleFromTheContext(t *testing.T) {
	_, _, buf, m := serveDBError(t, func(r *http.Request) *http.Request {
		ctx := httpx.WithModule(httpx.WithPhase(r.Context(), httpx.PhaseSessionLookup), "iam")
		return r.WithContext(ctx)
	}, pgTimeout())
	w := buf.lines(t, "statement timeout")[0]
	if w["phase"] != "session_lookup" || w["module"] != "iam" {
		t.Fatalf("warn = %v", w)
	}
	if m.get("db_statement_timeouts_total{phase,session_lookup,route,GET /items/{id}}") != 1 {
		t.Fatalf("counter = %v", m.counts)
	}
}

func TestOtherDBErrorsDoNotCountAsStatementTimeouts(t *testing.T) {
	for _, err := range []error{db.ErrOverloaded, db.ErrUnavailable} {
		_, _, buf, m := serveDBError(t, nil, err)
		if len(buf.lines(t, "statement timeout")) != 0 || m.get("db_statement_timeouts_total{phase,handler,route,GET /items/{id}}") != 0 {
			t.Fatalf("%v counted as statement timeout: %s %v", err, buf.String(), m.counts)
		}
	}
}

// Con el contexto de la petición terminado no se escribe nada ni se registra un 500.
func TestWriteDBErrorWithFinishedContextWritesNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
	}{
		{"client left", func() (context.Context, context.CancelFunc) {
			c, cancel := context.WithCancel(context.Background())
			cancel()
			return c, func() {}
		}},
		{"deadline", func() (context.Context, context.CancelFunc) {
			return context.WithDeadline(context.Background(), timeNow().Add(-1))
		}},
	} {
		for _, err := range []error{
			db.ErrOverloaded, db.ErrUnavailable, pgTimeout(),
			fmt.Errorf("beginning transaction: %w", context.Canceled),
			fmt.Errorf("beginning transaction: %w", context.DeadlineExceeded),
		} {
			t.Run(tc.name+"/"+err.Error(), func(t *testing.T) {
				ctx, cancel := tc.ctx()
				defer cancel()
				rec, wrote, buf, m := serveDBError(t, func(r *http.Request) *http.Request { return r.WithContext(mergeValues(r.Context(), ctx)) }, err)
				if !wrote || rec.Body.Len() != 0 || rec.Header().Get("Content-Type") != "" {
					t.Fatalf("wrote=%v, response %d %v %q", wrote, rec.Code, rec.Header(), rec.Body.String())
				}
				if strings.Contains(buf.String(), `"level":"ERROR"`) || len(buf.lines(t, "statement timeout")) != 0 || len(m.counts) != 0 {
					t.Fatalf("nothing must be logged or counted: %s %v", buf.String(), m.counts)
				}
			})
		}
	}
	// Un error que no es de base sigue siendo asunto de quien llama.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, wrote, _, _ := serveDBError(t, func(r *http.Request) *http.Request { return r.WithContext(mergeValues(r.Context(), ctx)) }, errors.New("boom"))
	if wrote {
		t.Fatal("an unrelated error must not be swallowed")
	}
}

// Si vence el plazo del handler y este devuelve el error de base, el cliente
// recibe solo el 503 overloaded del middleware.
func TestHandlerDeadlineWinsOverTheDBError(t *testing.T) {
	h, buf, _ := chainWith(t, httpx.RouterOptions{HandlerTimeout: testDeadline}, func(rt *httpx.Router) {
		rt.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
			httpx.WriteError(w, r, fmt.Errorf("query: %w", pgTimeoutAfterCancel()))
		})
	})
	rec := do(h, httptest.NewRequest(http.MethodGet, "/slow", nil))
	mustProblem(t, rec, http.StatusServiceUnavailable, "overloaded")
	time.Sleep(20 * time.Millisecond) // deja terminar al handler
	if strings.Contains(buf.String(), `"level":"ERROR"`) || len(buf.lines(t, "statement timeout")) != 0 {
		t.Fatalf("a deadline must not be logged as a 500 or a statement timeout: %s", buf.String())
	}
}

// Con el ctx terminado, db.typedErr no tipa 57014 como ErrStatementTimeout: llega el error de contexto.
func pgTimeoutAfterCancel() error { return fmt.Errorf("canceled: %w", context.DeadlineExceeded) }

// Una base caída o una conexión perdida deja una línea warn por respuesta, sin el
// texto de PostgreSQL, y el COMMIT incierto se distingue por su fase.
func TestDatabaseUnavailableLogsOneWarnPerResponse(t *testing.T) {
	pg := &pgconn.PgError{Code: "57P01", Message: "terminating connection due to administrator command"}
	cases := []struct {
		name     string
		err      error
		phase    string
		sqlstate any
	}{
		{"unavailable", fmt.Errorf("%w: connection refused", db.ErrUnavailable), "handler", nil},
		{"connection lost", fmt.Errorf("x: %w", fmt.Errorf("%w: %w", db.ErrUnavailable, pg)), "handler", "57P01"},
		{"commit uncertain", fmt.Errorf("%w: %w", db.ErrCommitUncertain, fmt.Errorf("%w: %w", db.ErrUnavailable, pg)), "commit", "57P01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, wrote, buf, m := serveDBError(t, nil, tc.err)
			if !wrote {
				t.Fatal("not written")
			}
			mustProblem(t, rec, http.StatusServiceUnavailable, "dependency_unavailable")
			warns := buf.lines(t, "database unavailable")
			if len(warns) != 1 {
				t.Fatalf("warn lines = %d: %s", len(warns), buf.String())
			}
			w := warns[0]
			if w["level"] != "WARN" || w["route"] != "GET /items/{id}" || w["phase"] != tc.phase || w["sqlstate"] != tc.sqlstate || w["request_id"] == nil {
				t.Fatalf("warn = %v", w)
			}
			warnLine, _ := json.Marshal(w)
			for _, banned := range []string{"terminating", "connection refused", "administrator", "SECRET-Q", "/items/42"} {
				if strings.Contains(string(warnLine), banned) {
					t.Fatalf("warn contains %q: %s", banned, warnLine)
				}
			}
			if m.get("db_unavailable_total{phase,"+tc.phase+",route,GET /items/{id}}") != 1 {
				t.Fatalf("counter = %v", m.counts)
			}
		})
	}
	// El pool lleno no es una base caída: sin warn.
	_, _, buf, m := serveDBError(t, nil, db.ErrOverloaded)
	if len(buf.lines(t, "database unavailable")) != 0 || len(m.counts) != 0 {
		t.Fatalf("ErrOverloaded must not log as unavailable: %s", buf.String())
	}
}
