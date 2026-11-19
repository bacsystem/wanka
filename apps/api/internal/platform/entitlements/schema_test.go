package entitlements_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
)

var platformTables = []string{
	"modules", "plans", "plan_modules", "subscriptions",
	"tenant_module_overrides", "feature_flags", "tenant_flag_overrides", "tenant_usage",
}

// Criterio 1: las ocho tablas existen en platform, sin RLS, con los GRANT del ADR 0001.
func TestTablesExistWithoutRLSAndWithTheADRGrants(t *testing.T) {
	e := newEnv(t)
	c := ctx(t)
	for _, tbl := range platformTables {
		var rls bool
		if err := e.admin.QueryRow(c, `SELECT c.relrowsecurity OR c.relforcerowsecurity FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'platform' AND c.relname = $1`, tbl).Scan(&rls); err != nil {
			t.Fatalf("platform.%s should exist: %v", tbl, err)
		}
		if rls {
			t.Errorf("platform.%s has RLS: the platform tables are global (ADR 0001)", tbl)
		}
		for _, role := range []string{dbtest.App, dbtest.Worker, dbtest.Webhooks, dbtest.Platform} {
			for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
				var has bool
				if err := e.admin.QueryRow(c, `SELECT has_table_privilege($1, $2, $3)`, role, "platform."+tbl, priv).Scan(&has); err != nil {
					t.Fatal(err)
				}
				// Matriz: app y worker leen todo; el worker escribe solo tenant_usage; nadie más recibe nada.
				want := priv == "SELECT" && (role == dbtest.App || role == dbtest.Worker) ||
					tbl == "tenant_usage" && role == dbtest.Worker && (priv == "INSERT" || priv == "UPDATE")
				if has != want {
					t.Errorf("%s %s on platform.%s = %v, want %v", role, priv, tbl, has, want)
				}
			}
		}
	}
}

// Los GRANT se ejercen con las conexiones reales, no solo con el catálogo.
func TestRealRolesCannotWriteExceptTheWorkerUsage(t *testing.T) {
	e := newEnv(t)
	c := ctx(t)
	tenant := e.tenant("activa")
	connect := func(role string) *pgx.Conn {
		conn, err := pgx.Connect(c, e.d.URL(role))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close(context.Background()) })
		return conn
	}
	app, worker := connect(dbtest.App), connect(dbtest.Worker)

	denied := func(conn *pgx.Conn, what, sql string, args ...any) {
		t.Helper()
		_, err := conn.Exec(c, sql, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%s: err = %v, want SQLSTATE 42501", what, err)
		}
	}
	denied(app, "app writes subscriptions", `UPDATE platform.subscriptions SET status = 'activa'`)
	denied(app, "app inserts overrides", `INSERT INTO platform.tenant_module_overrides (tenant_id, module_code, enabled) VALUES ($1, 'ventas', true)`, tenant)
	denied(app, "app writes usage", `INSERT INTO platform.tenant_usage (tenant_id, period) VALUES ($1, '2026-11-01')`, tenant)
	denied(app, "app deletes plans", `DELETE FROM platform.plans`)
	denied(worker, "worker writes subscriptions", `UPDATE platform.subscriptions SET status = 'activa'`)
	denied(worker, "worker writes plans", `UPDATE platform.plans SET name = 'x'`)
	denied(worker, "worker deletes usage", `DELETE FROM platform.tenant_usage`)

	// El worker sí escribe el uso (upsert, que además lee el conflicto).
	for range 2 {
		if _, err := worker.Exec(c, `INSERT INTO platform.tenant_usage (tenant_id, period, cpe_issued) VALUES ($1, '2026-11-01', 5)
			ON CONFLICT (tenant_id, period) DO UPDATE SET cpe_issued = platform.tenant_usage.cpe_issued + EXCLUDED.cpe_issued`, tenant); err != nil {
			t.Fatalf("worker upsert of tenant_usage: %v", err)
		}
	}
	var n int
	if err := app.QueryRow(c, `SELECT cpe_issued FROM platform.tenant_usage WHERE tenant_id = $1`, tenant).Scan(&n); err != nil || n != 10 {
		t.Errorf("app reads usage = %d, %v; want 10", n, err)
	}
}

// Una sola suscripción viva por empresa; las canceladas son historial.
func TestOneLiveSubscriptionPerTenant(t *testing.T) {
	e := newEnv(t)
	tenant := e.tenant("activa")
	plan := e.plan("p", `{}`, "ventas")
	e.subscribe(tenant, plan, "cancelada")
	e.subscribe(tenant, plan, "cancelada") // varias canceladas: historial
	e.subscribe(tenant, plan, "activa")
	_, err := e.admin.Exec(ctx(t), `INSERT INTO platform.subscriptions (id, tenant_id, plan_id, status) VALUES (gen_random_uuid(), $1, $2, 'trial')`, tenant, plan)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("second live subscription: err = %v, want unique violation", err)
	}
}

// Criterio 2: la semilla lista los módulos del ADR 0005 y el plan «semilla» los tiene todos.
func TestSeedHasEveryADRModuleAndASeedPlanWithAllOfThem(t *testing.T) {
	e := newEnv(t)
	c := ctx(t)
	rows, err := e.admin.Query(c, `SELECT code, kind FROM platform.modules ORDER BY code`)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for rows.Next() {
		var code, kind string
		if err := rows.Scan(&code, &kind); err != nil {
			t.Fatal(err)
		}
		kinds[code] = kind
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	// ADR 0005: core, los seis rubros y los addons que nombra.
	for _, code := range []string{"ventas", "inventario", "compras", "rubro.lubricentro", "rubro.libreria", "rubro.restaurante",
		"rubro.veterinaria", "rubro.odontologia", "rubro.retail", "agenda", "integraciones.whatsapp"} {
		if _, ok := kinds[code]; !ok {
			t.Errorf("module %q is missing from platform.modules", code)
		}
	}
	for code, kind := range kinds {
		switch {
		case len(code) > 6 && code[:6] == "rubro.":
			if kind != "rubro" {
				t.Errorf("%s kind = %s, want rubro", code, kind)
			}
		case code == "agenda" || code == "integraciones.whatsapp":
			if kind != "addon" {
				t.Errorf("%s kind = %s, want addon", code, kind)
			}
		default:
			if kind != "core" {
				t.Errorf("%s kind = %s, want core", code, kind)
			}
		}
	}

	var nonProd bool
	var limits string
	var inPlan, total int
	if err := e.admin.QueryRow(c, `SELECT p.non_production, p.limits::text,
		(SELECT count(*) FROM platform.plan_modules pm WHERE pm.plan_id = p.id), (SELECT count(*) FROM platform.modules)
		FROM platform.plans p WHERE p.code = 'semilla'`).Scan(&nonProd, &limits, &inPlan, &total); err != nil {
		t.Fatalf("plan semilla: %v", err)
	}
	if !nonProd {
		t.Error("plan semilla is not marked non_production")
	}
	if inPlan != total || total != len(kinds) {
		t.Errorf("plan semilla has %d of %d modules", inPlan, total)
	}
	var n int
	if err := e.admin.QueryRow(c, `SELECT count(*) FROM platform.subscriptions`).Scan(&n); err != nil || n != 0 {
		t.Errorf("subscriptions after migrating = %d, %v; the seed plan must not be assigned to anyone", n, err)
	}
}

// Los cambios que importan avisan por NOTIFY entitlements_changed con el tenant afectado, o vacío si alcanzan a todos.
func TestChangesNotifyTheAffectedTenant(t *testing.T) {
	e := newEnv(t)
	c := ctx(t)
	tenant := e.tenant("activa")
	plan := e.plan("p", `{}`, "ventas")

	listener, err := pgx.Connect(c, e.d.SuperuserURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close(context.Background()) })
	if _, err := listener.Exec(c, "LISTEN "+entitlements.Channel); err != nil {
		t.Fatal(err)
	}
	// El LISTEN empieza a ver los NOTIFY de transacciones confirmadas después de él.
	next := func() string {
		t.Helper()
		wc, cancel := context.WithTimeout(c, 5*time.Second)
		defer cancel()
		n, err := listener.WaitForNotification(wc)
		if err != nil {
			t.Fatalf("waiting for NOTIFY: %v", err)
		}
		return n.Payload
	}
	tid := tenant.String()

	e.subscribe(tenant, plan, "activa")
	if got := next(); got != tid {
		t.Errorf("subscription insert: payload = %q, want the tenant %s", got, tid)
	}
	e.exec(`UPDATE platform.subscriptions SET status = 'morosa' WHERE tenant_id = $1`, tenant)
	if got := next(); got != tid {
		t.Errorf("subscription update: payload = %q, want the tenant", got)
	}
	e.exec(`INSERT INTO platform.tenant_module_overrides (tenant_id, module_code, enabled) VALUES ($1, 'agenda', true)`, tenant)
	if got := next(); got != tid {
		t.Errorf("module override: payload = %q, want the tenant", got)
	}
	e.exec(`INSERT INTO platform.feature_flags (key) VALUES ('f1')`)
	if got := next(); got != "" {
		t.Errorf("feature flag insert: payload = %q, want empty (all tenants)", got)
	}
	e.exec(`INSERT INTO platform.tenant_flag_overrides (tenant_id, flag_key, enabled) VALUES ($1, 'f1', true)`, tenant)
	if got := next(); got != tid {
		t.Errorf("flag override: payload = %q, want the tenant", got)
	}
	e.exec(`UPDATE org.tenants SET status = 'cerrada', closed_at = now() WHERE id = $1`, tenant)
	if got := next(); got != tid {
		t.Errorf("closing the tenant: payload = %q, want the tenant", got)
	}
	e.exec(`INSERT INTO platform.plan_modules (plan_id, module_code) VALUES ($1, 'agenda')`, plan)
	if got := next(); got != "" {
		t.Errorf("plan modules: payload = %q, want empty", got)
	}
	e.exec(`UPDATE platform.plans SET name = 'otro' WHERE id = $1`, plan)
	if got := next(); got != "" {
		t.Errorf("plan update: payload = %q, want empty", got)
	}
	// El uso cambia con cada comprobante: no notifica.
	e.exec(`INSERT INTO platform.tenant_usage (tenant_id, period, cpe_issued) VALUES ($1, '2026-11-01', 1)`, tenant)
	e.exec(`UPDATE platform.modules SET description = description || '.' WHERE code = 'agenda'`)
	if got := next(); got != "" {
		t.Errorf("after a usage write and a module update the next payload = %q, want the module's (empty)", got)
	}
}

// Todo plan declara exactamente las tres claves de límite, números enteros no negativos: cierra, nunca "sin tope".
func TestPlanLimitsCheckRejectsEverythingButTheThreeIntegerKeys(t *testing.T) {
	e := newEnv(t)
	c := ctx(t)
	for name, limits := range map[string]string{
		"missing key":   `{"users": 1, "sites": 1}`,
		"empty":         `{}`,
		"extra key":     `{"users": 1, "sites": 1, "cpe_per_month": 1, "otro": 1}`,
		"string":        `{"users": "muchos", "sites": 1, "cpe_per_month": 1}`,
		"negative":      `{"users": -1, "sites": 1, "cpe_per_month": 1}`,
		"decimal":       `{"users": 1.5, "sites": 1, "cpe_per_month": 1}`,
		"null":          `{"users": null, "sites": 1, "cpe_per_month": 1}`,
		"too big":       `{"users": 1e30, "sites": 1, "cpe_per_month": 1}`,
		"boolean":       `{"users": true, "sites": 1, "cpe_per_month": 1}`,
		"array":         `{"users": [1], "sites": 1, "cpe_per_month": 1}`,
		"not an object": `[1, 2, 3]`,
	} {
		_, err := e.admin.Exec(c, `INSERT INTO platform.plans (code, name, limits) VALUES ($1, $1, $2::jsonb)`, "bad-"+name, limits)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("%s: err = %v, want SQLSTATE 23514", name, err)
		}
	}
	// Los tres enteros en el borde sí entran.
	if _, err := e.admin.Exec(c, `INSERT INTO platform.plans (code, name, limits) VALUES ('ok', 'ok', '{"users": 0, "sites": 9223372036854775807, "cpe_per_month": 3}')`); err != nil {
		t.Errorf("valid limits rejected: %v", err)
	}
	if _, err := e.admin.Exec(c, `INSERT INTO platform.plans (code, name) VALUES ('nolimits', 'x')`); err == nil {
		t.Error("a plan without limits was accepted (the column has no default)")
	}
}

// «none» está reservado en los contratos para «sin módulo»: no puede ser un código de módulo.
func TestModuleCodeNoneIsReserved(t *testing.T) {
	e := newEnv(t)
	_, err := e.admin.Exec(ctx(t), `INSERT INTO platform.modules (code, kind, description) VALUES ('none', 'core', 'x')`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Errorf("module none: err = %v, want SQLSTATE 23514", err)
	}
}
