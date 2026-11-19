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

	"github.com/bacsystem/wanka/apps/api/internal/platform/audit"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
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
	r.TenantID, r.UserID, r.ImpersonatorID, r.MFAMethod, r.ImpersonationScope = uid(3), pgtype.UUID{}, uid(9), "platform", auth.ScopeFull
	return r
}

// fakeResolver es el doble de auth.SessionResolver.
type fakeResolver struct {
	mu      sync.Mutex
	lookup  func(ctx context.Context, h [32]byte) (auth.SessionRecord, bool, error)
	detach  func(ctx context.Context, sid, mid pgtype.UUID) (bool, error)
	bind    func(ctx context.Context, sid, mid pgtype.UUID, v int64) (bool, error)
	revoke  func(ctx context.Context, sid pgtype.UUID) error
	hashes  [][32]byte
	detachs [][2]pgtype.UUID
	binds   []bindCall
	revokes []pgtype.UUID
}

type bindCall struct {
	sid, mid pgtype.UUID
	version  int64
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

func (f *fakeResolver) BindVersion(ctx context.Context, sid, mid pgtype.UUID, v int64) (bool, error) {
	f.mu.Lock()
	f.binds = append(f.binds, bindCall{sid, mid, v})
	f.mu.Unlock()
	if f.bind == nil {
		return true, nil
	}
	return f.bind(ctx, sid, mid, v)
}

func (f *fakeResolver) Revoke(ctx context.Context, sid pgtype.UUID) error {
	f.mu.Lock()
	f.revokes = append(f.revokes, sid)
	f.mu.Unlock()
	if f.revoke == nil {
		return nil
	}
	return f.revoke(ctx, sid)
}

func (f *fakeResolver) bound() []bindCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]bindCall(nil), f.binds...)
}

func (f *fakeResolver) revoked() []pgtype.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]pgtype.UUID(nil), f.revokes...)
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

// fakeEnts es el doble de auth.EntitlementSource: devuelve lo que fija fn y
// recuerda con qué tenant y fase se le pidió.
type fakeEnts struct {
	mu      sync.Mutex
	fn      func(ctx context.Context, tenant pgtype.UUID) (entitlements.Entitlements, error)
	tenants []pgtype.UUID
	phases  []string
}

var _ auth.EntitlementSource = (*fakeEnts)(nil)

func (f *fakeEnts) For(ctx context.Context, tenant pgtype.UUID) (entitlements.Entitlements, error) {
	f.mu.Lock()
	f.tenants = append(f.tenants, tenant)
	f.mu.Unlock()
	return f.fn(ctx, tenant)
}

func (f *fakeEnts) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.tenants)
}

// grants devuelve un doble que siempre responde e.
func grants(e entitlements.Entitlements) *fakeEnts {
	return &fakeEnts{fn: func(context.Context, pgtype.UUID) (entitlements.Entitlements, error) { return e, nil }}
}

func entFails(err error) *fakeEnts {
	return &fakeEnts{fn: func(context.Context, pgtype.UUID) (entitlements.Entitlements, error) {
		return entitlements.Entitlements{}, err
	}}
}

// plan es una empresa activa con los módulos ventas e inventario, tope de 5
// usuarios y 1 en uso.
func plan() entitlements.Entitlements {
	return entitlements.Entitlements{Status: entitlements.StatusActive, Modules: []string{"inventario", "ventas"},
		Limits: map[string]int64{"users": 5, "sites": 2, "cpe_per_month": 100},
		Usage:  map[string]int64{"users": 1, "cpe_per_month": 10}}
}

// fakePerms es el doble de auth.PermissionResolver. Por defecto concede todo el
// catálogo con la versión actual de la sesión; fn lo reemplaza.
type fakePerms struct {
	mu    sync.Mutex
	fn    func(ctx context.Context, s auth.Session) (auth.Authz, error)
	calls []auth.Session
}

var _ auth.PermissionResolver = (*fakePerms)(nil)

func (f *fakePerms) Resolve(ctx context.Context, s auth.Session) (auth.Authz, error) {
	f.mu.Lock()
	f.calls = append(f.calls, s)
	fn := f.fn
	f.mu.Unlock()
	if fn != nil {
		return fn(ctx, s)
	}
	return auth.Authz{Version: s.CurrentAuthzVersion.Int64, Permissions: permSet(permission.All()...)}, nil
}

func (f *fakePerms) set(fn func(ctx context.Context, s auth.Session) (auth.Authz, error)) {
	f.mu.Lock()
	f.fn = fn
	f.mu.Unlock()
}

func (f *fakePerms) resolves() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func permSet(codes ...permission.Code) map[permission.Code]struct{} {
	m := make(map[permission.Code]struct{}, len(codes))
	for _, c := range codes {
		m[c] = struct{}{}
	}
	return m
}

// grantOnly devuelve un resolver que concede solo codes, con la versión actual.
func grantOnly(codes ...permission.Code) func(context.Context, auth.Session) (auth.Authz, error) {
	return func(_ context.Context, s auth.Session) (auth.Authz, error) {
		return auth.Authz{Version: s.CurrentAuthzVersion.Int64, Permissions: permSet(codes...)}, nil
	}
}

// fakeDenials es el doble de auth.DenialRecorder.
type fakeDenials struct {
	mu     sync.Mutex
	err    error
	actors []db.Actor
	events []audit.Event
}

var _ auth.DenialRecorder = (*fakeDenials)(nil)

func (f *fakeDenials) RecordDenied(_ context.Context, a db.Actor, ev audit.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.actors, f.events = append(f.actors, a), append(f.events, ev)
	return f.err
}

func (f *fakeDenials) recorded() []audit.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]audit.Event(nil), f.events...)
}

type env struct {
	h       http.Handler
	perms   *fakePerms
	denials *fakeDenials
	ents    *fakeEnts
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
	return newEnvEnts(t, res, grants(plan()), mod)
}

func newEnvEnts(t *testing.T, res *fakeResolver, ents *fakeEnts, mod func(*auth.Options)) *env {
	t.Helper()
	e := &env{res: res, ents: ents, perms: &fakePerms{}, denials: &fakeDenials{}, clk: clock.NewFake(t0), log: &logBuf{}, metrics: &fakeMetrics{}, reached: map[string]int{}, seen: map[string]auth.Session{}}
	o := auth.Options{Clock: e.clk}
	if mod != nil {
		mod(&o)
	}
	rt := httpx.NewRouter(httpx.RouterOptions{})
	routes, err := auth.NewRoutes(rt, res, ents, e.perms, e.denials, o)
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
	none := auth.Spec{Module: auth.NoModule, Permission: auth.NoPermission}
	ventas := auth.Spec{Module: "ventas", Permission: "ventas.pos:read"}
	routes.HandleFunc("POST /v1/auth/login", auth.Public, none, record("public"))
	routes.HandleFunc("POST /v1/auth/mfa/enrollment/totp", auth.Public, none, record("enroll-public"))
	routes.HandleFunc("POST /v1/auth/mfa/totp/enroll", auth.NoTenant, none, record("enroll-session"))
	routes.HandleFunc("GET /v1/me", auth.NoTenant, none, record("notenant"))
	routes.HandleFunc("GET /v1/items", auth.Tenant, ventas, record("tenant"))
	for _, m := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		routes.HandleFunc(m+" /v1/items", auth.Tenant, ventas, record("tenant-write"))
	}
	routes.HandleFunc("GET /v1/other", auth.Tenant, auth.Spec{Module: "inventario", Permission: "inventario.stock:read"}, record("tenant2"))
	routes.HandleFunc("GET /v1/exports", auth.Tenant, auth.Spec{Module: auth.NoModule, Permission: "reportes.management:export"}, record("exports"))
	routes.HandleFunc("GET /v1/me/entitlements", auth.Tenant, none, record("me-entitlements"))
	routes.HandleFunc("GET /v1/vet", auth.Tenant, auth.Spec{Module: "rubro.veterinaria", Permission: "veterinaria.records:read"}, record("vet"))
	routes.HandleFunc("POST /v1/users", auth.Tenant, auth.Spec{Module: "ventas", Permission: "identidad.users:create", Limit: entitlements.LimitUsers}, record("users-create"))
	routes.HandleFunc("GET /v1/teapot", auth.Tenant, ventas, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	routes.HandleFunc("GET /v1/paid", auth.Tenant, ventas, record("paid"), httpx.Use(func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			e.mu.Lock()
			e.reached["entitlements"]++
			e.mu.Unlock()
			httpx.WriteProblem(w, r, "forbidden", "")
		})
	}))
	routes.HandleFunc("GET /v1/slow", auth.Tenant, ventas, func(w http.ResponseWriter, r *http.Request) {
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
