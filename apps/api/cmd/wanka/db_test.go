package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// runOnce corre el binario hasta que termina y devuelve su stdout como líneas JSON.
func runOnce(t *testing.T, env []string, args ...string) []map[string]any {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Env = env
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("wanka %v: %v\nstdout: %s\nstderr: %s", args, err, out, stderr.String())
	}
	var lines []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(l), &m) == nil {
			lines = append(lines, m)
		}
	}
	return lines
}

func find(lines []map[string]any, msg string) map[string]any {
	for _, l := range lines {
		if l["msg"] == msg {
			return l
		}
	}
	return nil
}

func TestMigrateUpTwiceTheSecondChangesNothing(t *testing.T) {
	d := dbtest.New(t)
	env := []string{"DATABASE_URL_MIGRATOR=" + d.URL(dbtest.Migrator)}

	first := runOnce(t, env, "migrate", "up")
	done := find(first, "migrations up to date")
	if done == nil || done["applied"].(float64) < 1 || find(first, "migration applied") == nil {
		t.Fatalf("first run did not apply migrations: %v", first)
	}
	second := runOnce(t, env, "migrate", "up")
	if done := find(second, "migrations up to date"); done == nil || done["applied"].(float64) != 0 || find(second, "migration applied") != nil {
		t.Fatalf("second run must be a no-op: %v", second)
	}
	pw := os.Getenv(dbtest.EnvRolePassword)
	for _, l := range append(first, second...) {
		if pw != "" && strings.Contains(toJSON(l), pw) {
			t.Fatalf("log leaks the password: %v", l)
		}
	}
}

func TestMigrateUpWithBadCredentialsFailsWithoutLeaking(t *testing.T) {
	// Una contraseña errónea debe fallar con salida distinta de 0 y sin secretos en la salida.
	d := dbtest.New(t)
	bad := strings.Replace(d.URL(dbtest.Migrator), "wanka_migrator:", "wanka_migrator:not-the-password", 1)
	cmd := exec.Command(binPath, "migrate", "up")
	cmd.Env = []string{"DATABASE_URL_MIGRATOR=" + bad}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected failure, got: %s", out)
	}
	if strings.Contains(string(out), "not-the-password") {
		t.Fatalf("output leaks the password: %s", out)
	}
}

func TestServeIsReadyAgainstARealDatabase(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	d.Migrate(ctx, t)

	for plane, role := range map[string]string{
		"tenant": dbtest.App, "platform": dbtest.Platform, "webhooks": dbtest.Webhooks,
	} {
		t.Run(plane, func(t *testing.T) {
			envVar := map[string]string{"tenant": "DATABASE_URL_APP", "platform": "DATABASE_URL_PLATFORM", "webhooks": "DATABASE_URL_WEBHOOKS"}[plane]
			p := start(t, []string{envVar + "=" + d.URL(role), "HTTP_ADDR=127.0.0.1:0", "DB_POOL_MAX_CONNS=2"}, "serve", "--plane="+plane)
			addr := p.waitMsg(t, "listening")["addr"].(string)
			resp, err := http.Get("http://" + addr + "/readyz")
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("readyz %d with a healthy database", resp.StatusCode)
			}
			// Cada plano se conecta con su propio rol.
			var n int
			c, err := pgx.Connect(ctx, d.URL(dbtest.Migrator))
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close(ctx)
			err = c.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname = $1 AND usename = $2 AND application_name LIKE 'wanka-%'`, d.Name, role).Scan(&n)
			if err != nil || n < 1 {
				t.Fatalf("no connection of role %s from %s (n=%d, err=%v)", role, plane, n, err)
			}
			p.terminate(t)
		})
	}
}

func toJSON(m map[string]any) string { b, _ := json.Marshal(m); return string(b) }
