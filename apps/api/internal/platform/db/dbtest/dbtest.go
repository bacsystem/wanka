// Package dbtest le da a cada test su propia base PostgreSQL, creada y borrada
// al vuelo, para que los tests de distintos paquetes (que Go corre en paralelo)
// nunca vean los datos de otro.
//
// Necesita un PostgreSQL 16 real cuya URL de superusuario esté en
// WANKA_TEST_ADMIN_URL y cuyos cinco roles LOGIN compartan la contraseña de
// WANKA_TEST_ROLE_PASSWORD (la CI y apps/api/README.md explican cómo
// prepararlos). El superusuario solo crea la base y aplica el bootstrap de
// roles; los tests se conectan con los roles reales. Sin WANKA_TEST_ADMIN_URL
// un test se salta en local, pero falla si CI está definida: una CI que olvidó
// la base no debe pasar.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/bootstrap"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/migrations"
)

// Variables de entorno que lee el helper.
const (
	EnvAdminURL     = "WANKA_TEST_ADMIN_URL"
	EnvRolePassword = "WANKA_TEST_ROLE_PASSWORD"
)

// Roles del bootstrap que pueden iniciar sesión.
const (
	Migrator = "wanka_migrator"
	App      = "wanka_app"
	Worker   = "wanka_worker"
	Webhooks = "wanka_webhooks"
	Platform = "wanka_platform"
)

// Advisory locks. Los roles son del clúster, así que los tests de distintos
// paquetes los comparten:
//   - bootstrapLockID serializa las corridas del bootstrap (dos ALTER ROLE
//     concurrentes sobre el mismo rol fallan con "tuple concurrently updated");
//   - clusterLockID lo toma en modo compartido cada test durante toda su vida y
//     en modo exclusivo el test que cambia los roles (NewExclusive), para que
//     corra solo y no rompa a los demás.
const (
	bootstrapLockID = 7162162
	clusterLockID   = 7162163
)

// DB es una base desechable con los roles ya creados por el bootstrap.
type DB struct {
	Name     string
	adminURL *url.URL
	password string
}

// New crea una base nueva, le aplica el bootstrap de roles y la borra al
// terminar el test. No migra: llamar a Migrate si el test necesita el esquema.
func New(t testing.TB) *DB {
	t.Helper()
	return create(t, false)
}

// NewExclusive es New para un test que altera los roles del clúster (por
// ejemplo, para probar que el bootstrap repara un atributo cambiado): espera a
// que no corra otro test y mantiene a los demás esperando hasta que termina.
func NewExclusive(t testing.TB) *DB {
	t.Helper()
	return create(t, true)
}

func create(t testing.TB, exclusive bool) *DB {
	t.Helper()
	raw := os.Getenv(EnvAdminURL)
	if raw == "" {
		if os.Getenv("CI") != "" {
			t.Fatalf("%s is not set in CI: the integration tests must run against a real database", EnvAdminURL)
		}
		t.Skipf("%s is not set: skipping the database integration test", EnvAdminURL)
	}
	admin, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("%s is not a valid URL", EnvAdminURL)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	holdClusterLock(ctx, t, raw, exclusive)

	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	d := &DB{Name: "wanka_t_" + hex.EncodeToString(b[:]), adminURL: admin, password: os.Getenv(EnvRolePassword)}

	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatalf("connecting as superuser: %v", scrub(err, raw))
	}
	defer conn.Close(ctx)
	// El lock cubre crear la base, el bootstrap y (más abajo) borrarla: crear y
	// borrar bases también toca el catálogo compartido de roles.
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, bootstrapLockID); err != nil {
		t.Fatalf("locking bootstrap: %v", err)
	}
	if _, err := conn.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{d.Name}.Sanitize()+` TEMPLATE template0`); err != nil {
		t.Fatalf("creating database: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		c, err := pgx.Connect(ctx, raw)
		if err != nil {
			t.Errorf("dropping database: %v", scrub(err, raw))
			return
		}
		defer c.Close(ctx)
		if _, err := c.Exec(ctx, `SELECT pg_advisory_lock($1)`, bootstrapLockID); err != nil {
			t.Errorf("locking before dropping: %v", err)
			return
		}
		if _, err := c.Exec(ctx, `DROP DATABASE IF EXISTS `+pgx.Identifier{d.Name}.Sanitize()+` WITH (FORCE)`); err != nil {
			t.Errorf("dropping database: %v", err)
		}
	})

	// Se aplica el bootstrap a la base nueva como superusuario, bajo el lock.
	dconn, err := pgx.Connect(ctx, d.adminURLFor(d.Name))
	if err != nil {
		t.Fatalf("connecting to the new database: %v", scrub(err, raw))
	}
	defer dconn.Close(ctx)
	// Exec sin argumentos usa el protocolo simple, que ejecuta el script completo.
	_, err = dconn.Exec(ctx, bootstrap.RolesSQL)
	if _, uerr := conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, bootstrapLockID); uerr != nil && err == nil {
		err = uerr
	}
	if err != nil {
		t.Fatalf("running bootstrap/roles.sql: %v", err)
	}
	return d
}

// holdClusterLock toma el lock del clúster en una conexión propia que queda
// abierta hasta que termina el test (cerrarla libera el lock).
func holdClusterLock(ctx context.Context, t testing.TB, raw string, exclusive bool) {
	t.Helper()
	c, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatalf("connecting as superuser: %v", scrub(err, raw))
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	fn := "pg_advisory_lock_shared"
	if exclusive {
		fn = "pg_advisory_lock"
	}
	if _, err := c.Exec(ctx, `SELECT `+fn+`($1)`, clusterLockID); err != nil {
		t.Fatalf("taking the cluster lock: %v", err)
	}
}

// URL devuelve la URL de conexión de role en esta base.
func (d *DB) URL(role string) string {
	u := *d.adminURL
	u.User = url.UserPassword(role, d.password)
	u.Path = "/" + d.Name
	u.RawQuery = ""
	return u.String()
}

// SuperuserURL devuelve la URL de superusuario de esta base. Las migraciones
// deben rechazarla; nada más de la aplicación debería usarla.
func (d *DB) SuperuserURL() string { return d.adminURLFor(d.Name) }

func (d *DB) adminURLFor(name string) string {
	u := *d.adminURL
	u.Path = "/" + name
	return u.String()
}

// Admin abre una conexión de superusuario a esta base, para comprobar el
// catálogo y hacer lo que los roles de la aplicación no pueden. Se cierra con el test.
func (d *DB) Admin(ctx context.Context, t testing.TB) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(ctx, d.adminURLFor(d.Name))
	if err != nil {
		t.Fatalf("connecting as superuser: %v", scrub(err, d.adminURL.String()))
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// Migrate aplica las migraciones embebidas como wanka_migrator.
func (d *DB) Migrate(ctx context.Context, t testing.TB) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := db.Migrate(ctx, d.URL(Migrator), migrations.FS, log); err != nil {
		t.Fatalf("migrating: %v", err)
	}
}

// scrub evita que la contraseña del superusuario aparezca en los mensajes de fallo.
func scrub(err error, raw string) string {
	msg := err.Error()
	if u, perr := url.Parse(raw); perr == nil && u.User != nil {
		if pw, ok := u.User.Password(); ok && pw != "" {
			return strings.ReplaceAll(msg, pw, "[REDACTED]")
		}
	}
	return msg
}

// MigrateWith aplica las migraciones embebidas más las de extra (migraciones
// solo de test, como una tabla con la política estándar). Los nombres de extra
// deben ordenarse después de los de producción para no alterar su orden.
func (d *DB) MigrateWith(ctx context.Context, t testing.TB, extra fs.FS) {
	t.Helper()
	merged := fstest.MapFS{}
	for _, src := range []fs.FS{migrations.FS, extra} {
		files, err := fs.Glob(src, "*.sql")
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range files {
			data, err := fs.ReadFile(src, name)
			if err != nil {
				t.Fatal(err)
			}
			if _, dup := merged[name]; dup {
				t.Fatalf("migration %s is in both file sets", name)
			}
			merged[name] = &fstest.MapFile{Data: data}
		}
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := db.Migrate(ctx, d.URL(Migrator), merged, log); err != nil {
		t.Fatalf("migrating: %v", err)
	}
}
