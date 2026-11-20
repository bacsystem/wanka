package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/config"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// El escucha de NOTIFY que arranca serve es el mismo Service que leen las rutas:
// un cambio de la suscripción llega a la respuesta de una ruta servida por serve
// mucho antes del TTL de 30 s.
func TestServeListenerInvalidatesTheServiceTheRoutesRead(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	d.Migrate(ctx, t)
	admin := d.Admin(ctx, t)
	s := d.SeedSession(t, dbtest.SessionSpec{})
	var planID string
	if err := admin.QueryRow(ctx, `INSERT INTO platform.plans (code, name, limits) VALUES ('p-serve', 'p', '{"users": 5, "sites": 5, "cpe_per_month": 5}') RETURNING id::text`).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO platform.plan_modules (plan_id, module_code) VALUES ('` + planID + `', 'ventas')`,
		`INSERT INTO platform.subscriptions (id, tenant_id, plan_id, status) VALUES (gen_random_uuid(), '` + s.TenantID + `', '` + planID + `', 'activa')`,
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}

	logs := &lockedBuf{}
	w := wiring{sweepInterval: time.Hour, mount: func(r *auth.Routes) {
		r.HandleFunc("GET /v1/items", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:read"},
			func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	}}
	cfg := config.Config{DatabaseURL: d.URL(dbtest.App), DBPoolMax: 4, HTTPAddr: "127.0.0.1:0"}
	sctx, cancel := context.WithCancel(ctx)
	errc := make(chan error, 1)
	go func() { errc <- serve(sctx, slog.New(slog.NewJSONHandler(logs, nil)), cfg, config.API, "tenant", w) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-errc:
		case <-time.After(10 * time.Second):
			t.Error("serve did not stop")
		}
	})

	addr := ""
	waitFor(t, "serve listening", func() bool {
		for _, ln := range strings.Split(logs.String(), "\n") {
			var m map[string]any
			if json.Unmarshal([]byte(ln), &m) == nil && m["msg"] == "listening" {
				addr, _ = m["addr"].(string)
			}
		}
		return addr != ""
	})
	waitFor(t, "the LISTEN connection", func() bool {
		var n int
		_ = admin.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE query LIKE 'LISTEN %' AND datname = current_database()`).Scan(&n)
		return n > 0
	})
	get := func() (int, string) {
		req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/v1/items", nil)
		req.Header.Set("Authorization", "Bearer "+s.Token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var p struct{ Code string }
		_ = json.NewDecoder(resp.Body).Decode(&p)
		return resp.StatusCode, p.Code
	}
	// Caliente la caché. Los entitlements dejan pasar (el módulo ventas está en el plan) y
	// decide el paso de permisos con el resolver de PostgreSQL que cablea serve (#233; el rol
	// sembrado no tiene permisos): 403 forbidden, y no module_not_enabled.
	if code, problem := get(); code != 403 || problem != "forbidden" {
		t.Fatalf("GET /v1/items = %d %s, want 403 forbidden", code, problem)
	}
	waitFor(t, "the 402 after the subscription turns morosa (NOTIFY, not the 30 s TTL)", func() bool {
		if _, err := admin.Exec(ctx, `UPDATE platform.subscriptions SET status = 'morosa' WHERE tenant_id = $1`, s.TenantID); err != nil {
			t.Fatal(err)
		}
		code, _ := get()
		return code == 402
	})
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for: %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
