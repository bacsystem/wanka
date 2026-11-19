package auth_test

import (
	"context"
	"encoding/base64"
	"encoding/hex"
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
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// --- autenticación sin consultar la base ---

func TestMalformedAuthorizationIs401UnauthenticatedWithoutTouchingTheDatabase(t *testing.T) {
	good := token(1)
	body := strings.TrimPrefix(good, "wk_at_")
	cases := map[string]string{
		"missing":              "",
		"basic":                "Basic " + good,
		"bearer without token": "Bearer ",
		"refresh as access":    "Bearer " + strings.Replace(good, "wk_at_", "wk_rt_", 1),
		"platform token":       "Bearer " + strings.Replace(good, "wk_at_", "wka_at_", 1),
		"42 characters":        "Bearer " + good[:len(good)-1],
		"44 characters":        "Bearer " + good + "A",
		"trailing =":           "Bearer " + good + "=",
		"plus":                 "Bearer wk_at_" + strings.Repeat("+", 43),
		"slash":                "Bearer wk_at_" + strings.Repeat("/", 43),
		"not canonical":        "Bearer wk_at_" + strings.Repeat("A", 42) + "B",
		"line feed inside":     "Bearer wk_at_" + body[:20] + "\n" + body[21:],
		"carriage return":      "Bearer wk_at_" + body[:20] + "\r" + body[21:],
		"two spaces":           "Bearer  " + good,
	}
	for name, authz := range cases {
		for _, path := range []string{"/v1/me", "/v1/items"} {
			t.Run(name+" "+path, func(t *testing.T) {
				e := newEnv(t, returns(bound()), nil)
				req := httptest.NewRequest(http.MethodGet, path, nil)
				if authz != "" {
					req.Header["Authorization"] = []string{authz} // sin validar el valor, como llegaría de la red
				}
				rec := httptest.NewRecorder()
				e.h.ServeHTTP(rec, req)
				wantUnauth(t, rec, "unauthenticated")
				if e.res.lookups() != 0 {
					t.Fatal("the database was consulted for a token that is not well formed")
				}
			})
		}
	}
}

func TestTwoAuthorizationHeadersAre401(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Add("Authorization", bearer(token(1)))
	req.Header.Add("Authorization", bearer(token(1)))
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	wantUnauth(t, rec, "unauthenticated")
	if e.res.lookups() != 0 {
		t.Fatal("lookup for two Authorization headers")
	}
}

func TestBearerSchemeIsCaseInsensitiveAndLookupGetsTheSha256(t *testing.T) {
	for _, scheme := range []string{"Bearer", "bearer", "BEARER"} {
		e := newEnv(t, returns(bound()), nil)
		tok := token(5)
		if rec := e.get("/v1/me", scheme+" "+tok); rec.Code != 200 {
			t.Fatalf("%s: status %d", scheme, rec.Code)
		}
		if len(e.res.hashes) != 1 || e.res.hashes[0] != hashOf(tok) {
			t.Fatalf("%s: hashes = %x", scheme, e.res.hashes)
		}
	}
}

// --- sin fila y MFA ---

func TestNoRowAndUnfinishedMFAAre401SessionExpired(t *testing.T) {
	unfinished := bound()
	unfinished.MFASatisfied = false
	for name, res := range map[string]*fakeResolver{"no row": noRow(), "mfa not satisfied": returns(unfinished)} {
		for _, path := range []string{"/v1/me", "/v1/items"} {
			t.Run(name+" "+path, func(t *testing.T) {
				e := newEnv(t, res, nil)
				wantUnauth(t, e.get(path, bearer(token(1))), "session_expired")
				if e.hits("notenant")+e.hits("tenant") != 0 {
					t.Fatal("a handler ran")
				}
			})
		}
	}
}

// --- fallos de la base: nunca 401 ni 500 ---

func TestLookupDatabaseFailuresAre503(t *testing.T) {
	cases := map[string]struct {
		err     error
		code    string
		counter string
	}{
		"overloaded":        {fmt.Errorf("acquiring: %w", db.ErrOverloaded), "overloaded", ""},
		"unavailable":       {fmt.Errorf("%w: %w", db.ErrUnavailable, errors.New("conn refused")), "dependency_unavailable", "db_unavailable_total{phase,session_lookup,route,GET /v1/items}"},
		"statement timeout": {statementTimeout(), "dependency_unavailable", "db_statement_timeouts_total{phase,session_lookup,route,GET /v1/items}"},
		"untyped 57014":     {&pgconn.PgError{Code: "57014", Message: "canceling statement due to user request"}, "dependency_unavailable", "db_unavailable_total{phase,session_lookup,route,GET /v1/items}"},
		"deadline error":    {fmt.Errorf("query: %w", context.DeadlineExceeded), "dependency_unavailable", "db_unavailable_total{phase,session_lookup,route,GET /v1/items}"},
	}
	for name, tc := range cases {
		for _, path := range []string{"/v1/me", "/v1/items"} {
			t.Run(name+" "+path, func(t *testing.T) {
				e := newEnv(t, fails(tc.err), nil)
				rec := e.get(path, bearer(token(1)))
				mustProblem(t, rec, 503, tc.code)
				if rec.Header().Get("Retry-After") != "1" || rec.Header().Get("WWW-Authenticate") != "" {
					t.Fatalf("headers %v", rec.Header())
				}
				noQuota(t, rec)
				if strings.Contains(rec.Body.String(), "57014") || strings.Contains(rec.Body.String(), "secret_column") {
					t.Fatalf("PostgreSQL text leaked: %s", rec.Body.String())
				}
				if e.hits("notenant")+e.hits("tenant") != 0 {
					t.Fatal("a handler ran")
				}
				if tc.counter != "" && path == "/v1/items" && e.metrics.get(tc.counter) != 1 {
					t.Fatalf("counter %s = %d; %v", tc.counter, e.metrics.get(tc.counter), e.metrics.counts)
				}
			})
		}
	}
}

func TestStatementTimeoutInLookupLogsOneWarnWithPhaseAndNoSQL(t *testing.T) {
	e := newEnv(t, fails(statementTimeout()), nil)
	e.get("/v1/items", bearer(token(1)))
	lines := e.log.lines(t, "statement timeout")
	if len(lines) != 1 {
		t.Fatalf("warn lines = %d; log %s", len(lines), e.log.String())
	}
	if l := lines[0]; l["level"] != "WARN" || l["phase"] != "session_lookup" || l["sqlstate"] != "57014" || l["route"] != "GET /v1/items" {
		t.Fatalf("log line = %v", l)
	}
	if strings.Contains(e.log.String(), "secret_column") {
		t.Fatal("PostgreSQL text in the log")
	}
}

// La fase se fija solo en el contexto de la búsqueda: el 57014 del handler de
// otra petición lleva phase=handler.
func TestStatementTimeoutPhaseIsLookupOnlyAndNeverReachesTheHandler(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	mustProblem(t, e.get("/v1/slow", bearer(token(1))), 503, "dependency_unavailable")
	lines := e.log.lines(t, "statement timeout")
	if len(lines) != 1 || lines[0]["phase"] != "handler" {
		t.Fatalf("handler timeout log = %v", lines)
	}
	e2 := newEnv(t, fails(statementTimeout()), nil)
	e2.get("/v1/items", bearer(token(1)))
	if l := e2.log.lines(t, "statement timeout"); len(l) != 1 || l[0]["phase"] != "session_lookup" {
		t.Fatalf("lookup timeout log = %v", l)
	}
}

func TestLookupDeadlineOfOneSecondIs503WhileTheRequestIsAlive(t *testing.T) {
	block := &fakeResolver{lookup: func(ctx context.Context, _ [32]byte) (auth.SessionRecord, bool, error) {
		<-ctx.Done()
		return auth.SessionRecord{}, false, ctx.Err()
	}}
	for _, path := range []string{"/v1/me", "/v1/items"} {
		e := newEnv(t, block, func(o *auth.Options) { o.LookupTimeout = 30 * time.Millisecond })
		start := time.Now()
		rec := e.get(path, bearer(token(1)))
		mustProblem(t, rec, 503, "dependency_unavailable")
		if rec.Header().Get("Retry-After") != "1" {
			t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
		}
		if time.Since(start) > 2*time.Second {
			t.Fatal("too slow")
		}
		lines := e.log.lines(t, "session lookup timed out")
		if len(lines) != 1 || lines[0]["phase"] != "session_lookup" || lines[0]["level"] != "WARN" {
			t.Fatalf("log = %s", e.log.String())
		}
		if e.hits("notenant")+e.hits("tenant") != 0 {
			t.Fatal("handler ran")
		}
	}
}

func TestLookupDefaultDeadlineIsOneSecond(t *testing.T) {
	var got time.Duration
	res := &fakeResolver{lookup: func(ctx context.Context, _ [32]byte) (auth.SessionRecord, bool, error) {
		dl, ok := ctx.Deadline()
		if !ok {
			t.Error("no deadline")
		}
		got = time.Until(dl)
		return bound(), true, nil
	}}
	newEnv(t, res, nil).get("/v1/me", bearer(token(1)))
	if got <= 900*time.Millisecond || got > time.Second {
		t.Fatalf("deadline in %v, want about 1s", got)
	}
}

func TestRequestContextEndedDuringLookupWritesNothingAndRunsNothing(t *testing.T) {
	outcomes := map[string]func() (auth.SessionRecord, bool, error){
		"database error": func() (auth.SessionRecord, bool, error) {
			return auth.SessionRecord{}, false, fmt.Errorf("%w: %w", db.ErrUnavailable, context.Canceled)
		},
		"untyped error": func() (auth.SessionRecord, bool, error) {
			return auth.SessionRecord{}, false, errors.New("scan failed")
		},
		"context error": func() (auth.SessionRecord, bool, error) { return auth.SessionRecord{}, false, context.Canceled },
		"no row":        func() (auth.SessionRecord, bool, error) { return auth.SessionRecord{}, false, nil },
		"valid row":     func() (auth.SessionRecord, bool, error) { return suspended(), true, nil },
	}
	for name, out := range outcomes {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			res := &fakeResolver{lookup: func(context.Context, [32]byte) (auth.SessionRecord, bool, error) {
				cancel()
				return out()
			}}
			e := newEnv(t, res, nil)
			req := httptest.NewRequest(http.MethodGet, "/v1/items", nil).WithContext(ctx)
			req.Header.Set("Authorization", bearer(token(1)))
			rec := httptest.NewRecorder()
			e.h.ServeHTTP(rec, req)
			if rec.Body.Len() != 0 || e.hits("tenant") != 0 || len(e.res.detaches()) != 0 {
				t.Fatalf("wrote %q, handler %d, detaches %d", rec.Body.String(), e.hits("tenant"), len(e.res.detaches()))
			}
			for _, msg := range []string{"database unavailable", "session lookup timed out", "session lookup failed with an unexpected error", "session record is inconsistent"} {
				if len(e.log.lines(t, msg)) != 0 {
					t.Fatalf("logged %q: %s", msg, e.log.String())
				}
			}
		})
	}
}

// --- error no tipado y fila incoherente: 500, el handler no corre ---

func TestUntypedErrorAndInconsistentRowAre500AndTheHandlerDoesNotRun(t *testing.T) {
	mod := func(f func(*auth.SessionRecord)) auth.SessionRecord { r := bound(); f(&r); return r }
	cases := map[string]*fakeResolver{
		"untyped error":             fails(errors.New("unexpected column type")),
		"tenant without membership": returns(mod(func(r *auth.SessionRecord) { r.MembershipID = pgtype.UUID{} })),
		"membership without tenant": returns(mod(func(r *auth.SessionRecord) { r.TenantID = pgtype.UUID{} })),
		"impersonator without tenant": returns(mod(func(r *auth.SessionRecord) {
			r.TenantID, r.MembershipID, r.ImpersonatorID = pgtype.UUID{}, pgtype.UUID{}, uid(9)
		})),
		"empty user": returns(mod(func(r *auth.SessionRecord) { r.UserID = pgtype.UUID{} })),
	}
	for name, res := range cases {
		for _, path := range []string{"/v1/me", "/v1/items"} {
			t.Run(name+" "+path, func(t *testing.T) {
				e := newEnv(t, res, nil)
				rec := e.get(path, bearer(token(1)))
				p := mustProblem(t, rec, 500, "internal")
				if p.Detail != "" {
					t.Fatalf("detail %q", p.Detail)
				}
				if e.hits("notenant")+e.hits("tenant") != 0 || len(res.detaches()) != 0 {
					t.Fatal("a handler ran or Detach was called")
				}
				var errs int
				for _, ln := range strings.Split(e.log.String(), "\n") {
					if strings.Contains(ln, `"level":"ERROR"`) {
						errs++
					}
				}
				if errs != 1 {
					t.Fatalf("error lines = %d; log %s", errs, e.log.String())
				}
			})
		}
	}
}

// --- membresía perdida (ADR 0011) ---

func lostCases() map[string]auth.SessionRecord {
	deleted := bound()
	deleted.CurrentAuthzVersion, deleted.MembershipStatus = pgtype.Int8{}, pgtype.Text{}
	noVersion := bound()
	noVersion.CurrentAuthzVersion = pgtype.Int8{} // versión ausente con estado activo: nunca "sin cambios"
	return map[string]auth.SessionRecord{"deleted": deleted, "suspended": suspended(), "no current version": noVersion}
}

func TestLostMembershipDetachesOnceAndAnswersByRouteClass(t *testing.T) {
	for name, rec := range lostCases() {
		t.Run(name+"/tenant route", func(t *testing.T) {
			e := newEnv(t, returns(rec), nil)
			resp := e.get("/v1/items", bearer(token(1)))
			p := mustProblem(t, resp, 403, "tenant_access_denied")
			if p.Detail != "membership is no longer active" {
				t.Fatalf("detail %q", p.Detail)
			}
			noQuota(t, resp)
			if resp.Header().Get("WWW-Authenticate") != "" {
				t.Fatal("a 403 must not carry WWW-Authenticate")
			}
			if d := e.res.detaches(); len(d) != 1 || d[0] != [2]pgtype.UUID{uid(1), uid(4)} {
				t.Fatalf("Detach calls = %v", d)
			}
			if e.res.lookups() != 1 || e.hits("tenant") != 0 {
				t.Fatalf("lookups %d, handler %d", e.res.lookups(), e.hits("tenant"))
			}
			if e.metrics.get("auth_session_detach_total{outcome,detached}") != 1 {
				t.Fatalf("counters %v", e.metrics.counts)
			}
		})
		t.Run(name+"/no-tenant route", func(t *testing.T) {
			e := newEnv(t, returns(rec), nil)
			if resp := e.get("/v1/me", bearer(token(1))); resp.Code != 200 {
				t.Fatalf("status %d", resp.Code)
			}
			s := e.session("notenant")
			if s.HasTenant() || s.Actor.TenantID.Valid || s.MembershipID.Valid || s.SiteID.Valid || s.CashRegisterID.Valid || s.SessionAuthzVersion.Valid || s.CurrentAuthzVersion.Valid {
				t.Fatalf("handler sees tenant columns: %+v", s)
			}
			if s.ID != uid(1) || s.Actor.UserID != uid(2) {
				t.Fatalf("identity lost: %+v", s)
			}
			if len(e.res.detaches()) != 1 {
				t.Fatalf("Detach calls = %v", e.res.detaches())
			}
		})
	}
}

func TestDetachWithZeroRowsOrAnErrorChangesNothingAndRetriesNextTime(t *testing.T) {
	outcomes := map[string]struct {
		detach  func(context.Context, pgtype.UUID, pgtype.UUID) (bool, error)
		outcome string
	}{
		"0 rows": {func(context.Context, pgtype.UUID, pgtype.UUID) (bool, error) { return false, nil }, "already"},
		"error": {func(context.Context, pgtype.UUID, pgtype.UUID) (bool, error) {
			return false, fmt.Errorf("%w: dial tcp 10.1.2.3:5432", db.ErrUnavailable)
		}, "failed"},
	}
	for name, tc := range outcomes {
		t.Run(name, func(t *testing.T) {
			res := returns(suspended())
			res.detach = tc.detach
			e := newEnv(t, res, nil)
			mustProblem(t, e.get("/v1/items", bearer(token(1))), 403, "tenant_access_denied")
			if resp := e.get("/v1/me", bearer(token(1))); resp.Code != 200 {
				t.Fatalf("no-tenant route %d", resp.Code)
			}
			if e.session("notenant").HasTenant() {
				t.Fatal("handler kept the tenant")
			}
			// La fila sigue ligada en la base: la petición siguiente vuelve a desligar.
			if n := len(res.detaches()); n != 2 {
				t.Fatalf("Detach calls = %d, want one per request", n)
			}
			if got := e.metrics.get("auth_session_detach_total{outcome," + tc.outcome + "}"); got != 2 {
				t.Fatalf("counter %s = %d; %v", tc.outcome, got, e.metrics.counts)
			}
			warns := e.log.lines(t, "session detach failed")
			if (tc.outcome == "failed") != (len(warns) == 2) {
				t.Fatalf("warn lines = %d", len(warns))
			}
			if strings.Contains(e.log.String(), "10.1.2.3") {
				t.Fatal("error text in the log")
			}
		})
	}
}

func TestConcurrentRequestsWithTheSameLostMembershipAllGet403(t *testing.T) {
	res := returns(suspended())
	var wins int
	var mu sync.Mutex
	res.detach = func(context.Context, pgtype.UUID, pgtype.UUID) (bool, error) {
		mu.Lock()
		defer mu.Unlock()
		wins++
		return wins == 1, nil // una petición gana el UPDATE, la otra actualiza 0 filas
	}
	e := newEnv(t, res, nil)
	var wg sync.WaitGroup
	recs := make([]*httptest.ResponseRecorder, 3)
	paths := []string{"/v1/items", "/v1/items", "/v1/me"}
	for i := range recs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recs[i] = e.get(paths[i], bearer(token(1)))
		}()
	}
	wg.Wait()
	mustProblem(t, recs[0], 403, "tenant_access_denied")
	mustProblem(t, recs[1], 403, "tenant_access_denied")
	if recs[2].Code != 200 {
		t.Fatalf("GET /v1/me = %d", recs[2].Code)
	}
}

func TestEntitlementStepsNeverRunForAFormerMember(t *testing.T) {
	e := newEnv(t, returns(suspended()), nil)
	mustProblem(t, e.get("/v1/paid", bearer(token(1))), 403, "tenant_access_denied")
	if e.hits("entitlements") != 0 || e.hits("paid") != 0 {
		t.Fatal("a step after the limiter ran for a former member")
	}
	// Con una sesión ligada y activa el mismo paso sí corre y responde 402.
	e = newEnv(t, returns(bound()), nil)
	mustProblem(t, e.get("/v1/paid", bearer(token(1))), 402, "subscription_inactive")
	if e.hits("entitlements") != 1 {
		t.Fatal("the test step never runs: the test proves nothing")
	}
}

func TestSessionWithoutCompanyOnATenantRouteIs403WithoutDetach(t *testing.T) {
	e := newEnv(t, returns(noCompany()), nil)
	resp := e.get("/v1/items", bearer(token(1)))
	if p := mustProblem(t, resp, 403, "tenant_access_denied"); p.Detail != "session has no company selected" {
		t.Fatalf("detail %q", p.Detail)
	}
	noQuota(t, resp)
	if len(e.res.detaches()) != 0 || e.hits("tenant") != 0 {
		t.Fatal("Detach called or handler ran")
	}
	if resp := e.get("/v1/me", bearer(token(1))); resp.Code != 200 {
		t.Fatalf("no-tenant route %d", resp.Code)
	}
}

func TestImpersonatedSessionIsNeverDetachedAndKeepsItsActor(t *testing.T) {
	stale := impersonated()
	stale.MembershipID = uid(4) // aunque el resolver devolviera una membresía, no se desliga
	stale.CurrentAuthzVersion = pgtype.Int8{}
	for name, rec := range map[string]auth.SessionRecord{"impersonated": impersonated(), "with a stale membership": stale} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, returns(rec), nil)
			if resp := e.get("/v1/items", bearer(token(1))); resp.Code != 200 {
				t.Fatalf("status %d: %s", resp.Code, resp.Body.String())
			}
			a := e.session("tenant").Actor
			if a != (db.Actor{TenantID: uid(3), UserID: uid(9), Kind: db.KindImpersonated, ImpersonatorID: uid(9)}) {
				t.Fatalf("actor = %+v", a)
			}
			if len(e.res.detaches()) != 0 {
				t.Fatal("Detach called")
			}
		})
	}
	// Grant revocado o vencido: el resolver no devuelve fila.
	wantUnauth(t, newEnv(t, noRow(), nil).get("/v1/items", bearer(token(1))), "session_expired")
}

func TestMembershipThatIsActiveAgainIsServedNormally(t *testing.T) {
	cur := returns(suspended())
	e := newEnv(t, cur, nil)
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 403, "tenant_access_denied")
	cur.lookup = func(context.Context, [32]byte) (auth.SessionRecord, bool, error) { return bound(), true, nil }
	if resp := e.get("/v1/items", bearer(token(1))); resp.Code != 200 {
		t.Fatalf("status %d", resp.Code)
	}
	if len(cur.detaches()) != 1 {
		t.Fatalf("Detach calls = %d", len(cur.detaches()))
	}
}

func TestTenantRouteNeverRunsItsHandlerWithoutATenant(t *testing.T) {
	for name, res := range map[string]*fakeResolver{"no company": returns(noCompany()), "lost membership": returns(suspended())} {
		e := newEnv(t, res, nil)
		for _, p := range []string{"/v1/items", "/v1/other", "/v1/teapot", "/v1/paid"} {
			if rec := e.get(p, bearer(token(1))); rec.Code != 403 {
				t.Fatalf("%s %s: %d", name, p, rec.Code)
			}
		}
		if e.hits("tenant")+e.hits("tenant2")+e.hits("paid") != 0 {
			t.Fatalf("%s: a tenant handler ran", name)
		}
	}
}

// --- contexto del handler ---

func TestHandlerSessionComesOnlyFromTheSession(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/items?tenantId="+uid(77).String()+"&tenant_id="+uid(77).String(), nil)
	req.Header.Set("Authorization", bearer(token(1)))
	req.Header.Set("X-Tenant-Id", uid(77).String())
	req.Header.Set("X-Site-Id", uid(78).String())
	e.h.ServeHTTP(httptest.NewRecorder(), req)
	s := e.session("tenant")
	want := auth.Session{ID: uid(1), Actor: db.Actor{TenantID: uid(3), UserID: uid(2), Kind: db.KindUser}, SiteID: uid(5), CashRegisterID: uid(6),
		MembershipID: uid(4), SessionAuthzVersion: i8(1), CurrentAuthzVersion: i8(1), MFAMethod: "none"}
	if s != want {
		t.Fatalf("session = %+v\nwant      %+v", s, want)
	}
}

func TestDetachedSessionActorFailsClosedInInTenantTx(t *testing.T) {
	e := newEnv(t, returns(suspended()), nil)
	e.get("/v1/me", bearer(token(1)))
	var p *db.Pool // validate corre antes de tocar el pool
	err := p.InTenantTx(context.Background(), e.session("notenant").Actor, nil)
	if !errors.Is(err, db.ErrNoTenant) {
		t.Fatalf("err = %v, want db.ErrNoTenant", err)
	}
}

// --- clases de ruta ---

func TestPublicRoutesNeverLookUpASessionNorCarryOne(t *testing.T) {
	e := newEnv(t, fails(errors.New("must not be called")), nil)
	for _, authz := range []string{"", bearer(token(1)), "garbage"} {
		for _, path := range []string{"/v1/auth/login", "/v1/auth/mfa/enrollment/totp"} {
			rec := e.do(http.MethodPost, path, authz)
			if rec.Code != 200 {
				t.Fatalf("%s: %d", path, rec.Code)
			}
			noQuota(t, rec)
		}
		if rec := e.get("/healthz", authz); rec.Code != 200 {
			t.Fatalf("health %d", rec.Code)
		}
	}
	if e.res.lookups() != 0 {
		t.Fatalf("lookups = %d", e.res.lookups())
	}
	if _, ok := e.seen["public"]; ok {
		t.Fatal("a public handler received a session")
	}
}

func TestTOTPEnrollmentRoutesHaveOneClassEach(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	wantUnauth(t, e.do(http.MethodPost, "/v1/auth/mfa/totp/enroll", ""), "unauthenticated")
	if rec := e.do(http.MethodPost, "/v1/auth/mfa/totp/enroll", bearer(token(1))); rec.Code != 200 || e.res.lookups() != 1 {
		t.Fatalf("session route: %d, lookups %d", rec.Code, e.res.lookups())
	}
	if rec := e.do(http.MethodPost, "/v1/auth/mfa/enrollment/totp", bearer(token(1))); rec.Code != 200 || e.res.lookups() != 1 {
		t.Fatalf("public route: %d, lookups %d", rec.Code, e.res.lookups())
	}
}

func TestRouteClassRulesPanicAtRegistration(t *testing.T) {
	type tc struct {
		name    string
		pattern string
		class   auth.RouteClass
		ok      bool
	}
	cases := []tc{
		{"zero class", "GET /v1/items", 0, false},
		{"undefined class", "GET /v1/items", 99, false},
		{"tenant route", "GET /v1/items", auth.Tenant, true},
		{"tenant route with wildcard", "GET /v1/items/{id}", auth.Tenant, true},
		{"public under auth", "POST /v1/auth/login", auth.Public, true},
		{"public outside auth", "GET /v1/items", auth.Public, false},
		{"public on /v1/me", "GET /v1/me", auth.Public, false},
		{"public on a look-alike prefix", "GET /v1/authx/login", auth.Public, false},
		{"public on /v1/auth itself", "GET /v1/auth", auth.Public, false},
		{"no-tenant under auth", "POST /v1/auth/logout", auth.NoTenant, true},
		{"no-tenant session/tenant", "POST /v1/auth/session/tenant", auth.NoTenant, true},
		{"no-tenant totp enroll", "POST /v1/auth/mfa/totp/enroll", auth.NoTenant, true},
		{"public totp enrollment", "POST /v1/auth/mfa/enrollment/totp", auth.Public, true},
		{"no-tenant GET /v1/me", "GET /v1/me", auth.NoTenant, true},
		{"no-tenant GET /v1/me/memberships", "GET /v1/me/memberships", auth.NoTenant, true},
		{"no-tenant PATCH /v1/me", "PATCH /v1/me", auth.NoTenant, false},
		{"no-tenant under /v1/me", "GET /v1/me/other", auth.NoTenant, false},
		{"no-tenant on a tenant resource", "GET /v1/items", auth.NoTenant, false},
		{"tenant under auth", "POST /v1/auth/login", auth.Tenant, false},
		{"tenant on GET /v1/me", "GET /v1/me", auth.Tenant, false},
		{"tenant on GET /v1/me/memberships", "GET /v1/me/memberships", auth.Tenant, false},
		{"tenant on PATCH /v1/me", "PATCH /v1/me", auth.Tenant, true},
		{"outside /v1", "GET /healthz", auth.Tenant, false},
		{"/v1x is not /v1", "GET /v1x/a", auth.Tenant, false},
		{"no method", "/v1/items", auth.Tenant, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			routes, err := auth.NewRoutes(httpx.NewRouter(httpx.RouterOptions{}), returns(bound()), auth.Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if r := recover(); (r == nil) != c.ok {
					t.Fatalf("panic = %v, want panic: %v", r, !c.ok)
				}
			}()
			routes.HandleFunc(c.pattern, c.class, func(http.ResponseWriter, *http.Request) {})
		})
	}
}

func TestNewRoutesValidatesItsInputs(t *testing.T) {
	rt := httpx.NewRouter(httpx.RouterOptions{})
	if _, err := auth.NewRoutes(nil, returns(bound()), auth.Options{}); err == nil {
		t.Error("nil router accepted")
	}
	if _, err := auth.NewRoutes(rt, nil, auth.Options{}); err == nil {
		t.Error("nil resolver accepted")
	}
	if _, err := auth.NewRoutes(rt, returns(bound()), auth.Options{SessionLimiter: httpx.LimiterConfig{Capacity: 1}}); err == nil {
		t.Error("invalid session limiter accepted")
	}
	if _, err := auth.NewRoutes(rt, returns(bound()), auth.Options{TenantLimiter: httpx.LimiterConfig{Capacity: 1}}); err == nil {
		t.Error("invalid tenant limiter accepted")
	}
}

// --- WWW-Authenticate ---

func TestWWWAuthenticateOnlyOnTheTwoKernel401Codes(t *testing.T) {
	for code, want := range map[string]string{"unauthenticated": "Bearer", "session_expired": "Bearer", "invalid_credentials": "", "forbidden": "", "internal": ""} {
		rec := httptest.NewRecorder()
		httpx.WriteProblem(rec, httptest.NewRequest(http.MethodGet, "/x", nil), code, "")
		if got := rec.Header().Get("WWW-Authenticate"); got != want {
			t.Errorf("%s: WWW-Authenticate = %q, want %q", code, got, want)
		}
	}
}

// --- el token y su hash no se registran ---

func TestNoLogLineCarriesTheTokenOrItsHash(t *testing.T) {
	tok := token(11)
	h := hashOf(tok)
	forms := []string{tok, strings.TrimPrefix(tok, "wk_at_"), hex.EncodeToString(h[:]), strings.ToUpper(hex.EncodeToString(h[:])),
		base64.StdEncoding.EncodeToString(h[:]), base64.RawStdEncoding.EncodeToString(h[:]),
		base64.URLEncoding.EncodeToString(h[:]), base64.RawURLEncoding.EncodeToString(h[:]), fmt.Sprint(h), fmt.Sprintf("%v", h[:])}
	failingDetach := returns(suspended())
	failingDetach.detach = func(context.Context, pgtype.UUID, pgtype.UUID) (bool, error) {
		return false, errors.New("update failed")
	}
	incoherent := bound()
	incoherent.MembershipID = pgtype.UUID{}
	scenarios := map[string]*fakeResolver{
		"ok": returns(bound()), "no row": noRow(), "incoherent row": returns(incoherent),
		"unavailable":       fails(fmt.Errorf("%w: boom", db.ErrUnavailable)),
		"statement timeout": fails(statementTimeout()),
		"unknown error":     fails(errors.New("unexpected")),
		"overloaded":        fails(db.ErrOverloaded),
		"detach fails":      failingDetach,
	}
	for name, res := range scenarios {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, res, nil)
			for _, p := range []string{"/v1/me", "/v1/items"} {
				e.get(p, bearer(tok))
			}
			e.get("/v1/items", "Bearer  "+tok)
			e.get("/v1/items", "Bearer "+strings.Replace(tok, "wk_at_", "wk_rt_", 1))
			out := e.log.String()
			if out == "" {
				t.Fatal("no log output: the test proves nothing")
			}
			for _, f := range forms {
				if strings.Contains(out, f) {
					t.Fatalf("log contains %q:\n%s", f, out)
				}
			}
		})
	}
}

// --- limitador ---

// sessions devuelve un resolver que mapea cada token a su fila.
func sessions(m map[string]auth.SessionRecord) *fakeResolver {
	return &fakeResolver{lookup: func(_ context.Context, h [32]byte) (auth.SessionRecord, bool, error) {
		for tok, r := range m {
			if h == hashOf(tok) {
				return r, true, nil
			}
		}
		return auth.SessionRecord{}, false, nil
	}}
}

func limits(sess, tenant int, sessRefill, tenantRefill time.Duration) func(*auth.Options) {
	return func(o *auth.Options) {
		o.SessionLimiter = httpx.LimiterConfig{Capacity: sess, Refill: sessRefill, MaxKeys: 100}
		o.TenantLimiter = httpx.LimiterConfig{Capacity: tenant, Refill: tenantRefill, MaxKeys: 100}
	}
}

func sibling(session, user byte) auth.SessionRecord {
	r := bound()
	r.SessionID, r.UserID = uid(session), uid(user)
	return r
}

func TestDefaultSessionLimitIs20Burst(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	for i := 1; i <= 20; i++ {
		rec := e.get("/v1/items", bearer(token(1)))
		if rec.Code != 200 || rec.Header().Get("RateLimit-Limit") != "20" {
			t.Fatalf("request %d: %d %v", i, rec.Code, rec.Header())
		}
	}
	rec := e.get("/v1/items", bearer(token(1)))
	mustProblem(t, rec, 429, "rate_limited")
	if rec.Header().Get("Retry-After") != "1" || rec.Header().Get("RateLimit-Limit") != "20" || rec.Header().Get("RateLimit-Remaining") != "0" {
		t.Fatalf("headers %v", rec.Header())
	}
	if e.hits("tenant") != 20 {
		t.Fatalf("handler ran %d times", e.hits("tenant"))
	}
	e.clk.Advance(100 * time.Millisecond) // 10/s: una ficha
	if e.get("/v1/items", bearer(token(1))).Code != 200 || e.get("/v1/items", bearer(token(1))).Code != 429 {
		t.Fatal("refill is not one token per 100 ms")
	}
}

func TestDefaultTenantLimitIs200BurstAndRefillsEvery10ms(t *testing.T) {
	var mu sync.Mutex
	n := byte(0)
	res := &fakeResolver{lookup: func(context.Context, [32]byte) (auth.SessionRecord, bool, error) {
		mu.Lock()
		defer mu.Unlock()
		n++
		return sibling(100+n%100, 100+n%100), true, nil // 100 sesiones de la misma empresa: ninguna agota la suya
	}}
	e := newEnv(t, res, func(o *auth.Options) {
		o.SessionLimiter = httpx.LimiterConfig{Capacity: 1000, Refill: time.Nanosecond, MaxKeys: 100}
	})
	for i := 1; i <= 200; i++ {
		if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 || rec.Header().Get("RateLimit-Limit") != "200" && rec.Header().Get("RateLimit-Limit") != "1000" {
			t.Fatalf("request %d: %d %v", i, rec.Code, rec.Header())
		}
	}
	rec := e.get("/v1/items", bearer(token(1)))
	mustProblem(t, rec, 429, "rate_limited")
	if rec.Header().Get("RateLimit-Limit") != "200" {
		t.Fatalf("headers %v", rec.Header())
	}
	e.clk.Advance(10 * time.Millisecond)
	if e.get("/v1/items", bearer(token(1))).Code != 200 || e.get("/v1/items", bearer(token(1))).Code != 429 {
		t.Fatal("tenant refill is not one token per 10 ms")
	}
}

// Escenario 1: el rechazo del bucket de la sesión no gasta ficha del tenant.
func TestSessionRejectionDoesNotSpendATenantToken(t *testing.T) {
	e := newEnv(t, sessions(map[string]auth.SessionRecord{token(1): sibling(1, 2), token(2): sibling(11, 12)}), limits(3, 5, time.Hour, time.Hour))
	var s1 []int
	for i := 0; i < 4; i++ {
		s1 = append(s1, e.get("/v1/items", bearer(token(1))).Code)
	}
	if fmt.Sprint(s1) != "[200 200 200 429]" {
		t.Fatalf("S1: %v", s1)
	}
	ok := 0
	var last *httptest.ResponseRecorder
	for i := 0; i < 10; i++ {
		last = e.get("/v1/items", bearer(token(2)))
		if last.Code == 429 {
			break
		}
		ok++
	}
	if ok != 2 {
		t.Fatalf("S2 got %d 200s, want exactly 2 (the S1 rejection spent a tenant token if it got 1)", ok)
	}
	if mustProblem(t, last, 429, "rate_limited"); last.Header().Get("RateLimit-Limit") != "5" {
		t.Fatalf("headers %v", last.Header())
	}
}

// Escenario 2: el rechazo del tenant devuelve la ficha de la sesión.
func TestTenantRejectionRefundsTheSessionToken(t *testing.T) {
	e := newEnv(t, returns(bound()), limits(3, 2, time.Hour, time.Second))
	var first []int
	for i := 0; i < 3; i++ {
		first = append(first, e.get("/v1/items", bearer(token(1))).Code)
	}
	if fmt.Sprint(first) != "[200 200 429]" {
		t.Fatalf("first three: %v", first)
	}
	// La 3.ª fue del tenant (RateLimit-Limit 2) y no debe haber gastado la ficha de la sesión.
	e.clk.Advance(2 * time.Second)
	ok := 0
	var last *httptest.ResponseRecorder
	for i := 0; i < 10; i++ {
		last = e.get("/v1/items", bearer(token(1)))
		if last.Code == 429 {
			break
		}
		ok++
	}
	if ok != 1 {
		t.Fatalf("got %d 200s after the refill, want exactly 1 (the session bucket kept the refunded token)", ok)
	}
	if last.Header().Get("RateLimit-Limit") != "3" {
		t.Fatalf("the second rejection is the session's: headers %v", last.Header())
	}
}

func TestTenantBucketIsSharedAcrossSessionsButNotAcrossTenantsNorNoTenantRoutes(t *testing.T) {
	other := sibling(21, 22)
	other.TenantID = uid(30)
	e := newEnv(t, sessions(map[string]auth.SessionRecord{token(1): sibling(1, 2), token(2): sibling(11, 12), token(3): other}), limits(50, 2, time.Hour, time.Hour))
	if e.get("/v1/items", bearer(token(1))).Code != 200 || e.get("/v1/other", bearer(token(2))).Code != 200 {
		t.Fatal("setup")
	}
	for _, tok := range []string{token(1), token(2)} {
		mustProblem(t, e.get("/v1/items", bearer(tok)), 429, "rate_limited")
	}
	if rec := e.get("/v1/items", bearer(token(3))); rec.Code != 200 {
		t.Fatalf("another tenant was affected: %d", rec.Code)
	}
	for i := 0; i < 5; i++ {
		rec := e.get("/v1/me", bearer(token(1)))
		if rec.Code != 200 || rec.Header().Get("RateLimit-Limit") != "50" {
			t.Fatalf("GET /v1/me after the tenant ran out: %d %v", rec.Code, rec.Header())
		}
	}
}

// La clave del bucket de sesión es el id de la sesión: dos sesiones del mismo
// usuario y de la misma empresa no comparten bucket.
func TestSessionBucketKeyIsTheSessionID(t *testing.T) {
	a, b := bound(), bound()
	b.SessionID = uid(11) // mismo usuario, otra sesión
	e := newEnv(t, sessions(map[string]auth.SessionRecord{token(1): a, token(2): b}), limits(1, 100, time.Hour, time.Hour))
	if e.get("/v1/items", bearer(token(1))).Code != 200 || e.get("/v1/items", bearer(token(2))).Code != 200 {
		t.Fatal("two sessions of the same user share a bucket")
	}
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 429, "rate_limited")
	// Dos tokens de la misma sesión sí comparten el bucket.
	e = newEnv(t, sessions(map[string]auth.SessionRecord{token(1): a, token(2): a}), limits(1, 100, time.Hour, time.Hour))
	e.get("/v1/items", bearer(token(1)))
	mustProblem(t, e.get("/v1/items", bearer(token(2))), 429, "rate_limited")
}

func TestQuotaHeadersOnHandlerResponsesAndNotOnEarlyRejections(t *testing.T) {
	e := newEnv(t, returns(bound()), nil)
	for _, p := range []string{"/v1/items", "/v1/teapot"} {
		rec := e.get(p, bearer(token(1)))
		if rec.Header().Get("RateLimit-Limit") == "" || rec.Header().Get("RateLimit-Remaining") == "" || rec.Header().Get("RateLimit-Reset") == "" {
			t.Fatalf("%s %d lacks RateLimit-*: %v", p, rec.Code, rec.Header())
		}
	}
}

func TestEarlyRejectionsSpendNoTokens(t *testing.T) {
	for name, res := range map[string]*fakeResolver{"401": noRow(), "403 lost membership": returns(suspended()), "403 no company": returns(noCompany())} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, res, limits(1, 1, time.Hour, time.Hour))
			for i := 0; i < 5; i++ {
				rec := e.get("/v1/items", bearer(token(1)))
				if rec.Code != 401 && rec.Code != 403 {
					t.Fatalf("status %d", rec.Code)
				}
				noQuota(t, rec)
			}
			if e.routes.Len() != 0 {
				t.Fatalf("%d buckets created by rejected requests", e.routes.Len())
			}
		})
	}
}

func TestSweepDropsFullBucketsOfBothLimiters(t *testing.T) {
	e := newEnv(t, sessions(map[string]auth.SessionRecord{token(1): sibling(1, 2), token(2): sibling(11, 12)}), limits(3, 5, time.Second, time.Second))
	e.get("/v1/items", bearer(token(1)))
	e.get("/v1/items", bearer(token(2)))
	if e.routes.Len() != 3 { // dos sesiones y un tenant
		t.Fatalf("Len = %d, want 3", e.routes.Len())
	}
	if n := e.routes.Sweep(); n != 0 {
		t.Fatalf("Sweep dropped %d buckets that were not full", n)
	}
	e.clk.Advance(6 * time.Second) // más que Capacity x Refill del tenant (5 s)
	if n := e.routes.Sweep(); n != 3 || e.routes.Len() != 0 {
		t.Fatalf("Sweep = %d, Len = %d, want 3 and 0", n, e.routes.Len())
	}
}

func TestDefaultKeyCaps(t *testing.T) {
	if testing.Short() {
		t.Skip("sends 60 000 requests")
	}
	var mu sync.Mutex
	n := 0
	res := &fakeResolver{lookup: func(context.Context, [32]byte) (auth.SessionRecord, bool, error) {
		mu.Lock()
		defer mu.Unlock()
		n++
		r := bound()
		r.SessionID = pgtype.UUID{Bytes: [16]byte{13: byte(n >> 16), 14: byte(n >> 8), 15: byte(n)}, Valid: true}
		r.TenantID = pgtype.UUID{Bytes: [16]byte{12: 1, 13: byte(n >> 16), 14: byte(n >> 8), 15: byte(n)}, Valid: true}
		return r, true, nil
	}}
	for name, tc := range map[string]struct {
		path string
		max  int
	}{"tenants": {"/v1/items", 10000}, "sessions": {"/v1/me", 50000}} {
		t.Run(name, func(t *testing.T) {
			n = 0
			e := newEnv(t, res, nil)
			for i := 1; i <= tc.max; i++ {
				if rec := e.get(tc.path, bearer(token(1))); rec.Code != 200 {
					t.Fatalf("key %d of %d: %d", i, tc.max, rec.Code)
				}
			}
			rec := e.get(tc.path, bearer(token(1)))
			mustProblem(t, rec, 503, "overloaded")
			noQuota(t, rec)
		})
	}
}
