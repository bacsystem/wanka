package permission_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

func connectAs(t *testing.T, d *dbtest.DB, role string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), d.URL(role))
	if err != nil {
		t.Fatalf("connecting as %s: %v", role, err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

func wantState(t *testing.T, err error, code, what string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Errorf("%s: err = %v, want SQLSTATE %s", what, err, code)
	}
}

func dbCodes(ctx context.Context, t *testing.T, c *pgx.Conn) []string {
	t.Helper()
	rows, err := c.Query(ctx, `SELECT code FROM iam.permissions ORDER BY code`)
	if err != nil {
		t.Fatalf("selecting iam.permissions: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			t.Fatal(err)
		}
		out = append(out, code)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// Criterio 1 y 2: la base migrada tiene exactamente los códigos de resources.md y del código Go,
// sin RLS, y cada rol que el ADR 0001 lista la lee.
func TestPermissionsTableMatchesDocumentAndGo(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	d.Migrate(ctx, t)

	doc := docCatalog(t)
	for _, role := range []string{dbtest.App, dbtest.Worker, dbtest.Platform} {
		t.Run(role, func(t *testing.T) {
			got := dbCodes(ctx, t, connectAs(t, d, role))
			if m := missingFrom(doc, got); len(m) > 0 {
				t.Errorf("in resources.md but not in iam.permissions: %v", m)
			}
			if m := missingFrom(got, doc); len(m) > 0 {
				t.Errorf("in iam.permissions but not in resources.md: %v", m)
			}
			if m := missingFrom(goCatalog(), got); len(m) > 0 {
				t.Errorf("in the Go catalog but not in iam.permissions: %v", m)
			}
			if len(got) != len(doc) {
				t.Errorf("iam.permissions has %d rows, resources.md has %d codes", len(got), len(doc))
			}
		})
	}

	admin := d.Admin(ctx, t)
	var rls, force bool
	if err := admin.QueryRow(ctx, `SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE oid = 'iam.permissions'::regclass`).Scan(&rls, &force); err != nil {
		t.Fatal(err)
	}
	if rls || force {
		t.Errorf("iam.permissions has RLS (enabled=%v forced=%v), it is a global catalog", rls, force)
	}
	var empty int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM iam.permissions WHERE description = ''`).Scan(&empty); err != nil || empty != 0 {
		t.Errorf("rows without description = %d (err %v)", empty, err)
	}
}

// Criterio 4: el catálogo no cambia en runtime. Ni wanka_app ni los demás roles de aplicación
// escriben; wanka_webhooks ni siquiera lo lee.
func TestPermissionsAreFrozenAtRuntime(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	d.Migrate(ctx, t)

	writes := map[string]string{
		"INSERT":   `INSERT INTO iam.permissions (code, description) VALUES ('ventas.pos:void', 'Anular ventas')`,
		"UPDATE":   `UPDATE iam.permissions SET description = 'x'`,
		"DELETE":   `DELETE FROM iam.permissions`,
		"TRUNCATE": `TRUNCATE iam.permissions`,
	}
	for _, role := range []string{dbtest.App, dbtest.Worker, dbtest.Platform, dbtest.Webhooks} {
		c := connectAs(t, d, role)
		for verb, q := range writes {
			_, err := c.Exec(ctx, q)
			wantState(t, err, "42501", role+" "+verb)
		}
	}

	_, err := connectAs(t, d, dbtest.Webhooks).Exec(ctx, `SELECT 1 FROM iam.permissions`)
	wantState(t, err, "42501", "wanka_webhooks SELECT")

	// Nada cambió.
	got := dbCodes(ctx, t, connectAs(t, d, dbtest.App))
	if len(got) != len(docCatalog(t)) {
		t.Errorf("rows after the denied writes = %d, want %d", len(got), len(docCatalog(t)))
	}
}

// El CHECK del código repite la lista de acciones y la clave primaria impide repetir: ni siquiera
// el dueño (una migración) puede sembrar una acción fuera de la lista ni un código dos veces.
func TestPermissionsTableRejectsBadCodes(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	d.Migrate(ctx, t)
	owner := connectAs(t, d, dbtest.Migrator)

	tests := []struct {
		name, code, state string
	}{
		{"action outside the list", "ventas.pos:delete", "23514"},
		{"uppercase", "Ventas.pos:read", "23514"},
		{"no action", "ventas.pos", "23514"},
		{"platform code", "plataforma.tenants:suspend", "23514"},
		{"duplicate", "ventas.pos:read", "23505"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := owner.Exec(ctx, `INSERT INTO iam.permissions (code, description) VALUES ($1, 'x')`, tt.code)
			wantState(t, err, tt.state, tt.code)
		})
	}
	_, err := owner.Exec(ctx, `INSERT INTO iam.permissions (code, description) VALUES ('ventas.pos:void', '')`)
	wantState(t, err, "23514", "empty description")

	// Cada acción permitida entra: el CHECK y Actions() dicen lo mismo.
	for _, a := range []string{"read", "read_all", "create", "update", "void", "export", "import", "approve", "send", "retry"} {
		if !slices.Contains(permission.Actions(), a) {
			t.Errorf("action %s missing from permission.Actions()", a)
		}
		code := "zzprobe.resource:" + a
		if _, err := owner.Exec(ctx, `INSERT INTO iam.permissions (code, description) VALUES ($1, 'x')`, code); err != nil {
			t.Errorf("owner cannot insert %s: %v", code, err)
		}
	}
}
