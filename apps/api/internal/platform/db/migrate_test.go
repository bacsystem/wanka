package db_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/migrations"
)

// reserved son los esquemas del data-model §1 más app (funciones del kernel).
var reserved = []string{
	"app", "iam", "org", "crm", "catalog", "inv", "sales", "einv", "purch", "fin", "rpt", "agenda", "integ",
	"audit", "notify", "lube", "books", "resto", "vet", "dental", "platform", "ops",
}

func migrationFiles(t *testing.T) []string {
	t.Helper()
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil || len(names) == 0 {
		t.Fatalf("no embedded migrations: %v", err)
	}
	return names
}

func queryOne[T any](t *testing.T, conn *pgx.Conn, sql string, args ...any) T {
	t.Helper()
	var v T
	if err := conn.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

func TestMigrationFilesAreForwardOnlyAndCreateNoRoles(t *testing.T) {
	name := regexp.MustCompile(`^[0-9]{14}_[a-z0-9_]+\.sql$`)
	forbidden := regexp.MustCompile(`(?i)\b(CREATE|ALTER|DROP)\s+ROLE\b|\bCREATE\s+USER\b|\bBYPASSRLS\b|\bSUPERUSER\b|\bPASSWORD\b`)
	for _, f := range migrationFiles(t) {
		if !name.MatchString(f) {
			t.Errorf("%s: name must be <yyyymmddhhmmss>_<name>.sql", f)
		}
		body, err := fs.ReadFile(migrations.FS, f)
		if err != nil {
			t.Fatal(err)
		}
		if m := forbidden.Find(body); m != nil {
			t.Errorf("%s: migrations must not touch roles or passwords (found %q): that is the bootstrap's job", f, m)
		}
		if regexp.MustCompile(`(?m)^\s*--\s*\+goose\s+Down`).Match(body) {
			t.Errorf("%s: no Down blocks: forward only (data-model §6)", f)
		}
	}
}

func TestMigrateUpAppliesFromScratchAndTwiceIsANoOp(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	log := discardLog()
	want := len(migrationFiles(t))

	n, err := db.Migrate(ctx, d.URL(dbtest.Migrator), migrations.FS, log)
	if err != nil || n != want {
		t.Fatalf("first run: applied %d (want %d), err %v", n, want, err)
	}
	admin := d.Admin(ctx, t)
	before := queryOne[string](t, admin, `SELECT string_agg(version_id::text || ':' || is_applied::text || ':' || tstamp::text, ',' ORDER BY id) FROM goose.goose_db_version`)

	n, err = db.Migrate(ctx, d.URL(dbtest.Migrator), migrations.FS, log)
	if err != nil || n != 0 {
		t.Fatalf("second run: applied %d, err %v; want a no-op", n, err)
	}
	after := queryOne[string](t, admin, `SELECT string_agg(version_id::text || ':' || is_applied::text || ':' || tstamp::text, ',' ORDER BY id) FROM goose.goose_db_version`)
	if before != after {
		t.Fatalf("version table changed on the second run:\n%s\n%s", before, after)
	}
}

func TestConcurrentMigrateAppliesEachMigrationOnce(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	want := len(migrationFiles(t))

	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := db.Migrate(ctx, d.URL(dbtest.Migrator), migrations.FS, discardLog())
			mu.Lock()
			total += n
			mu.Unlock()
			if err != nil {
				t.Errorf("Migrate: %v", err)
			}
		}()
	}
	wg.Wait()
	if total != want {
		t.Fatalf("migrations applied %d times in total, want %d", total, want)
	}
}

func TestInitialMigrationCreatesSchemasAndNullifFunctions(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	d.Migrate(ctx, t)
	admin := d.Admin(ctx, t)

	for _, s := range reserved {
		if !queryOne[bool](t, admin, `SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)`, s) {
			t.Errorf("schema %q was not created", s)
		}
	}

	// app.current_tenant / app.current_user_id: NULL sin fijar o con '' (una
	// conexión del pool que ya corrió set_config(..., true)), el uuid si está
	// fijado, y el NULLIF está en el código fuente.
	const id = "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"
	for _, fn := range []struct{ name, setting string }{{"app.current_tenant", "app.tenant_id"}, {"app.current_user_id", "app.user_id"}} {
		t.Run(fn.name, func(t *testing.T) {
			conn, err := pgx.Connect(ctx, d.URL(dbtest.App))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close(ctx)
			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			var got *string
			read := func() *string {
				got = nil
				if err := tx.QueryRow(ctx, `SELECT `+fn.name+`()::text`).Scan(&got); err != nil {
					t.Fatalf("%s(): %v", fn.name, err)
				}
				return got
			}
			if read() != nil {
				t.Fatal("unset setting must give NULL")
			}
			if _, err := tx.Exec(ctx, `SELECT set_config($1, '', true)`, fn.setting); err != nil {
				t.Fatal(err)
			}
			if read() != nil {
				t.Fatal("empty setting must give NULL, not a cast error")
			}
			if _, err := tx.Exec(ctx, `SELECT set_config($1, $2, true)`, fn.setting, id); err != nil {
				t.Fatal(err)
			}
			if g := read(); g == nil || *g != id {
				t.Fatalf("got %v, want %s", g, id)
			}
		})
		src := queryOne[string](t, admin, `SELECT prosrc FROM pg_proc WHERE oid = $1::regproc`, fn.name)
		if !strings.Contains(src, "NULLIF(current_setting('"+fn.setting+"', true), '')::uuid") {
			t.Errorf("%s source lacks the NULLIF: %s", fn.name, src)
		}
	}
}

func TestMigratedObjectsBelongToMigratorAndRolesGotTheirGrants(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	d.Migrate(ctx, t)
	admin := d.Admin(ctx, t)

	// Invariante: wanka_app y wanka_worker (y cualquier otro rol de servicio) no son dueños de nada.
	// Única excepción: wanka_definer es dueño de las funciones SECURITY DEFINER (ADR 0001), y de nada más.
	owned := queryOne[int](t, admin, `
		SELECT (SELECT count(*) FROM pg_namespace n JOIN pg_roles r ON r.oid = n.nspowner
		         WHERE r.rolname LIKE 'wanka\_%' AND r.rolname <> 'wanka_migrator' AND n.nspname !~ '^pg_')
		     + (SELECT count(*) FROM pg_class c JOIN pg_roles r ON r.oid = c.relowner
		         WHERE r.rolname LIKE 'wanka\_%' AND r.rolname <> 'wanka_migrator')
		     + (SELECT count(*) FROM pg_proc p JOIN pg_roles r ON r.oid = p.proowner
		         WHERE r.rolname LIKE 'wanka\_%' AND r.rolname <> 'wanka_migrator'
		           AND NOT (r.rolname = 'wanka_definer' AND p.prosecdef))`)
	if owned != 0 {
		t.Fatalf("%d objects owned by a role other than wanka_migrator", owned)
	}
	if o := queryOne[string](t, admin, `SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname = 'app'`); o != "wanka_migrator" {
		t.Fatalf("schema app owner = %s", o)
	}
	for _, role := range []string{"wanka_app", "wanka_worker", "wanka_webhooks", "wanka_platform"} {
		for _, schema := range []string{"app", "ops"} {
			if !queryOne[bool](t, admin, `SELECT has_schema_privilege($1, $2, 'USAGE')`, role, schema) {
				t.Errorf("%s lacks USAGE on %s", role, schema)
			}
		}
	}
}

// Un GRANT a un rol que no existe falla y la migración se revierte: así se nota
// un bootstrap olvidado, fallando cerrado (ADR 0001).
func TestMissingRoleMakesMigrationFailAndRollBack(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	body, err := fs.ReadFile(migrations.FS, migrationFiles(t)[0])
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(body), "wanka_platform;", "wanka_role_that_does_not_exist;", 1)
	if mutated == string(body) {
		t.Fatal("test setup: the initial migration no longer ends its GRANT with wanka_platform")
	}
	fsys := fstest.MapFS{"20000101000000_init.sql": {Data: []byte(mutated)}}

	n, err := db.Migrate(ctx, d.URL(dbtest.Migrator), fsys, discardLog())
	if err == nil {
		t.Fatal("expected an error for the missing role")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42704" {
		t.Fatalf("err = %v, want SQLSTATE 42704 (undefined_object)", err)
	}
	if n != 0 {
		t.Fatalf("applied = %d", n)
	}
	if queryOne[bool](t, d.Admin(ctx, t), `SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'app')`) {
		t.Fatal("a failed migration left its schemas behind")
	}
}

func TestMigrateRefusesPrivilegedRoles(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	admin := d.Admin(ctx, t)

	if _, err := db.Migrate(ctx, d.SuperuserURL(), migrations.FS, discardLog()); err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("superuser: err = %v, want a refusal", err)
	}
	if queryOne[bool](t, admin, `SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'app')`) {
		t.Fatal("a refused migration changed the database")
	}
}

func TestMigratorSessionHasLockTimeoutAndCannotCreateRoles(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()

	// Una migración que verifica la configuración de la sesión en que corre.
	fsys := fstest.MapFS{"20000101000000_session.sql": {Data: []byte(`-- +goose Up
-- +goose StatementBegin
DO $$ BEGIN
  IF current_setting('lock_timeout') <> '5s' THEN RAISE EXCEPTION 'lock_timeout is %', current_setting('lock_timeout'); END IF;
  IF current_user <> 'wanka_migrator' THEN RAISE EXCEPTION 'running as %', current_user; END IF;
END $$;
-- +goose StatementEnd
`)}}
	if _, err := db.Migrate(ctx, d.URL(dbtest.Migrator), fsys, discardLog()); err != nil {
		t.Fatal(err)
	}

	// Invariante: wanka_migrator no puede crear roles.
	conn, err := pgx.Connect(ctx, d.URL(dbtest.Migrator))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	for _, stmt := range []string{`CREATE ROLE wanka_intruder`, `CREATE ROLE wanka_intruder SUPERUSER`, `ALTER ROLE wanka_app BYPASSRLS`, `ALTER ROLE wanka_app SET statement_timeout = '1h'`} {
		_, err := conn.Exec(ctx, stmt)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("%s: err = %v, want SQLSTATE 42501", stmt, err)
		}
	}
}

func TestMigrateErrorsDoNotLeakTheURL(t *testing.T) {
	d := dbtest.New(t)
	bad := strings.Replace(d.URL(dbtest.Migrator), "://wanka_migrator:", "://wanka_migrator:WRONG-PW-XYZ", 1)
	_, err := db.Migrate(context.Background(), bad, migrations.FS, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("expected authentication failure")
	}
	if strings.Contains(err.Error(), "WRONG-PW-XYZ") || strings.Contains(err.Error(), bad) {
		t.Fatalf("error leaks the URL: %v", err)
	}
}

// Un rol LOGIN con BYPASSRLS pero sin SUPERUSER tampoco puede migrar: RLS
// parecería correcto sin aplicarse. El rol vive fuera del prefijo wanka_ para
// no descuadrar la autoverificación del bootstrap.
func TestMigrateRefusesBypassRLSWithoutSuperuser(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	admin := d.Admin(ctx, t)

	role := "bypass_" + strings.TrimPrefix(d.Name, "wanka_t_")
	if _, err := admin.Exec(ctx, `CREATE ROLE `+role+` LOGIN NOSUPERUSER BYPASSRLS PASSWORD 'pw'`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), `DROP OWNED BY `+role+`; DROP ROLE `+role) })
	if _, err := admin.Exec(ctx, `GRANT CREATE ON DATABASE `+pgx.Identifier{d.Name}.Sanitize()+` TO `+role); err != nil {
		t.Fatal(err)
	}

	u := strings.Replace(d.URL(dbtest.Migrator), "wanka_migrator:"+os.Getenv(dbtest.EnvRolePassword), role+":pw", 1)
	if u == d.URL(dbtest.Migrator) {
		t.Fatal("test setup: could not build the URL of the bypass role")
	}
	_, err := db.Migrate(ctx, u, migrations.FS, discardLog())
	if err == nil || !strings.Contains(err.Error(), "BYPASSRLS") {
		t.Fatalf("err = %v, want a refusal that names BYPASSRLS", err)
	}
	if queryOne[bool](t, admin, `SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'app')`) {
		t.Fatal("a refused migration changed the database")
	}
}

// Una sentencia sin esquema en una migración no puede caer en public: el
// migrador no tiene CREATE allí y falla con 42501, en vez de crear objetos que
// todos los roles verían.
func TestMigrationWithoutSchemaCannotCreateInPublic(t *testing.T) {
	d := dbtest.New(t)
	fsys := fstest.MapFS{"20000101000000_noschema.sql": {Data: []byte("-- +goose Up\nCREATE TABLE no_schema (id int);\n")}}
	_, err := db.Migrate(context.Background(), d.URL(dbtest.Migrator), fsys, discardLog())
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("err = %v, want SQLSTATE 42501", err)
	}
}

// goose guarda sus versiones en el esquema goose (del migrador), no en public.
func TestGooseVersionTableLivesInItsOwnSchema(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	d.Migrate(ctx, t)
	admin := d.Admin(ctx, t)
	if !queryOne[bool](t, admin, `SELECT to_regclass('goose.goose_db_version') IS NOT NULL`) {
		t.Fatal("goose.goose_db_version does not exist")
	}
	if queryOne[bool](t, admin, `SELECT to_regclass('public.goose_db_version') IS NOT NULL`) {
		t.Fatal("goose_db_version must not live in public")
	}
	if o := queryOne[string](t, admin, `SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname = 'goose'`); o != "wanka_migrator" {
		t.Fatalf("schema goose owner = %s", o)
	}
	for _, role := range []string{"wanka_app", "wanka_worker", "wanka_webhooks", "wanka_platform"} {
		if queryOne[bool](t, admin, `SELECT has_schema_privilege($1, 'goose', 'USAGE')`, role) {
			t.Errorf("%s can use the goose schema", role)
		}
	}
}
