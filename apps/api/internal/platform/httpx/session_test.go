package httpx_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

func uid(b byte) pgtype.UUID { return pgtype.UUID{Bytes: [16]byte{15: b}, Valid: true} }

// token arma un token de acceso válido cuyo cuerpo depende de b.
func token(b byte) string {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = b + byte(i)
	}
	return "wk_at_" + base64.RawURLEncoding.EncodeToString(raw)
}

// fakeResolver es el doble de auth.SessionResolver: responde lo que dicen sus
// funciones y recuerda las llamadas.
type fakeResolver struct {
	mu       sync.Mutex
	lookup   func(ctx context.Context, h auth.TokenHash) (auth.Session, error)
	unbind   func(ctx context.Context, sid, mid pgtype.UUID) error
	hashes   []auth.TokenHash
	unbinds  [][2]pgtype.UUID
	lookupFn int
}

func (f *fakeResolver) Lookup(ctx context.Context, h auth.TokenHash) (auth.Session, error) {
	f.mu.Lock()
	f.hashes = append(f.hashes, h)
	f.mu.Unlock()
	return f.lookup(ctx, h)
}

func (f *fakeResolver) Unbind(ctx context.Context, sid, mid pgtype.UUID) error {
	f.mu.Lock()
	f.unbinds = append(f.unbinds, [2]pgtype.UUID{sid, mid})
	f.mu.Unlock()
	if f.unbind == nil {
		return nil
	}
	return f.unbind(ctx, sid, mid)
}

func (f *fakeResolver) lookups() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.hashes)
}

func (f *fakeResolver) unbound() [][2]pgtype.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][2]pgtype.UUID(nil), f.unbinds...)
}

var _ auth.SessionResolver = (*fakeResolver)(nil)

func ver(n int64) *int64 { return &n }

// bound es una sesión ligada a la empresa 3, con membresía activa.
func bound() auth.Session {
	return auth.Session{ID: uid(1), UserID: uid(2), TenantID: uid(3), SiteID: uid(5), CashRegisterID: uid(6), MembershipID: uid(4),
		AuthzVersion: ver(1), MFASatisfied: true, MFAMethod: "none", CurrentAuthzVersion: ver(1), MembershipStatus: auth.MembershipActive}
}

func returns(s auth.Session) *fakeResolver {
	return &fakeResolver{lookup: func(context.Context, auth.TokenHash) (auth.Session, error) { return s, nil }}
}

func fails(err error) *fakeResolver {
	return &fakeResolver{lookup: func(context.Context, auth.TokenHash) (auth.Session, error) { return auth.Session{}, err }}
}

type sessionEnv struct {
	h       http.Handler
	log     *logBuf
	metrics *fakeMetrics
	res     *fakeResolver
	clk     *clock.Fake
	// reached cuenta las veces que corrió un handler.
	mu      sync.Mutex
	reached map[string]int
	seen    map[string]auth.Session
}

func (e *sessionEnv) hits(route string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.reached[route]
}

// newSessionEnv monta tres rutas: una pública, una sin tenant y una de tenant.
func newSessionEnv(t *testing.T, res *fakeResolver, mod func(*httpx.SessionOptions)) *sessionEnv {
	t.Helper()
	clk := clock.NewFake(t0)
	so := &httpx.SessionOptions{
		Resolver:       res,
		SessionLimiter: mustDefault(t, httpx.NewSessionLimiter(clk)),
		TenantLimiter:  mustDefault(t, httpx.NewTenantLimiter(clk)),
	}
	if mod != nil {
		mod(so)
	}
	e := &sessionEnv{res: res, clk: clk, reached: map[string]int{}, seen: map[string]auth.Session{}}
	record := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			e.mu.Lock()
			e.reached[name]++
			if s, ok := auth.SessionFrom(r.Context()); ok {
				e.seen[name] = s
			}
			e.mu.Unlock()
			_, _ = io.WriteString(w, "ok")
		}
	}
	e.h, e.log, e.metrics = chainWith(t, httpx.RouterOptions{Session: so}, func(rt *httpx.Router) {
		rt.HandleFunc("POST /v1/auth/login", record("public"), httpx.Public())
		rt.HandleFunc("GET /v1/me", record("notenant"), httpx.NoTenant())
		rt.HandleFunc("GET /v1/items", record("tenant"), httpx.Tenant())
		rt.HandleFunc("GET /v1/other", record("tenant2"), httpx.Tenant())
		rt.HandleFunc("GET /healthz", record("health"))
	})
	return e
}

func mustDefault(t *testing.T, l *httpx.Limiter) *httpx.Limiter {
	t.Helper()
	if l == nil {
		t.Fatal("nil limiter")
	}
	return l
}

func (e *sessionEnv) get(path, authz string) *httptest.ResponseRecorder {
	method := http.MethodGet
	if path == "/v1/auth/login" {
		method = http.MethodPost
	}
	req := httptest.NewRequest(method, path, nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	return do(e.h, req)
}

func bearer(tok string) string { return "Bearer " + tok }

func wantUnauth(t *testing.T, rec *httptest.ResponseRecorder, code string) {
	t.Helper()
	mustProblem(t, rec, 401, code)
	if got := rec.Header().Values("WWW-Authenticate"); len(got) != 1 || got[0] != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q, want exactly [Bearer]", got)
	}
	noQuota(t, rec)
}

func noQuota(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	for _, h := range []string{"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset"} {
		if v := rec.Header().Get(h); v != "" {
			t.Fatalf("%s = %q on a response rejected before the limiter", h, v)
		}
	}
}

// --- token ---

func TestMalformedAuthorizationIs401UnauthenticatedWithoutTouchingTheDatabase(t *testing.T) {
	good := token(1)
	cases := map[string]string{
		"missing":           "",
		"other scheme":      "Basic " + good,
		"refresh as access": "Bearer " + strings.Replace(good, "wk_at_", "wk_rt_", 1),
		"platform token":    "Bearer " + strings.Replace(good, "wk_at_", "wka_at_", 1),
		"short":             "Bearer " + good[:len(good)-1],
		"two spaces":        "Bearer  " + good,
	}
	for name, authz := range cases {
		for _, path := range []string{"/v1/me", "/v1/items"} {
			t.Run(name+" "+path, func(t *testing.T) {
				e := newSessionEnv(t, returns(bound()), nil)
				rec := e.get(path, authz)
				wantUnauth(t, rec, "unauthenticated")
				if e.res.lookups() != 0 {
					t.Fatal("the database was consulted for a token that is not well formed")
				}
			})
		}
	}
}

func TestTwoAuthorizationHeadersAre401(t *testing.T) {
	e := newSessionEnv(t, returns(bound()), nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Add("Authorization", bearer(token(1)))
	req.Header.Add("Authorization", bearer(token(1)))
	wantUnauth(t, do(e.h, req), "unauthenticated")
	if e.res.lookups() != 0 {
		t.Fatal("lookup for two Authorization headers")
	}
}

func TestLookupReceivesSha256OfTheWholeTokenWithItsPrefix(t *testing.T) {
	tok := token(5)
	e := newSessionEnv(t, returns(bound()), nil)
	if rec := e.get("/v1/me", bearer(tok)); rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	if len(e.res.hashes) != 1 || e.res.hashes[0] != auth.TokenHash(sha256.Sum256([]byte(tok))) {
		t.Fatalf("hashes = %x", e.res.hashes)
	}
}

func TestBearerSchemeIsCaseInsensitive(t *testing.T) {
	e := newSessionEnv(t, returns(bound()), nil)
	if rec := e.get("/v1/me", "bEaReR "+token(1)); rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
}

// --- sin fila ---

func TestSessionNotFoundIs401SessionExpiredOnEveryRouteClass(t *testing.T) {
	for _, path := range []string{"/v1/me", "/v1/items"} {
		e := newSessionEnv(t, fails(auth.ErrSessionNotFound), nil)
		wantUnauth(t, e.get(path, bearer(token(1))), "session_expired")
		if e.hits("notenant")+e.hits("tenant") != 0 {
			t.Fatal("a handler ran without a session")
		}
	}
}

func TestSessionNotFoundWrappedIsStill401(t *testing.T) {
	e := newSessionEnv(t, fails(fmt.Errorf("looking up: %w", auth.ErrSessionNotFound)), nil)
	wantUnauth(t, e.get("/v1/items", bearer(token(1))), "session_expired")
}

// --- fallos de la base: nunca 401 ni 500 ---

func assertDependencyDown(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	p := mustProblem(t, rec, 503, "dependency_unavailable")
	if rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	if rec.Header().Get("WWW-Authenticate") != "" {
		t.Fatal("a 503 must not ask the client to authenticate")
	}
	noQuota(t, rec)
	_ = p
}

func TestLookupDatabaseFailuresAre503NeverAuthErrors(t *testing.T) {
	pg := &pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout"}
	cases := map[string]struct {
		err      error
		wantCode string
		counter  string
	}{
		"unavailable":       {fmt.Errorf("%w: %w", db.ErrUnavailable, errors.New("conn refused")), "dependency_unavailable", "db_unavailable_total{phase,session_lookup,route,GET /v1/items}"},
		"statement timeout": {fmt.Errorf("%w: %w", db.ErrStatementTimeout, pg), "dependency_unavailable", "db_statement_timeouts_total{phase,session_lookup,route,GET /v1/items}"},
		"pool exhausted":    {fmt.Errorf("acquiring: %w", db.ErrOverloaded), "overloaded", ""},
		"unknown error":     {errors.New("scan: unexpected column type"), "dependency_unavailable", "db_unavailable_total{phase,session_lookup,route,GET /v1/items}"},
	}
	for name, tc := range cases {
		for _, path := range []string{"/v1/me", "/v1/items"} {
			t.Run(name+" "+path, func(t *testing.T) {
				e := newSessionEnv(t, fails(tc.err), nil)
				rec := e.get(path, bearer(token(1)))
				if tc.wantCode == "overloaded" {
					mustProblem(t, rec, 503, "overloaded")
				} else {
					assertDependencyDown(t, rec)
				}
				if rec.Header().Get("Retry-After") != "1" {
					t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
				}
				if e.hits("notenant")+e.hits("tenant") != 0 {
					t.Fatal("a handler ran after a failed lookup")
				}
				if tc.counter != "" && path == "/v1/items" && e.metrics.get(tc.counter) != 1 {
					t.Fatalf("counter %s = %d; counts %v", tc.counter, e.metrics.get(tc.counter), e.metrics.counts)
				}
			})
		}
	}
}

func TestStatementTimeoutInLookupLogsOneWarnWithPhaseAndNoSQL(t *testing.T) {
	pg := &pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout: SELECT secret_column"}
	e := newSessionEnv(t, fails(fmt.Errorf("%w: %w", db.ErrStatementTimeout, pg)), nil)
	rec := e.get("/v1/items", bearer(token(1)))
	assertDependencyDown(t, rec)
	lines := e.log.lines(t, "statement timeout")
	if len(lines) != 1 {
		t.Fatalf("warn lines = %d; log %s", len(lines), e.log.String())
	}
	l := lines[0]
	if l["level"] != "WARN" || l["phase"] != "session_lookup" || l["sqlstate"] != "57014" || l["route"] != "GET /v1/items" || l["requestId"] == nil && l["request_id"] == nil {
		t.Fatalf("log line = %v", l)
	}
	if strings.Contains(e.log.String(), "secret_column") || strings.Contains(rec.Body.String(), "57014") {
		t.Fatal("PostgreSQL text leaked")
	}
}

func TestLookupDeadlineOfOneSecondIs503WhileTheRequestIsAlive(t *testing.T) {
	block := &fakeResolver{lookup: func(ctx context.Context, _ auth.TokenHash) (auth.Session, error) {
		<-ctx.Done()
		return auth.Session{}, ctx.Err()
	}}
	for _, path := range []string{"/v1/me", "/v1/items"} {
		e := newSessionEnv(t, block, func(o *httpx.SessionOptions) { o.LookupTimeout = 30 * time.Millisecond })
		start := time.Now()
		rec := e.get(path, bearer(token(1)))
		assertDependencyDown(t, rec)
		if d := time.Since(start); d > 2*time.Second {
			t.Fatalf("took %v", d)
		}
		if e.hits("notenant")+e.hits("tenant") != 0 {
			t.Fatal("handler ran")
		}
	}
	e := newSessionEnv(t, block, func(o *httpx.SessionOptions) { o.LookupTimeout = 30 * time.Millisecond })
	e.get("/v1/items", bearer(token(1)))
	if n := e.metrics.get("db_unavailable_total{phase,session_lookup,route,GET /v1/items}"); n != 1 {
		t.Fatalf("counter = %d; %v", n, e.metrics.counts)
	}
	if lines := e.log.lines(t, "session lookup timed out"); len(lines) != 1 || lines[0]["phase"] != "session_lookup" {
		t.Fatalf("log = %s", e.log.String())
	}
}

func TestLookupDefaultDeadlineIsOneSecond(t *testing.T) {
	var got time.Duration
	res := &fakeResolver{lookup: func(ctx context.Context, _ auth.TokenHash) (auth.Session, error) {
		dl, ok := ctx.Deadline()
		if !ok {
			t.Error("lookup context has no deadline")
		}
		got = time.Until(dl)
		return bound(), nil
	}}
	e := newSessionEnv(t, res, nil)
	e.get("/v1/me", bearer(token(1)))
	if got <= 900*time.Millisecond || got > time.Second {
		t.Fatalf("lookup deadline in %v, want about 1s", got)
	}
}

func TestLookupFailureWithTheRequestAlreadyCancelledWritesNothing(t *testing.T) {
	for name, err := range map[string]error{
		"database error": fmt.Errorf("%w: %w", db.ErrUnavailable, context.Canceled),
		"unknown error":  errors.New("scan failed"),
		"context error":  context.Canceled,
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			res := &fakeResolver{lookup: func(context.Context, auth.TokenHash) (auth.Session, error) {
				cancel()
				return auth.Session{}, err
			}}
			e := newSessionEnv(t, res, nil)
			req := httptest.NewRequest(http.MethodGet, "/v1/items", nil).WithContext(ctx)
			req.Header.Set("Authorization", bearer(token(1)))
			rec := do(e.h, req)
			if rec.Body.Len() != 0 {
				t.Fatalf("wrote %q", rec.Body.String())
			}
			for _, msg := range []string{"database unavailable", "session lookup failed", "session lookup timed out"} {
				if len(e.log.lines(t, msg)) != 0 {
					t.Fatalf("logged %q for a client that left: %s", msg, e.log.String())
				}
			}
		})
	}
}

// --- membresía perdida (ADR 0011) ---

func TestLostMembershipUnbindsWithoutRevokingAndAnswersByRouteClass(t *testing.T) {
	lost := map[string]auth.Session{}
	deleted := bound()
	deleted.CurrentAuthzVersion, deleted.MembershipStatus = nil, ""
	lost["deleted"] = deleted
	suspended := bound()
	suspended.MembershipStatus = "suspendido"
	lost["suspended"] = suspended
	noVersion := bound()
	noVersion.CurrentAuthzVersion = nil // versión ausente con estado activo: nunca se trata como "sin cambios"
	lost["no current version"] = noVersion
	for name, s := range lost {
		t.Run(name+"/tenant route", func(t *testing.T) {
			e := newSessionEnv(t, returns(s), nil)
			rec := e.get("/v1/items", bearer(token(1)))
			mustProblem(t, rec, 403, "tenant_access_denied")
			noQuota(t, rec)
			if rec.Header().Get("WWW-Authenticate") != "" {
				t.Fatal("a 403 must not carry WWW-Authenticate")
			}
			if u := e.res.unbound(); len(u) != 1 || u[0] != [2]pgtype.UUID{uid(1), uid(4)} {
				t.Fatalf("Unbind calls = %v", u)
			}
			if e.hits("tenant") != 0 {
				t.Fatal("handler ran")
			}
		})
		t.Run(name+"/no-tenant route", func(t *testing.T) {
			e := newSessionEnv(t, returns(s), nil)
			rec := e.get("/v1/me", bearer(token(1)))
			if rec.Code != 200 || e.hits("notenant") != 1 {
				t.Fatalf("status %d", rec.Code)
			}
			if got := e.seen["notenant"]; got.HasTenant() || got.MembershipID.Valid || got.SiteID.Valid || got.CashRegisterID.Valid || got.AuthzVersion != nil {
				t.Fatalf("handler sees tenant columns: %+v", got)
			}
			if got := e.seen["notenant"]; got.ID != uid(1) || got.UserID != uid(2) {
				t.Fatalf("handler lost the session identity: %+v", got)
			}
			if len(e.res.unbound()) != 1 {
				t.Fatalf("Unbind calls = %v", e.res.unbound())
			}
		})
	}
}

func TestLostMembershipTenantRouteNeverRevealsTheCompany(t *testing.T) {
	s := bound()
	s.MembershipStatus = "suspendido"
	e := newSessionEnv(t, returns(s), nil)
	body := e.get("/v1/items", bearer(token(1))).Body.String()
	for _, secret := range []string{uid(3).String(), "suspendido", "morosa", "402"} {
		if strings.Contains(body, secret) {
			t.Fatalf("403 body leaks %q: %s", secret, body)
		}
	}
}

func TestUnbindFailureDoesNotChangeTheDecision(t *testing.T) {
	s := bound()
	s.MembershipStatus = "suspendido"
	res := returns(s)
	res.unbind = func(context.Context, pgtype.UUID, pgtype.UUID) error {
		return fmt.Errorf("%w: dial tcp 10.1.2.3:5432", db.ErrUnavailable)
	}
	e := newSessionEnv(t, res, nil)
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 403, "tenant_access_denied")
	if rec := e.get("/v1/me", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("no-tenant route status %d", rec.Code)
	}
	if got := e.seen["notenant"]; got.HasTenant() {
		t.Fatal("handler kept the tenant after a failed UPDATE")
	}
	if len(e.log.lines(t, "session unbind failed")) != 2 {
		t.Fatalf("log = %s", e.log.String())
	}
	if strings.Contains(e.log.String(), "10.1.2.3") {
		t.Fatal("error text in the log")
	}
}

func TestUnbindIsSkippedWhenNothingWasLost(t *testing.T) {
	neverChose := bound().WithoutTenant()
	imp := auth.Session{ID: uid(1), UserID: uid(2), TenantID: uid(3), ImpersonatorID: uid(9), MFASatisfied: true, MFAMethod: "platform"}
	impWithMembership := imp
	impWithMembership.MembershipID = uid(4)
	for name, s := range map[string]auth.Session{"active": bound(), "never chose a company": neverChose, "impersonated": imp, "impersonated with a stale membership": impWithMembership} {
		t.Run(name, func(t *testing.T) {
			e := newSessionEnv(t, returns(s), nil)
			e.get("/v1/me", bearer(token(1)))
			e.get("/v1/items", bearer(token(1)))
			if u := e.res.unbound(); len(u) != 0 {
				t.Fatalf("Unbind called: %v", u)
			}
		})
	}
}

func TestSessionWithoutCompanyOnATenantRouteIs403AndNoTenantRouteIs200(t *testing.T) {
	e := newSessionEnv(t, returns(bound().WithoutTenant()), nil)
	rec := e.get("/v1/items", bearer(token(1)))
	mustProblem(t, rec, 403, "tenant_access_denied")
	noQuota(t, rec)
	if rec := e.get("/v1/me", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestBoundSessionReachesTheHandlerWithItsActor(t *testing.T) {
	e := newSessionEnv(t, returns(bound()), nil)
	if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	s := e.seen["tenant"]
	a := s.Actor()
	if a.TenantID != uid(3) || a.UserID != uid(2) || a.Kind != db.KindUser || s.SiteID != uid(5) || s.CashRegisterID != uid(6) {
		t.Fatalf("session = %+v", s)
	}
}

func TestImpersonatedSessionPassesATenantRoute(t *testing.T) {
	imp := auth.Session{ID: uid(1), UserID: uid(2), TenantID: uid(3), ImpersonatorID: uid(9), MFASatisfied: true, MFAMethod: "platform"}
	e := newSessionEnv(t, returns(imp), nil)
	if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if a := e.seen["tenant"].Actor(); a.Kind != db.KindImpersonated || a.ImpersonatorID != uid(9) {
		t.Fatalf("actor = %+v", a)
	}
}

// --- rutas públicas y fuera de /v1 ---

func TestPublicAndUnclassifiedRoutesNeverLookUpASession(t *testing.T) {
	e := newSessionEnv(t, fails(errors.New("must not be called")), nil)
	for _, authz := range []string{"", bearer(token(1)), "garbage"} {
		if rec := e.get("/v1/auth/login", authz); rec.Code != 200 {
			t.Fatalf("public status %d", rec.Code)
		}
		if rec := e.get("/healthz", authz); rec.Code != 200 {
			t.Fatalf("health status %d", rec.Code)
		}
	}
	if e.res.lookups() != 0 {
		t.Fatalf("lookups = %d", e.res.lookups())
	}
	if _, ok := e.seen["public"]; ok {
		t.Fatal("a public handler received a session")
	}
}

// --- el token y su hash no se registran ---

func TestNoLogLineCarriesTheTokenOrItsHash(t *testing.T) {
	tok := token(11)
	h := sha256.Sum256([]byte(tok))
	body := strings.TrimPrefix(tok, "wk_at_")
	forms := []string{tok, body, hex.EncodeToString(h[:]), strings.ToUpper(hex.EncodeToString(h[:])),
		base64.StdEncoding.EncodeToString(h[:]), base64.RawStdEncoding.EncodeToString(h[:]),
		base64.URLEncoding.EncodeToString(h[:]), base64.RawURLEncoding.EncodeToString(h[:]), fmt.Sprint(h), fmt.Sprintf("%v", h[:])}
	lost := bound()
	lost.MembershipStatus = "suspendido"
	pg := &pgconn.PgError{Code: "57014", Message: "canceling statement"}
	scenarios := map[string]*fakeResolver{
		"ok": returns(bound()), "not found": fails(auth.ErrSessionNotFound),
		"unavailable":       fails(fmt.Errorf("%w: boom", db.ErrUnavailable)),
		"statement timeout": fails(fmt.Errorf("%w: %w", db.ErrStatementTimeout, pg)),
		"unknown error":     fails(errors.New("unexpected")),
		"overloaded":        fails(db.ErrOverloaded),
		"unbind fails": func() *fakeResolver {
			r := returns(lost)
			r.unbind = func(context.Context, pgtype.UUID, pgtype.UUID) error { return errors.New("update failed") }
			return r
		}(),
	}
	for name, res := range scenarios {
		t.Run(name, func(t *testing.T) {
			e := newSessionEnv(t, res, nil)
			for _, p := range []string{"/v1/me", "/v1/items"} {
				e.get(p, bearer(tok))
			}
			// También una petición malformada que repite el token en otra forma.
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

// --- límite de peticiones ---

func rateEnv(t *testing.T, sessCap, tenCap int, res *fakeResolver) *sessionEnv {
	t.Helper()
	return newSessionEnv(t, res, func(o *httpx.SessionOptions) {
		o.SessionLimiter = newLimiter(t, clock.NewFake(t0), httpx.LimiterConfig{Name: "session", Capacity: sessCap, Refill: time.Second, MaxKeys: 10, Key: sessionKey})
		o.TenantLimiter = newLimiter(t, clock.NewFake(t0), httpx.LimiterConfig{Name: "tenant", Capacity: tenCap, Refill: time.Second, MaxKeys: 10, Key: tenantKey})
	})
}

func sessionKey(r *http.Request) (string, bool) {
	s, ok := auth.SessionFrom(r.Context())
	return s.ID.String(), ok
}

func tenantKey(r *http.Request) (string, bool) {
	s, ok := auth.SessionFrom(r.Context())
	return s.TenantID.String(), ok
}

func TestRateLimitedSessionIs429OnAnyRouteAndTheTenantKeepsItsToken(t *testing.T) {
	// Dos sesiones (A y B) de la misma empresa. A gasta su bucket de 3.
	sessions := map[string]auth.Session{}
	a, b := bound(), bound()
	b.ID, b.UserID = uid(11), uid(12)
	sessions[token(1)], sessions[token(2)] = a, b
	res := &fakeResolver{lookup: func(_ context.Context, h auth.TokenHash) (auth.Session, error) {
		for tok, s := range sessions {
			if h == auth.TokenHash(sha256.Sum256([]byte(tok))) {
				return s, nil
			}
		}
		return auth.Session{}, auth.ErrSessionNotFound
	}}
	e := rateEnv(t, 3, 5, res)
	for i := 0; i < 3; i++ {
		if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 {
			t.Fatalf("request %d: %d", i, rec.Code)
		}
	}
	rec := e.get("/v1/other", bearer(token(1)))
	mustProblem(t, rec, 429, "rate_limited")
	if ra := rec.Header().Get("Retry-After"); ra != "1" {
		t.Fatalf("Retry-After = %q", ra)
	}
	// RateLimit-* es el del bucket de menos fichas: el de la sesión, sin fichas.
	if rec.Header().Get("RateLimit-Limit") != "3" || rec.Header().Get("RateLimit-Remaining") != "0" {
		t.Fatalf("quota headers: %v", rec.Header())
	}
	if e.hits("tenant2") != 0 {
		t.Fatal("handler ran")
	}
	// La sesión B (misma empresa): el tenant gastó 3 de 5, la 429 de A no le quitó una cuarta.
	recB := e.get("/v1/items", bearer(token(2)))
	if recB.Code != 200 {
		t.Fatalf("B status %d", recB.Code)
	}
	if got := recB.Header().Get("RateLimit-Remaining"); got != "1" || recB.Header().Get("RateLimit-Limit") != "5" {
		t.Fatalf("B quota %s/%s, want 1 of 5 (tenant bucket)", got, recB.Header().Get("RateLimit-Limit"))
	}
}

func TestNoTenantRoutesUseOnlyTheSessionBucket(t *testing.T) {
	e := rateEnv(t, 2, 1, returns(bound()))
	for i := 0; i < 2; i++ {
		rec := e.get("/v1/me", bearer(token(1)))
		if rec.Code != 200 || rec.Header().Get("RateLimit-Limit") != "2" {
			t.Fatalf("request %d: %d limit %q", i, rec.Code, rec.Header().Get("RateLimit-Limit"))
		}
	}
	mustProblem(t, e.get("/v1/me", bearer(token(1))), 429, "rate_limited")
	// El bucket del tenant (1 ficha) sigue lleno: nadie lo tocó.
	e2 := rateEnv(t, 5, 1, returns(bound()))
	e2.get("/v1/me", bearer(token(1)))
	e2.get("/v1/me", bearer(token(1)))
	if rec := e2.get("/v1/items", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("tenant bucket was charged by no-tenant routes: %d", rec.Code)
	}
}

func TestTenantBucketLimitsTenantRoutesAcrossSessions(t *testing.T) {
	n := byte(0)
	var mu sync.Mutex
	res := &fakeResolver{lookup: func(context.Context, auth.TokenHash) (auth.Session, error) {
		mu.Lock()
		defer mu.Unlock()
		n++
		s := bound()
		s.ID = uid(100 + n) // cada petición, otra sesión de la misma empresa
		return s, nil
	}}
	e := rateEnv(t, 5, 2, res)
	for i := 0; i < 2; i++ {
		if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 {
			t.Fatalf("request %d: %d", i, rec.Code)
		}
	}
	rec := e.get("/v1/items", bearer(token(1)))
	mustProblem(t, rec, 429, "rate_limited")
	if rec.Header().Get("RateLimit-Limit") != "2" {
		t.Fatalf("headers %v", rec.Header())
	}
}

func TestRejectionsBeforeTheLimiterSpendNoTokens(t *testing.T) {
	lostS := bound()
	lostS.MembershipStatus = "suspendido"
	for name, res := range map[string]*fakeResolver{"401 not found": fails(auth.ErrSessionNotFound), "403 lost membership": returns(lostS), "403 no company": returns(bound().WithoutTenant())} {
		t.Run(name, func(t *testing.T) {
			e := rateEnv(t, 1, 1, res)
			for i := 0; i < 5; i++ {
				rec := e.get("/v1/items", bearer(token(1)))
				if rec.Code != 401 && rec.Code != 403 {
					t.Fatalf("status %d", rec.Code)
				}
				noQuota(t, rec)
			}
		})
	}
	// Sesión de verdad: tras 5 rechazos de otra sesión sin empresa, el bucket del tenant 3 sigue lleno.
	cur := returns(bound().WithoutTenant())
	e := rateEnv(t, 1, 1, cur)
	for i := 0; i < 3; i++ {
		e.get("/v1/items", bearer(token(1)))
	}
	cur.lookup = func(context.Context, auth.TokenHash) (auth.Session, error) { return bound(), nil }
	if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 {
		t.Fatalf("status %d: the rejected requests spent tokens", rec.Code)
	}
}

// El orden de la secuencia 08: el bucket de la sesión va antes que el del tenant.
// Se nota cuando el del tenant ha llegado a su tope de claves: una sesión sin
// fichas recibe 429 (no 503) porque el limitador del tenant no llega a mirarla.
func TestSessionBucketRunsBeforeTheTenantBucket(t *testing.T) {
	clk := clock.NewFake(t0)
	tenantL := newLimiter(t, clk, httpx.LimiterConfig{Name: "tenant", Capacity: 5, Refill: time.Second, MaxKeys: 1, Key: tenantKey})
	other := bound()
	other.ID, other.TenantID = uid(21), uid(22)
	cur := returns(bound())
	e := newSessionEnv(t, cur, func(o *httpx.SessionOptions) {
		o.SessionLimiter = newLimiter(t, clk, httpx.LimiterConfig{Name: "session", Capacity: 1, Refill: time.Second, MaxKeys: 10, Key: sessionKey})
		o.TenantLimiter = tenantL
	})
	e.get("/v1/me", bearer(token(1))) // gasta la única ficha de la sesión 1 sin tocar el tenant
	cur.lookup = func(context.Context, auth.TokenHash) (auth.Session, error) { return other, nil }
	if rec := e.get("/v1/items", bearer(token(2))); rec.Code != 200 { // otra empresa ocupa la única clave del tenant
		t.Fatalf("status %d", rec.Code)
	}
	cur.lookup = func(context.Context, auth.TokenHash) (auth.Session, error) { return bound(), nil }
	mustProblem(t, e.get("/v1/items", bearer(token(1))), 429, "rate_limited")
}

func TestPublicRoutesCarryNoQuotaHeaders(t *testing.T) {
	e := rateEnv(t, 1, 1, returns(bound()))
	for i := 0; i < 3; i++ {
		rec := e.get("/v1/auth/login", "")
		noQuota(t, rec)
		if rec.Code != 200 {
			t.Fatalf("status %d", rec.Code)
		}
	}
}

func TestDefaultLimitsAreTheContractOnes(t *testing.T) {
	cases := map[string]struct {
		mk      func(clock.Clock) *httpx.Limiter
		burst   int
		refill  time.Duration
		maxKeys int
	}{
		"session": {httpx.NewSessionLimiter, 20, 100 * time.Millisecond, 50000},
		"tenant":  {httpx.NewTenantLimiter, 200, 10 * time.Millisecond, 10000},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			clk := clock.NewFake(t0)
			s := bound()
			res := returns(s)
			e := newSessionEnv(t, res, func(o *httpx.SessionOptions) {
				if name == "session" {
					o.SessionLimiter = tc.mk(clk)
					o.TenantLimiter = newLimiter(t, clk, httpx.LimiterConfig{Name: "t", Capacity: 100000, Refill: time.Nanosecond, MaxKeys: 10, Key: tenantKey})
				} else {
					o.TenantLimiter = tc.mk(clk)
					o.SessionLimiter = newLimiter(t, clk, httpx.LimiterConfig{Name: "s", Capacity: 100000, Refill: time.Nanosecond, MaxKeys: 10, Key: sessionKey})
				}
			})
			for i := 0; i < tc.burst; i++ {
				if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 {
					t.Fatalf("request %d of the burst: %d", i, rec.Code)
				}
			}
			mustProblem(t, e.get("/v1/items", bearer(token(1))), 429, "rate_limited")
			clk.Advance(tc.refill)
			if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 200 {
				t.Fatalf("after one refill interval: %d", rec.Code)
			}
			if rec := e.get("/v1/items", bearer(token(1))); rec.Code != 429 {
				t.Fatalf("a refill interval gives more than one token: %d", rec.Code)
			}
		})
	}
}

func TestDefaultKeyCapsAre50000SessionsAnd10000Tenants(t *testing.T) {
	// Se prueba el limitador solo, con una sesión distinta por petición: ahorra
	// el resto de la cadena, que ya cubren los demás tests.
	pass := func(l *httpx.Limiter, i int) *httptest.ResponseRecorder {
		s := bound()
		s.ID = pgtype.UUID{Bytes: [16]byte{13: byte(i >> 16), 14: byte(i >> 8), 15: byte(i)}, Valid: true}
		s.TenantID = pgtype.UUID{Bytes: [16]byte{12: 1, 13: byte(i >> 16), 14: byte(i >> 8), 15: byte(i)}, Valid: true}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(auth.WithSession(req.Context(), s))
		rec := httptest.NewRecorder()
		l.Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)
		return rec
	}
	for name, tc := range map[string]struct {
		l   *httpx.Limiter
		max int
	}{"session": {httpx.NewSessionLimiter(clock.NewFake(t0)), 50000}, "tenant": {httpx.NewTenantLimiter(clock.NewFake(t0)), 10000}} {
		t.Run(name, func(t *testing.T) {
			for i := 1; i <= tc.max; i++ {
				if rec := pass(tc.l, i); rec.Code != 200 {
					t.Fatalf("key %d of %d: %d", i, tc.max, rec.Code)
				}
			}
			if rec := pass(tc.l, tc.max+1); rec.Code != 503 {
				t.Fatalf("key %d: %d, want 503 overloaded", tc.max+1, rec.Code)
			}
		})
	}
}

// --- WWW-Authenticate ---

func TestWWWAuthenticateOnlyOnTheTwoKernel401Codes(t *testing.T) {
	for code, want := range map[string]string{"unauthenticated": "Bearer", "session_expired": "Bearer", "invalid_credentials": "", "forbidden": "", "internal": ""} {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		rec := httptest.NewRecorder()
		httpx.WriteProblem(rec, req, code, "")
		if got := rec.Header().Get("WWW-Authenticate"); got != want {
			t.Errorf("%s: WWW-Authenticate = %q, want %q", code, got, want)
		}
	}
}

// --- clase de ruta ---

func TestRouteClassRulesPanicAtRegistration(t *testing.T) {
	handler := func(http.ResponseWriter, *http.Request) {}
	type tc struct {
		name    string
		pattern string
		opts    []httpx.RouteOption
		ok      bool
	}
	cases := []tc{
		{"/v1 route without class", "GET /v1/items", nil, false},
		{"/v1 root without class", "GET /v1", nil, false},
		{"tenant route", "GET /v1/items", []httpx.RouteOption{httpx.Tenant()}, true},
		{"tenant route with wildcard", "GET /v1/items/{id}", []httpx.RouteOption{httpx.Tenant()}, true},
		{"public under auth", "POST /v1/auth/login", []httpx.RouteOption{httpx.Public()}, true},
		{"public with wildcard under auth", "POST /v1/auth/password/{step}", []httpx.RouteOption{httpx.Public()}, true},
		{"public outside auth", "GET /v1/items", []httpx.RouteOption{httpx.Public()}, false},
		{"public on /v1/me", "GET /v1/me", []httpx.RouteOption{httpx.Public()}, false},
		{"public on a look-alike prefix", "GET /v1/authx/login", []httpx.RouteOption{httpx.Public()}, false},
		{"no-tenant under auth", "POST /v1/auth/logout", []httpx.RouteOption{httpx.NoTenant()}, true},
		{"no-tenant session/tenant", "POST /v1/auth/session/tenant", []httpx.RouteOption{httpx.NoTenant()}, true},
		{"no-tenant GET /v1/me", "GET /v1/me", []httpx.RouteOption{httpx.NoTenant()}, true},
		{"no-tenant GET /v1/me/memberships", "GET /v1/me/memberships", []httpx.RouteOption{httpx.NoTenant()}, true},
		{"no-tenant PATCH /v1/me", "PATCH /v1/me", []httpx.RouteOption{httpx.NoTenant()}, false},
		{"no-tenant under /v1/me", "GET /v1/me/other", []httpx.RouteOption{httpx.NoTenant()}, false},
		{"no-tenant on a tenant resource", "GET /v1/items", []httpx.RouteOption{httpx.NoTenant()}, false},
		{"tenant under auth", "POST /v1/auth/login", []httpx.RouteOption{httpx.Tenant()}, false},
		{"tenant on GET /v1/me", "GET /v1/me", []httpx.RouteOption{httpx.Tenant()}, false},
		{"tenant on GET /v1/me/memberships", "GET /v1/me/memberships", []httpx.RouteOption{httpx.Tenant()}, false},
		{"tenant on PATCH /v1/me", "PATCH /v1/me", []httpx.RouteOption{httpx.Tenant()}, true},
		{"two classes", "GET /v1/items", []httpx.RouteOption{httpx.Tenant(), httpx.NoTenant()}, false},
		{"the same class twice", "GET /v1/items", []httpx.RouteOption{httpx.Tenant(), httpx.Tenant()}, false},
		{"class outside /v1", "GET /healthz", []httpx.RouteOption{httpx.Tenant()}, false},
		{"no class outside /v1", "GET /healthz", nil, true},
		{"/v1x is not /v1", "GET /v1x/a", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rt := httpx.NewRouter(httpx.RouterOptions{Session: &httpx.SessionOptions{Resolver: returns(bound())}})
			defer func() {
				if r := recover(); (r == nil) != c.ok {
					t.Fatalf("panic = %v, want panic: %v", r, !c.ok)
				}
			}()
			rt.HandleFunc(c.pattern, handler, c.opts...)
		})
	}
}

func TestClassifiedRouteNeedsASessionStep(t *testing.T) {
	rt := httpx.NewRouter(httpx.RouterOptions{})
	defer func() {
		if recover() == nil {
			t.Fatal("registering a classified route without a resolver did not panic")
		}
	}()
	rt.HandleFunc("GET /v1/items", func(http.ResponseWriter, *http.Request) {}, httpx.Tenant())
}

func TestNewRouterRejectsSessionOptionsWithoutResolver(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("no panic")
		}
	}()
	httpx.NewRouter(httpx.RouterOptions{Session: &httpx.SessionOptions{}})
}
