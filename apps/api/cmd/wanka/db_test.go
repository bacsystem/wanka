package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

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

// El worker con una base real levanta las cinco colas y se detiene limpio con SIGTERM.
func TestWorkerStartsTheFiveQueuesAndStopsOnSIGTERM(t *testing.T) {
	d := dbtest.New(t)
	d.Migrate(context.Background(), t)
	p := start(t, []string{"DATABASE_URL_WORKER=" + d.URL(dbtest.Worker), "JOBS_CONCURRENCY_REPORTS=1"}, "worker")
	p.waitMsg(t, "started")

	ctx := context.Background()
	conn := d.Admin(ctx, t)
	deadline := time.Now().Add(10 * time.Second)
	var queues []string
	for {
		queues = queues[:0]
		rows, err := conn.Query(ctx, `SELECT name FROM ops.river_queue ORDER BY name`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				t.Fatal(err)
			}
			queues = append(queues, n)
		}
		rows.Close()
		if len(queues) == 5 || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if strings.Join(queues, ",") != "cpe,maintenance,notifications,reports,webhooks" {
		t.Fatalf("queues started by the worker: %v", queues)
	}
	p.terminate(t)
}

// El comando worker registra la limpieza de claves de idempotencia: un job de ese kind lo ejecuta y borra solo las vencidas.
func TestWorkerRunsThePurgeOfIdempotencyKeys(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	d.Migrate(ctx, t)
	s := d.SeedSession(t, dbtest.SessionSpec{})
	conn := d.Admin(ctx, t)
	if _, err := conn.Exec(ctx, `INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash, status_code, created_at) VALUES
		($1, 'old', sha256('a'), 200, now() - interval '25 hours'), ($1, 'new', sha256('b'), 200, now())`, s.TenantID); err != nil {
		t.Fatal(err)
	}
	p := start(t, []string{"DATABASE_URL_WORKER=" + d.URL(dbtest.Worker)}, "worker")
	p.waitMsg(t, "started")
	if _, err := conn.Exec(ctx, `INSERT INTO ops.river_job (kind, args, queue) VALUES ('ops.purge_idempotency_keys', '{}', 'maintenance')`); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		var state string
		err := conn.QueryRow(ctx, `SELECT state::text FROM ops.river_job WHERE kind = 'ops.purge_idempotency_keys'`).Scan(&state)
		if err == nil && state == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the purge job did not complete (state %q, err %v)", state, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	var keys []string
	rows, err := conn.Query(ctx, `SELECT key FROM ops.idempotency_keys ORDER BY key`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k)
	}
	rows.Close()
	if strings.Join(keys, ",") != "new" {
		t.Fatalf("keys left: %v, want only the recent one", keys)
	}
	p.terminate(t)
}
