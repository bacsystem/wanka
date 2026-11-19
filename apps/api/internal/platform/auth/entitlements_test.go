package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// extProblem lee los miembros de extensión del problema.
type extProblem struct {
	Code   string `json:"code"`
	Module string `json:"module"`
	Limit  *int64 `json:"limit"`
	Used   *int64 `json:"used"`
}

func ext(t *testing.T, rec *httptest.ResponseRecorder) extProblem {
	t.Helper()
	var p extProblem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body: %q", rec.Body.String())
	}
	return p
}

func ofPlan(f func(*entitlements.Entitlements)) *fakeEnts {
	e := plan()
	f(&e)
	return grants(e)
}

func (e *env) post(path, authz string) *httptest.ResponseRecorder {
	return e.do(http.MethodPost, path, authz)
}

// Criterio 1: módulo no habilitado -> 403 module_not_enabled con {module}.
func TestModuleNotEnabledCarriesTheModule(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	rec := e.get("/v1/vet", bearer(token(1)))
	mustProblem(t, rec, 403, "module_not_enabled")
	if p := ext(t, rec); p.Module != "rubro.veterinaria" || p.Limit != nil || p.Used != nil {
		t.Fatalf("problem = %+v", p)
	}
	if e.hits("vet") != 0 {
		t.Fatal("the handler ran without its module")
	}
	if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("an enabled module = %d", rec.Code)
	}
}

// Criterio 2: suscripción morosa, suspendida o cancelada -> 402 a toda ruta del
// plano tenant, también GET; solo pasan las de la lista fija.
func TestInactiveSubscriptionBlocksEveryTenantRouteButTheFixedList(t *testing.T) {
	for _, st := range []entitlements.Status{entitlements.StatusPastDue, entitlements.StatusSuspended, entitlements.StatusCancelled} {
		t.Run(string(st), func(t *testing.T) {
			e := newEnvEnts(t, returns(bound()), ofPlan(func(e *entitlements.Entitlements) { e.Status, e.Modules = st, []string{} }), nil)
			for _, c := range []struct{ method, path string }{
				{"GET", "/v1/items"}, {"POST", "/v1/items"}, {"GET", "/v1/other"}, {"GET", "/v1/vet"}, {"POST", "/v1/users"}, {"GET", "/v1/paid"},
			} {
				mustProblem(t, e.do(c.method, c.path, bearer(token(1))), 402, "subscription_inactive")
			}
			if e.hits("tenant")+e.hits("tenant-write")+e.hits("tenant2")+e.hits("vet")+e.hits("users-create")+e.hits("paid") != 0 {
				t.Fatal("a handler ran with an inactive subscription")
			}
			for _, p := range []string{"/v1/exports", "/v1/me/entitlements"} {
				if rec := e.get(p, bearer(token(1))); rec.Code != 200 {
					t.Fatalf("GET %s = %d, want 200: the fixed list passes", p, rec.Code)
				}
			}
			// Las rutas sin tenant no pasan por entitlements.
			before := e.ents.calls()
			if rec := e.get("/v1/me", bearer(token(1))); rec.Code != 200 {
				t.Fatalf("GET /v1/me = %d", rec.Code)
			}
			if e.ents.calls() != before {
				t.Fatal("a NoTenant route read the entitlements")
			}
		})
	}
}

// El mismo método en una ruta de la lista no abre otras: solo el GET declarado.
func TestFixedListIsPerMethodAndPath(t *testing.T) {
	e := newEnvEnts(t, returns(bound()), ofPlan(func(e *entitlements.Entitlements) { e.Status, e.Modules = entitlements.StatusPastDue, []string{} }), nil)
	// No hay POST /v1/exports registrado: es 404 de ruta, no un pase.
	mustProblem(t, e.post("/v1/exports", bearer(token(1))), 404, "not_found")
	mustProblem(t, e.get("/v1/exports/extra", bearer(token(1))), 404, "not_found")
}

// Un tenant sin plan no es una suscripción morosa: no hay a quién cobrar, el
// módulo no está habilitado (403) y las rutas sin módulo siguen.
func TestTenantWithoutAPlanIs403NotA402(t *testing.T) {
	e := newEnvEnts(t, returns(bound()), ofPlan(func(e *entitlements.Entitlements) {
		e.Status, e.Modules, e.Limits = entitlements.StatusNoPlan, []string{}, map[string]int64{}
	}), nil)
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 403, "module_not_enabled")
	if rec := e.get("/v1/me/entitlements", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("GET /v1/me/entitlements = %d: the UI must be able to show why", rec.Code)
	}
}

// Criterio 3: used >= limit -> 403 plan_limit_reached con {limit, used}; used < limit pasa.
func TestPlanLimitAtTheEdge(t *testing.T) {
	for _, tc := range []struct {
		name  string
		used  int64
		limit int64
		ok    bool
	}{
		{"below", 4, 5, true},
		{"equal", 5, 5, false},
		{"above", 9, 5, false},
		{"zero limit, zero used", 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnvEnts(t, returns(bound()), ofPlan(func(e *entitlements.Entitlements) {
				e.Limits["users"], e.Usage["users"] = tc.limit, tc.used
			}), nil)
			rec := e.post("/v1/users", bearer(token(1)))
			if tc.ok {
				if rec.Code != 200 || e.hits("users-create") != 1 {
					t.Fatalf("status %d, hits %d", rec.Code, e.hits("users-create"))
				}
				return
			}
			mustProblem(t, rec, 403, "plan_limit_reached")
			p := ext(t, rec)
			if p.Limit == nil || p.Used == nil || *p.Limit != tc.limit || *p.Used != tc.used || p.Module != "" {
				t.Fatalf("problem = %+v, want limit %d used %d", p, tc.limit, tc.used)
			}
			if e.hits("users-create") != 0 {
				t.Fatal("the handler ran at the limit")
			}
		})
	}
	// Una clave de límite que el plan no trae cierra.
	e := newEnvEnts(t, returns(bound()), ofPlan(func(e *entitlements.Entitlements) { delete(e.Limits, "users") }), nil)
	mustProblem(t, e.post("/v1/users", bearer(token(1))), 403, "plan_limit_reached")
	// Una ruta que no consume el límite no se ve afectada por él.
	e = newEnvEnts(t, returns(bound()), ofPlan(func(e *entitlements.Entitlements) { e.Usage["users"] = 99 }), nil)
	if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("a route without a limit = %d", rec.Code)
	}
}

// Criterio 4: empresa cerrada -> todo método distinto de GET es 403 tenant_read_only; los GET y los exportes pasan.
func TestClosedCompanyOnlyReads(t *testing.T) {
	closed := func(e *entitlements.Entitlements) { e.ReadOnly = true }
	e := newEnvEnts(t, returns(bound()), ofPlan(closed), nil)
	for _, m := range []string{"POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		rec := e.do(m, "/v1/items", bearer(token(1)))
		if m == "OPTIONS" {
			mustProblem(t, rec, 404, "not_found") // sin CORS: nunca llega a la cadena
			continue
		}
		mustProblem(t, rec, 403, "tenant_read_only")
	}
	if e.hits("tenant-write") != 0 || e.hits("users-create") != 0 {
		t.Fatal("a write handler ran for a closed company")
	}
	for _, p := range []string{"/v1/items", "/v1/exports"} {
		if rec := e.do("HEAD", p, bearer(token(1))); rec.Code != 200 {
			t.Fatalf("HEAD %s = %d, want 200: HEAD is a read", p, rec.Code)
		}
	}
	for _, p := range []string{"/v1/items", "/v1/exports", "/v1/me/entitlements"} {
		if rec := e.get(p, bearer(token(1))); rec.Code != 200 {
			t.Fatalf("GET %s = %d", p, rec.Code)
		}
	}
	mustProblem(t, e.post("/v1/users", bearer(token(1))), 403, "tenant_read_only")
}

// A una empresa cerrada no se le evalúa la suscripción: su único bloqueo es tenant_read_only.
func TestClosedCompanyIsNotAskedForTheSubscription(t *testing.T) {
	e := newEnvEnts(t, returns(bound()), ofPlan(func(e *entitlements.Entitlements) {
		e.ReadOnly, e.Status = true, entitlements.StatusSuspended
	}), nil)
	mustProblem(t, e.post("/v1/items", bearer(token(1))), 403, "tenant_read_only")
	for _, p := range []string{"/v1/exports", "/v1/me/entitlements"} {
		if rec := e.get(p, bearer(token(1))); rec.Code != 200 {
			t.Fatalf("GET %s = %d, want 200", p, rec.Code)
		}
	}
	// Cerrada + suspendida con el módulo del último plan (la empresa cerrada lo conserva): el GET da 200, no 402.
	e = newEnvEnts(t, returns(bound()), ofPlan(func(e *entitlements.Entitlements) {
		e.ReadOnly, e.Status = true, entitlements.StatusSuspended
	}), nil)
	if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("GET /v1/items = %d, want 200", rec.Code)
	}
	mustProblem(t, e.post("/v1/items", bearer(token(1))), 403, "tenant_read_only")
	// Sin el módulo (override ausente) el GET sí es module_not_enabled.
	e = newEnvEnts(t, returns(bound()), ofPlan(func(e *entitlements.Entitlements) {
		e.ReadOnly, e.Status, e.Modules = true, entitlements.StatusSuspended, []string{}
	}), nil)
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 403, "module_not_enabled")
}

// Criterios 5 y 7: sin poder leer los entitlements, 503 dependency_unavailable
// con Retry-After 1, la petición no pasa, y el log y el contador llevan phase=entitlements.
func TestUnreadableEntitlementsAre503AndNeverPass(t *testing.T) {
	cases := map[string]struct {
		err     error
		code    string
		counter string
		logMsg  string
	}{
		"unavailable":       {fmt.Errorf("reading entitlements: %w: %w", db.ErrUnavailable, errors.New("conn refused")), "dependency_unavailable", "db_unavailable_total{phase,entitlements,route,GET /v1/items}", "database unavailable"},
		"statement timeout": {fmt.Errorf("reading entitlements: %w", statementTimeout()), "dependency_unavailable", "db_statement_timeouts_total{phase,entitlements,route,GET /v1/items}", "statement timeout"},
		"commit uncertain":  {fmt.Errorf("%w: %w", db.ErrUnavailable, db.ErrCommitUncertain), "dependency_unavailable", "db_unavailable_total{phase,commit,route,GET /v1/items}", "database unavailable"},
		"untyped 57014":     {&pgconn.PgError{Code: "57014", Message: "canceling statement due to user request"}, "dependency_unavailable", "db_unavailable_total{phase,entitlements,route,GET /v1/items}", "entitlements read timed out"},
		"deadline error":    {fmt.Errorf("query: %w", context.DeadlineExceeded), "dependency_unavailable", "db_unavailable_total{phase,entitlements,route,GET /v1/items}", "entitlements read timed out"},
		"overloaded":        {fmt.Errorf("acquiring: %w", db.ErrOverloaded), "overloaded", "", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnvEnts(t, returns(bound()), entFails(tc.err), nil)
			rec := e.get("/v1/items", bearer(token(1)))
			mustProblem(t, rec, 503, tc.code)
			if rec.Header().Get("Retry-After") != "1" {
				t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
			}
			if strings.Contains(rec.Body.String(), "57014") || strings.Contains(rec.Body.String(), "secret_column") {
				t.Fatalf("PostgreSQL text leaked: %s", rec.Body.String())
			}
			if e.hits("tenant") != 0 {
				t.Fatal("the handler ran without entitlements")
			}
			if tc.counter != "" && e.metrics.get(tc.counter) != 1 {
				t.Fatalf("counter %s = %d; %v", tc.counter, e.metrics.get(tc.counter), e.metrics.counts)
			}
			if tc.logMsg != "" {
				lines := e.log.lines(t, tc.logMsg)
				if len(lines) != 1 || lines[0]["level"] != "WARN" {
					t.Fatalf("log %q = %v", tc.logMsg, lines)
				}
				want := httpx.PhaseEntitlements
				if name == "commit uncertain" {
					want = httpx.PhaseCommit
				}
				if lines[0]["phase"] != want || lines[0]["module"] != "ventas" {
					t.Fatalf("warn line = %v, want phase %s and module ventas", lines[0], want)
				}
			}
		})
	}
}

// Un error que no es de base es un bug: 500 sin detalle, y la petición no pasa.
func TestUnexpectedEntitlementsErrorIs500(t *testing.T) {
	e := newEnvEnts(t, returns(bound()), entFails(errors.New("password=hunter2 something odd")), nil)
	rec := e.get("/v1/items", bearer(token(1)))
	mustProblem(t, rec, 500, "internal")
	if strings.Contains(rec.Body.String(), "hunter2") || strings.Contains(e.log.String(), "hunter2") {
		t.Fatal("the error text leaked")
	}
	if e.hits("tenant") != 0 {
		t.Fatal("the handler ran")
	}
}

// Una sesión cuya empresa ya no existe no entra.
func TestVanishedTenantIs403(t *testing.T) {
	e := newEnvEnts(t, returns(bound()), entFails(fmt.Errorf("x: %w", entitlements.ErrTenantNotFound)), nil)
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 403, "tenant_access_denied")
	if e.hits("tenant") != 0 {
		t.Fatal("the handler ran")
	}
}

// La fase entitlements vive solo en el contexto de la lectura: un error del
// handler de la misma petición lleva phase=handler.
func TestHandlerErrorAfterEntitlementsCarriesPhaseHandler(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	mustProblem(t, e.get("/v1/slow", bearer(token(1))), 503, "dependency_unavailable")
	lines := e.log.lines(t, "statement timeout")
	if len(lines) != 1 || lines[0]["phase"] != httpx.PhaseHandler {
		t.Fatalf("log = %v, want phase=handler", lines)
	}
	if e.metrics.get("db_statement_timeouts_total{phase,handler,route,GET /v1/slow}") != 1 {
		t.Fatalf("counters = %v", e.metrics.counts)
	}
	if lines[0]["module"] != "ventas" {
		t.Fatalf("the handler log lost the module: %v", lines[0])
	}
}

// Criterio 6: orden. Gana la primera que falla: suscripción -> módulo -> límite -> permiso (paso de ruta).
func TestFirstFailingRuleWins(t *testing.T) {
	rules := func(mods []string, st entitlements.Status, usersUsed int64) *fakeEnts {
		return ofPlan(func(e *entitlements.Entitlements) { e.Modules, e.Status, e.Usage["users"] = mods, st, usersUsed })
	}
	// /v1/users: módulo ventas + límite users (5). /v1/paid: módulo ventas + un paso de ruta que responde 403 forbidden.
	cases := []struct {
		name   string
		ents   *fakeEnts
		method string
		path   string
		code   string
	}{
		{"subscription, module and limit all fail", rules([]string{}, entitlements.StatusPastDue, 5), "POST", "/v1/users", "subscription_inactive"},
		{"module and limit fail", rules([]string{}, entitlements.StatusActive, 5), "POST", "/v1/users", "module_not_enabled"},
		{"limit and permission fail", rules([]string{"ventas"}, entitlements.StatusActive, 5), "POST", "/v1/users", "plan_limit_reached"},
		{"module and permission fail", rules([]string{}, entitlements.StatusActive, 0), "GET", "/v1/paid", "module_not_enabled"},
		{"subscription and permission fail", rules([]string{"ventas"}, entitlements.StatusSuspended, 0), "GET", "/v1/paid", "subscription_inactive"},
		{"only the permission fails", rules([]string{"ventas"}, entitlements.StatusActive, 0), "GET", "/v1/paid", "forbidden"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnvEnts(t, returns(bound()), c.ents, nil)
			rec := e.do(c.method, c.path, bearer(token(1)))
			if rec.Code == 200 {
				t.Fatal("passed")
			}
			if p := ext(t, rec); p.Code != c.code {
				t.Fatalf("code = %s, want %s", p.Code, c.code)
			}
			if c.code != "forbidden" && e.hits("entitlements") != 0 {
				t.Fatal("the route step (RBAC) ran after a failed entitlement rule")
			}
		})
	}
}

// Criterio 8: el paso va después de los limitadores y antes de los de la ruta; las
// rutas NoTenant y Public no pasan por él.
func TestStepOrderInTheChain(t *testing.T) {
	t.Run("a rate-limited tenant request never reads entitlements", func(t *testing.T) {
		e := newEnv(t, returns(bound()), func(o *auth.Options) {
			o.TenantLimiter = httpx.LimiterConfig{Capacity: 1, Refill: time.Hour, MaxKeys: 10}
		})
		if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 {
			t.Fatalf("first = %d", rec.Code)
		}
		calls := e.ents.calls()
		mustProblem(t, e.get("/v1/items", bearer(token(1))), 429, "rate_limited")
		if e.ents.calls() != calls {
			t.Fatal("entitlements were read for a request the limiter rejected")
		}
	})
	t.Run("a session without a tenant never reads entitlements", func(t *testing.T) {
		e := newEnv(t, returns(noCompany()), nil)
		mustProblem(t, e.get("/v1/items", bearer(token(1))), 403, "tenant_access_denied")
		if e.ents.calls() != 0 {
			t.Fatal("entitlements were read without a tenant")
		}
	})
	t.Run("an unauthenticated request never reads entitlements", func(t *testing.T) {
		e := newEnv(t, noRow(), nil)
		wantUnauth(t, e.get("/v1/items", ""), "unauthenticated")
		mustProblem(t, e.get("/v1/items", bearer(token(1))), 401, "session_expired")
		if e.ents.calls() != 0 {
			t.Fatal("entitlements were read before the session")
		}
	})
	t.Run("NoTenant and Public routes do not go through entitlements", func(t *testing.T) {
		e := newEnvEnts(t, returns(bound()), entFails(db.ErrUnavailable), nil)
		for _, c := range []struct{ method, path string }{{"GET", "/v1/me"}, {"POST", "/v1/auth/mfa/totp/enroll"}, {"POST", "/v1/auth/login"}} {
			if rec := e.do(c.method, c.path, bearer(token(1))); rec.Code != 200 {
				t.Fatalf("%s %s = %d with the entitlements down", c.method, c.path, rec.Code)
			}
		}
		if e.ents.calls() != 0 {
			t.Fatal("a route without a tenant read the entitlements")
		}
	})
	t.Run("the entitlements are read for the session tenant", func(t *testing.T) {
		e := newEnv(t, returns(bound()), nil)
		e.get("/v1/items", bearer(token(1)))
		if len(e.ents.tenants) != 1 || e.ents.tenants[0] != uid(3) {
			t.Fatalf("tenants = %v", e.ents.tenants)
		}
	})
	t.Run("an impersonated session goes through the same rules", func(t *testing.T) {
		e := newEnvEnts(t, returns(impersonated()), ofPlan(func(e *entitlements.Entitlements) { e.ReadOnly = true }), nil)
		mustProblem(t, e.post("/v1/items", bearer(token(1))), 403, "tenant_read_only")
	})
}

// Criterio 11: dos tenants con planes distintos reciben cada uno su resultado.
func TestTwoTenantsWithDifferentPlans(t *testing.T) {
	byToken := map[[32]byte]auth.SessionRecord{}
	a, b := bound(), bound()
	b.TenantID, b.SessionID = uid(30), uid(31)
	byToken[hashOf(token(1))], byToken[hashOf(token(2))] = a, b
	res := &fakeResolver{lookup: func(_ context.Context, h [32]byte) (auth.SessionRecord, bool, error) {
		r, ok := byToken[h]
		return r, ok, nil
	}}
	ents := &fakeEnts{fn: func(_ context.Context, tenant pgtype.UUID) (entitlements.Entitlements, error) {
		e := plan()
		if tenant == uid(30) {
			e.Modules = []string{"inventario"} // sin ventas
		}
		return e, nil
	}}
	e := newEnvEnts(t, res, ents, nil)
	if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("tenant A = %d", rec.Code)
	}
	mustProblem(t, e.get("/v1/items", bearer(token(2))), 403, "module_not_enabled")
	if rec := e.get("/v1/other", bearer(token(2))); rec.Code != 200 {
		t.Fatalf("tenant B on its own module = %d", rec.Code)
	}
}

// Invariante: una ruta sin entitlement resuelto nunca se ejecuta, bajo carga y
// con la lectura fallando a ratos (corre con -race).
func TestHandlerNeverRunsWithoutResolvedEntitlements(t *testing.T) {
	var mu sync.Mutex
	n := 0
	ents := &fakeEnts{fn: func(context.Context, pgtype.UUID) (entitlements.Entitlements, error) {
		mu.Lock()
		defer mu.Unlock()
		n++
		if n%2 == 0 {
			return entitlements.Entitlements{}, db.ErrUnavailable
		}
		return plan(), nil
	}}
	e := newEnvEnts(t, returns(bound()), ents, func(o *auth.Options) {
		huge := httpx.LimiterConfig{Capacity: 1 << 30, Refill: time.Nanosecond, MaxKeys: 10}
		o.SessionLimiter, o.TenantLimiter = huge, huge
	})
	var wg sync.WaitGroup
	var ok, unavailable int
	var cm sync.Mutex
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := e.get("/v1/items", bearer(token(1)))
			cm.Lock()
			defer cm.Unlock()
			switch rec.Code {
			case 200:
				ok++
			case 503:
				unavailable++
			default:
				t.Errorf("status %d", rec.Code)
			}
		}()
	}
	wg.Wait()
	if ok != e.hits("tenant") || ok+unavailable != 64 || ok == 0 || unavailable == 0 {
		t.Fatalf("ok %d, 503 %d, handler ran %d times", ok, unavailable, e.hits("tenant"))
	}
}

// Criterio 9: el arranque falla si una ruta no declara módulo o permiso, o si
// el spec no cumple el ADR 0009.
func TestRouteSpecIsValidatedAtRegistration(t *testing.T) {
	none := auth.Spec{Module: auth.NoModule, Permission: auth.NoPermission}
	cases := []struct {
		name    string
		pattern string
		class   auth.RouteClass
		spec    auth.Spec
		ok      bool
	}{
		{"tenant route with a module", "GET /v1/items", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:read"}, true},
		{"tenant route with no spec at all", "GET /v1/items", auth.Tenant, auth.Spec{}, false},
		{"empty module", "GET /v1/items", auth.Tenant, auth.Spec{Module: "", Permission: "ventas.pos:read"}, false},
		{"empty permission", "GET /v1/items", auth.Tenant, auth.Spec{Module: "ventas"}, false},
		{"NoPermission on a tenant route is allowed", "GET /v1/items", auth.Tenant, auth.Spec{Module: "ventas", Permission: auth.NoPermission}, true},
		{"NoModule on an ordinary tenant route", "GET /v1/items", auth.Tenant, none, false},
		{"NoModule on GET /v1/exports", "GET /v1/exports", auth.Tenant, none, true},
		{"NoModule on GET /v1/me/entitlements", "GET /v1/me/entitlements", auth.Tenant, none, true},
		{"a module on GET /v1/exports", "GET /v1/exports", auth.Tenant, auth.Spec{Module: "reportes", Permission: auth.NoPermission}, false},
		{"NoModule on POST /v1/exports", "POST /v1/exports", auth.Tenant, none, false},
		{"NoModule on NoTenant", "GET /v1/me", auth.NoTenant, none, true},
		{"a module on NoTenant", "GET /v1/me", auth.NoTenant, auth.Spec{Module: "ventas", Permission: auth.NoPermission}, false},
		{"empty module on NoTenant", "GET /v1/me", auth.NoTenant, auth.Spec{Permission: auth.NoPermission}, false},
		{"NoModule on Public", "POST /v1/auth/login", auth.Public, none, true},
		{"empty spec on Public", "POST /v1/auth/login", auth.Public, auth.Spec{}, false},
		{"a module on Public", "POST /v1/auth/login", auth.Public, auth.Spec{Module: "ventas", Permission: auth.NoPermission}, false},
		{"sites has no usage measurement yet", "POST /v1/sites", auth.Tenant, auth.Spec{Module: "empresas", Permission: "x:y", Limit: entitlements.LimitSites}, false},
		{"a known limit", "POST /v1/users", auth.Tenant, auth.Spec{Module: "identidad", Permission: "x:y", Limit: entitlements.LimitUsers}, true},
		{"an unknown limit", "POST /v1/users", auth.Tenant, auth.Spec{Module: "identidad", Permission: "x:y", Limit: "seats"}, false},
		{"a limit on a NoTenant route", "GET /v1/me", auth.NoTenant, auth.Spec{Module: auth.NoModule, Permission: auth.NoPermission, Limit: entitlements.LimitUsers}, false},
		{"the literal is none, in lowercase", "GET /v1/items", auth.Tenant, auth.Spec{Module: "NONE", Permission: "x:y"}, true}, // "NONE" es un código de módulo cualquiera, no el reservado
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			routes, err := auth.NewRoutes(httpx.NewRouter(httpx.RouterOptions{}), returns(bound()), grants(plan()), auth.Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if r := recover(); (r == nil) != c.ok {
					t.Fatalf("panic = %v, want panic: %v", r, !c.ok)
				}
			}()
			routes.HandleFunc(c.pattern, c.class, c.spec, func(http.ResponseWriter, *http.Request) {})
		})
	}
	if auth.NoModule != "none" || auth.NoPermission != "none" {
		t.Fatalf("NoModule = %q, NoPermission = %q, want the literal none", auth.NoModule, auth.NoPermission)
	}
}

// Una ruta NoModule no marca el módulo en el log.
func TestNoModuleRouteLogsNoModule(t *testing.T) {
	e := newEnvEnts(t, returns(bound()), entFails(db.ErrUnavailable), nil)
	mustProblem(t, e.get("/v1/exports", bearer(token(1))), 503, "dependency_unavailable")
	lines := e.log.lines(t, "database unavailable")
	if len(lines) != 1 {
		t.Fatalf("lines = %v", lines)
	}
	if _, has := lines[0]["module"]; has {
		t.Fatalf("a NoModule route logged a module: %v", lines[0])
	}
}
