package identidad_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/identidad"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// fixture es una base migrada con el pool de wanka_app y una conexión de superusuario para sembrar.
type fixture struct {
	d     *dbtest.DB
	admin *pgx.Conn
	pool  *db.Pool
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d, admin := migratedAdmin(t)
	pool, err := db.Open(ctx, d.URL(dbtest.App), db.Options{MaxConns: 4, AppName: "identidad-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return &fixture{d: d, admin: admin, pool: pool}
}

func (f *fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.admin.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func scalar[T any](t *testing.T, f *fixture, sql string, args ...any) T {
	t.Helper()
	var v T
	if err := f.admin.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

// tenant crea una empresa y devuelve su id.
func (f *fixture) tenant(t *testing.T) pgtype.UUID {
	t.Helper()
	return scalar[pgtype.UUID](t, f, `INSERT INTO org.tenants (id) VALUES (gen_random_uuid()) RETURNING id`)
}

func (f *fixture) user(t *testing.T) pgtype.UUID {
	t.Helper()
	return scalar[pgtype.UUID](t, f, `INSERT INTO iam.users (id, email) SELECT i, i::text || '@seed.test' FROM gen_random_uuid() i RETURNING id`)
}

// asTenant corre fn en una transacción de tenant de lectura y escritura de wanka_app (rol system, sin usuario).
func (f *fixture) asTenant(t *testing.T, tenant pgtype.UUID, fn func(pgx.Tx) error) error {
	t.Helper()
	return f.pool.InTenantTx(context.Background(), db.Actor{TenantID: tenant, Kind: db.KindSystem}, fn)
}

func (f *fixture) seedRoles(t *testing.T, tenant pgtype.UUID) {
	t.Helper()
	if err := f.asTenant(t, tenant, func(tx pgx.Tx) error { return identidad.SeedTenantRoles(context.Background(), tx, tenant) }); err != nil {
		t.Fatalf("SeedTenantRoles: %v", err)
	}
}

func sqlState(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

type roleRow struct {
	Name, Template string
	System, MFA    bool
	Version        int64
}

func (f *fixture) roles(t *testing.T, tenant pgtype.UUID) map[string]roleRow {
	t.Helper()
	rows, err := f.admin.Query(context.Background(), `SELECT name, coalesce(template_code, ''), system, requires_mfa, version FROM iam.roles WHERE tenant_id = $1`, tenant)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]roleRow{}
	for rows.Next() {
		var r roleRow
		if err := rows.Scan(&r.Name, &r.Template, &r.System, &r.MFA, &r.Version); err != nil {
			t.Fatal(err)
		}
		out[r.Template+"/"+r.Name] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// La fachada crea los seis roles base con system, template_code y version 1;
// una segunda llamada no duplica ni falla; no copia permisos.
func TestSeedTenantRolesCreatesTheSixBaseRolesOnce(t *testing.T) {
	f := newFixture(t)
	a, b := f.tenant(t), f.tenant(t)
	f.seedRoles(t, a)
	f.seedRoles(t, a) // idempotente
	want := map[string]roleRow{
		"dueno/Dueño":                 {"Dueño", "dueno", true, true, 1},
		"administrador/Administrador": {"Administrador", "administrador", true, true, 1},
		"contador/Contador":           {"Contador", "contador", true, true, 1},
		"cajero/Cajero":               {"Cajero", "cajero", true, false, 1},
		"vendedor/Vendedor":           {"Vendedor", "vendedor", true, false, 1},
		"almacenero/Almacenero":       {"Almacenero", "almacenero", true, false, 1},
	}
	got := f.roles(t, a)
	if len(got) != len(want) {
		t.Fatalf("roles = %d, want 6: %v", len(got), got)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %+v, want %+v", k, got[k], w)
		}
	}
	if n := scalar[int](t, f, `SELECT count(*) FROM iam.role_permissions WHERE tenant_id = $1`, a); n != 0 {
		t.Errorf("%d permission rows were copied, want 0 (they come from the template)", n)
	}
	if n := len(f.roles(t, b)); n != 0 {
		t.Errorf("the other tenant got %d roles", n)
	}
	f.seedRoles(t, b)
	if n := len(f.roles(t, b)); n != 6 {
		t.Errorf("second tenant roles = %d, want 6", n)
	}
	if n := scalar[int](t, f, `SELECT count(*) FROM iam.roles WHERE system`); n != 12 {
		t.Errorf("system roles = %d, want 12", n)
	}
}

// Una transacción de otra empresa no puede sembrar roles en esta (RLS), y un id vacío falla sin tocar la base.
func TestSeedTenantRolesRejectsAnotherTenantAndAnEmptyID(t *testing.T) {
	f := newFixture(t)
	a, b := f.tenant(t), f.tenant(t)
	err := f.asTenant(t, b, func(tx pgx.Tx) error { return identidad.SeedTenantRoles(context.Background(), tx, a) })
	if err == nil {
		t.Fatal("seeding the roles of tenant A inside tenant B's transaction succeeded")
	}
	if n := len(f.roles(t, a)); n != 0 {
		t.Fatalf("tenant A got %d roles from tenant B's transaction", n)
	}
	err = f.asTenant(t, a, func(tx pgx.Tx) error { return identidad.SeedTenantRoles(context.Background(), tx, pgtype.UUID{}) })
	if err == nil {
		t.Fatal("an empty tenant id was accepted")
	}
}

// Sin plantillas no se da de alta una empresa sin roles: la siembra falla.
func TestSeedTenantRolesFailsWithoutTemplates(t *testing.T) {
	f := newFixture(t)
	a := f.tenant(t)
	f.exec(t, `TRUNCATE iam.role_template_permissions, iam.role_templates CASCADE`)
	err := f.asTenant(t, a, func(tx pgx.Tx) error { return identidad.SeedTenantRoles(context.Background(), tx, a) })
	if err == nil {
		t.Fatal("seeding without templates succeeded")
	}
}

// Un rol propio que ya usa el nombre de una plantilla no deja a la empresa sin ese rol base en silencio.
func TestSeedTenantRolesFailsOnANameClash(t *testing.T) {
	f := newFixture(t)
	a := f.tenant(t)
	f.exec(t, `INSERT INTO iam.roles (id, tenant_id, name) VALUES (gen_random_uuid(), $1, 'cajero')`, a)
	err := f.asTenant(t, a, func(tx pgx.Tx) error { return identidad.SeedTenantRoles(context.Background(), tx, a) })
	if sqlState(err) != "23505" {
		t.Fatalf("err = %v, want 23505", err)
	}
	if n := len(f.roles(t, a)); n != 1 {
		t.Fatalf("roles = %d, want only the custom one (the seed rolled back)", n)
	}
}

// Las dos reglas de la base que la siembra respeta, probadas contra los roles sembrados (23505 / 42501).
func TestDatabaseRejectsASecondBaseRoleAndALowerMFA(t *testing.T) {
	f := newFixture(t)
	a := f.tenant(t)
	f.seedRoles(t, a)
	cases := []struct {
		name, sql, state string
	}{
		{"second role with the same template", `INSERT INTO iam.roles (id, tenant_id, name, system, template_code, requires_mfa) VALUES (gen_random_uuid(), $1, 'Otro dueño', true, 'dueno', true)`, "23505"},
		{"lowering requires_mfa of a seeded role", `UPDATE iam.roles SET requires_mfa = false WHERE tenant_id = $1 AND template_code = 'dueno'`, "42501"},
		{"promoting an own role to a base role", `UPDATE iam.roles SET template_code = 'dueno', system = true WHERE tenant_id = $1 AND name = 'Propio'`, "42501"},
		{"renaming a base role", `UPDATE iam.roles SET name = 'Jefe' WHERE tenant_id = $1 AND template_code = 'dueno'`, "42501"},
		{"deleting a base role", `DELETE FROM iam.roles WHERE tenant_id = $1 AND template_code = 'dueno'`, "42501"},
	}
	f.exec(t, `INSERT INTO iam.roles (id, tenant_id, name) VALUES (gen_random_uuid(), $1, 'Propio')`, a)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := f.asTenant(t, a, func(tx pgx.Tx) error {
				_, err := tx.Exec(context.Background(), c.sql, a)
				return err
			})
			if got := sqlState(err); got != c.state {
				t.Fatalf("state = %q (%v), want %s", got, err, c.state)
			}
		})
	}
	// requires_mfa por debajo de la plantilla en un INSERT: en una empresa sin su rol base, para que el índice único no se adelante.
	b := f.tenant(t)
	err := f.asTenant(t, b, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			`INSERT INTO iam.roles (id, tenant_id, name, system, template_code, requires_mfa) VALUES (gen_random_uuid(), $1, 'Contador', true, 'contador', false)`, b)
		return err
	})
	if got := sqlState(err); got != "42501" {
		t.Fatalf("insert with lower requires_mfa: state = %q (%v), want 42501", got, err)
	}
	// Subir requires_mfa de un rol base sí se puede.
	err = f.asTenant(t, a, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE iam.roles SET requires_mfa = true WHERE tenant_id = $1 AND template_code = 'cajero'`, a)
		return err
	})
	if err != nil {
		t.Fatalf("raising requires_mfa of cajero: %v", err)
	}
}

// wanka_worker no tiene ningún grant sobre las plantillas: 42501.
func TestWorkerCannotReadTheTemplates(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	w, err := pgx.Connect(ctx, f.d.URL(dbtest.Worker))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(ctx) })
	for _, tbl := range []string{"iam.role_templates", "iam.role_template_permissions"} {
		_, err := w.Exec(ctx, `SELECT count(*) FROM `+tbl)
		if sqlState(err) != "42501" {
			t.Errorf("worker reading %s: %v, want 42501", tbl, err)
		}
	}
	// Y el resolver las lee como wanka_app.
	var n int
	if err := f.pool.InTenantReadTx(ctx, db.Actor{TenantID: f.tenant(t), Kind: db.KindSystem}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM iam.role_templates`).Scan(&n)
	}); err != nil || n != 6 {
		t.Errorf("wanka_app reads %d templates (%v), want 6", n, err)
	}
}
