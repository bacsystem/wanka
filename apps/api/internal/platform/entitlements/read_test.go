package entitlements_test

import (
	"slices"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
)

// Criterio 3: plan ± overrides + flags, con suscripción activa o trial.
func TestForCombinesPlanOverridesAndFlags(t *testing.T) {
	for _, status := range []string{"activa", "trial"} {
		t.Run(status, func(t *testing.T) {
			e := newEnv(t)
			tenant := e.tenant("activa")
			plan := e.plan("basic", `{"users": 5, "sites": 2, "cpe_per_month": 300}`, "ventas", "inventario", "compras")
			e.subscribe(tenant, plan, status)
			e.exec(`INSERT INTO platform.tenant_module_overrides (tenant_id, module_code, enabled, reason) VALUES ($1, 'compras', false, 'moroso en compras')`, tenant)
			e.exec(`INSERT INTO platform.tenant_module_overrides (tenant_id, module_code, enabled) VALUES ($1, 'agenda', true)`, tenant)
			e.exec(`INSERT INTO platform.feature_flags (key, default_on) VALUES ('nuevo-pos', true), ('beta', false), ('forzado', false), ('apagado', true)`)
			e.exec(`INSERT INTO platform.tenant_flag_overrides (tenant_id, flag_key, enabled) VALUES ($1, 'forzado', true), ($1, 'apagado', false)`, tenant)
			e.exec(`INSERT INTO platform.tenant_usage (tenant_id, period, cpe_issued, active_users) VALUES ($1, date_trunc('month', now() AT TIME ZONE 'America/Lima')::date, 120, 3)`, tenant)

			s := e.service(entitlements.Options{})
			got := mustFor(t, s, tenant)

			if want := []string{"agenda", "inventario", "ventas"}; !slices.Equal(got.Modules, want) {
				t.Errorf("Modules = %v, want %v (plan, less the disabled override, plus the enabled one)", got.Modules, want)
			}
			if want := []string{"forzado", "nuevo-pos"}; !slices.Equal(got.Flags, want) {
				t.Errorf("Flags = %v, want %v", got.Flags, want)
			}
			if got.Limits[entitlements.LimitUsers] != 5 || got.Limits[entitlements.LimitSites] != 2 || got.Limits[entitlements.LimitCPEPerMonth] != 300 {
				t.Errorf("Limits = %v", got.Limits)
			}
			if got.Usage[entitlements.LimitCPEPerMonth] != 120 || got.Usage[entitlements.LimitUsers] != 3 {
				t.Errorf("Usage = %v, want cpe 120 and users 3", got.Usage)
			}
			if string(got.Status) != status || got.ReadOnly || got.Version == "" {
				t.Errorf("Status/ReadOnly/Version = %q/%v/%q", got.Status, got.ReadOnly, got.Version)
			}
			if !got.HasModule("ventas") || got.HasModule("compras") || !got.HasFlag("forzado") || got.HasFlag("apagado") {
				t.Error("HasModule/HasFlag disagree with the lists")
			}
		})
	}
}

// Criterio 5 y la invariante «la falta de dato cierra»: una suscripción que no es usable no habilita nada.
func TestInactiveSubscriptionEnablesNothingAndIsFlagged(t *testing.T) {
	for _, status := range []string{"morosa", "suspendida", "cancelada"} {
		t.Run(status, func(t *testing.T) {
			e := newEnv(t)
			tenant := e.tenant("activa")
			plan := e.plan("p", `{"users": 5}`, "ventas", "inventario")
			e.subscribe(tenant, plan, status)
			e.exec(`INSERT INTO platform.tenant_module_overrides (tenant_id, module_code, enabled) VALUES ($1, 'agenda', true)`, tenant)
			e.exec(`INSERT INTO platform.feature_flags (key, default_on) VALUES ('f', true)`)

			got := mustFor(t, e.service(entitlements.Options{}), tenant)
			if !got.Status.Inactive() || got.Status.Usable() || string(got.Status) != status {
				t.Errorf("Status = %q, want %s flagged inactive", got.Status, status)
			}
			if len(got.Modules) != 0 || len(got.Flags) != 0 {
				t.Errorf("inactive subscription enables modules %v and flags %v", got.Modules, got.Flags)
			}
			if got.ReadOnly {
				t.Error("ReadOnly must only mean a closed company")
			}
		})
	}
}

// Una cancelada más una viva: cuenta la viva. Solo canceladas: cuenta la más reciente (inactiva).
func TestTheLiveSubscriptionWinsOverCancelledOnes(t *testing.T) {
	e := newEnv(t)
	tenant := e.tenant("activa")
	old := e.plan("old", `{}`, "compras")
	cur := e.plan("cur", `{}`, "ventas")
	e.subscribe(tenant, old, "cancelada")
	e.subscribe(tenant, cur, "activa")
	got := mustFor(t, e.service(entitlements.Options{}), tenant)
	if !got.Status.Usable() || !slices.Equal(got.Modules, []string{"ventas"}) {
		t.Errorf("got %v %v, want the live subscription's plan", got.Status, got.Modules)
	}
}

// Criterio 5: empresa cerrada -> readOnly, y su suscripción se sigue leyendo aparte.
func TestClosedTenantIsReadOnly(t *testing.T) {
	e := newEnv(t)
	closed := e.tenant("cerrada")
	open := e.tenant("activa")
	plan := e.plan("p", `{}`, "ventas")
	e.subscribe(closed, plan, "activa")
	e.subscribe(open, plan, "activa")
	s := e.service(entitlements.Options{})

	if got := mustFor(t, s, closed); !got.ReadOnly || !got.HasModule("ventas") {
		t.Errorf("closed tenant: readOnly=%v modules=%v, want readOnly and the plan's modules", got.ReadOnly, got.Modules)
	}
	if got := mustFor(t, s, open); got.ReadOnly {
		t.Error("an open tenant is read only")
	}
	// Suspendida (estado de org.tenants) no es cerrada.
	if got := mustFor(t, s, e.tenant("suspendida")); got.ReadOnly {
		t.Error("a suspended tenant is read only")
	}
}

// Criterio 6 e invariante: sin suscripción no hay ningún módulo, aunque existan el plan semilla,
// overrides y flags encendidos para todos.
func TestTenantWithoutSubscriptionGetsNoPlanNotTheSeedPlan(t *testing.T) {
	e := newEnv(t)
	tenant := e.tenant("activa")
	e.exec(`INSERT INTO platform.tenant_module_overrides (tenant_id, module_code, enabled) VALUES ($1, 'ventas', true)`, tenant)
	e.exec(`INSERT INTO platform.feature_flags (key, default_on, rollout_pct) VALUES ('f', true, 100)`)
	e.exec(`INSERT INTO platform.tenant_usage (tenant_id, period, cpe_issued) VALUES ($1, date_trunc('month', now() AT TIME ZONE 'America/Lima')::date, 7)`, tenant)

	got := mustFor(t, e.service(entitlements.Options{}), tenant)
	if got.Status != entitlements.StatusNoPlan || got.Status.Usable() || got.Status.Inactive() {
		t.Errorf("Status = %q, want %q", got.Status, entitlements.StatusNoPlan)
	}
	if len(got.Modules) != 0 || len(got.Flags) != 0 || len(got.Limits) != 0 {
		t.Errorf("no plan enables modules %v flags %v limits %v", got.Modules, got.Flags, got.Limits)
	}
	if got.Modules == nil || got.Flags == nil {
		t.Error("empty lists must be empty, not nil (JSON [] and not null)")
	}
}

// Criterio 4: el rollout es determinista por hash(tenant_id, flag), igual en cualquier réplica;
// un override del tenant gana sobre el porcentaje.
func TestFlagRolloutIsDeterministicAndOverridesWin(t *testing.T) {
	e := newEnv(t)
	plan := e.plan("p", `{}`, "ventas")
	e.exec(`INSERT INTO platform.feature_flags (key, rollout_pct) VALUES ('r0', 0), ('r30', 30), ('r100', 100)`)

	const tenants = 400
	ids := make([]string, 0, tenants)
	for range tenants {
		id := e.tenant("activa")
		e.subscribe(id, plan, "activa")
		ids = append(ids, id.String())
	}
	// Dos réplicas (dos servicios con su propio pool) coinciden empresa por empresa.
	a, b := e.service(entitlements.Options{}), e.service(entitlements.Options{})
	on30 := 0
	for _, raw := range ids {
		var u = e.uuidOf(raw)
		ga, gb := mustFor(t, a, u), mustFor(t, b, u)
		if !slices.Equal(ga.Flags, gb.Flags) || ga.Version != gb.Version {
			t.Fatalf("replicas disagree for %s: %v vs %v", raw, ga.Flags, gb.Flags)
		}
		if ga.HasFlag("r0") {
			t.Fatalf("rollout 0 enabled the flag for %s", raw)
		}
		if !ga.HasFlag("r100") {
			t.Fatalf("rollout 100 left %s out", raw)
		}
		if ga.HasFlag("r30") {
			on30++
		}
	}
	// 30 % de 400 = 120; con un hash uniforme la desviación típica es ~9.
	if on30 < 80 || on30 > 160 {
		t.Errorf("rollout 30 %% enabled %d of %d tenants", on30, tenants)
	}

	// Un override gana en los dos sentidos.
	var inside, outside string
	for _, raw := range ids {
		if mustFor(t, a, e.uuidOf(raw)).HasFlag("r30") {
			inside = raw
		} else {
			outside = raw
		}
	}
	e.exec(`INSERT INTO platform.tenant_flag_overrides (tenant_id, flag_key, enabled) VALUES ($1, 'r30', false), ($2, 'r30', true), ($2, 'r0', true)`,
		e.uuidOf(inside), e.uuidOf(outside))
	fresh := e.service(entitlements.Options{})
	if mustFor(t, fresh, e.uuidOf(inside)).HasFlag("r30") {
		t.Error("an override disabling the flag lost against the rollout percentage")
	}
	if g := mustFor(t, fresh, e.uuidOf(outside)); !g.HasFlag("r30") || !g.HasFlag("r0") {
		t.Error("an override enabling the flag lost against the rollout percentage")
	}
}

// Criterio 9: dos empresas con planes distintos ven solo lo suyo, en un mismo servicio y una misma caché.
func TestEachTenantSeesOnlyItsOwnPlan(t *testing.T) {
	e := newEnv(t)
	a, b := e.tenant("activa"), e.tenant("activa")
	e.subscribe(a, e.plan("pa", `{"users": 1}`, "ventas", "rubro.lubricentro"), "activa")
	e.subscribe(b, e.plan("pb", `{"users": 9}`, "inventario", "rubro.libreria"), "trial")
	e.exec(`INSERT INTO platform.tenant_module_overrides (tenant_id, module_code, enabled) VALUES ($1, 'agenda', true)`, b)
	e.exec(`INSERT INTO platform.tenant_usage (tenant_id, period, cpe_issued) VALUES ($1, date_trunc('month', now() AT TIME ZONE 'America/Lima')::date, 50)`, b)
	e.exec(`INSERT INTO platform.feature_flags (key) VALUES ('solo-b')`)
	e.exec(`INSERT INTO platform.tenant_flag_overrides (tenant_id, flag_key, enabled) VALUES ($1, 'solo-b', true)`, b)

	s := e.service(entitlements.Options{})
	for range 2 { // la segunda vuelta sale de la caché: tampoco se mezclan ahí
		ga, gb := mustFor(t, s, a), mustFor(t, s, b)
		if !slices.Equal(ga.Modules, []string{"rubro.lubricentro", "ventas"}) || ga.Limits[entitlements.LimitUsers] != 1 ||
			len(ga.Flags) != 0 || ga.Usage[entitlements.LimitCPEPerMonth] != 0 || ga.Status != entitlements.StatusActive {
			t.Errorf("tenant A sees %+v", ga)
		}
		if !slices.Equal(gb.Modules, []string{"agenda", "inventario", "rubro.libreria"}) || gb.Limits[entitlements.LimitUsers] != 9 ||
			!slices.Equal(gb.Flags, []string{"solo-b"}) || gb.Usage[entitlements.LimitCPEPerMonth] != 50 || gb.Status != entitlements.StatusTrial {
			t.Errorf("tenant B sees %+v", gb)
		}
	}
}

// El uso es el del mes de Lima: a las 22:00 del 30 de noviembre en Lima ya es 1 de diciembre en UTC.
func TestUsageIsTheCurrentMonthInLima(t *testing.T) {
	e := newEnv(t)
	tenant := e.tenant("activa")
	e.subscribe(tenant, e.plan("p", `{}`, "ventas"), "activa")
	e.exec(`INSERT INTO platform.tenant_usage (tenant_id, period, cpe_issued) VALUES ($1, '2026-11-01', 11), ($1, '2026-12-01', 12)`, tenant)

	clk := newFake()
	clk.Advance(time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC).Sub(clk.Now())) // 22:00 del 30 de nov. en Lima
	if got := mustFor(t, e.service(entitlements.Options{Clock: clk}), tenant); got.Usage[entitlements.LimitCPEPerMonth] != 11 {
		t.Errorf("at 22:00 Lima on Nov 30 the usage is %d, want November's 11", got.Usage[entitlements.LimitCPEPerMonth])
	}
	clk.Advance(3 * time.Hour) // 01:00 del 1 de dic. en Lima
	if got := mustFor(t, e.service(entitlements.Options{Clock: clk}), tenant); got.Usage[entitlements.LimitCPEPerMonth] != 12 {
		t.Errorf("at 01:00 Lima on Dec 1 the usage is %d, want December's 12", got.Usage[entitlements.LimitCPEPerMonth])
	}
}

// Version: cambia con lo que la empresa puede hacer, no con el uso.
func TestVersionChangesWithTheContentNotWithTheUsage(t *testing.T) {
	e := newEnv(t)
	tenant := e.tenant("activa")
	e.subscribe(tenant, e.plan("p", `{}`, "ventas"), "activa")
	clk := newFake()
	s := e.service(entitlements.Options{Clock: clk, TTL: time.Second})

	v1 := mustFor(t, s, tenant).Version
	e.exec(`INSERT INTO platform.tenant_usage (tenant_id, period, cpe_issued) VALUES ($1, date_trunc('month', now() AT TIME ZONE 'America/Lima')::date, 9)`, tenant)
	clk.Advance(2 * time.Second)
	if v := mustFor(t, s, tenant).Version; v != v1 {
		t.Errorf("version changed with the usage: %s -> %s", v1, v)
	}
	e.exec(`INSERT INTO platform.tenant_module_overrides (tenant_id, module_code, enabled) VALUES ($1, 'agenda', true)`, tenant)
	clk.Advance(2 * time.Second)
	if v := mustFor(t, s, tenant).Version; v == v1 {
		t.Error("version did not change when a module was enabled")
	}
}

// Desactivar un módulo no borra datos: el override es una fila, el plan sigue intacto, y reactivarlo lo recupera.
func TestDisablingAModuleKeepsTheDataAndReenablingRecoversIt(t *testing.T) {
	e := newEnv(t)
	tenant := e.tenant("activa")
	e.subscribe(tenant, e.plan("p", `{}`, "ventas", "inventario"), "activa")
	clk := newFake()
	s := e.service(entitlements.Options{Clock: clk, TTL: time.Second})

	e.exec(`INSERT INTO platform.tenant_module_overrides (tenant_id, module_code, enabled) VALUES ($1, 'inventario', false)`, tenant)
	clk.Advance(2 * time.Second)
	if got := mustFor(t, s, tenant); got.HasModule("inventario") {
		t.Fatal("disabled module still enabled")
	}
	var planModules int
	if err := e.admin.QueryRow(ctx(t), `SELECT count(*) FROM platform.plan_modules pm JOIN platform.plans p ON p.id = pm.plan_id WHERE p.code = 'p'`).Scan(&planModules); err != nil || planModules != 2 {
		t.Errorf("plan modules after disabling = %d, %v; want 2 (nothing deleted)", planModules, err)
	}
	e.exec(`DELETE FROM platform.tenant_module_overrides WHERE tenant_id = $1`, tenant)
	clk.Advance(2 * time.Second)
	if got := mustFor(t, s, tenant); !got.HasModule("inventario") {
		t.Error("reenabling the module did not recover it")
	}
}
