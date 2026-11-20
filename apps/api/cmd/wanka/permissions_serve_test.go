package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/identidad"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/config"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// startServe arranca serve con el cableado w en el plano tenant, sobre la base de d, y devuelve su dirección.
// Se detiene al terminar el test.
func startServe(t *testing.T, d *dbtest.DB, w wiring, poolMax int32) string {
	t.Helper()
	logs := &lockedBuf{}
	cfg := config.Config{DatabaseURL: d.URL(dbtest.App), DBPoolMax: poolMax, HTTPAddr: "127.0.0.1:0"}
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- serve(ctx, slog.New(slog.NewJSONHandler(logs, nil)), cfg, config.API, "tenant", w) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-errc:
		case <-time.After(10 * time.Second):
			t.Error("serve did not stop")
		}
	})
	addr := ""
	waitFor(t, "serve listening", func() bool {
		for _, ln := range strings.Split(logs.String(), "\n") {
			var m map[string]any
			if json.Unmarshal([]byte(ln), &m) == nil && m["msg"] == "listening" {
				addr, _ = m["addr"].(string)
			}
		}
		return addr != ""
	})
	return addr
}

// grantVentas da a todas las empresas un plan con el módulo ventas, para que los entitlements dejen pasar.
func grantVentas(ctx context.Context, t *testing.T, admin *pgx.Conn) {
	t.Helper()
	for _, q := range []string{
		`INSERT INTO platform.plans (code, name, limits) VALUES ('p-perms', 'p', '{"users": 5, "sites": 5, "cpe_per_month": 5}')`,
		`INSERT INTO platform.plan_modules (plan_id, module_code) SELECT id, 'ventas' FROM platform.plans WHERE code = 'p-perms'`,
		`INSERT INTO platform.subscriptions (id, tenant_id, plan_id, status) SELECT gen_random_uuid(), t.id, p.id, 'activa' FROM org.tenants t, platform.plans p WHERE p.code = 'p-perms'`,
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
}

func get(t *testing.T, addr, path, token string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "http://"+addr+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var p struct{ Code string }
	_ = json.NewDecoder(resp.Body).Decode(&p)
	return resp.StatusCode, p.Code
}

// El cableado de producción (defaultWiring, con solo las rutas de prueba agregadas) usa el resolver de PostgreSQL y no
// DenyAllPermissions: con DenyAllPermissions toda ruta con permiso daría 403, y aquí la ruta da 200 con el permiso del
// rol, vuelve a 403 al quitárselo y la caché se invalida con authz_version, no con el contenido.
func TestServeUsesThePostgresPermissionResolver(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	d.Migrate(ctx, t)
	admin := d.Admin(ctx, t)
	s := d.SeedSession(t, dbtest.SessionSpec{})
	grantVentas(ctx, t, admin)

	w := defaultWiring
	w.mount = func(r *auth.Routes) {
		ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
		r.HandleFunc("GET /v1/items", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:read"}, ok)
		r.HandleFunc("GET /v1/sales-extra", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:create"}, ok)
	}
	addr := startServe(t, d, w, 4)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := admin.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	want := func(step, path string, status int, problem string) {
		t.Helper()
		if code, p := get(t, addr, path, s.Token); code != status || p != problem {
			t.Fatalf("%s: GET %s = %d %q, want %d %q", step, path, code, p, status, problem)
		}
	}

	want("role without permissions", "/v1/items", 403, "forbidden") // queda en la caché con la versión 1
	exec(`INSERT INTO iam.role_permissions (tenant_id, role_id, permission_code) VALUES ($1, $2, 'ventas.pos:read')`, s.TenantID, s.RoleID)
	want("permission added without a version bump: the cache of version 1 still answers", "/v1/items", 403, "forbidden")
	exec(`UPDATE iam.memberships SET authz_version = authz_version + 1 WHERE id = $1`, s.MembershipID)
	want("version bumped: the resolver is asked again", "/v1/items", 200, "")
	want("a permission the role does not have", "/v1/sales-extra", 403, "forbidden")
	exec(`DELETE FROM iam.role_permissions WHERE tenant_id = $1 AND role_id = $2`, s.TenantID, s.RoleID)
	exec(`UPDATE iam.memberships SET authz_version = authz_version + 1 WHERE id = $1`, s.MembershipID)
	want("permission removed and version bumped", "/v1/items", 403, "forbidden")

	// Un rol base (system) toma los permisos de su plantilla: el cajero cobra pero no aprueba descuentos.
	tx, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var tenantUUID pgtype.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM org.tenants WHERE id = $1`, s.TenantID).Scan(&tenantUUID); err != nil {
		t.Fatal(err)
	}
	if err := identidad.SeedTenantRoles(ctx, tx, tenantUUID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE iam.memberships SET role_id = (SELECT id FROM iam.roles WHERE tenant_id = $1 AND template_code = 'cajero'), authz_version = authz_version + 1 WHERE id = $2`, s.TenantID, s.MembershipID)
	want("cajero template: ventas.pos:read", "/v1/items", 200, "")
	want("cajero template: ventas.pos:create", "/v1/sales-extra", 200, "")

	// Una membresía suspendida: la búsqueda de sesión la desliga y la ruta responde 403 tenant_access_denied.
	exec(`UPDATE iam.memberships SET status = 'suspendido' WHERE id = $1`, s.MembershipID)
	want("suspended membership", "/v1/items", 403, "tenant_access_denied")
}

// Criterio no funcional de #233, en la ruta con permiso servida por serve (cableado de producción): 50 usuarios
// virtuales (una cuenta cada uno) en 20 empresas. Cada usuario tiene una membresía (rol base cajero, así lee la
// plantilla) y una sesión en cada empresa. Por cada membresía el usuario virtual pide la ruta dos veces seguidas: la
// primera con la caché de permisos fría (resolver, y BindVersion si hiciera falta) y la segunda caliente; las dos
// latencias se recogen en la misma pasada, intercaladas, y el costo que agrega el paso es p95(fría) - p95(caliente).
// Lo que está en vuelo nunca pasa de las 20 conexiones del pool (un semáforo del mismo tamaño). Se afirma la mediana de
// 3 repeticiones (cada una con membresías frías propias) < 50 ms y errores < 1 %. Se salta con -race y corre sin él en
// el paso de cotas de tiempo de la CI; la cifra sale con -v.
func TestPermissionRouteUnder50ms(t *testing.T) {
	if raceEnabled {
		t.Skip("timing bound: runs without -race in its own CI step")
	}
	const (
		vus     = 50
		tenants = 20
		poolMax = 20
		reps    = 3
	)
	ctx := context.Background()
	d := dbtest.New(t)
	d.Migrate(ctx, t)
	admin := d.Admin(ctx, t)

	tenantIDs := make([]pgtype.UUID, tenants)
	for i := range tenantIDs {
		if err := admin.QueryRow(ctx, `INSERT INTO org.tenants (id) VALUES (gen_random_uuid()) RETURNING id`).Scan(&tenantIDs[i]); err != nil {
			t.Fatal(err)
		}
		tx, err := admin.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := identidad.SeedTenantRoles(ctx, tx, tenantIDs[i]); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	grantVentas(ctx, t, admin)
	// tokens[rep][vu]: un token por empresa; el último usuario de cada repetición no existe, el de calentamiento va aparte.
	seedUser := func() []string {
		var user pgtype.UUID
		if err := admin.QueryRow(ctx, `INSERT INTO iam.users (id, email) SELECT i, i::text || '@perf.test' FROM gen_random_uuid() i RETURNING id`).Scan(&user); err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, ten := range tenantIDs {
			var membership pgtype.UUID
			if err := admin.QueryRow(ctx, `INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status)
				SELECT gen_random_uuid(), $1, $2, r.id, 'activo' FROM iam.roles r WHERE r.tenant_id = $1 AND r.template_code = 'cajero' RETURNING id`, ten, user).Scan(&membership); err != nil {
				t.Fatal(err)
			}
			var raw [32]byte
			_, _ = rand.Read(raw[:])
			token := "wk_at_" + base64.RawURLEncoding.EncodeToString(raw[:])
			hash := sha256.Sum256([]byte(token))
			if _, err := admin.Exec(ctx, `INSERT INTO iam.sessions (id, user_id, tenant_id, site_id, cash_register_id, membership_id, access_hash, access_expires_at,
				authz_version, mfa_satisfied, mfa_method, refresh_family) VALUES (gen_random_uuid(), $1, $2, gen_random_uuid(), gen_random_uuid(), $3, $4, now() + interval '1 hour', 1, true, 'none', gen_random_uuid())`,
				user, ten, membership, hash[:]); err != nil {
				t.Fatal(err)
			}
			out = append(out, token)
		}
		return out
	}
	tokens := make([][][]string, reps)
	for rep := range tokens {
		for u := 0; u < vus; u++ {
			tokens[rep] = append(tokens[rep], seedUser())
		}
	}
	warmup := seedUser()

	// Los limitadores por sesión y por empresa (conventions §7) devuelven 429 a esta ráfaga, que no es la carga que se mide:
	// se abren para que el 429 no cuente como error del paso de permisos. El cableado real lo prueba el otro test.
	huge := httpx.LimiterConfig{Capacity: 1 << 30, Refill: time.Nanosecond, MaxKeys: 100_000}
	w := defaultWiring
	w.auth.SessionLimiter, w.auth.TenantLimiter = huge, huge
	w.mount = func(r *auth.Routes) {
		r.HandleFunc("GET /v1/items", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:read"},
			func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	}
	addr := startServe(t, d, w, poolMax)
	inFlight := make(chan struct{}, poolMax) // nunca más peticiones a la vez que conexiones en el pool

	do := func(token string) (time.Duration, bool) {
		req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/v1/items", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		inFlight <- struct{}{}
		defer func() { <-inFlight }()
		start := time.Now()
		resp, err := http.DefaultClient.Do(req)
		el := time.Since(start)
		ok := err == nil && resp.StatusCode == http.StatusOK
		if err == nil {
			_ = resp.Body.Close()
		}
		if !ok {
			code := 0
			if err == nil {
				code = resp.StatusCode
			}
			t.Logf("failed request: status %d err %v", code, err)
		}
		return el, ok
	}
	// Calentamiento fuera de la medición: entitlements de las 20 empresas y conexiones HTTP y del pool.
	for _, token := range warmup {
		if _, ok := do(token); !ok {
			t.Fatal("warm-up request failed")
		}
	}

	p95 := func(l []time.Duration) time.Duration {
		slices.Sort(l)
		return l[len(l)*95/100]
	}
	var diffs []time.Duration
	errs, total := 0, 0
	for rep := range tokens {
		var (
			mu         sync.Mutex
			cold, warm []time.Duration
			wg         sync.WaitGroup
		)
		for u := range tokens[rep] {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for _, token := range tokens[rep][u] {
					c, ok1 := do(token) // caché fría
					h, ok2 := do(token) // caliente
					mu.Lock()
					cold, warm = append(cold, c), append(warm, h)
					total += 2
					if !ok1 {
						errs++
					}
					if !ok2 {
						errs++
					}
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		c95, w95 := p95(cold), p95(warm)
		diffs = append(diffs, c95-w95)
		t.Logf("rep %d: cold p95 %v, warm p95 %v, the permission step adds %v", rep+1, c95, w95, c95-w95)
	}
	sorted := slices.Clone(diffs)
	slices.Sort(sorted)
	median := sorted[len(sorted)/2]
	t.Logf("route with permission, cold cache, %d VUs, %d tenants, at most %d in flight: median added p95 of %d reps = %v (bound 50ms); errors %d/%d", vus, tenants, poolMax, reps, median, errs, total)
	if median >= 50*time.Millisecond {
		t.Errorf("the permission step adds %v at p95 (median of %d), want < 50ms", median, reps)
	}
	if float64(errs) >= 0.01*float64(total) {
		t.Errorf("errors = %d of %d, want < 1%%", errs, total)
	}
}

// Con el cableado de producción, una resolución que no termina a tiempo (aquí iam.role_template_permissions bloqueada
// por otra transacción) responde 503 dependency_unavailable y no deja pasar ni responde un 200 vacío; al liberar el
// bloqueo, la misma membresía vuelve a resolver.
func TestServeAnswers503WhenThePermissionLookupTimesOut(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	d.Migrate(ctx, t)
	admin := d.Admin(ctx, t)
	s := d.SeedSession(t, dbtest.SessionSpec{})
	grantVentas(ctx, t, admin)
	tx, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := identidad.SeedTenantRoles(ctx, tx, mustUUID(t, s.TenantID)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `UPDATE iam.memberships SET role_id = (SELECT id FROM iam.roles WHERE tenant_id = $1 AND template_code = 'cajero') WHERE id = $2`, s.TenantID, s.MembershipID); err != nil {
		t.Fatal(err)
	}

	w := defaultWiring
	w.auth.LookupTimeout = 300 * time.Millisecond
	w.mount = func(r *auth.Routes) {
		r.HandleFunc("GET /v1/items", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:read"},
			func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	}
	addr := startServe(t, d, w, 4)

	lock, err := d.Admin(ctx, t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lock.Exec(ctx, `LOCK TABLE iam.role_template_permissions IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	if code, p := get(t, addr, "/v1/items", s.Token); code != 503 || p != "dependency_unavailable" {
		t.Fatalf("locked templates: GET /v1/items = %d %q, want 503 dependency_unavailable", code, p)
	}
	if err := lock.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if code, p := get(t, addr, "/v1/items", s.Token); code != 200 {
		t.Fatalf("after the lock: GET /v1/items = %d %q, want 200", code, p)
	}
}

func mustUUID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatal(err)
	}
	return u
}
