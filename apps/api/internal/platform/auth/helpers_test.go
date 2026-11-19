package auth_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

var t0 = time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

func uid(b byte) pgtype.UUID { return pgtype.UUID{Bytes: [16]byte{15: b}, Valid: true} }

func i8(n int64) pgtype.Int8 { return pgtype.Int8{Int64: n, Valid: true} }

// token arma un token de acceso válido cuyo cuerpo depende de b.
func token(b byte) string {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = b + byte(i)
	}
	return "wk_at_" + base64.RawURLEncoding.EncodeToString(raw)
}

func bearer(tok string) string { return "Bearer " + tok }

func hashOf(tok string) [32]byte { return sha256.Sum256([]byte(tok)) }

// bound es una fila de sesión ligada a la empresa 3, con membresía activa.
func bound() auth.SessionRecord {
	return auth.SessionRecord{SessionID: uid(1), UserID: uid(2), TenantID: uid(3), SiteID: uid(5), CashRegisterID: uid(6), MembershipID: uid(4),
		SessionAuthzVersion: i8(1), CurrentAuthzVersion: i8(1), MembershipStatus: pgtype.Text{String: auth.MembershipActive, Valid: true},
		MFASatisfied: true, MFAMethod: "none"}
}

func suspended() auth.SessionRecord {
	r := bound()
	r.MembershipStatus.String = "suspendido"
	return r
}

func noCompany() auth.SessionRecord {
	r := bound()
	r.TenantID, r.SiteID, r.CashRegisterID, r.MembershipID = pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}
	r.SessionAuthzVersion, r.CurrentAuthzVersion, r.MembershipStatus = pgtype.Int8{}, pgtype.Int8{}, pgtype.Text{}
	return r
}

func impersonated() auth.SessionRecord {
	r := noCompany()
	r.TenantID, r.UserID, r.ImpersonatorID, r.MFAMethod = uid(3), pgtype.UUID{}, uid(9), "platform"
	return r
}

// fakeResolver es el doble de auth.SessionResolver.
type fakeResolver struct {
	mu      sync.Mutex
	lookup  func(ctx context.Context, h [32]byte) (auth.SessionRecord, bool, error)
	detach  func(ctx context.Context, sid, mid pgtype.UUID) (bool, error)
	hashes  [][32]byte
	detachs [][2]pgtype.UUID
}

var _ auth.SessionResolver = (*fakeResolver)(nil)

func (f *fakeResolver) Lookup(ctx context.Context, h [32]byte) (auth.SessionRecord, bool, error) {
	f.mu.Lock()
	f.hashes = append(f.hashes, h)
	f.mu.Unlock()
	return f.lookup(ctx, h)
}

func (f *fakeResolver) Detach(ctx context.Context, sid, mid pgtype.UUID) (bool, error) {
	f.mu.Lock()
	f.detachs = append(f.detachs, [2]pgtype.UUID{sid, mid})
	f.mu.Unlock()
	if f.detach == nil {
		return true, nil
	}
	return f.detach(ctx, sid, mid)
}

func (f *fakeResolver) lookups() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.hashes)
}

func (f *fakeResolver) detaches() [][2]pgtype.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][2]pgtype.UUID(nil), f.detachs...)
}

func returns(r auth.SessionRecord) *fakeResolver {
	return &fakeResolver{lookup: func(context.Context, [32]byte) (auth.SessionRecord, bool, error) { return r, true, nil }}
}

func fails(err error) *fakeResolver {
	return &fakeResolver{lookup: func(context.Context, [32]byte) (auth.SessionRecord, bool, error) {
		return auth.SessionRecord{}, false, err
	}}
}

func noRow() *fakeResolver {
	return &fakeResolver{lookup: func(context.Context, [32]byte) (auth.SessionRecord, bool, error) {
		return auth.SessionRecord{}, false, nil
	}}
}

// logBuf es un bytes.Buffer seguro para las goroutines del servidor y del test.
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

func (l *logBuf) lines(t *testing.T, msg string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, ln := range strings.Split(strings.TrimSpace(l.String()), "\n") {
		if ln == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("log line is not JSON: %q", ln)
		}
		if m["msg"] == msg {
			out = append(out, m)
		}
	}
	return out
}

// fakeMetrics cuenta los eventos que recibe.
type fakeMetrics struct {
	mu     sync.Mutex
	counts map[string]int
}

func (f *fakeMetrics) Inc(name string, labels ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts == nil {
		f.counts = map[string]int{}
	}
	f.counts[name+"{"+strings.Join(labels, ",")+"}"]++
}

func (f *fakeMetrics) get(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[key]
}

type problem struct {
	Status    int    `json:"status"`
	Code      string `json:"code"`
	Detail    string `json:"detail"`
	RequestID string `json:"requestId"`
}

// mustProblem comprueba status, application/problem+json y code.
func mustProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) problem {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, status, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var p problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body is not JSON: %q", rec.Body.String())
	}
	if p.Code != code || p.Status != status || p.RequestID == "" || p.RequestID != rec.Header().Get("X-Request-Id") {
		t.Fatalf("problem = %+v, want code %s", p, code)
	}
	return p
}

type env struct {
	h       http.Handler
	log     *logBuf
	metrics *fakeMetrics
	res     *fakeResolver
	clk     *clock.Fake
	routes  *auth.Routes

	mu      sync.Mutex
	reached map[string]int
	seen    map[string]auth.Session
}

func (e *env) hits(name string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.reached[name]
}

func (e *env) session(name string) auth.Session {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.seen[name]
}

// newEnv monta las rutas de prueba sobre Routes y Chain.
func newEnv(t *testing.T, res *fakeResolver, mod func(*auth.Options)) *env {
	t.Helper()
	e := &env{res: res, clk: clock.NewFake(t0), log: &logBuf{}, metrics: &fakeMetrics{}, reached: map[string]int{}, seen: map[string]auth.Session{}}
	o := auth.Options{Clock: e.clk}
	if mod != nil {
		mod(&o)
	}
	rt := httpx.NewRouter(httpx.RouterOptions{})
	routes, err := auth.NewRoutes(rt, res, o)
	if err != nil {
		t.Fatal(err)
	}
	e.routes = routes
	record := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			e.mu.Lock()
			e.reached[name]++
			if s, ok := auth.FromContext(r.Context()); ok {
				e.seen[name] = s
			}
			e.mu.Unlock()
			_, _ = io.WriteString(w, "ok")
		}
	}
	routes.HandleFunc("POST /v1/auth/login", auth.Public, record("public"))
	routes.HandleFunc("POST /v1/auth/mfa/enrollment/totp", auth.Public, record("enroll-public"))
	routes.HandleFunc("POST /v1/auth/mfa/totp/enroll", auth.NoTenant, record("enroll-session"))
	routes.HandleFunc("GET /v1/me", auth.NoTenant, record("notenant"))
	routes.HandleFunc("GET /v1/items", auth.Tenant, record("tenant"))
	routes.HandleFunc("GET /v1/other", auth.Tenant, record("tenant2"))
	routes.HandleFunc("GET /v1/teapot", auth.Tenant, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	routes.HandleFunc("GET /v1/paid", auth.Tenant, record("paid"), httpx.Use(func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			e.mu.Lock()
			e.reached["entitlements"]++
			e.mu.Unlock()
			httpx.WriteProblem(w, r, "subscription_inactive", "")
		})
	}))
	routes.HandleFunc("GET /v1/slow", auth.Tenant, func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, r, statementTimeout())
	})
	rt.HandleFunc("GET /healthz", record("health"))
	e.h = httpx.Chain(rt, logging.New(e.log, slog.LevelDebug), e.metrics)
	return e
}

func (e *env) do(method, path, authz string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func (e *env) get(path, authz string) *httptest.ResponseRecorder {
	return e.do(http.MethodGet, path, authz)
}

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
