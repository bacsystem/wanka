package pgsession_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/audit"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth/pgsession"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

// plan_limit_reached y module_not_enabled llevan sus miembros de extensión.
type extBody struct {
	Code   string `json:"code"`
	Module string `json:"module"`
	Limit  *int64 `json:"limit"`
	Used   *int64 `json:"used"`
}

func (r result) ext(t *testing.T) extBody {
	t.Helper()
	var b extBody
	if err := json.Unmarshal(r.body, &b); err != nil {
		t.Fatalf("body: %q", r.body)
	}
	return b
}

// subscribe da a la empresa de la sesión un plan con esos módulos y límites y una suscripción en ese estado.
func (e *env) subscribe(tenant, status, limits string, modules ...string) {
	e.t.Helper()
	ctx := context.Background()
	admin := e.d.Admin(ctx, e.t)
	code := "p-" + tenant
	var planID string
	if err := admin.QueryRow(ctx, `INSERT INTO platform.plans (code, name, limits)
		VALUES ($1, $1, '{"users": 100, "sites": 100, "cpe_per_month": 100}'::jsonb || $2::jsonb) RETURNING id::text`, code, limits).Scan(&planID); err != nil {
		e.t.Fatal(err)
	}
	for _, m := range modules {
		e.exec(`INSERT INTO platform.plan_modules (plan_id, module_code) VALUES ($1, $2)`, planID, m)
	}
	e.exec(`INSERT INTO platform.subscriptions (id, tenant_id, plan_id, status) VALUES (gen_random_uuid(), $1, $2, $3)`, tenant, planID, status)
}

func (e *env) usage(tenant string, users int) {
	e.t.Helper()
	e.exec(`INSERT INTO platform.tenant_usage (tenant_id, period, active_users) VALUES ($1, date_trunc('month', now() AT TIME ZONE 'America/Lima')::date, $2)`, tenant, users)
}

func (e *env) send(method, path, token string) result { return e.request(method, path, token) }

// Criterios 1 a 4 y 11 contra PostgreSQL 16: dos empresas con planes distintos, cada una con su resultado.
func TestEntitlementRulesAgainstPostgres(t *testing.T) {
	e := newEnv(t, envConfig{realEnts: true})

	active := e.seed(dbtest.SessionSpec{})
	e.subscribe(active.TenantID, "activa", `{"users": 3}`, "ventas")
	e.usage(active.TenantID, 2)

	atLimit := e.seed(dbtest.SessionSpec{})
	e.subscribe(atLimit.TenantID, "activa", `{"users": 3}`, "ventas")
	e.usage(atLimit.TenantID, 3)

	noVet := e.seed(dbtest.SessionSpec{})
	e.subscribe(noVet.TenantID, "activa", `{}`, "ventas")

	withVet := e.seed(dbtest.SessionSpec{})
	e.subscribe(withVet.TenantID, "activa", `{}`, "ventas", "rubro.veterinaria")

	closed := e.seed(dbtest.SessionSpec{})
	e.subscribe(closed.TenantID, "activa", `{}`, "ventas")
	e.exec(`UPDATE org.tenants SET status = 'cerrada', closed_at = now() WHERE id = $1`, closed.TenantID)

	lapsed := map[string]dbtest.Seeded{}
	for _, st := range []string{"morosa", "suspendida", "cancelada"} {
		lapsed[st] = e.seed(dbtest.SessionSpec{})
		e.subscribe(lapsed[st].TenantID, st, `{}`, "ventas")
	}
	noPlan := e.seed(dbtest.SessionSpec{})

	t.Run("active company reaches a module route and a limited route below its limit", func(t *testing.T) {
		if r := e.get("/v1/items", active.Token); r.status != 200 {
			t.Fatalf("GET /v1/items = %d %s", r.status, r.body)
		}
		if r := e.send("POST", "/v1/users", active.Token); r.status != 200 {
			t.Fatalf("POST /v1/users = %d %s", r.status, r.body)
		}
	})
	t.Run("used == limit is plan_limit_reached with limit and used", func(t *testing.T) {
		before := e.writes.Load()
		r := e.send("POST", "/v1/users", atLimit.Token)
		r.wantProblem(t, 403, "plan_limit_reached")
		if b := r.ext(t); b.Limit == nil || b.Used == nil || *b.Limit != 3 || *b.Used != 3 {
			t.Fatalf("problem = %+v", b)
		}
		if e.writes.Load() != before {
			t.Fatal("the handler ran at the limit")
		}
	})
	t.Run("module absent from the plan is module_not_enabled with the module; the other company has it", func(t *testing.T) {
		before := e.ran.Load()
		r := e.get("/v1/vet", noVet.Token)
		r.wantProblem(t, 403, "module_not_enabled")
		if b := r.ext(t); b.Module != "rubro.veterinaria" {
			t.Fatalf("problem = %+v", b)
		}
		if e.ran.Load() != before {
			t.Fatal("the handler ran without its module")
		}
		if r := e.get("/v1/vet", withVet.Token); r.status != 200 {
			t.Fatalf("company with the module = %d %s", r.status, r.body)
		}
	})
	t.Run("closed company: writes are tenant_read_only, reads and exports pass", func(t *testing.T) {
		before := e.writes.Load()
		for _, p := range []string{"/v1/items", "/v1/users"} {
			e.send("POST", p, closed.Token).wantProblem(t, 403, "tenant_read_only")
		}
		if e.writes.Load() != before {
			t.Fatal("a write handler ran for a closed company")
		}
		for _, p := range []string{"/v1/items", "/v1/exports"} {
			if r := e.get(p, closed.Token); r.status != 200 {
				t.Fatalf("GET %s = %d %s", p, r.status, r.body)
			}
		}
	})
	t.Run("morosa, suspendida and cancelada: 402 everywhere but the fixed list", func(t *testing.T) {
		for st, s := range lapsed {
			e.get("/v1/items", s.Token).wantProblem(t, 402, "subscription_inactive")
			e.send("POST", "/v1/items", s.Token).wantProblem(t, 402, "subscription_inactive")
			for _, p := range []string{"/v1/exports", "/v1/me/entitlements"} {
				if r := e.get(p, s.Token); r.status != 200 {
					t.Fatalf("%s: GET %s = %d %s", st, p, r.status, r.body)
				}
			}
			if r := e.get("/v1/me", s.Token); r.status != 200 {
				t.Fatalf("%s: GET /v1/me = %d", st, r.status)
			}
		}
	})
	t.Run("a company without a subscription is 403 module_not_enabled, not 402", func(t *testing.T) {
		e.get("/v1/items", noPlan.Token).wantProblem(t, 403, "module_not_enabled")
		if r := e.get("/v1/exports", noPlan.Token); r.status != 200 {
			t.Fatalf("GET /v1/exports = %d", r.status)
		}
	})
}

// Un cambio de la base llega a la respuesta por NOTIFY, mucho antes del TTL de 30 s.
func TestNotifyInvalidationReachesTheResponse(t *testing.T) {
	e := newEnv(t, envConfig{realEnts: true})
	s := e.seed(dbtest.SessionSpec{})
	e.subscribe(s.TenantID, "activa", `{}`, "ventas")

	if r := e.get("/v1/items", s.Token); r.status != 200 { // caliente la caché
		t.Fatalf("GET /v1/items = %d %s", r.status, r.body)
	}
	waitUntil(t, "the 402 after the subscription turns morosa", func() bool {
		e.exec(`UPDATE platform.subscriptions SET status = 'morosa' WHERE tenant_id = $1`, s.TenantID)
		return e.get("/v1/items", s.Token).status == 402
	})
	waitUntil(t, "the write block after the company closes", func() bool {
		e.exec(`UPDATE platform.subscriptions SET status = 'activa' WHERE tenant_id = $1`, s.TenantID)
		e.exec(`UPDATE org.tenants SET status = 'cerrada', closed_at = now() WHERE id = $1`, s.TenantID)
		return e.send("POST", "/v1/items", s.Token).status == 403
	})
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	// El TTL es de 30 s: si pasa solo por vencimiento, la prueba no habría probado el NOTIFY.
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for: %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// lockSubscriptions bloquea la tabla que lee la consulta de entitlements, no la de sesiones.
func lockSubscriptions(t *testing.T, e *env) {
	t.Helper()
	ctx := context.Background()
	conn := e.d.Admin(ctx, t)
	if _, err := conn.Exec(ctx, `BEGIN`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `LOCK TABLE platform.subscriptions IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { _, _ = conn.Exec(ctx, `ROLLBACK`) }) })
}

func (e *env) waitEntitlementsBackend() int {
	e.t.Helper()
	ctx := context.Background()
	admin := e.d.Admin(ctx, e.t)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var pid int
		err := admin.QueryRow(ctx, `SELECT pid FROM pg_stat_activity WHERE application_name = $1 AND state = 'active'
			AND query LIKE '%platform.subscriptions%' AND pid <> pg_backend_pid() LIMIT 1`, appName).Scan(&pid)
		if err == nil {
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.t.Fatal("the entitlements read never reached the database")
	return 0
}

// Criterio 7 contra PostgreSQL: un 57014 al leer los entitlements es 503 con Retry-After 1 y phase=entitlements; la petición no pasa.
func TestEntitlementsStatementTimeoutIs503WithPhase(t *testing.T) {
	e := newEnv(t, envConfig{realEnts: true, roleSettings: [][2]string{{"statement_timeout", "300ms"}}})
	s := e.seed(dbtest.SessionSpec{})
	e.subscribe(s.TenantID, "activa", `{}`, "ventas")
	lockSubscriptions(t, e)

	r := e.get("/v1/items", s.Token)
	r.wantUnavailable(t)
	lines := e.logs.lines(t, "statement timeout")
	if len(lines) != 1 || lines[0]["phase"] != "entitlements" || lines[0]["sqlstate"] != "57014" || lines[0]["module"] != "ventas" {
		t.Fatalf("warn lines = %v", lines)
	}
	if got := e.metrics.get("db_statement_timeouts_total{phase=entitlements}"); got != 1 {
		t.Fatalf("db_statement_timeouts_total{phase=entitlements} = %d (%v)", got, e.metrics.n)
	}
	if e.ran.Load() != 0 {
		t.Fatal("the handler ran without entitlements")
	}
	e.wantNoConnectionLeak()
}

// Criterio 5 contra PostgreSQL: la conexión se pierde al leer -> 503, no pasa, y la siguiente petición con la base de vuelta sí.
func TestEntitlementsConnectionLossIs503ThenRecovers(t *testing.T) {
	e := newEnv(t, envConfig{realEnts: true})
	s := e.seed(dbtest.SessionSpec{})
	e.subscribe(s.TenantID, "activa", `{}`, "ventas")
	conn := e.d.Admin(context.Background(), t)
	if _, err := conn.Exec(context.Background(), `BEGIN`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(context.Background(), `LOCK TABLE platform.subscriptions IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}

	done := make(chan result, 1)
	go func() { done <- e.get("/v1/items", s.Token) }()
	pid := e.waitEntitlementsBackend()
	e.exec(`SELECT pg_terminate_backend($1)`, pid)

	r := <-done
	r.wantUnavailable(t)
	if got := e.metrics.get("db_unavailable_total{phase=entitlements}"); got != 1 {
		t.Fatalf("db_unavailable_total{phase=entitlements} = %d (%v)", got, e.metrics.n)
	}
	if e.ran.Load() != 0 {
		t.Fatal("the handler ran without entitlements")
	}
	if _, err := conn.Exec(context.Background(), `ROLLBACK`); err != nil {
		t.Fatal(err)
	}
	// Nada se guardó en la caché: la base de vuelta responde sin esperar el TTL.
	if r := e.get("/v1/items", s.Token); r.status != 200 {
		t.Fatalf("after recovery = %d %s", r.status, r.body)
	}
	e.wantNoConnectionLeak()
}

// countingDB cuenta las lecturas de entitlements que llegan a la base.
type countingDB struct {
	*db.Pool
	reads atomic.Int64
}

func (c *countingDB) InTenantReadTx(ctx context.Context, a db.Actor, fn func(pgx.Tx) error) error {
	c.reads.Add(1)
	return c.Pool.InTenantReadTx(ctx, a, fn)
}

// frozen hace la búsqueda de sesión una vez y la repite: el p95 mide la cadena, no la base.
type frozen struct {
	inner auth.SessionResolver
	once  sync.Once
	rec   auth.SessionRecord
	found bool
	err   error
}

func (f *frozen) BindVersion(ctx context.Context, sid, mid pgtype.UUID, v int64) (bool, error) {
	return f.inner.BindVersion(ctx, sid, mid, v)
}

func (f *frozen) Revoke(ctx context.Context, sid pgtype.UUID) error { return f.inner.Revoke(ctx, sid) }

func (f *frozen) Lookup(ctx context.Context, h [32]byte) (auth.SessionRecord, bool, error) {
	f.once.Do(func() { f.rec, f.found, f.err = f.inner.Lookup(ctx, h) })
	return f.rec, f.found, f.err
}

func (f *frozen) Detach(ctx context.Context, sid, mid pgtype.UUID) (bool, error) {
	return f.inner.Detach(ctx, sid, mid)
}

// Criterio no funcional: con la caché caliente la cadena completa (sesión, dos
// limitadores y entitlements, sin red ni base de sesión) responde con p95 < 1 ms,
// así que el paso de entitlements añade menos que eso.
func TestHotCacheStepP95UnderOneMillisecond(t *testing.T) {
	e := newEnv(t, envConfig{realEnts: true})
	s := e.seed(dbtest.SessionSpec{})
	e.subscribe(s.TenantID, "activa", `{}`, "ventas")

	rt := httpx.NewRouter(httpx.RouterOptions{})
	huge := httpx.LimiterConfig{Capacity: 1 << 30, Refill: time.Nanosecond, MaxKeys: 10}
	counted := &countingDB{Pool: e.pool}
	routes, err := auth.NewRoutes(rt, &frozen{inner: pgsession.New(e.pool)}, entitlements.New(counted, entitlements.Options{}), allowAll{}, audit.NewWriter(e.pool), auth.Options{SessionLimiter: huge, TenantLimiter: huge})
	if err != nil {
		t.Fatal(err)
	}
	routes.HandleFunc("GET /v1/items", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:read"}, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := httpx.Chain(rt, logging.New(io.Discard, slog.LevelInfo), nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/items", nil)
	req.Header.Set("Authorization", "Bearer "+s.Token)
	serve := func() int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := serve(); code != 200 { // calienta la sesión y la caché
		t.Fatalf("warm-up = %d", code)
	}
	reads := counted.reads.Load()
	const n = 3000
	d := make([]time.Duration, n)
	for i := range d {
		start := time.Now()
		if code := serve(); code != 200 {
			t.Fatalf("status %d", code)
		}
		d[i] = time.Since(start)
	}
	if counted.reads.Load() != reads {
		t.Fatal("the hot path touched the database: the cache was not hot")
	}
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	p95 := d[n*95/100]
	t.Logf("hot cache, whole tenant chain: p50 %v p95 %v p99 %v", d[n/2], p95, d[n*99/100])
	if p95 >= time.Millisecond {
		t.Fatalf("p95 = %v, want < 1ms", p95)
	}
}
