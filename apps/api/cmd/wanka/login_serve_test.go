package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/adapters/argon2"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/config"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// Todo lo que registra el router de producción bajo /v1/auth/* rechaza Idempotency-Key antes de ejecutar nada
// (conventions.md §3). Recorre Registered() del mismo mountPlane que usa serve: una ruta futura queda cubierta sin tocar esto.
func TestEveryAuthRouteRejectsIdempotencyKey(t *testing.T) {
	rt := httpx.NewRouter(httpx.RouterOptions{})
	hit := false
	w := defaultWiring
	w.mount = func(r *auth.Routes) {
		// Una ruta NoTenant de /v1/auth/*, para cubrir la otra clase que no lleva sesión previa a la guarda.
		r.HandleFunc("POST /v1/auth/probe", auth.NoTenant, auth.Spec{Module: auth.NoModule, Permission: auth.NoPermission},
			func(http.ResponseWriter, *http.Request) { hit = true })
	}
	sw, _, err := mountPlane("tenant", rt, nil, w)
	if err != nil {
		t.Fatal(err)
	}
	routes := sw.(*auth.Routes)
	n := 0
	for _, ri := range routes.Registered() {
		method, path, _ := strings.Cut(ri.Pattern, " ")
		if !strings.HasPrefix(path, "/v1/auth/") {
			continue
		}
		n++
		r := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", "0b8f4f5e-3a8e-4c0e-9b7e-1d2f3a4b5c6d")
		rec := httptest.NewRecorder()
		httpx.Chain(rt, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil).ServeHTTP(rec, r)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), `"malformed_request"`) {
			t.Errorf("%s: status = %d, body %s, want 400 malformed_request", ri.Pattern, rec.Code, rec.Body)
		}
	}
	if n < 2 || hit {
		t.Fatalf("checked %d /v1/auth routes, the probe handler ran = %v", n, hit)
	}
}

// El login que sirve serve usa API_TRUST_CLIENT_IP de la configuración: con true guarda en la sesión la IP de
// X-Client-Ip, con false la del socket.
func TestServedLoginHonoursAPITrustClientIP(t *testing.T) {
	for _, trust := range []bool{true, false} {
		name := "API_TRUST_CLIENT_IP=false"
		if trust {
			name = "API_TRUST_CLIENT_IP=true"
		}
		t.Run(name, func(t *testing.T) {
			d := dbtest.New(t)
			ctx := context.Background()
			d.Migrate(ctx, t)
			admin := d.Admin(ctx, t)
			h, err := mustHasher().Hash("correct horse battery staple")
			if err != nil {
				t.Fatal(err)
			}
			var uid string
			if err := admin.QueryRow(ctx, `INSERT INTO iam.users (id, email, password_hash) VALUES (gen_random_uuid(), 'ana@x.pe', $1) RETURNING id::text`, h).Scan(&uid); err != nil {
				t.Fatal(err)
			}
			if _, err := admin.Exec(ctx, `
				WITH t AS (INSERT INTO org.tenants (id) VALUES (gen_random_uuid()) RETURNING id),
				     r AS (INSERT INTO iam.roles (id, tenant_id, name) SELECT gen_random_uuid(), t.id, 'Rol' FROM t RETURNING id, tenant_id)
				INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status) SELECT gen_random_uuid(), r.tenant_id, $1::uuid, r.id, 'activo' FROM r`, uid); err != nil {
				t.Fatal(err)
			}

			logs := &lockedBuf{}
			cfg := config.Config{DatabaseURL: d.URL(dbtest.App), DBPoolMax: 4, HTTPAddr: "127.0.0.1:0", TrustClientIP: trust}
			sctx, cancel := context.WithCancel(ctx)
			errc := make(chan error, 1)
			go func() {
				errc <- serve(sctx, slog.New(slog.NewJSONHandler(logs, nil)), cfg, config.API, "tenant", wiring{sweepInterval: time.Hour})
			}()
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
			req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/auth/login",
				strings.NewReader(`{"email":"ana@x.pe","password":"correct horse battery staple"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Client-Ip", "203.0.113.77")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("status = %d", resp.StatusCode)
			}
			var ip string
			if err := admin.QueryRow(ctx, `SELECT host(ip) FROM iam.sessions WHERE user_id = $1::uuid`, uid).Scan(&ip); err != nil {
				t.Fatal(err)
			}
			want := "127.0.0.1"
			if trust {
				want = "203.0.113.77"
			}
			if ip != want {
				t.Fatalf("session ip = %q, want %q", ip, want)
			}
		})
	}
}

func mustHasher() *argon2.Hasher {
	h, err := argon2.New(argon2.Default)
	if err != nil {
		panic(err)
	}
	return h
}
