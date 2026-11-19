package db_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// El check de CI (scripts/migrations-check.sh) y goose deben coincidir en qué
// anotaciones sacan una migración de la transacción. Aquí se contrasta el
// script con el comportamiento real de goose: cada variante se aplica con una
// sentencia final que falla; si el esquema creado antes sobrevive, goose no
// usó transacción. El script debe detectar toda variante que goose acepte (puede
// ser más estricto, nunca más laxo).
func TestMigrationsCheckDetectsEveryNoTransactionFormGooseAccepts(t *testing.T) {
	variants := []struct{ name, annotation string }{
		{"canonical", "-- +goose NO TRANSACTION"},
		{"lower case", "-- +goose no transaction"},
		{"mixed case without space", "--+goose No Transaction"},
		{"+goose glued to the word", "-- +gooseNO TRANSACTION"},
		{"words before +goose", "-- NO TRANSACTION +goose"},
		{"extra spaces", "--   +goose    NO   TRANSACTION"},
		{"long s (U+017F) in the word", "-- +goose NO TRAN\u017fACTION"},
		{"form feed at the end", "-- +goose NO TRANSACTION\f"},
		{"vertical tab after +goose", "-- +goose\v NO TRANSACTION"},
		{"no-break space at the end", "-- +goose NO TRANSACTION\u00a0"},
		{"em space after +goose", "-- +goose\u2003 NO TRANSACTION"},
		{"leading space", "   -- +goose NO TRANSACTION"},
		{"not an annotation", "-- NO TRANSACTION"},
		{"plain comment", "-- this migration is not in a transaction"},
	}
	script, err := filepath.Abs("../../../scripts/migrations-check.sh")
	if err != nil {
		t.Fatal(err)
	}
	d := dbtest.New(t)
	admin := d.Admin(context.Background(), t)

	for i, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			schema := fmt.Sprintf("probe_%d", i)
			body := v.annotation + "\n-- +goose Up\nCREATE SCHEMA " + schema + ";\nSELECT 1/0;\n"

			// Comportamiento real de goose: ¿sobrevive el esquema al error?
			_, err := db.Migrate(context.Background(), d.URL(dbtest.Migrator),
				fstest.MapFS{fmt.Sprintf("2000010100%04d_probe.sql", i): {Data: []byte(body)}}, discardLog())
			var pgErr *pgconn.PgError
			if err != nil && strings.Contains(err.Error(), "invalid annotation") {
				// goose rechaza la variante al leerla: nunca la aplica, no hay nada que contrastar.
				t.Logf("goose rejects %q: %v", v.annotation, err)
				return
			}
			if !errors.As(err, &pgErr) || pgErr.Code != "22012" {
				t.Fatalf("the probe migration should fail with division_by_zero, got %v", err)
			}
			gooseNoTx := queryOne[bool](t, admin, `SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)`, schema)

			// Lo que dice el script sobre el mismo archivo.
			f := filepath.Join(t.TempDir(), "m.sql")
			if err := os.WriteFile(f, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			// Exit 0: lo trata como NO TRANSACTION; 2: no reconoce la anotación; ambos lo bloquean.
			// Solo el 1 lo deja pasar.
			var ee *exec.ExitError
			err = exec.Command("sh", script, "--detect", f).Run()
			scriptNoTx := err == nil || (errors.As(err, &ee) && ee.ExitCode() == 2)

			if gooseNoTx && !scriptNoTx {
				t.Fatalf("goose runs %q outside a transaction but migrations-check.sh does not see it", v.annotation)
			}
			t.Logf("goose no-tx=%v, script no-tx=%v", gooseNoTx, scriptNoTx)
		})
	}
}

// Las anotaciones ENVSUB ON/OFF son válidas en goose: aplica la migración y el
// check no las toma por una anotación desconocida (sale con 1: no es NO TRANSACTION).
func TestMigrationsCheckAcceptsEnvsubAnnotations(t *testing.T) {
	script, err := filepath.Abs("../../../scripts/migrations-check.sh")
	if err != nil {
		t.Fatal(err)
	}
	d := dbtest.New(t)
	body := "-- +goose Up\n-- +goose ENVSUB ON\nCREATE SCHEMA envsub_probe;\n-- +goose ENVSUB OFF\nSELECT 1;\n"

	if _, err := db.Migrate(context.Background(), d.URL(dbtest.Migrator),
		fstest.MapFS{"20000101000000_envsub.sql": {Data: []byte(body)}}, discardLog()); err != nil {
		t.Fatalf("goose rejects ENVSUB annotations: %v", err)
	}
	f := filepath.Join(t.TempDir(), "m.sql")
	if err := os.WriteFile(f, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var ee *exec.ExitError
	err = exec.Command("sh", script, "--detect", f).Run()
	if !errors.As(err, &ee) || ee.ExitCode() != 1 {
		t.Fatalf("--detect on an ENVSUB migration = %v, want exit 1 (valid, not NO TRANSACTION)", err)
	}
}
