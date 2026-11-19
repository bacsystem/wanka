package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

type permProblem struct {
	Code       string `json:"code"`
	Permission string `json:"permission"`
}

func permOf(t *testing.T, rec *httptest.ResponseRecorder) permProblem {
	t.Helper()
	var p permProblem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body: %q", rec.Body.String())
	}
	return p
}

// Criterio 1: sin el permiso de la ruta -> 403 forbidden con {permission}, el
// handler no corre y el intento se audita.
func TestMissingPermissionIs403AndIsAudited(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	e.perms.set(grantOnly(permission.InventarioStockRead))
	rec := e.get("/v1/items", bearer(token(1)))
	mustProblem(t, rec, 403, "forbidden")
	if p := permOf(t, rec); p.Permission != "ventas.pos:read" {
		t.Fatalf("permission = %q", p.Permission)
	}
	if e.hits("tenant") != 0 {
		t.Fatal("the handler ran without its permission")
	}
	evs := e.denials.recorded()
	if len(evs) != 1 || evs[0].Action != "ventas.pos:read" {
		t.Fatalf("audited = %+v", evs)
	}
	diff, _ := evs[0].Diff.(map[string]string)
	if diff["route"] != "GET /v1/items" || diff["reason"] != "missing_permission" {
		t.Fatalf("diff = %v", evs[0].Diff)
	}
	if a := e.denials.actors[0]; a.TenantID != uid(3) || a.UserID != uid(2) || a.Kind != db.KindUser {
		t.Fatalf("actor = %+v", a)
	}
	if e.metrics.get("authz_decision_total{outcome,forbidden}") != 1 {
		t.Fatal("no forbidden decision counted")
	}
	// Con el permiso pasa, y no se audita nada más.
	e.perms.set(grantOnly(permission.VentasPosRead))
	e.clk.Advance(2 * time.Minute) // la caché del rol anterior ya venció
	if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("with the permission = %d", rec.Code)
	}
	if len(e.denials.recorded()) != 1 {
		t.Fatal("an allowed request was audited as denied")
	}
}

// Un resolver que devuelve nil es el conjunto vacío: todo se niega.
func TestNilPermissionSetDeniesEverything(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	e.perms.set(func(_ context.Context, s auth.Session) (auth.Authz, error) {
		return auth.Authz{Version: s.CurrentAuthzVersion.Int64}, nil
	})
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 403, "forbidden")
}

// El resolver que se cablea hasta #29 niega toda ruta con permiso, también la
// que la caché de otra versión permitiría, y deja pasar la de NoPermission.
func TestDenyAllPermissionsDeniesEveryRouteWithAPermission(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	e.perms.set(auth.DenyAllPermissions{}.Resolve)
	for _, p := range []string{"/v1/items", "/v1/other", "/v1/exports"} {
		mustProblem(t, e.get(p, bearer(token(1))), 403, "forbidden")
	}
	if rec := e.get("/v1/me/entitlements", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("a NoPermission route = %d", rec.Code)
	}
	if got := e.res.bound(); len(got) != 0 {
		t.Fatalf("DenyAll rebound the session version: %+v", got)
	}
}

// Criterio de orden: el paso de permisos va después de entitlements y antes de
// los pasos de la ruta y de la idempotencia; lo impone Routes.Handle.
func TestAuthzRunsAfterEntitlementsAndBeforeRouteStepsAndIdempotency(t *testing.T) {
	var order []string
	var step, idem atomic.Int32
	mark := func(name string) httpx.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	e := newEnv(t, returns(bound()), nil)
	// Innermost declarado primero: aun así corre después de Use.
	e.routes.HandleFunc("POST /v1/ordered", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:create"},
		func(w http.ResponseWriter, _ *http.Request) { order = append(order, "handler") },
		httpx.Innermost(mark("idempotency"), func(n http.Handler) http.Handler { idem.Add(1); return n }),
		httpx.Use(mark("route-step"), func(n http.Handler) http.Handler { step.Add(1); return n }))
	e.perms.set(func(ctx context.Context, s auth.Session) (auth.Authz, error) {
		order = append(order, "authz")
		return auth.Authz{Version: s.CurrentAuthzVersion.Int64, Permissions: permSet(permission.VentasPosCreate)}, nil
	})
	if rec := e.do("POST", "/v1/ordered", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := strings.Join(order, ","); got != "authz,route-step,idempotency,handler" {
		t.Fatalf("order = %s", got)
	}

	// Denegada: ni el paso de la ruta ni la idempotencia ni el handler corren, y con
	// la suscripción inactiva decide antes entitlements (402), no el permiso.
	order = nil
	e.perms.set(grantOnly())
	e.clk.Advance(2 * time.Minute)
	rec := e.do("POST", "/v1/ordered", bearer(token(1)))
	mustProblem(t, rec, 403, "forbidden")
	if len(order) != 0 {
		t.Fatalf("something ran after a denied permission: %v", order)
	}
	e2 := newEnvEnts(t, returns(bound()), ofPlan(func(p *entitlements.Entitlements) { p.Modules = []string{} }), nil)
	e2.perms.set(grantOnly())
	mustProblem(t, e2.get("/v1/items", bearer(token(1))), 403, "module_not_enabled")
	if e2.perms.resolves() != 0 || len(e2.denials.recorded()) != 0 {
		t.Fatal("authz ran before entitlements")
	}
}

// Criterio 2 e invariante: la caché es por (membresía, versión) con TTL 60 s; un
// cambio de versión surte efecto en la siguiente petición.
func TestPermissionsAreCachedPerMembershipAndVersion(t *testing.T) {
	rec := bound()
	var cur atomic.Int64
	cur.Store(1)
	res := &fakeResolver{lookup: func(context.Context, [32]byte) (auth.SessionRecord, bool, error) {
		r := rec
		r.CurrentAuthzVersion = i8(cur.Load())
		return r, true, nil
	}}
	e := newEnv(t, res, nil)
	grant := func(codes ...permission.Code) func(context.Context, auth.Session) (auth.Authz, error) {
		return func(_ context.Context, s auth.Session) (auth.Authz, error) {
			return auth.Authz{Version: s.CurrentAuthzVersion.Int64, Permissions: permSet(codes...)}, nil
		}
	}
	e.perms.set(grant(permission.VentasPosRead))
	for i := 0; i < 3; i++ {
		if rc := e.get("/v1/items", bearer(token(1))); rc.Code != 200 {
			t.Fatalf("request %d = %d", i, rc.Code)
		}
	}
	if n := e.perms.resolves(); n != 1 {
		t.Fatalf("resolved %d times, want 1 (cache)", n)
	}
	if e.metrics.get("authz_cache_total{result,hit}") != 2 || e.metrics.get("authz_cache_total{result,miss}") != 1 {
		t.Fatalf("cache metrics: %v", e.metrics.counts)
	}

	// El rol cambia y la versión sube: la caché caliente de la versión vieja no se usa.
	e.perms.set(grant())
	cur.Store(2)
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 403, "forbidden")
	if n := e.perms.resolves(); n != 2 {
		t.Fatalf("resolved %d times after the version change, want 2", n)
	}
	// La sesión guardaba la 1: se ligó la 2.
	if b := res.bound(); len(b) != 1 || b[0].version != 2 || b[0].sid != uid(1) || b[0].mid != uid(4) {
		t.Fatalf("binds = %+v", b)
	}

	// TTL de 60 s: a los 59 se usa la caché, a los 60 se resuelve de nuevo.
	e.clk.Advance(59 * time.Second)
	e.get("/v1/items", bearer(token(1)))
	if n := e.perms.resolves(); n != 2 {
		t.Fatalf("resolved %d times at 59 s, want 2", n)
	}
	e.clk.Advance(time.Second)
	e.get("/v1/items", bearer(token(1)))
	if n := e.perms.resolves(); n != 3 {
		t.Fatalf("resolved %d times at 60 s, want 3", n)
	}
}

// La clave es la membresía, no el tenant ni el usuario: dos membresías con la
// misma versión no comparten permisos.
func TestCacheIsNeverSharedBetweenMemberships(t *testing.T) {
	a, b := bound(), bound()
	b.MembershipID, b.SessionID = uid(40), uid(10)
	res := &fakeResolver{lookup: func(_ context.Context, h [32]byte) (auth.SessionRecord, bool, error) {
		if h == hashOf(token(1)) {
			return a, true, nil
		}
		return b, true, nil
	}}
	e := newEnv(t, res, nil)
	e.perms.set(func(_ context.Context, s auth.Session) (auth.Authz, error) {
		if s.MembershipID == uid(4) {
			return auth.Authz{Version: 1, Permissions: permSet(permission.VentasPosRead)}, nil
		}
		return auth.Authz{Version: 1}, nil
	})
	if rc := e.get("/v1/items", bearer(token(1))); rc.Code != 200 {
		t.Fatalf("membership A = %d", rc.Code)
	}
	mustProblem(t, e.get("/v1/items", bearer(token(2))), 403, "forbidden")
}

// Con la caché llena no se cachea (se resuelve en cada petición) y se cuenta.
func TestFullCacheDoesNotCacheAndCountsIt(t *testing.T) {
	var n atomic.Int64
	res := &fakeResolver{lookup: func(context.Context, [32]byte) (auth.SessionRecord, bool, error) {
		r := bound()
		id := n.Add(1)
		var m [16]byte
		m[8], m[9], m[10], m[11] = byte(id>>24), byte(id>>16), byte(id>>8), byte(id)
		r.MembershipID = pgtype.UUID{Bytes: m, Valid: true}
		return r, true, nil
	}}
	huge := httpx.LimiterConfig{Capacity: 1 << 30, Refill: time.Nanosecond, MaxKeys: 10}
	e := newEnv(t, res, func(o *auth.Options) { o.SessionLimiter, o.TenantLimiter = huge, huge })
	e.perms.set(grantOnly(permission.VentasPosRead))
	const max = 50_000
	for i := 0; i < max; i++ {
		if rc := e.get("/v1/items", bearer(token(1))); rc.Code != 200 {
			t.Fatalf("request %d = %d", i, rc.Code)
		}
	}
	if e.metrics.get("authz_cache_full_total{}") != 0 {
		t.Fatal("the cache reported full too early")
	}
	for i := 0; i < 3; i++ {
		if rc := e.get("/v1/items", bearer(token(1))); rc.Code != 200 {
			t.Fatalf("with a full cache = %d", rc.Code)
		}
	}
	if got := e.metrics.get("authz_cache_full_total{}"); got != 3 {
		t.Fatalf("authz_cache_full_total = %d, want 3", got)
	}
}

// Criterio 3: la versión de la sesión se actualiza antes de decidir.
func TestSessionVersionIsBoundWhenItDiffers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		session pgtype.Int8
		current int64
		binds   int
	}{
		{"equal", i8(3), 3, 0},
		{"older", i8(2), 3, 1},
		{"null counts as different", pgtype.Int8{}, 3, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := bound()
			r.SessionAuthzVersion, r.CurrentAuthzVersion = tc.session, i8(tc.current)
			e := newEnv(t, returns(r), nil)
			if rc := e.get("/v1/items", bearer(token(1))); rc.Code != 200 {
				t.Fatalf("status = %d", rc.Code)
			}
			b := e.res.bound()
			if len(b) != tc.binds || (tc.binds == 1 && b[0].version != tc.current) {
				t.Fatalf("binds = %+v, want %d", b, tc.binds)
			}
		})
	}
}

// Criterio 4: el UPDATE con 0 filas (desligada en paralelo) -> 403
// tenant_access_denied sin ejecutar el handler y sin cachear.
func TestLostBindIs403TenantAccessDeniedAndNothingIsCached(t *testing.T) {
	r := bound()
	r.SessionAuthzVersion = i8(1)
	r.CurrentAuthzVersion = i8(2)
	res := returns(r)
	res.bind = func(context.Context, pgtype.UUID, pgtype.UUID, int64) (bool, error) { return false, nil }
	e := newEnv(t, res, nil)
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 403, "tenant_access_denied")
	if e.hits("tenant") != 0 {
		t.Fatal("the handler ran on a session unbound in parallel")
	}
	if len(e.denials.recorded()) != 0 {
		t.Fatal("tenant_access_denied was audited as a permission denial")
	}
	// Nada cacheado: la siguiente petición vuelve a resolver.
	e.get("/v1/items", bearer(token(1)))
	if n := e.perms.resolves(); n != 2 {
		t.Fatalf("resolves = %d, want 2: the failed bind must not cache", n)
	}
}

// Criterio 5: rol con requires_mfa y mfa_method none -> revoca y 403 mfa_required.
func TestRoleRequiringMFARevokesTheSession(t *testing.T) {
	for _, tc := range []struct {
		name    string
		method  string
		require bool
		code    int
		revoked int
	}{
		{"required and none", "none", true, 403, 1},
		{"required and totp", "totp", true, 200, 0},
		{"not required and none", "none", false, 200, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := bound()
			r.MFAMethod = tc.method
			e := newEnv(t, returns(r), nil)
			e.perms.set(func(_ context.Context, s auth.Session) (auth.Authz, error) {
				return auth.Authz{Version: 1, RequiresMFA: tc.require, Permissions: permSet(permission.VentasPosRead)}, nil
			})
			rec := e.get("/v1/items", bearer(token(1)))
			if rec.Code != tc.code {
				t.Fatalf("status = %d, want %d", rec.Code, tc.code)
			}
			if tc.code == 403 {
				mustProblem(t, rec, 403, "mfa_required")
				if e.hits("tenant") != 0 {
					t.Fatal("the handler ran without the required MFA")
				}
			}
			if got := e.res.revoked(); len(got) != tc.revoked || (tc.revoked == 1 && got[0] != uid(1)) {
				t.Fatalf("revoked = %v, want %d", got, tc.revoked)
			}
		})
	}
}

// 2FA exigida con una ruta de NoPermission también corta, y la caché no la salta.
func TestMFARequirementAppliesToNoPermissionRoutesAndCachedRoles(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	e.perms.set(func(_ context.Context, s auth.Session) (auth.Authz, error) {
		return auth.Authz{Version: 1, RequiresMFA: true, Permissions: permSet(permission.VentasPosRead)}, nil
	})
	mustProblem(t, e.get("/v1/me/entitlements", bearer(token(1))), 403, "mfa_required")
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 403, "mfa_required") // desde la caché
	if e.perms.resolves() != 1 {
		t.Fatal("the second request did not use the cache")
	}
}

// Si la revocación falla, el rechazo sigue: 403 mfa_required y un log.
func TestFailedRevokeStillAnswersMFARequired(t *testing.T) {
	res := returns(bound())
	res.revoke = func(context.Context, pgtype.UUID) error { return errors.New("boom secret") }
	e := newEnv(t, res, nil)
	e.perms.set(func(_ context.Context, s auth.Session) (auth.Authz, error) {
		return auth.Authz{Version: 1, RequiresMFA: true, Permissions: permSet(permission.VentasPosRead)}, nil
	})
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 403, "mfa_required")
	if l := e.log.lines(t, "session revoke for mfa_required failed"); len(l) != 1 {
		t.Fatalf("logs = %v", l)
	}
	if strings.Contains(e.log.String(), "boom secret") {
		t.Fatal("the revoke error text reached the log")
	}
}

// Si la auditoría de la denegación falla, la respuesta es 403 igual.
func TestDenialAuditFailureStillAnswers403(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	e.perms.set(grantOnly())
	e.denials.err = errors.New("audit down secret")
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 403, "forbidden")
	if e.metrics.get("authz_denied_audit_failed_total{}") != 1 {
		t.Fatal("the audit failure was not counted")
	}
	if l := e.log.lines(t, "denied attempt could not be audited"); len(l) != 1 {
		t.Fatalf("logs = %v", l)
	}
	if strings.Contains(e.log.String(), "audit down secret") {
		t.Fatal("the audit error text reached the log")
	}
}

// Un error del resolver o de la base no deja pasar: 503, o 500 si no está tipado.
func TestResolverFailuresNeverLetTheRequestThrough(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"database unavailable", db.ErrUnavailable, 503, "dependency_unavailable"},
		{"overloaded", db.ErrOverloaded, 503, "overloaded"},
		{"statement timeout", db.ErrStatementTimeout, 503, "dependency_unavailable"},
		{"untyped", errors.New("unexpected"), 500, "internal"},
		{"own deadline", context.DeadlineExceeded, 503, "dependency_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, returns(bound()), nil)
			e.perms.set(func(context.Context, auth.Session) (auth.Authz, error) { return auth.Authz{}, tc.err })
			mustProblem(t, e.get("/v1/items", bearer(token(1))), tc.status, tc.code)
			if e.hits("tenant") != 0 {
				t.Fatal("the handler ran after a failed resolution")
			}
		})
	}
	// Un fallo al ligar la versión tampoco deja pasar.
	r := bound()
	r.SessionAuthzVersion = i8(0)
	res := returns(r)
	res.bind = func(context.Context, pgtype.UUID, pgtype.UUID, int64) (bool, error) { return false, db.ErrUnavailable }
	e := newEnv(t, res, nil)
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 503, "dependency_unavailable")
	if e.hits("tenant") != 0 {
		t.Fatal("the handler ran after a failed bind")
	}
}

// La resolución corre con la fase authz (la que ve el log de la base).
func TestResolutionRunsInThePhaseAuthz(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	e.perms.set(func(context.Context, auth.Session) (auth.Authz, error) { return auth.Authz{}, db.ErrStatementTimeout })
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 503, "dependency_unavailable")
	if l := e.log.lines(t, "statement timeout"); len(l) != 1 || l[0]["phase"] != "authz" {
		t.Fatalf("phase lines = %v", l)
	}
}

// --- Impersonada ---

// Listas escritas a mano desde el ADR 0001 («Impersonación de soporte»): no se
// calculan con la fórmula del código.
var (
	impersonationExcluded = []permission.Code{
		"identidad.support_access:read", "identidad.support_access:approve",
		"identidad.users:read", "identidad.users:create", "identidad.users:update",
		"identidad.roles:read", "identidad.roles:create", "identidad.roles:update",
		"identidad.sessions:read", "identidad.sessions:void",
		"empresas.einvoicing:update",
	}
	// empresas.einvoicing:read sí entra en full; solo :update está excluida.
)

func isExcluded(c permission.Code) bool {
	for _, x := range impersonationExcluded {
		if x == c {
			return true
		}
	}
	return false
}

// permRoutes registra una ruta Tenant por cada código del catálogo y devuelve
// el camino de cada una.
func permRoutes(e *env) map[permission.Code]string {
	paths := map[permission.Code]string{}
	for i, c := range permission.All() {
		p := fmt.Sprintf("/v1/perm/%d", i)
		paths[c] = p
		e.routes.HandleFunc("GET "+p, auth.Tenant, auth.Spec{Module: "ventas", Permission: string(c)}, func(w http.ResponseWriter, _ *http.Request) {})
	}
	return paths
}

func impersonatedWith(scope auth.ImpersonationScope) auth.SessionRecord {
	r := impersonated()
	r.ImpersonationScope = scope
	return r
}

// Criterio 6: los permisos de la impersonada son los del alcance del grant,
// nunca los de un rol; las acciones excluidas dan 403 aun con alcance full.
func TestImpersonatedPermissionsAreTheGrantScope(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scope auth.ImpersonationScope
		want  func(permission.Code) bool
	}{
		{"read_only is only the read action", auth.ScopeReadOnly, func(c permission.Code) bool { return strings.HasSuffix(string(c), ":read") }},
		{"full is everything but the excluded", auth.ScopeFull, func(c permission.Code) bool { return !isExcluded(c) }},
		{"unknown scope is nothing", 0, func(permission.Code) bool { return false }},
		{"a scope outside the type is nothing", 7, func(permission.Code) bool { return false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			huge := httpx.LimiterConfig{Capacity: 1 << 30, Refill: time.Nanosecond, MaxKeys: 10}
			e := newEnv(t, returns(impersonatedWith(tc.scope)), func(o *auth.Options) { o.SessionLimiter, o.TenantLimiter = huge, huge })
			paths := permRoutes(e)
			e.perms.set(func(context.Context, auth.Session) (auth.Authz, error) {
				t.Error("a role was resolved for an impersonated session")
				return auth.Authz{Permissions: permSet(permission.All()...)}, nil
			})
			allowed := 0
			for _, c := range permission.All() {
				rec := e.get(paths[c], bearer(token(1)))
				if tc.want(c) {
					allowed++
					if rec.Code != 200 {
						t.Errorf("%s = %d, want 200", c, rec.Code)
					}
				} else if rec.Code != 403 || permOf(t, rec).Code != "forbidden" || permOf(t, rec).Permission != string(c) {
					t.Errorf("%s = %d %s, want 403 forbidden", c, rec.Code, rec.Body.String())
				}
			}
			if tc.scope == auth.ScopeFull && allowed != len(permission.All())-len(impersonationExcluded) {
				t.Errorf("allowed %d codes", allowed)
			}
			if tc.scope == 0 && allowed != 0 {
				t.Errorf("unknown scope allowed %d codes", allowed)
			}
		})
	}
}

// La denegación de la impersonada se audita con el actor del staff, y no se
// toca la versión ni la 2FA.
func TestImpersonatedDenialIsAuditedWithTheStaffActor(t *testing.T) {
	e := newEnv(t, returns(impersonatedWith(auth.ScopeReadOnly)), nil)
	mustProblem(t, e.do("POST", "/v1/users", bearer(token(1))), 403, "forbidden")
	evs := e.denials.recorded()
	if len(evs) != 1 {
		t.Fatalf("audited = %+v", evs)
	}
	if d, _ := evs[0].Diff.(map[string]string); d["reason"] != "impersonation_scope" {
		t.Fatalf("diff = %v", evs[0].Diff)
	}
	if a := e.denials.actors[0]; a.Kind != db.KindImpersonated || a.ImpersonatorID != uid(9) || a.TenantID != uid(3) {
		t.Fatalf("actor = %+v", a)
	}
	if len(e.res.bound()) != 0 || len(e.res.revoked()) != 0 {
		t.Fatal("an impersonated session was bound or revoked")
	}
}

// Una impersonada llega a las rutas NoPermission (como cualquier sesión), con
// o sin alcance.
func TestImpersonatedWithoutScopeStillReachesNoPermissionRoutes(t *testing.T) {
	e := newEnv(t, returns(impersonatedWith(0)), nil)
	if rec := e.get("/v1/me/entitlements", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
}

// --- Sedes ---

func TestCheckSite(t *testing.T) {
	siteA, siteB, siteC := uid(5), uid(50), uid(51)
	for _, tc := range []struct {
		name  string
		authz auth.Authz
		site  pgtype.UUID
		ok    bool
	}{
		{"the session's site", auth.Authz{Version: 1}, siteA, true},
		{"all sites", auth.Authz{Version: 1, AllSites: true}, siteB, true},
		{"a site of the membership", auth.Authz{Version: 1, Sites: []pgtype.UUID{siteB}}, siteB, true},
		{"a site outside the scope", auth.Authz{Version: 1, Sites: []pgtype.UUID{siteB}}, siteC, false},
		{"no scope data", auth.Authz{Version: 1}, siteC, false},
		{"absent resource site", auth.Authz{Version: 1, AllSites: true}, pgtype.UUID{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, returns(bound()), nil)
			tc.authz.Permissions = permSet(permission.VentasPosRead)
			e.perms.set(func(context.Context, auth.Session) (auth.Authz, error) { return tc.authz, nil })
			e.routes.HandleFunc("GET /v1/sited", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:read"}, func(w http.ResponseWriter, r *http.Request) {
				if err := auth.CheckSite(r.Context(), tc.site); err != nil {
					httpx.WriteError(w, r, err)
				}
			})
			rec := e.get("/v1/sited", bearer(token(1)))
			if tc.ok {
				if rec.Code != 200 {
					t.Fatalf("status = %d", rec.Code)
				}
				return
			}
			mustProblem(t, rec, 403, "site_out_of_scope")
		})
	}
	// Sin sesión en el contexto, deniega.
	if err := auth.CheckSite(context.Background(), siteA); err == nil {
		t.Fatal("CheckSite passed without a session")
	}
}

// Una impersonada no trae alcance de sedes: sin datos, solo la sede de la sesión.
func TestCheckSiteForImpersonatedHasNoSiteScope(t *testing.T) {
	r := impersonatedWith(auth.ScopeFull)
	e := newEnv(t, returns(r), nil)
	e.routes.HandleFunc("GET /v1/sited", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:read"}, func(w http.ResponseWriter, r *http.Request) {
		if err := auth.CheckSite(r.Context(), uid(77)); err != nil {
			httpx.WriteError(w, r, err)
		}
	})
	mustProblem(t, e.get("/v1/sited", bearer(token(1))), 403, "site_out_of_scope")
}

// --- Registro ---

func TestRegisteredListsEveryRouteWithItsSpec(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	got := map[string]auth.RouteInfo{}
	for _, ri := range e.routes.Registered() {
		got[ri.Pattern] = ri
	}
	if ri := got["GET /v1/items"]; ri.Class != auth.Tenant || ri.Spec.Permission != "ventas.pos:read" || ri.Spec.Module != "ventas" {
		t.Fatalf("GET /v1/items = %+v", ri)
	}
	if ri := got["POST /v1/auth/login"]; ri.Class != auth.Public {
		t.Fatalf("login = %+v", ri)
	}
	if _, ok := got["GET /healthz"]; ok {
		t.Fatal("a route outside /v1 is listed")
	}
}

// El paso decide con el contexto de la petición: el cliente que se va no deja
// pasar nada y el handler no corre.
func TestCancelledRequestNeverReachesTheHandler(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	e.perms.set(func(ctx context.Context, _ auth.Session) (auth.Authz, error) { return auth.Authz{}, ctx.Err() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/v1/items", nil).WithContext(ctx)
	req.Header.Set("Authorization", bearer(token(1)))
	e.h.ServeHTTP(httptest.NewRecorder(), req)
	if e.hits("tenant") != 0 {
		t.Fatal("the handler ran on a cancelled request")
	}
}
