package pgsession_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth/pgsession"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

const appName = "pgsession-it"

// syncBuffer es el destino del logger: la cadena escribe desde varias goroutines.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// lines devuelve las líneas del log con ese mensaje.
func (s *syncBuffer) lines(t *testing.T, msg string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, ln := range strings.Split(strings.TrimSpace(s.String()), "\n") {
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

// counters cuenta por nombre y etiqueta phase, sin la ruta.
type counters struct {
	mu sync.Mutex
	n  map[string]int
}

func (c *counters) Inc(name string, labels ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.n == nil {
		c.n = map[string]int{}
	}
	key := name
	for i := 0; i+1 < len(labels); i += 2 {
		if labels[i] == "phase" || labels[i] == "outcome" {
			key += "{" + labels[i] + "=" + labels[i+1] + "}"
		}
	}
	c.n[key]++
}

func (c *counters) get(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n[key]
}

// view es lo que las rutas de prueba devuelven de la sesión que ven.
type view struct {
	Kind           string   `json:"kind"`
	Tenant         string   `json:"tenant"`
	User           string   `json:"user"`
	Impersonator   string   `json:"impersonator"`
	Site           string   `json:"site"`
	Cash           string   `json:"cash"`
	Membership     string   `json:"membership"`
	SessionVersion *int64   `json:"sessionVersion"`
	CurrentVersion *int64   `json:"currentVersion"`
	Roles          []string `json:"roles"` // tenant_id de cada fila de iam.roles que ve InTenantTx
}

func uuidStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return u.String()
}

func int8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

// openSource da a toda empresa el módulo ventas: estos tests prueban la sesión,
// no los entitlements (esos están en entitlements_chain_test.go).
type openSource struct{}

func (openSource) For(context.Context, pgtype.UUID) (entitlements.Entitlements, error) {
	return entitlements.Entitlements{Status: entitlements.StatusActive, Modules: []string{"ventas"}}, nil
}

type envConfig struct {
	poolMax        int32
	acquireTimeout time.Duration
	// roleSetting es un ALTER ROLE wanka_app IN DATABASE ... SET <setting> previo a abrir el pool.
	roleSettings [][2]string
	wrap         func(auth.SessionResolver) auth.SessionResolver
	// realEnts arma el servicio real de entitlements (y su escucha de NOTIFY) en vez de openSource.
	realEnts bool
}

// env es la cadena completa: Chain -> Router -> auth.Routes -> resolver de PostgreSQL, sobre una base real y wanka_app.
type env struct {
	t       *testing.T
	d       *dbtest.DB
	pool    *db.Pool
	srv     *httptest.Server
	logs    *syncBuffer
	metrics *counters
	tokens  []string
	ran     atomic.Int64 // veces que corrió el handler de la ruta Tenant
	writes  atomic.Int64 // veces que corrió el handler de una ruta de escritura
	ents    *entitlements.Service
}

func newEnv(t *testing.T, cfg envConfig) *env {
	t.Helper()
	if cfg.poolMax == 0 {
		cfg.poolMax = 5
	}
	d := dbtest.New(t)
	ctx := context.Background()
	d.Migrate(ctx, t)
	admin := d.Admin(ctx, t)
	for _, s := range cfg.roleSettings {
		if _, err := admin.Exec(ctx, `ALTER ROLE wanka_app IN DATABASE `+pgx.Identifier{d.Name}.Sanitize()+` SET `+s[0]+` = '`+s[1]+`'`); err != nil {
			t.Fatal(err)
		}
	}
	pool, err := db.Open(ctx, d.URL(dbtest.App), db.Options{MaxConns: cfg.poolMax, AcquireTimeout: cfg.acquireTimeout, AppName: appName})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	e := &env{t: t, d: d, pool: pool, logs: &syncBuffer{}, metrics: &counters{}}
	var resolver auth.SessionResolver = pgsession.New(pool)
	if cfg.wrap != nil {
		resolver = cfg.wrap(resolver)
	}
	var source auth.EntitlementSource = openSource{}
	if cfg.realEnts {
		e.ents = entitlements.New(pool, entitlements.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
		source = e.ents
		runCtx, stop := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() { defer close(done); _ = e.ents.Run(runCtx) }()
		t.Cleanup(func() { stop(); <-done })
		// Los NOTIFY anteriores al LISTEN no llegan: se espera a que escuche.
		deadline := time.Now().Add(5 * time.Second)
		for {
			var n int
			_ = admin.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE query LIKE 'LISTEN %' AND datname = current_database()`).Scan(&n)
			if n > 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("the entitlements listener never connected")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	rt := httpx.NewRouter(httpx.RouterOptions{})
	routes, err := auth.NewRoutes(rt, resolver, source, auth.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ok := func(counter *atomic.Int64) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { counter.Add(1); writeView(w, view{}) }
	}
	none := auth.Spec{Module: auth.NoModule, Permission: auth.NoPermission}
	routes.HandleFunc("POST /v1/items", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:create"}, ok(&e.writes))
	routes.HandleFunc("POST /v1/users", auth.Tenant, auth.Spec{Module: "ventas", Permission: "identidad.users:create", Limit: entitlements.LimitUsers}, ok(&e.writes))
	routes.HandleFunc("GET /v1/vet", auth.Tenant, auth.Spec{Module: "rubro.veterinaria", Permission: "vet.pets:read"}, ok(&e.ran))
	routes.HandleFunc("GET /v1/exports", auth.Tenant, none, ok(&e.ran))
	routes.HandleFunc("GET /v1/me/entitlements", auth.Tenant, none, ok(&e.ran))
	// GET /v1/me: ruta NoTenant de prueba; la real llega con #27.
	routes.HandleFunc("GET /v1/me", auth.NoTenant, auth.Spec{Module: auth.NoModule, Permission: auth.NoPermission}, func(w http.ResponseWriter, r *http.Request) {
		s, _ := auth.FromContext(r.Context())
		writeView(w, sessionView(s, nil))
	})
	// GET /v1/items: ruta Tenant de prueba; abre InTenantTx con el Actor del contexto.
	routes.HandleFunc("GET /v1/items", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:read"}, func(w http.ResponseWriter, r *http.Request) {
		e.ran.Add(1)
		s, _ := auth.FromContext(r.Context())
		var roles []string
		err := pool.InTenantReadTx(r.Context(), s.Actor, func(tx pgx.Tx) error {
			rows, err := tx.Query(r.Context(), `SELECT tenant_id::text FROM iam.roles`)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					return err
				}
				roles = append(roles, id)
			}
			return rows.Err()
		})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		writeView(w, sessionView(s, roles))
	})
	e.srv = httptest.NewServer(httpx.Chain(rt, logging.New(e.logs, slog.LevelDebug), e.metrics))
	t.Cleanup(e.srv.Close)
	// Ninguna línea del log contiene el token ni su sha256 (hex o base64), en ningún test.
	t.Cleanup(e.assertNoSecretsInLogs)
	return e
}

func sessionView(s auth.Session, roles []string) view {
	return view{Kind: s.Actor.Kind.String(), Tenant: uuidStr(s.Actor.TenantID), User: uuidStr(s.Actor.UserID),
		Impersonator: uuidStr(s.Actor.ImpersonatorID), Site: uuidStr(s.SiteID), Cash: uuidStr(s.CashRegisterID),
		Membership: uuidStr(s.MembershipID), SessionVersion: int8Ptr(s.SessionAuthzVersion), CurrentVersion: int8Ptr(s.CurrentAuthzVersion), Roles: roles}
}

func writeView(w http.ResponseWriter, v view) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (e *env) seed(spec dbtest.SessionSpec) dbtest.Seeded {
	e.t.Helper()
	s := e.d.SeedSession(e.t, spec)
	e.tokens = append(e.tokens, s.Token)
	return s
}

func (e *env) assertNoSecretsInLogs() {
	logs := e.logs.String()
	for _, tok := range e.tokens {
		h := sha256.Sum256([]byte(tok))
		for _, secret := range []string{tok, hex.EncodeToString(h[:]), strings.ToUpper(hex.EncodeToString(h[:])),
			base64.StdEncoding.EncodeToString(h[:]), base64.RawStdEncoding.EncodeToString(h[:]),
			base64.URLEncoding.EncodeToString(h[:]), base64.RawURLEncoding.EncodeToString(h[:])} {
			if strings.Contains(logs, secret) {
				e.t.Errorf("the log contains a token or its hash (%d bytes of secret): %s", len(secret), logs)
				return
			}
		}
	}
}

type result struct {
	status  int
	header  http.Header
	body    []byte
	elapsed time.Duration
}

func (e *env) get(path, token string) result { return e.request(http.MethodGet, path, token) }

func (e *env) request(method, path, token string) result {
	e.t.Helper()
	req, err := http.NewRequest(method, e.srv.URL+path, nil)
	if err != nil {
		e.t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return e.do(req)
}

func (e *env) do(req *http.Request) result {
	e.t.Helper()
	start := time.Now()
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	return result{status: resp.StatusCode, header: resp.Header, body: body, elapsed: time.Since(start)}
}

func (r result) problemCode(t *testing.T) string {
	t.Helper()
	var p struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(r.body, &p); err != nil {
		t.Fatalf("body is not a problem: %q", r.body)
	}
	return p.Code
}

func (r result) view(t *testing.T) view {
	t.Helper()
	var v view
	if err := json.Unmarshal(r.body, &v); err != nil {
		t.Fatalf("body is not a session view (status %d): %q", r.status, r.body)
	}
	return v
}

func (r result) wantProblem(t *testing.T, status int, code string) {
	t.Helper()
	if r.status != status || r.problemCode(t) != code {
		t.Fatalf("response = %d %s, want %d %s", r.status, r.body, status, code)
	}
}

func (r result) wantUnauthorized(t *testing.T) {
	t.Helper()
	r.wantProblem(t, 401, "session_expired")
	if got := r.header.Values("WWW-Authenticate"); len(got) != 1 || got[0] != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q, want [Bearer]", got)
	}
}

func (r result) wantUnavailable(t *testing.T) {
	t.Helper()
	r.wantProblem(t, 503, "dependency_unavailable")
	if r.header.Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q, want 1", r.header.Get("Retry-After"))
	}
	// El cuerpo es genérico: nada de PostgreSQL ni del mecanismo.
	low := strings.ToLower(string(r.body))
	for _, leak := range []string{"57014", "statement", "timeout", "canceling", "pg_", "sqlstate"} {
		if strings.Contains(low, leak) {
			t.Fatalf("the body leaks %q: %s", leak, r.body)
		}
	}
}

// sessionRow es el estado de la fila de iam.sessions que importa al desligue.
type sessionRow struct {
	tenant, membership, site, cash, version, user string // "<null>" si es NULL
	revoked                                       bool
	expiresAt                                     string
}

func (e *env) row(id string) sessionRow {
	e.t.Helper()
	ctx := context.Background()
	var r sessionRow
	err := e.d.Admin(ctx, e.t).QueryRow(ctx, `SELECT coalesce(tenant_id::text, '<null>'), coalesce(membership_id::text, '<null>'),
		coalesce(site_id::text, '<null>'), coalesce(cash_register_id::text, '<null>'), coalesce(authz_version::text, '<null>'),
		coalesce(user_id::text, '<null>'), revoked_at IS NOT NULL, access_expires_at::text FROM iam.sessions WHERE id = $1`, id).
		Scan(&r.tenant, &r.membership, &r.site, &r.cash, &r.version, &r.user, &r.revoked, &r.expiresAt)
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	ctx := context.Background()
	if _, err := e.d.Admin(ctx, e.t).Exec(ctx, sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

func wantDetached(t *testing.T, r sessionRow) {
	t.Helper()
	if r.tenant != "<null>" || r.membership != "<null>" || r.site != "<null>" || r.cash != "<null>" || r.version != "<null>" || r.revoked {
		t.Fatalf("row = %+v, want tenant, membership, site, cash register and authz_version NULL and not revoked", r)
	}
}

// ---- sesión ligada, membresía activa ----

func TestBoundSessionReachesTheTenantRouteWithItsActor(t *testing.T) {
	e := newEnv(t, envConfig{})
	a := e.seed(dbtest.SessionSpec{SessionVersion: 3, MembershipVersion: 3})
	b := e.seed(dbtest.SessionSpec{})

	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/v1/items", nil)
	req.Header.Set("Authorization", "Bearer "+a.Token)
	// Tenant y sede salen solo de la fila de la sesión, nunca de la petición.
	req.Header.Set("X-Tenant-Id", b.TenantID)
	req.Header.Set("X-Site-Id", b.SiteID)
	res := e.do(req)
	if res.status != 200 {
		t.Fatalf("status = %d: %s", res.status, res.body)
	}
	v := res.view(t)
	want := view{Kind: "user", Tenant: a.TenantID, User: a.UserID, Site: a.SiteID, Cash: a.CashRegisterID, Membership: a.MembershipID}
	if v.Kind != want.Kind || v.Tenant != want.Tenant || v.User != want.User || v.Site != want.Site || v.Cash != want.Cash ||
		v.Membership != want.Membership || v.Impersonator != "" {
		t.Fatalf("session = %+v, want %+v", v, want)
	}
	if v.SessionVersion == nil || *v.SessionVersion != 3 || v.CurrentVersion == nil || *v.CurrentVersion != 3 {
		t.Fatalf("versions = %v / %v, want 3 / 3", v.SessionVersion, v.CurrentVersion)
	}
	// InTenantTx ve solo las filas de su tenant, aunque hay otro tenant con datos.
	if len(v.Roles) != 1 || v.Roles[0] != a.TenantID {
		t.Fatalf("InTenantTx saw roles of tenants %v, want only %s", v.Roles, a.TenantID)
	}
	if e.ran.Load() != 1 {
		t.Fatalf("handler ran %d times", e.ran.Load())
	}
}

// ---- membresía perdida ----

func TestLostMembershipDetachesWithoutRevoking(t *testing.T) {
	loss := map[string]string{
		"deleted":   `DELETE FROM iam.memberships WHERE id = $1`,
		"suspended": `UPDATE iam.memberships SET status = 'suspendido' WHERE id = $1`,
	}
	for name, sql := range loss {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, envConfig{})
			s := e.seed(dbtest.SessionSpec{})
			e.exec(sql, s.MembershipID)

			res := e.get("/v1/items", s.Token)
			res.wantProblem(t, 403, "tenant_access_denied")
			if res.header.Get("Retry-After") != "" || res.header.Get("WWW-Authenticate") != "" {
				t.Fatalf("a 403 must not carry Retry-After or WWW-Authenticate: %v", res.header)
			}
			if e.ran.Load() != 0 {
				t.Fatal("the handler ran for a session that lost its membership")
			}
			wantDetached(t, e.row(s.SessionID))
			if e.metrics.get("auth_session_detach_total{outcome=detached}") != 1 {
				t.Fatalf("detach metric = %v", e.metrics.n)
			}

			// El usuario sigue autenticado: GET /v1/me responde 200 con la sesión sin tenant.
			me := e.get("/v1/me", s.Token)
			if me.status != 200 {
				t.Fatalf("GET /v1/me = %d %s", me.status, me.body)
			}
			if v := me.view(t); v.Tenant != "" || v.Membership != "" || v.Site != "" || v.User != s.UserID {
				t.Fatalf("session after detach = %+v", v)
			}
			// Y la siguiente petición a la ruta Tenant sigue siendo 403 (sesión sin empresa).
			e.get("/v1/items", s.Token).wantProblem(t, 403, "tenant_access_denied")
		})
	}
}

func TestSuspendedThenActiveAgainKeepsTheSession(t *testing.T) {
	e := newEnv(t, envConfig{})
	s := e.seed(dbtest.SessionSpec{MembershipStatus: "suspendido"})
	// Vuelve a activo antes de la siguiente petición: nadie la vio suspendida.
	e.exec(`UPDATE iam.memberships SET status = 'activo' WHERE id = $1`, s.MembershipID)
	before := e.row(s.SessionID)

	res := e.get("/v1/items", s.Token)
	if res.status != 200 {
		t.Fatalf("status = %d: %s", res.status, res.body)
	}
	if v := res.view(t); v.Tenant != s.TenantID || v.Membership != s.MembershipID {
		t.Fatalf("session = %+v", v)
	}
	if after := e.row(s.SessionID); after != before {
		t.Fatalf("row changed: %+v -> %+v", before, after)
	}
}

// Dos peticiones leen la membresía perdida antes de que cualquiera desligue: el
// UPDATE afecta 1 fila en una y 0 en la otra, y las dos responden 403.
type barrierResolver struct {
	auth.SessionResolver
	arrived  atomic.Int32
	release  chan struct{}
	detached chan bool
}

func (b *barrierResolver) Lookup(ctx context.Context, h [32]byte) (auth.SessionRecord, bool, error) {
	rec, found, err := b.SessionResolver.Lookup(ctx, h)
	if err == nil && found && b.arrived.Add(1) == 2 {
		close(b.release)
	}
	select {
	case <-b.release:
	case <-time.After(10 * time.Second):
	}
	return rec, found, err
}

func (b *barrierResolver) Detach(ctx context.Context, sid, mid pgtype.UUID) (bool, error) {
	ok, err := b.SessionResolver.Detach(ctx, sid, mid)
	b.detached <- ok
	return ok, err
}

func TestConcurrentRequestsDetachOnce(t *testing.T) {
	var barrier *barrierResolver
	e := newEnv(t, envConfig{wrap: func(r auth.SessionResolver) auth.SessionResolver {
		barrier = &barrierResolver{SessionResolver: r, release: make(chan struct{}), detached: make(chan bool, 2)}
		return barrier
	}})
	s := e.seed(dbtest.SessionSpec{})
	e.exec(`DELETE FROM iam.memberships WHERE id = $1`, s.MembershipID)

	var wg sync.WaitGroup
	results := make([]result, 2)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = e.get("/v1/items", s.Token)
		}()
	}
	wg.Wait()
	for i, r := range results {
		if r.status != 403 || r.problemCode(t) != "tenant_access_denied" {
			t.Fatalf("request %d = %d %s", i, r.status, r.body)
		}
	}
	first, second := <-barrier.detached, <-barrier.detached
	if first == second {
		t.Fatalf("UPDATE results = %v, %v; want exactly one row affected", first, second)
	}
	wantDetached(t, e.row(s.SessionID))
	if e.ran.Load() != 0 {
		t.Fatal("the handler ran")
	}
}

// ---- sesión impersonada ----

func TestImpersonatedSessionIgnoresMembershipLoss(t *testing.T) {
	loss := map[string]string{
		"deleted":   `DELETE FROM iam.memberships WHERE tenant_id = $1`,
		"suspended": `UPDATE iam.memberships SET status = 'suspendido' WHERE tenant_id = $1`,
	}
	for name, sql := range loss {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, envConfig{})
			s := e.seed(dbtest.SessionSpec{Kind: dbtest.SessionImpersonated})
			e.exec(sql, s.TenantID)
			before := e.row(s.SessionID)

			res := e.get("/v1/items", s.Token)
			if res.status != 200 {
				t.Fatalf("status = %d: %s", res.status, res.body)
			}
			v := res.view(t)
			if v.Kind != "impersonated" || v.Impersonator != s.ImpersonatorID || v.User != s.ImpersonatorID || v.Tenant != s.TenantID {
				t.Fatalf("session = %+v, want impersonated by %s in %s", v, s.ImpersonatorID, s.TenantID)
			}
			if before.user != "<null>" {
				t.Fatalf("iam.sessions.user_id = %s, want NULL", before.user)
			}
			if after := e.row(s.SessionID); after != before {
				t.Fatalf("row changed: %+v -> %+v", before, after)
			}
			if e.metrics.get("auth_session_detach_total{outcome=detached}") != 0 {
				t.Fatal("an impersonated session was detached")
			}
		})
	}
}

func TestImpersonatedSessionRevokedOrExpiredIs401(t *testing.T) {
	cases := map[string]string{
		"revoked": `UPDATE iam.sessions SET revoked_at = now() WHERE id = $1`,
		"expired": `UPDATE iam.sessions SET access_expires_at = now() - interval '1 minute' WHERE id = $1`,
	}
	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, envConfig{})
			s := e.seed(dbtest.SessionSpec{Kind: dbtest.SessionImpersonated})
			if e.get("/v1/items", s.Token).status != 200 {
				t.Fatal("the impersonated session must work before it is cut")
			}
			e.exec(sql, s.SessionID)
			ran := e.ran.Load()
			for _, path := range []string{"/v1/items", "/v1/me"} {
				e.get(path, s.Token).wantUnauthorized(t)
			}
			if e.ran.Load() != ran {
				t.Fatal("the handler ran after the session was cut")
			}
		})
	}
}

// ---- 401 ----

func TestNoRowIs401SessionExpired(t *testing.T) {
	e := newEnv(t, envConfig{})
	unknown := "wk_at_" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	e.tokens = append(e.tokens, unknown)
	cases := map[string]string{
		"unknown token": unknown,
		"expired":       e.seed(dbtest.SessionSpec{Kind: dbtest.SessionExpired}).Token,
		"revoked":       e.seed(dbtest.SessionSpec{Kind: dbtest.SessionRevoked}).Token,
		"half login":    e.seed(dbtest.SessionSpec{Kind: dbtest.SessionHalfLogin}).Token,
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			for _, path := range []string{"/v1/items", "/v1/me"} {
				e.get(path, tok).wantUnauthorized(t)
			}
		})
	}
	if e.ran.Load() != 0 {
		t.Fatal("the handler ran")
	}
}

// ---- fallos de la base ----

// lockSessions toma ACCESS EXCLUSIVE sobre iam.sessions desde otra conexión
// hasta que se llame al release (o termine el test).
func lockSessions(t *testing.T, e *env) (release func()) {
	t.Helper()
	ctx := context.Background()
	conn := e.d.Admin(ctx, t)
	if _, err := conn.Exec(ctx, `BEGIN`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `LOCK TABLE iam.sessions IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	release = func() { once.Do(func() { _, _ = conn.Exec(ctx, `ROLLBACK`) }) }
	t.Cleanup(release)
	return release
}

// waitBackend espera a que la búsqueda de sesión esté ejecutándose en la base y devuelve su pid.
func (e *env) waitBackend() int {
	e.t.Helper()
	ctx := context.Background()
	admin := e.d.Admin(ctx, e.t)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var pid int
		err := admin.QueryRow(ctx, `SELECT pid FROM pg_stat_activity WHERE application_name = $1 AND state = 'active'
			AND query LIKE '%iam.session_lookup%' AND pid <> pg_backend_pid() LIMIT 1`, appName).Scan(&pid)
		if err == nil {
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.t.Fatal("the session lookup never reached the database")
	return 0
}

func (e *env) wantNoConnectionLeak() {
	e.t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for e.pool.AcquiredConns() != 0 {
		if time.Now().After(deadline) {
			e.t.Fatalf("AcquiredConns = %d, the connection did not return to the pool", e.pool.AcquiredConns())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStatementTimeoutIs503AndCounted(t *testing.T) {
	e := newEnv(t, envConfig{roleSettings: [][2]string{{"statement_timeout", "50ms"}}})
	s := e.seed(dbtest.SessionSpec{})
	lockSessions(t, e)

	res := e.get("/v1/items", s.Token)
	res.wantUnavailable(t)
	lines := e.logs.lines(t, "statement timeout")
	if len(lines) != 1 || lines[0]["level"] != "WARN" || lines[0]["phase"] != "session_lookup" || lines[0]["sqlstate"] != "57014" {
		t.Fatalf("warn lines = %v", lines)
	}
	if got := e.metrics.get("db_statement_timeouts_total{phase=session_lookup}"); got != 1 {
		t.Fatalf("db_statement_timeouts_total{phase=session_lookup} = %d, want 1 (%v)", got, e.metrics.n)
	}
	if e.ran.Load() != 0 {
		t.Fatal("the handler ran")
	}
	e.wantNoConnectionLeak()
}

func TestPgCancelBackendIs503AndCounted(t *testing.T) {
	e := newEnv(t, envConfig{})
	s := e.seed(dbtest.SessionSpec{})
	lockSessions(t, e)

	done := make(chan result, 1)
	go func() { done <- e.get("/v1/items", s.Token) }()
	pid := e.waitBackend()
	e.exec(`SELECT pg_cancel_backend($1)`, pid)

	res := <-done
	res.wantUnavailable(t)
	if res.elapsed > 900*time.Millisecond {
		t.Fatalf("took %v: the cancellation did not cut the lookup", res.elapsed)
	}
	if got := e.metrics.get("db_statement_timeouts_total{phase=session_lookup}"); got != 1 {
		t.Fatalf("db_statement_timeouts_total{phase=session_lookup} = %d, want 1 (%v)", got, e.metrics.n)
	}
	e.wantNoConnectionLeak()
}

func TestLookupOwnDeadlineIs503Within1s(t *testing.T) {
	e := newEnv(t, envConfig{})
	s := e.seed(dbtest.SessionSpec{})
	lockSessions(t, e)

	res := e.get("/v1/items", s.Token)
	res.wantUnavailable(t)
	if res.elapsed < 900*time.Millisecond || res.elapsed > 2*time.Second {
		t.Fatalf("took %v, want ~1 s (not the 5 s statement_timeout nor the 10 s handler deadline)", res.elapsed)
	}
	if e.ran.Load() != 0 {
		t.Fatal("the handler ran")
	}
	e.wantNoConnectionLeak()
	lines := e.logs.lines(t, "session lookup timed out")
	if len(lines) != 1 || lines[0]["phase"] != "session_lookup" {
		t.Fatalf("log lines = %v", lines)
	}
}

func TestPgTerminateBackendIs503AndCounted(t *testing.T) {
	e := newEnv(t, envConfig{})
	s := e.seed(dbtest.SessionSpec{})
	lockSessions(t, e)

	done := make(chan result, 1)
	go func() { done <- e.get("/v1/items", s.Token) }()
	pid := e.waitBackend()
	e.exec(`SELECT pg_terminate_backend($1)`, pid)

	res := <-done
	res.wantUnavailable(t)
	if got := e.metrics.get("db_unavailable_total{phase=session_lookup}"); got != 1 {
		t.Fatalf("db_unavailable_total{phase=session_lookup} = %d, want 1 (%v)", got, e.metrics.n)
	}
	e.wantNoConnectionLeak()
}

func TestFullPoolIs503Overloaded(t *testing.T) {
	e := newEnv(t, envConfig{poolMax: 1})
	s := e.seed(dbtest.SessionSpec{})

	hold, release := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(release)
		_ = e.pool.InNoTenantReadTx(context.Background(), db.NoTenantScope{}, func(pgx.Tx) error { <-hold; return nil })
	}()
	t.Cleanup(func() {
		select {
		case <-hold:
		default:
			close(hold)
		}
		<-release
	})
	for e.pool.AcquiredConns() != 1 {
		time.Sleep(5 * time.Millisecond)
	}

	res := e.get("/v1/items", s.Token)
	res.wantProblem(t, 503, "overloaded")
	if res.header.Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q", res.header.Get("Retry-After"))
	}
	close(hold)
	<-release
	// Con la conexión libre otra vez, la misma petición responde.
	if res := e.get("/v1/items", s.Token); res.status != 200 {
		t.Fatalf("after the pool freed: %d %s", res.status, res.body)
	}
}

// El UPDATE de desligar falla (la fila de la sesión está bloqueada y el
// statement_timeout es corto): igual 403, un warn, y la siguiente petición desliga.
func TestFailedDetachStillAnswers403AndTheNextRequestRetries(t *testing.T) {
	e := newEnv(t, envConfig{roleSettings: [][2]string{{"statement_timeout", "50ms"}}})
	s := e.seed(dbtest.SessionSpec{})
	e.exec(`DELETE FROM iam.memberships WHERE id = $1`, s.MembershipID)

	ctx := context.Background()
	locker := e.d.Admin(ctx, t)
	if _, err := locker.Exec(ctx, `BEGIN`); err != nil {
		t.Fatal(err)
	}
	if _, err := locker.Exec(ctx, `SELECT 1 FROM iam.sessions WHERE id = $1 FOR UPDATE`, s.SessionID); err != nil {
		t.Fatal(err)
	}

	e.get("/v1/items", s.Token).wantProblem(t, 403, "tenant_access_denied")
	lines := e.logs.lines(t, "session detach failed")
	if len(lines) != 1 || lines[0]["level"] != "WARN" {
		t.Fatalf("detach warn lines = %v", lines)
	}
	if e.metrics.get("auth_session_detach_total{outcome=failed}") != 1 {
		t.Fatalf("metrics = %v", e.metrics.n)
	}
	if e.row(s.SessionID).tenant == "<null>" {
		t.Fatal("the session was detached even though the UPDATE could not run")
	}
	if e.ran.Load() != 0 {
		t.Fatal("the handler ran")
	}

	if _, err := locker.Exec(ctx, `ROLLBACK`); err != nil {
		t.Fatal(err)
	}
	e.get("/v1/items", s.Token).wantProblem(t, 403, "tenant_access_denied")
	wantDetached(t, e.row(s.SessionID))
}

// ---- limpieza de la conexión ----

// Una conexión física con app.tenant_id = B fijado a nivel de sesión (el pool
// tiene una sola, que nace después del ALTER ROLE) no cambia la respuesta de una
// sesión ligada a A, y la transacción de la búsqueda no termina con
// app.tenant_id = B (la limpieza de InNoTenantReadTx, #198).
func TestSessionLevelTenantOnTheConnectionDoesNotLeak(t *testing.T) {
	e := newEnv(t, envConfig{poolMax: 1})
	a := e.seed(dbtest.SessionSpec{})
	b := e.seed(dbtest.SessionSpec{})
	e.exec(`ALTER ROLE wanka_app IN DATABASE ` + pgx.Identifier{e.d.Name}.Sanitize() + ` SET app.tenant_id = '` + b.TenantID + `'`)

	for _, path := range []string{"/v1/items", "/v1/me"} {
		res := e.get(path, a.Token)
		if res.status != 200 {
			t.Fatalf("%s = %d %s", path, res.status, res.body)
		}
		v := res.view(t)
		if v.Tenant != a.TenantID {
			t.Fatalf("%s: tenant = %s, want A (%s)", path, v.Tenant, a.TenantID)
		}
		if path == "/v1/items" && (len(v.Roles) != 1 || v.Roles[0] != a.TenantID) {
			t.Fatalf("InTenantTx saw roles of tenants %v, want only A", v.Roles)
		}
	}

	// Es la misma conexión (el pool tiene una) y la búsqueda termina con la variable vacía.
	hash := sha256.Sum256([]byte(a.Token))
	var tenant pgtype.UUID
	var end string
	err := e.pool.InNoTenantReadTx(context.Background(), db.NoTenantScope{}, func(tx pgx.Tx) error {
		ctx := context.Background()
		if err := tx.QueryRow(ctx, `SELECT tenant_id FROM iam.session_lookup($1)`, hash[:]).Scan(&tenant); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT current_setting('app.tenant_id', true)`).Scan(&end)
	})
	if err != nil {
		t.Fatal(err)
	}
	if uuidStr(tenant) != a.TenantID || end != "" {
		t.Fatalf("lookup tenant = %s, app.tenant_id at the end = %q; want A and empty", uuidStr(tenant), end)
	}
}
