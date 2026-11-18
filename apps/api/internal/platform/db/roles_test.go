package db_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/bootstrap"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

var loginRoles = []string{"wanka_migrator", "wanka_app", "wanka_worker", "wanka_webhooks", "wanka_platform"}

type roleAttrs struct {
	login, super, createDB, createRole, replication, bypassRLS bool
}

func attrs(t *testing.T, c *pgx.Conn, role string) roleAttrs {
	t.Helper()
	var a roleAttrs
	err := c.QueryRow(context.Background(),
		`SELECT rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls FROM pg_roles WHERE rolname = $1`, role).
		Scan(&a.login, &a.super, &a.createDB, &a.createRole, &a.replication, &a.bypassRLS)
	if err != nil {
		t.Fatalf("role %s: %v", role, err)
	}
	return a
}

func rerunBootstrap(t *testing.T, c *pgx.Conn) {
	t.Helper()
	if _, err := c.Exec(context.Background(), bootstrap.RolesSQL); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
}

func TestBootstrapCreatesTheRolesWithTheADRAttributes(t *testing.T) {
	d := dbtest.New(t) // New ya corrió el bootstrap
	c := d.Admin(context.Background(), t)

	for _, r := range loginRoles {
		if got, want := attrs(t, c, r), (roleAttrs{login: true}); got != want {
			t.Errorf("%s = %+v, want only LOGIN", r, got)
		}
	}
	if got, want := attrs(t, c, "wanka_definer"), (roleAttrs{}); got != want {
		t.Errorf("wanka_definer = %+v, want NOLOGIN and nothing else", got)
	}
	if !queryOne[bool](t, c, `SELECT pg_has_role('wanka_migrator', 'wanka_definer', 'MEMBER')`) {
		t.Error("wanka_migrator is not a member of wanka_definer")
	}
	if !queryOne[bool](t, c, `SELECT has_database_privilege('wanka_migrator', current_database(), 'CREATE')`) {
		t.Error("wanka_migrator lacks CREATE on the database")
	}
	for _, r := range []string{"wanka_app", "wanka_worker", "wanka_webhooks", "wanka_platform", "wanka_definer"} {
		if queryOne[bool](t, c, `SELECT has_database_privilege($1, current_database(), 'CREATE')`, r) {
			t.Errorf("%s must not have CREATE on the database", r)
		}
	}
	if got := queryOne[string](t, c, `SELECT s.setconfig[1] FROM pg_db_role_setting s JOIN pg_roles r ON r.oid = s.setrole WHERE r.rolname = 'wanka_app' AND s.setdatabase = 0`); got != "statement_timeout=5s" {
		t.Errorf("wanka_app setting = %q", got)
	}
}

func TestBootstrapHasNoPasswordsAndNoPsqlMetaCommands(t *testing.T) {
	var code []string
	for _, line := range strings.Split(bootstrap.RolesSQL, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		code = append(code, line)
	}
	sql := strings.Join(code, "\n")
	if regexp.MustCompile(`(?i)\bpassword\b|\bencrypted\b`).MatchString(sql) {
		t.Error("roles.sql must not set passwords: the admin does it with \\password")
	}
	if regexp.MustCompile(`(?m)^\s*\\`).MatchString(sql) {
		t.Error("roles.sql must stay plain SQL (no psql meta-commands) so tests run the same text")
	}
}

func TestBootstrapIsIdempotentAndRestoresDriftedAttributes(t *testing.T) {
	d := dbtest.NewExclusive(t) // cambia roles del clúster: corre solo
	ctx := context.Background()
	c := d.Admin(ctx, t)

	rerunBootstrap(t, c) // segunda corrida sobre un clúster intacto: sin error y mismos atributos
	for _, r := range loginRoles {
		if got := attrs(t, c, r); got != (roleAttrs{login: true}) {
			t.Fatalf("%s changed on the second run: %+v", r, got)
		}
	}

	// Alguien altera atributos a mano. Los roles son del clúster, así que la
	// limpieza los repara aunque el test falle.
	drift := func() {
		for _, stmt := range []string{
			`ALTER ROLE wanka_app NOLOGIN`,
			`ALTER ROLE wanka_worker CREATEDB CREATEROLE BYPASSRLS`,
			`ALTER ROLE wanka_webhooks REPLICATION`,
			`ALTER ROLE wanka_platform NOLOGIN CREATEDB`,
			`ALTER ROLE wanka_definer LOGIN`,
			`ALTER ROLE wanka_app RESET statement_timeout`,
		} {
			if _, err := c.Exec(ctx, stmt); err != nil {
				t.Fatalf("%s: %v", stmt, err)
			}
		}
	}
	t.Cleanup(func() { _, _ = c.Exec(context.Background(), bootstrap.RolesSQL) })
	drift()
	if attrs(t, c, "wanka_app").login {
		t.Fatal("test setup: drift did not apply")
	}

	rerunBootstrap(t, c)

	for _, r := range loginRoles {
		if got := attrs(t, c, r); got != (roleAttrs{login: true}) {
			t.Errorf("%s not restored: %+v", r, got)
		}
	}
	if got := attrs(t, c, "wanka_definer"); got != (roleAttrs{}) {
		t.Errorf("wanka_definer not restored: %+v", got)
	}
	if got := queryOne[string](t, c, `SELECT s.setconfig[1] FROM pg_db_role_setting s JOIN pg_roles r ON r.oid = s.setrole WHERE r.rolname = 'wanka_app' AND s.setdatabase = 0`); got != "statement_timeout=5s" {
		t.Errorf("wanka_app statement_timeout not restored: %q", got)
	}
}

// El bootstrap se verifica a sí mismo: un estado que no puede reparar lo
// aborta. Un rol wanka_ de más descuadra la cuenta de roles.
func TestBootstrapSelfCheckAbortsOnAnUnexpectedRole(t *testing.T) {
	d := dbtest.NewExclusive(t)
	ctx := context.Background()
	c := d.Admin(ctx, t)
	if _, err := c.Exec(ctx, `CREATE ROLE wanka_zz_unexpected`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = c.Exec(context.Background(), `DROP ROLE IF EXISTS wanka_zz_unexpected`) }()
	_, err := c.Exec(ctx, bootstrap.RolesSQL)
	if err == nil || !strings.Contains(err.Error(), "faltan o sobran roles") {
		t.Fatalf("err = %v, want the self-check to abort", err)
	}
}
