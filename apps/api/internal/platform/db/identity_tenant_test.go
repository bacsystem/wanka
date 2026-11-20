package db_test

// Identidad de la ola 1, F2 (#232): tablas con RLS, plantillas de rol, triggers de guarda, políticas por usuario y la
// vista iam.list_my_memberships. Todo corre con los roles reales de la aplicación; el superusuario solo siembra (los
// triggers de guarda lo dejan pasar, como a las semillas de prueba).

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// Ids fijos del escenario: dos empresas, tres usuarios.
const (
	tenA, tenB       = "aaaaaaaa-0000-0000-0000-000000000001", "bbbbbbbb-0000-0000-0000-000000000002"
	usr1, usr2, usr3 = "11111111-0000-0000-0000-000000000001", "11111111-0000-0000-0000-000000000002", "11111111-0000-0000-0000-000000000003"
	roleA, roleB     = "a0000000-0000-0000-0000-0000000000a1", "b0000000-0000-0000-0000-0000000000b1"
	siteA1, siteA2   = "a5000000-0000-0000-0000-0000000000a1", "a5000000-0000-0000-0000-0000000000a2"
	siteB1           = "b5000000-0000-0000-0000-0000000000b1"
	crA1, crA2, crA3 = "ca000000-0000-0000-0000-0000000000a1", "ca000000-0000-0000-0000-0000000000a2", "ca000000-0000-0000-0000-0000000000a3"
	crB1             = "cb000000-0000-0000-0000-0000000000b1"
	memU1A, memU1B   = "3e000000-0000-0000-0000-0000000000a1", "3e000000-0000-0000-0000-0000000000b1"
	memU2A, memU3A   = "3e000000-0000-0000-0000-0000000000a2", "3e000000-0000-0000-0000-0000000000a3"
)

// seedIdentity siembra, como superusuario, el escenario que usan casi todos los casos:
//   - u1 es miembro activo de A (solo la sede A1) y de B (todas las sedes); u2 está invitado en A; u3 suspendido en A;
//   - A tiene dos sedes (A1 con dos cajas, A2 con una); B tiene una sede con una caja;
//   - plantillas: dueno exige 2FA, cajero no.
func seedIdentity(ctx context.Context, t *testing.T, d *dbtest.DB) *pgx.Conn {
	t.Helper()
	admin := d.Admin(ctx, t)
	for _, q := range []string{
		`INSERT INTO iam.users (id) VALUES ('` + usr1 + `'), ('` + usr2 + `'), ('` + usr3 + `')`,
		`INSERT INTO org.tenants (id, ruc, legal_name, trade_name, industry) VALUES
		   ('` + tenA + `', '20100000001', 'Razon A SAC', 'Comercial A', 'retail'),
		   ('` + tenB + `', '20100000002', 'Razon B SAC', NULL, 'veterinaria')`,
		`INSERT INTO iam.role_templates (code, name, requires_mfa) VALUES ('dueno', 'Dueño', true), ('cajero', 'Cajero', false)`,
		`INSERT INTO iam.roles (id, tenant_id, name, requires_mfa) VALUES ('` + roleA + `', '` + tenA + `', 'Supervisor', false),
		   ('` + roleB + `', '` + tenB + `', 'Supervisor', false)`,
		`INSERT INTO org.sites (id, tenant_id, code, name) VALUES ('` + siteA1 + `', '` + tenA + `', 'S1', 'Sede 1'),
		   ('` + siteA2 + `', '` + tenA + `', 'S2', 'Sede 2'), ('` + siteB1 + `', '` + tenB + `', 'S1', 'Sede B')`,
		`INSERT INTO org.cash_registers (id, tenant_id, site_id, code) VALUES ('` + crA1 + `', '` + tenA + `', '` + siteA1 + `', 'CAJA-01'),
		   ('` + crA2 + `', '` + tenA + `', '` + siteA1 + `', 'CAJA-02'), ('` + crA3 + `', '` + tenA + `', '` + siteA2 + `', 'CAJA-03'),
		   ('` + crB1 + `', '` + tenB + `', '` + siteB1 + `', 'CAJA-01')`,
		`INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, all_sites, status) VALUES
		   ('` + memU1A + `', '` + tenA + `', '` + usr1 + `', '` + roleA + `', false, 'activo'),
		   ('` + memU1B + `', '` + tenB + `', '` + usr1 + `', '` + roleB + `', true, 'activo'),
		   ('` + memU2A + `', '` + tenA + `', '` + usr2 + `', '` + roleA + `', true, 'invitado'),
		   ('` + memU3A + `', '` + tenA + `', '` + usr3 + `', '` + roleA + `', true, 'suspendido')`,
		`INSERT INTO iam.membership_sites (tenant_id, membership_id, site_id) VALUES ('` + tenA + `', '` + memU1A + `', '` + siteA1 + `')`,
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatalf("seeding %q: %v", q, err)
		}
	}
	return admin
}

// asApp ejecuta sql como wanka_app con las variables de la petición en una transacción que siempre se revierte, y
// devuelve el error de la sentencia. tenant o user vacíos no se fijan.
func asApp(t *testing.T, c *pgx.Conn, tenant, user, sql string, args ...any) error {
	t.Helper()
	ctx := context.Background()
	tx, err := c.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // la prueba nunca confirma
	setVars(t, tx, tenant, user)
	_, err = tx.Exec(ctx, sql, args...)
	return err
}

func setVars(t *testing.T, tx pgx.Tx, tenant, user string) {
	t.Helper()
	ctx := context.Background()
	if tenant != "" {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenant); err != nil {
			t.Fatal(err)
		}
	}
	if user != "" {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.user_id', $1, true)`, user); err != nil {
			t.Fatal(err)
		}
	}
}

// appStrings corre una consulta de una columna de texto como wanka_app con esas variables y devuelve los valores ordenados.
func appStrings(t *testing.T, c *pgx.Conn, tenant, user, sql string) []string {
	t.Helper()
	ctx := context.Background()
	tx, err := c.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // solo lectura
	setVars(t, tx, tenant, user)
	rows, err := tx.Query(ctx, sql)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func wantStrings(t *testing.T, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIdentityMigrationShape(t *testing.T) {
	ctx := context.Background()
	d := migrated(t)
	admin := d.Admin(ctx, t)

	// RLS forzada y tenant_isolation en las cuatro tablas nuevas de tenant.
	for _, tbl := range []string{"org.sites", "org.cash_registers", "iam.role_permissions", "iam.membership_sites"} {
		if !queryOne[bool](t, admin, `SELECT relrowsecurity AND relforcerowsecurity FROM pg_class WHERE oid = $1::regclass`, tbl) {
			t.Errorf("%s: RLS is not enabled and forced", tbl)
		}
		if n := queryOne[int](t, admin, `SELECT count(*) FROM pg_policies WHERE schemaname || '.' || tablename = $1 AND policyname = 'tenant_isolation'`, tbl); n != 1 {
			t.Errorf("%s: tenant_isolation policies = %d, want 1", tbl, n)
		}
	}
	// Las plantillas son un catálogo global sin RLS y nacen vacías (las siembra #233).
	for _, tbl := range []string{"iam.role_templates", "iam.role_template_permissions"} {
		if queryOne[bool](t, admin, `SELECT relrowsecurity FROM pg_class WHERE oid = $1::regclass`, tbl) {
			t.Errorf("%s must not have RLS", tbl)
		}
		if n := queryOne[int](t, admin, `SELECT count(*) FROM `+tbl); n != 0 {
			t.Errorf("%s has %d rows, want it empty until #233", tbl, n)
		}
	}
	// Defaults y nulabilidad.
	if got := queryOne[string](t, admin, `SELECT column_default FROM information_schema.columns WHERE table_schema = 'iam' AND table_name = 'roles' AND column_name = 'version'`); got != "1" {
		t.Errorf("roles.version default = %q, want 1", got)
	}
	for _, col := range []string{"ruc", "legal_name", "trade_name", "industry"} {
		if got := queryOne[string](t, admin, `SELECT is_nullable FROM information_schema.columns WHERE table_schema = 'org' AND table_name = 'tenants' AND column_name = $1`, col); got != "YES" {
			t.Errorf("org.tenants.%s nullable = %s, want YES", col, got)
		}
	}
	// Lista escrita a mano de los triggers y de lo que NO es definer.
	for _, fn := range []string{"iam.guard_role_writes()", "iam.guard_membership_deletes()"} {
		if queryOne[bool](t, admin, `SELECT prosecdef FROM pg_proc WHERE oid = $1::regprocedure`, fn) {
			t.Errorf("%s must not be SECURITY DEFINER", fn)
		}
		if got := queryOne[string](t, admin, `SELECT proconfig::text FROM pg_proc WHERE oid = $1::regprocedure`, fn); got != `{"search_path=pg_catalog, pg_temp"}` {
			t.Errorf("%s proconfig = %s", fn, got)
		}
	}
	// Índices que usa la vista y que cubren las FK, con lista escrita a mano: el plan lo decide el planificador, que con pocas
	// filas elige bien un Seq Scan, así que se comprueba que existan.
	for _, idx := range []string{
		"memberships_user_id_idx", "sites_tenant_id_id_key", "cash_registers_site_idx", "membership_sites_pkey",
		"membership_sites_site_idx", "role_permissions_pkey", "role_permissions_permission_idx", "roles_template_code_idx",
		"roles_tenant_lower_name_key", "roles_tenant_template_key", "role_template_permissions_permission_idx",
	} {
		if !queryOne[bool](t, admin, `SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = $1 AND schemaname IN ('iam', 'org'))`, idx) {
			t.Errorf("index %s does not exist", idx)
		}
	}
	if !queryOne[bool](t, admin, `SELECT reloptions @> ARRAY['security_invoker=true'] FROM pg_class WHERE oid = 'iam.list_my_memberships'::regclass`) {
		t.Error("iam.list_my_memberships must be security_invoker")
	}
}

func TestIdentityTenantsColumnChecks(t *testing.T) {
	ctx := context.Background()
	d := migrated(t)
	admin := d.Admin(ctx, t)
	for _, tc := range []struct{ name, sql, state string }{
		{"a row without the new columns is valid", `INSERT INTO org.tenants (id) VALUES (gen_random_uuid())`, ""},
		{"ruc with letters", `INSERT INTO org.tenants (id, ruc) VALUES (gen_random_uuid(), '2010000000A')`, "23514"},
		{"ruc with 10 digits", `INSERT INTO org.tenants (id, ruc) VALUES (gen_random_uuid(), '2010000000')`, "23514"},
		{"ruc with 12 digits", `INSERT INTO org.tenants (id, ruc) VALUES (gen_random_uuid(), '201000000011')`, "23514"},
		{"ruc with 11 digits", `INSERT INTO org.tenants (id, ruc) VALUES (gen_random_uuid(), '20100000009')`, ""},
		{"ruc is unique across the platform", `INSERT INTO org.tenants (id, ruc) VALUES (gen_random_uuid(), '20100000009')`, "23505"},
		{"unknown industry", `INSERT INTO org.tenants (id, industry) VALUES (gen_random_uuid(), 'minería')`, "23514"},
		{"known industry", `INSERT INTO org.tenants (id, industry) VALUES (gen_random_uuid(), 'odontologia')`, ""},
		{"empty legal name", `INSERT INTO org.tenants (id, legal_name) VALUES (gen_random_uuid(), '  ')`, "23514"},
		{"legal name of 200 characters", `INSERT INTO org.tenants (id, legal_name) VALUES (gen_random_uuid(), repeat('a', 200))`, ""},
		{"legal name of 201 characters", `INSERT INTO org.tenants (id, legal_name) VALUES (gen_random_uuid(), repeat('a', 201))`, "23514"},
		{"empty trade name", `INSERT INTO org.tenants (id, trade_name) VALUES (gen_random_uuid(), '')`, "23514"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := admin.Exec(ctx, tc.sql)
			if tc.state == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			wantState(t, err, tc.state, tc.name)
		})
	}
}

func TestIdentityKeysAndCrossTenantRejections(t *testing.T) {
	ctx := context.Background()
	d := migrated(t)
	seedIdentity(ctx, t, d)
	app := connectAs(t, d, dbtest.App)
	const perm = "identidad.users:read"
	for _, tc := range []struct {
		name, tenant, sql, state string
	}{
		{"role_permissions: own role and a catalog permission", tenA, `INSERT INTO iam.role_permissions VALUES ('` + tenA + `', '` + roleA + `', '` + perm + `')`, ""},
		{"role_permissions: a role of another tenant", tenA, `INSERT INTO iam.role_permissions VALUES ('` + tenA + `', '` + roleB + `', '` + perm + `')`, "23503"},
		{"role_permissions: a permission outside the catalog", tenA, `INSERT INTO iam.role_permissions VALUES ('` + tenA + `', '` + roleA + `', 'identidad.users:fly')`, "23503"},
		{"role_permissions: another tenant's row is refused by RLS", tenA, `INSERT INTO iam.role_permissions VALUES ('` + tenB + `', '` + roleB + `', '` + perm + `')`, "42501"},
		{"membership_sites: a site of the tenant", tenA, `INSERT INTO iam.membership_sites VALUES ('` + tenA + `', '` + memU2A + `', '` + siteA2 + `')`, ""},
		{"membership_sites: a site of another tenant", tenA, `INSERT INTO iam.membership_sites VALUES ('` + tenA + `', '` + memU2A + `', '` + siteB1 + `')`, "23503"},
		{"membership_sites: a membership of another tenant", tenA, `INSERT INTO iam.membership_sites VALUES ('` + tenA + `', '` + memU1B + `', '` + siteA1 + `')`, "23503"},
		{"membership_sites: the same row twice", tenA, `INSERT INTO iam.membership_sites VALUES ('` + tenA + `', '` + memU1A + `', '` + siteA1 + `')`, "23505"},
		{"sites: the code is unique per tenant", tenA, `INSERT INTO org.sites (id, tenant_id, code, name) VALUES (gen_random_uuid(), '` + tenA + `', 'S1', 'Otra')`, "23505"},
		{"sites: the same code in another tenant is fine", tenB, `INSERT INTO org.sites (id, tenant_id, code, name) VALUES (gen_random_uuid(), '` + tenB + `', 'S2', 'Otra')`, ""},
		{"sites: a code with spaces", tenA, `INSERT INTO org.sites (id, tenant_id, code, name) VALUES (gen_random_uuid(), '` + tenA + `', 'S 9', 'X')`, "23514"},
		{"cash_registers: a site of the tenant", tenA, `INSERT INTO org.cash_registers (id, tenant_id, site_id, code) VALUES (gen_random_uuid(), '` + tenA + `', '` + siteA2 + `', 'CAJA-09')`, ""},
		{"cash_registers: a site of another tenant", tenA, `INSERT INTO org.cash_registers (id, tenant_id, site_id, code) VALUES (gen_random_uuid(), '` + tenA + `', '` + siteB1 + `', 'CAJA-09')`, "23503"},
		{"cash_registers: the code is unique per tenant", tenA, `INSERT INTO org.cash_registers (id, tenant_id, site_id, code) VALUES (gen_random_uuid(), '` + tenA + `', '` + siteA2 + `', 'CAJA-01')`, "23505"},
		{"roles: the name is unique without regard to case", tenA, `INSERT INTO iam.roles (id, tenant_id, name) VALUES (gen_random_uuid(), '` + tenA + `', 'SUPERVISOR')`, "23505"},
		{"roles: the same name in another tenant is fine", tenB, `INSERT INTO iam.roles (id, tenant_id, name) VALUES (gen_random_uuid(), '` + tenB + `', 'Otro')`, ""},
		{"roles: a new role starts at version 1", tenA, `INSERT INTO iam.roles (id, tenant_id, name, version) VALUES (gen_random_uuid(), '` + tenA + `', 'Cero', 0)`, "23514"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := asApp(t, app, tc.tenant, "", tc.sql)
			if tc.state == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			wantState(t, err, tc.state, tc.name)
		})
	}

	// version nace en 1.
	var v int64
	ctx2 := context.Background()
	tx, err := app.Begin(ctx2)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx2) //nolint:errcheck // solo lectura
	setVars(t, tx, tenA, "")
	if err := tx.QueryRow(ctx2, `INSERT INTO iam.roles (id, tenant_id, name) VALUES (gen_random_uuid(), $1, 'Nuevo') RETURNING version`, tenA).Scan(&v); err != nil || v != 1 {
		t.Errorf("new role version = %d, %v; want 1", v, err)
	}
}

func TestIdentitySystemRoleCheckAndTemplateUnique(t *testing.T) {
	ctx := context.Background()
	d := migrated(t)
	admin := seedIdentity(ctx, t, d)
	app := connectAs(t, d, dbtest.App)

	// CHECK system = (template_code IS NOT NULL), también para el superusuario (los triggers lo dejan pasar, el CHECK no).
	for _, tc := range []struct{ name, sql, state string }{
		{"system without a template", `INSERT INTO iam.roles (id, tenant_id, name, system) VALUES (gen_random_uuid(), '` + tenA + `', 'Falso', true)`, "23514"},
		{"a template on a role that is not system", `INSERT INTO iam.roles (id, tenant_id, name, template_code) VALUES (gen_random_uuid(), '` + tenA + `', 'Falso2', 'cajero')`, "23514"},
		{"an unknown template", `INSERT INTO iam.roles (id, tenant_id, name, system, template_code) VALUES (gen_random_uuid(), '` + tenA + `', 'Falso3', true, 'inventado')`, "23503"},
		{"a base role", `INSERT INTO iam.roles (id, tenant_id, name, system, template_code, requires_mfa) VALUES ('c0000000-0000-0000-0000-000000000001', '` + tenA + `', 'Dueño', true, 'dueno', true)`, ""},
		{"a second role of the same template in the tenant", `INSERT INTO iam.roles (id, tenant_id, name, system, template_code, requires_mfa) VALUES (gen_random_uuid(), '` + tenA + `', 'Dueño 2', true, 'dueno', true)`, "23505"},
		{"the same template in another tenant", `INSERT INTO iam.roles (id, tenant_id, name, system, template_code, requires_mfa) VALUES (gen_random_uuid(), '` + tenB + `', 'Dueño', true, 'dueno', true)`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := admin.Exec(ctx, tc.sql)
			if tc.state == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			wantState(t, err, tc.state, tc.name)
		})
	}
	// Nadie crea otro dueno aunque inserte con system = true: lo rechaza el índice único, no solo el trigger.
	wantState(t, asApp(t, app, tenA, "", `INSERT INTO iam.roles (id, tenant_id, name, system, template_code, requires_mfa)
		VALUES (gen_random_uuid(), '`+tenA+`', 'Otro dueño', true, 'dueno', true)`), "23505", "wanka_app inserts a second dueno")
}

func TestGuardRoleWritesRejectsEachRuleForWankaApp(t *testing.T) {
	ctx := context.Background()
	d := migrated(t)
	admin := seedIdentity(ctx, t, d)
	const sysDueno, sysCajero, ownA = "c0000000-0000-0000-0000-000000000001", "c0000000-0000-0000-0000-000000000002", roleA
	for _, q := range []string{
		`INSERT INTO iam.roles (id, tenant_id, name, system, template_code, requires_mfa) VALUES ('` + sysDueno + `', '` + tenA + `', 'Dueño', true, 'dueno', true)`,
		`INSERT INTO iam.roles (id, tenant_id, name, system, template_code, requires_mfa) VALUES ('` + sysCajero + `', '` + tenA + `', 'Cajero', true, 'cajero', false)`,
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	app := connectAs(t, d, dbtest.App)
	denied := []struct{ name, sql string }{
		{"delete a system role", `DELETE FROM iam.roles WHERE id = '` + sysDueno + `'`},
		{"turn a system role into an own one", `UPDATE iam.roles SET system = false, template_code = NULL WHERE id = '` + sysDueno + `'`},
		{"make an own role system with a template", `UPDATE iam.roles SET template_code = 'dueno', system = true WHERE id = '` + ownA + `'`},
		{"change only the template of an own role", `UPDATE iam.roles SET template_code = 'cajero' WHERE id = '` + ownA + `'`},
		{"change only system of an own role", `UPDATE iam.roles SET system = true WHERE id = '` + ownA + `'`},
		{"move a system role to another template", `UPDATE iam.roles SET template_code = 'cajero' WHERE id = '` + sysDueno + `'`},
		{"change the id of a system role", `UPDATE iam.roles SET id = gen_random_uuid() WHERE id = '` + sysDueno + `'`},
		{"change created_at of a system role", `UPDATE iam.roles SET created_at = '2000-01-01' WHERE id = '` + sysDueno + `'`},
		{"rename a system role", `UPDATE iam.roles SET name = 'Jefe' WHERE id = '` + sysDueno + `'`},
		{"lower requires_mfa of a system role below its template", `UPDATE iam.roles SET requires_mfa = false WHERE id = '` + sysDueno + `'`},
		{"insert a system role below the template requires_mfa", `INSERT INTO iam.roles (id, tenant_id, name, system, template_code, requires_mfa) VALUES (gen_random_uuid(), '` + tenB + `', 'Dueño', true, 'dueno', false)`},
	}
	for _, tc := range denied {
		t.Run("denied/"+tc.name, func(t *testing.T) {
			tenant := tenA
			if strings.Contains(tc.sql, tenB) {
				tenant = tenB
			}
			wantState(t, asApp(t, app, tenant, "", tc.sql), "42501", tc.name)
		})
	}
	allowed := []struct{ name, sql string }{
		{"rename an own role", `UPDATE iam.roles SET name = 'Jefe', version = version + 1 WHERE id = '` + ownA + `'`},
		{"raise requires_mfa of an own role", `UPDATE iam.roles SET requires_mfa = true WHERE id = '` + ownA + `'`},
		{"raise requires_mfa of a system role whose template does not require it", `UPDATE iam.roles SET requires_mfa = true WHERE id = '` + sysCajero + `'`},
		{"keep requires_mfa of a system role at its template value", `UPDATE iam.roles SET requires_mfa = true WHERE id = '` + sysDueno + `'`},
		{"bump the version of a system role", `UPDATE iam.roles SET version = version + 1 WHERE id = '` + sysCajero + `'`},
		{"delete an own role without members", `INSERT INTO iam.roles (id, tenant_id, name) VALUES ('c0000000-0000-0000-0000-0000000000f1', '` + tenA + `', 'Efímero');
			DELETE FROM iam.roles WHERE id = 'c0000000-0000-0000-0000-0000000000f1'`},
	}
	for _, tc := range allowed {
		t.Run("allowed/"+tc.name, func(t *testing.T) {
			if err := asApp(t, app, tenA, "", tc.sql); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
	// Un rol propio con miembros no se borra: lo rechaza la FK de iam.memberships, no el trigger.
	wantState(t, asApp(t, app, tenA, "", `DELETE FROM iam.roles WHERE id = '`+ownA+`'`), "23503", "delete an own role in use")
	// Borrar un rol propio se lleva sus permisos (ON DELETE CASCADE) sin DELETE explícito sobre la tabla.
	if err := asApp(t, app, tenA, "", `INSERT INTO iam.roles (id, tenant_id, name) VALUES ('c0000000-0000-0000-0000-0000000000f2', '`+tenA+`', 'Con permisos');
		INSERT INTO iam.role_permissions VALUES ('`+tenA+`', 'c0000000-0000-0000-0000-0000000000f2', 'identidad.users:read');
		DELETE FROM iam.roles WHERE id = 'c0000000-0000-0000-0000-0000000000f2'`); err != nil {
		t.Errorf("deleting an own role with permissions: %v", err)
	}

	// Mutación: sin el trigger, cada una de las reglas rechazadas pasa. Demuestra que el test depende del trigger.
	if _, err := admin.Exec(ctx, `DROP TRIGGER roles_guard_writes ON iam.roles`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range denied {
		if strings.Contains(tc.sql, "INSERT") && strings.Contains(tc.sql, tenB) {
			continue // el CHECK y la FK siguen valiendo; este caso se prueba con la plantilla
		}
		t.Run("without the trigger/"+tc.name, func(t *testing.T) {
			err := asApp(t, app, tenA, "", tc.sql)
			if sqlState(err) == "42501" {
				t.Errorf("%s still gets 42501 with the trigger dropped: the test does not discriminate", tc.name)
			}
		})
	}
}

func TestGuardMembershipDeletes(t *testing.T) {
	ctx := context.Background()
	d := migrated(t)
	admin := seedIdentity(ctx, t, d)
	app := connectAs(t, d, dbtest.App)
	for _, id := range []string{memU1A, memU3A} { // activo y suspendido
		wantState(t, asApp(t, app, tenA, "", `DELETE FROM iam.memberships WHERE id = '`+id+`'`), "42501", "delete membership "+id)
	}
	// Una invitada se borra y se lleva sus sedes.
	if _, err := admin.Exec(ctx, `INSERT INTO iam.membership_sites VALUES ('`+tenA+`', '`+memU2A+`', '`+siteA1+`')`); err != nil {
		t.Fatal(err)
	}
	if err := asApp(t, app, tenA, "", `DELETE FROM iam.memberships WHERE id = '`+memU2A+`'`); err != nil {
		t.Fatalf("deleting an invited membership: %v", err)
	}
	// Las transiciones legítimas no se bloquean: cada UPDATE afecta exactamente una fila.
	for _, tc := range []struct{ name, id, from, to string }{
		{"invitado to activo", memU2A, "invitado", "activo"},
		{"suspendido to activo", memU3A, "suspendido", "activo"},
		{"activo to suspendido", memU1A, "activo", "suspendido"},
	} {
		t.Run("transition/"+tc.name, func(t *testing.T) {
			tx, err := app.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx) //nolint:errcheck // la prueba nunca confirma
			setVars(t, tx, tenA, "")
			tag, err := tx.Exec(ctx, `UPDATE iam.memberships SET status = $1 WHERE id = $2 AND status = $3`, tc.to, tc.id, tc.from)
			if err != nil || tag.RowsAffected() != 1 {
				t.Errorf("%s: rows = %d, err = %v; want 1 row", tc.name, tag.RowsAffected(), err)
			}
		})
	}
	// Volver a invitado para saltar la guarda del DELETE: activo y suspendido lo rechazan.
	for _, id := range []string{memU1A, memU3A} {
		wantState(t, asApp(t, app, tenA, "", `UPDATE iam.memberships SET status = 'invitado' WHERE id = '`+id+`'`), "42501", "re-invite "+id)
	}
	// Mutación: sin el trigger de estado, activo -> invitado -> DELETE borra una membresía activa.
	if _, err := admin.Exec(ctx, `DROP TRIGGER memberships_guard_status ON iam.memberships`); err != nil {
		t.Fatal(err)
	}
	if err := asApp(t, app, tenA, "", `UPDATE iam.memberships SET status = 'invitado' WHERE id = '`+memU1A+`'`); err != nil {
		t.Errorf("without the status trigger the re-invite should pass: %v", err)
	}
	// Mutación: sin el trigger la activa se borra.
	if _, err := admin.Exec(ctx, `DROP TRIGGER memberships_guard_deletes ON iam.memberships`); err != nil {
		t.Fatal(err)
	}
	if err := asApp(t, app, tenA, "", `DELETE FROM iam.memberships WHERE id = '`+memU3A+`'`); err != nil {
		t.Errorf("without the trigger the suspended membership should be deletable: %v", err)
	}
}

func TestRolePermissionsRejectsSystemRoles(t *testing.T) {
	ctx := context.Background()
	d := migrated(t)
	admin := seedIdentity(ctx, t, d)
	const sysDueno = "c0000000-0000-0000-0000-000000000001"
	if _, err := admin.Exec(ctx, `INSERT INTO iam.roles (id, tenant_id, name, system, template_code, requires_mfa) VALUES ('`+sysDueno+`', '`+tenA+`', 'Dueño', true, 'dueno', true)`); err != nil {
		t.Fatal(err)
	}
	app := connectAs(t, d, dbtest.App)
	ins := `INSERT INTO iam.role_permissions VALUES ('` + tenA + `', '` + sysDueno + `', 'identidad.users:read')`
	wantState(t, asApp(t, app, tenA, "", ins), "42501", "permissions of a system role")
	if err := asApp(t, app, tenA, "", `INSERT INTO iam.role_permissions VALUES ('`+tenA+`', '`+roleA+`', 'identidad.users:read')`); err != nil {
		t.Errorf("an own role takes permissions: %v", err)
	}
	// Mutación: sin el trigger, el rol base recibe la fila.
	if _, err := admin.Exec(ctx, `DROP TRIGGER role_permissions_guard_writes ON iam.role_permissions`); err != nil {
		t.Fatal(err)
	}
	if err := asApp(t, app, tenA, "", ins); err != nil {
		t.Errorf("without the trigger the insert should pass: %v", err)
	}
}

func TestRoleTemplatesAreReadOnlyForEveryApplicationRole(t *testing.T) {
	ctx := context.Background()
	d := migrated(t)
	admin := seedIdentity(ctx, t, d)
	if _, err := admin.Exec(ctx, `INSERT INTO iam.role_template_permissions VALUES ('cajero', 'ventas.pos:read')`); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{dbtest.App, dbtest.Worker, dbtest.Webhooks, dbtest.Platform} {
		c := connectAs(t, d, role)
		for _, tbl := range []string{"iam.role_templates", "iam.role_template_permissions"} {
			_, err := c.Exec(ctx, `SELECT count(*) FROM `+tbl)
			if role == dbtest.App && err != nil {
				t.Errorf("wanka_app cannot read %s: %v", tbl, err)
			}
			if role != dbtest.App {
				wantState(t, err, "42501", role+" reads "+tbl)
			}
		}
		for _, sql := range []string{
			`INSERT INTO iam.role_templates (code, name, requires_mfa) VALUES ('x', 'X', false)`,
			`UPDATE iam.role_templates SET name = 'X'`,
			`DELETE FROM iam.role_templates`,
			`TRUNCATE iam.role_templates CASCADE`,
			`INSERT INTO iam.role_template_permissions VALUES ('cajero', 'ventas.pos:create')`,
			`UPDATE iam.role_template_permissions SET permission_code = 'ventas.pos:create'`,
			`DELETE FROM iam.role_template_permissions`,
			`TRUNCATE iam.role_template_permissions`,
		} {
			_, err := c.Exec(ctx, sql)
			wantState(t, err, "42501", role+": "+sql)
		}
	}
	// wanka_definer lee las plantillas (no puede conectarse: SET ROLE como superusuario).
	for _, tbl := range []string{"iam.role_templates", "iam.role_template_permissions"} {
		if !queryOne[bool](t, admin, `SELECT has_table_privilege('wanka_definer', $1, 'SELECT')`, tbl) {
			t.Errorf("wanka_definer cannot read %s", tbl)
		}
		for _, p := range []string{"INSERT", "UPDATE", "DELETE", "TRUNCATE"} {
			if queryOne[bool](t, admin, `SELECT has_table_privilege('wanka_definer', $1, $2)`, tbl, p) {
				t.Errorf("wanka_definer has %s on %s", p, tbl)
			}
		}
	}
}

func TestSitesAndCashRegistersGrantsAreByColumn(t *testing.T) {
	ctx := context.Background()
	d := migrated(t)
	admin := d.Admin(ctx, t)
	// Lista escrita a mano: lo que wanka_app puede escribir y lo que no. created_at lo pone el servidor y las columnas que
	// #33 agregue no entran en un GRANT de tabla.
	for _, tc := range []struct {
		table, col, priv string
		want             bool
	}{
		{"org.sites", "code", "INSERT", true}, {"org.sites", "name", "UPDATE", true}, {"org.sites", "active", "UPDATE", true},
		{"org.sites", "updated_at", "UPDATE", true}, {"org.sites", "created_at", "INSERT", false},
		{"org.sites", "created_at", "UPDATE", false}, {"org.sites", "id", "UPDATE", false}, {"org.sites", "tenant_id", "UPDATE", false},
		{"org.cash_registers", "code", "UPDATE", true}, {"org.cash_registers", "active", "UPDATE", true},
		{"org.cash_registers", "site_id", "INSERT", true}, {"org.cash_registers", "site_id", "UPDATE", false},
		{"org.cash_registers", "tenant_id", "UPDATE", false}, {"org.cash_registers", "id", "UPDATE", false},
	} {
		if got := queryOne[bool](t, admin, `SELECT has_column_privilege('wanka_app', $1, $2, $3)`, tc.table, tc.col, tc.priv); got != tc.want {
			t.Errorf("wanka_app %s on %s.%s = %v, want %v", tc.priv, tc.table, tc.col, got, tc.want)
		}
	}
	for _, tbl := range []string{"org.sites", "org.cash_registers"} {
		if queryOne[bool](t, admin, `SELECT has_table_privilege('wanka_app', $1, 'DELETE')`, tbl) {
			t.Errorf("wanka_app can DELETE %s", tbl)
		}
		for _, role := range []string{"wanka_worker", "wanka_webhooks", "wanka_platform"} {
			if queryOne[bool](t, admin, `SELECT has_any_column_privilege($1, $2, 'SELECT,INSERT,UPDATE')`, role, tbl) {
				t.Errorf("%s has access to %s", role, tbl)
			}
		}
	}
}

func TestMemberPoliciesShowOnlyOwnMembershipsAndWhatTheyReference(t *testing.T) {
	ctx := context.Background()
	d := migrated(t)
	seedIdentity(ctx, t, d)
	app := connectAs(t, d, dbtest.App)
	const (
		sitesQ = `SELECT code || '@' || tenant_id::text FROM org.sites`
		crQ    = `SELECT code || '@' || site_id::text FROM org.cash_registers`
		rolesQ = `SELECT id::text FROM iam.roles`
		memQ   = `SELECT id::text FROM iam.memberships`
		tenQ   = `SELECT id::text FROM org.tenants`
		msQ    = `SELECT membership_id::text || '/' || site_id::text FROM iam.membership_sites`
	)
	t.Run("only app.user_id: u1 sees its two memberships and what they reference", func(t *testing.T) {
		wantStrings(t, appStrings(t, app, "", usr1, memQ), memU1A, memU1B)
		wantStrings(t, appStrings(t, app, "", usr1, tenQ), tenA, tenB)
		wantStrings(t, appStrings(t, app, "", usr1, rolesQ), roleA, roleB)
		// En A solo la sede del alcance (A1); en B todas (all_sites).
		wantStrings(t, appStrings(t, app, "", usr1, sitesQ), "S1@"+tenA, "S1@"+tenB)
		wantStrings(t, appStrings(t, app, "", usr1, crQ), "CAJA-01@"+siteA1, "CAJA-02@"+siteA1, "CAJA-01@"+siteB1)
		wantStrings(t, appStrings(t, app, "", usr1, msQ), memU1A+"/"+siteA1)
	})
	t.Run("an invited member sees the company, the membership and the role but no sites", func(t *testing.T) {
		wantStrings(t, appStrings(t, app, "", usr2, memQ), memU2A)
		wantStrings(t, appStrings(t, app, "", usr2, tenQ), tenA)
		wantStrings(t, appStrings(t, app, "", usr2, rolesQ), roleA)
		wantStrings(t, appStrings(t, app, "", usr2, sitesQ))
		wantStrings(t, appStrings(t, app, "", usr2, crQ))
	})
	t.Run("a suspended member sees nothing, with or without a tenant (ADR 0011)", func(t *testing.T) {
		for _, q := range []string{memQ, tenQ, rolesQ, sitesQ, crQ, msQ} {
			wantStrings(t, appStrings(t, app, "", usr3, q))
		}
	})
	t.Run("nobody: no user and no tenant sees nothing", func(t *testing.T) {
		for _, q := range []string{memQ, tenQ, rolesQ, sitesQ, crQ, msQ} {
			wantStrings(t, appStrings(t, app, "", "", q))
		}
	})
	t.Run("a member of A and B with app.tenant_id = A sees no row of B", func(t *testing.T) {
		for _, q := range []string{
			`SELECT id::text FROM org.tenants WHERE id = '` + tenB + `'`,
			`SELECT id::text FROM iam.roles WHERE tenant_id = '` + tenB + `'`,
			`SELECT id::text FROM iam.memberships WHERE tenant_id = '` + tenB + `'`,
			`SELECT id::text FROM org.sites WHERE tenant_id = '` + tenB + `'`,
			`SELECT id::text FROM org.cash_registers WHERE tenant_id = '` + tenB + `'`,
			`SELECT membership_id::text FROM iam.membership_sites WHERE tenant_id = '` + tenB + `'`,
		} {
			wantStrings(t, appStrings(t, app, tenA, usr1, q))
		}
		// Y sin filtro: solo A, aun con app.user_id de un miembro de las dos.
		wantStrings(t, appStrings(t, app, tenA, usr1, tenQ), tenA)
		wantStrings(t, appStrings(t, app, tenA, usr1, sitesQ), "S1@"+tenA, "S2@"+tenA)
	})
	t.Run("a deleted membership leaves no trace", func(t *testing.T) {
		admin := d.Admin(ctx, t)
		if _, err := admin.Exec(ctx, `DELETE FROM iam.memberships WHERE id = $1`, memU1B); err != nil {
			t.Fatal(err)
		}
		wantStrings(t, appStrings(t, app, "", usr1, tenQ), tenA)
		wantStrings(t, appStrings(t, app, "", usr1, rolesQ), roleA)
		wantStrings(t, appStrings(t, app, "", usr1, `SELECT tenant_name FROM iam.list_my_memberships`), "Comercial A")
	})
}

type membershipRow struct {
	tenant, industry, tstatus, mstatus, role string
	mfa                                      bool
	sites, crs                               []string
}

func listMyMemberships(t *testing.T, c *pgx.Conn, tenant, user string) []membershipRow {
	t.Helper()
	ctx := context.Background()
	tx, err := c.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // solo lectura
	setVars(t, tx, tenant, user)
	rows, err := tx.Query(ctx, `
		SELECT coalesce(tenant_name, ''), coalesce(industry, ''), tenant_status, membership_status, role_name, requires_mfa,
		       ARRAY(SELECT e->>'code' FROM jsonb_array_elements(sites) e ORDER BY 1),
		       ARRAY(SELECT e->>'code' || '@' || (SELECT s->>'code' FROM jsonb_array_elements(sites) s WHERE s->>'id' = e->>'site_id')
		               FROM jsonb_array_elements(cash_registers) e ORDER BY 1)
		  FROM iam.list_my_memberships ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []membershipRow
	for rows.Next() {
		var r membershipRow
		if err := rows.Scan(&r.tenant, &r.industry, &r.tstatus, &r.mstatus, &r.role, &r.mfa, &r.sites, &r.crs); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestListMyMembershipsView(t *testing.T) {
	ctx := context.Background()
	d := migrated(t)
	admin := seedIdentity(ctx, t, d)
	app := connectAs(t, d, dbtest.App)
	if _, err := admin.Exec(ctx, `UPDATE iam.roles SET requires_mfa = true WHERE id = $1`, roleB); err != nil {
		t.Fatal(err)
	}

	t.Run("u1: role, company, sites and cash registers per membership", func(t *testing.T) {
		got := fmt.Sprintf("%+v", listMyMemberships(t, app, "", usr1))
		// A: nombre comercial y solo la sede del alcance; B: sin nombre comercial, cae a la razón social, todas las sedes.
		want := "[{tenant:Comercial A industry:retail tstatus:trial mstatus:activo role:Supervisor mfa:false sites:[S1] crs:[CAJA-01@S1 CAJA-02@S1]} " +
			"{tenant:Razon B SAC industry:veterinaria tstatus:trial mstatus:activo role:Supervisor mfa:true sites:[S1] crs:[CAJA-01@S1]}]"
		if got != want {
			t.Errorf("view =\n%s\nwant\n%s", got, want)
		}
	})
	t.Run("an invited member has an empty, non-null scope", func(t *testing.T) {
		got := fmt.Sprintf("%+v", listMyMemberships(t, app, "", usr2))
		if want := "[{tenant:Comercial A industry:retail tstatus:trial mstatus:invitado role:Supervisor mfa:false sites:[] crs:[]}]"; got != want {
			t.Errorf("view = %s, want %s", got, want)
		}
		if n := queryOne[int](t, admin, `SELECT count(*) FROM iam.list_my_memberships WHERE sites IS NULL OR cash_registers IS NULL`); n != 0 {
			t.Errorf("%d rows with NULL sites or cash registers", n)
		}
	})
	t.Run("a suspended member does not appear", func(t *testing.T) {
		if got := listMyMemberships(t, app, "", usr3); len(got) != 0 {
			t.Errorf("rows = %+v, want none", got)
		}
	})
	t.Run("never rows of other users, with or without a tenant", func(t *testing.T) {
		if got := listMyMemberships(t, app, "", ""); len(got) != 0 {
			t.Errorf("no user: rows = %+v", got)
		}
		// Con tenant A fijado la vista sigue filtrando por usuario: u2 no recibe la membresía de u1, y viceversa.
		if got := listMyMemberships(t, app, tenA, usr2); len(got) != 1 || got[0].mstatus != "invitado" {
			t.Errorf("u2 with tenant A: rows = %+v, want only its own invited membership", got)
		}
		if got := listMyMemberships(t, app, tenA, usr3); len(got) != 0 {
			t.Errorf("suspended u3 with tenant A: rows = %+v, want none", got)
		}
	})
	t.Run("an inactive site and cash register are not offered", func(t *testing.T) {
		if _, err := admin.Exec(ctx, `UPDATE org.cash_registers SET active = false WHERE id = $1`, crA2); err != nil {
			t.Fatal(err)
		}
		if _, err := admin.Exec(ctx, `UPDATE org.sites SET active = false WHERE id = $1`, siteB1); err != nil {
			t.Fatal(err)
		}
		got := fmt.Sprintf("%+v", listMyMemberships(t, app, "", usr1))
		if !strings.Contains(got, "sites:[S1] crs:[CAJA-01@S1]}") || !strings.Contains(got, "sites:[] crs:[]}") {
			t.Errorf("view = %s", got)
		}
	})
}

// TestListMyMembershipsUnder50ms mide la magnitud que nombra el criterio: p95 de la vista para un usuario con 20
// membresías, 5 sedes y 3 cajas por sede (60 sedes y 180 cajas en total, as wanka_app y con las políticas). Se salta con
// -race y corre sin él en el paso de cotas de tiempo de la CI (api.yml). También exige que el plan use índices.
func TestListMyMembershipsUnder50ms(t *testing.T) {
	if raceEnabled {
		t.Skip("timing bound: runs without -race in its own CI step")
	}
	ctx := context.Background()
	d := migrated(t)
	admin := d.Admin(ctx, t)
	const user = "99999999-0000-0000-0000-000000000001"
	if _, err := admin.Exec(ctx, `INSERT INTO iam.users (id) VALUES ($1)`, user); err != nil {
		t.Fatal(err)
	}
	// 20 empresas (de 5 sedes con 3 cajas cada una) y un rol por empresa; la mitad de las membresías con alcance parcial.
	// Más ruido: 200 empresas de otros usuarios con el mismo volumen, para que un plan sin índices se note.
	seed := `
	  INSERT INTO org.tenants (id, ruc, legal_name, industry)
	    SELECT ('00000000-0000-0000-0001-' || lpad(n::text, 12, '0'))::uuid, lpad((20000000000 + n)::text, 11, '0'), 'Empresa ' || n, 'retail'
	      FROM generate_series(1, 220) n;
	  INSERT INTO iam.users (id) SELECT ('00000000-0000-0000-0002-' || lpad(n::text, 12, '0'))::uuid FROM generate_series(1, 200) n;
	  INSERT INTO iam.roles (id, tenant_id, name)
	    SELECT ('00000000-0000-0000-0003-' || lpad(n::text, 12, '0'))::uuid, ('00000000-0000-0000-0001-' || lpad(n::text, 12, '0'))::uuid, 'Rol' FROM generate_series(1, 220) n;
	  INSERT INTO org.sites (id, tenant_id, code, name)
	    SELECT ('00000000-0000-0000-0004-' || lpad((n * 10 + s)::text, 12, '0'))::uuid, ('00000000-0000-0000-0001-' || lpad(n::text, 12, '0'))::uuid, 'S' || s, 'Sede ' || s
	      FROM generate_series(1, 220) n, generate_series(1, 5) s;
	  INSERT INTO org.cash_registers (id, tenant_id, site_id, code)
	    SELECT ('00000000-0000-0000-0005-' || lpad((n * 100 + s * 10 + c)::text, 12, '0'))::uuid, ('00000000-0000-0000-0001-' || lpad(n::text, 12, '0'))::uuid,
	           ('00000000-0000-0000-0004-' || lpad((n * 10 + s)::text, 12, '0'))::uuid, 'C' || s || c
	      FROM generate_series(1, 220) n, generate_series(1, 5) s, generate_series(1, 3) c;
	  INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, all_sites, status)
	    SELECT ('00000000-0000-0000-0006-' || lpad(n::text, 12, '0'))::uuid, ('00000000-0000-0000-0001-' || lpad(n::text, 12, '0'))::uuid,
	           CASE WHEN n <= 20 THEN '` + user + `'::uuid ELSE ('00000000-0000-0000-0002-' || lpad(((n - 21) % 200 + 1)::text, 12, '0'))::uuid END,
	           ('00000000-0000-0000-0003-' || lpad(n::text, 12, '0'))::uuid, n % 2 = 0, 'activo'
	      FROM generate_series(1, 220) n;
	  INSERT INTO iam.membership_sites (tenant_id, membership_id, site_id)
	    SELECT ('00000000-0000-0000-0001-' || lpad(n::text, 12, '0'))::uuid, ('00000000-0000-0000-0006-' || lpad(n::text, 12, '0'))::uuid,
	           ('00000000-0000-0000-0004-' || lpad((n * 10 + s)::text, 12, '0'))::uuid
	      FROM generate_series(1, 220) n, generate_series(1, 5) s WHERE n % 2 = 1 AND s <= 3;
	  ANALYZE;`
	if _, err := admin.Exec(ctx, seed); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	app := connectAs(t, d, dbtest.App)
	run := func() time.Duration {
		tx, err := app.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx) //nolint:errcheck // solo lectura
		setVars(t, tx, "", user)
		start := time.Now()
		rows, err := tx.Query(ctx, `SELECT membership_id, sites, cash_registers FROM iam.list_my_memberships`)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for rows.Next() {
			var id, s, c any
			if err := rows.Scan(&id, &s, &c); err != nil {
				t.Fatal(err)
			}
			n++
		}
		rows.Close()
		if rows.Err() != nil || n != 20 {
			t.Fatalf("rows = %d, %v; want 20", n, rows.Err())
		}
		return time.Since(start)
	}
	for range 10 { // calentamiento: plan y caché de catálogo
		run()
	}
	const n = 200
	lat := make([]time.Duration, n)
	for i := range lat {
		lat[i] = run()
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	p95 := lat[n*95/100]
	t.Logf("iam.list_my_memberships (20 memberships, 5 sites, 3 cash registers per site; 220 tenants of noise): p50=%v p95=%v max=%v", lat[n/2], p95, lat[n-1])
	if p95 >= 50*time.Millisecond {
		t.Errorf("p95 = %v, want < 50ms", p95)
	}
}
