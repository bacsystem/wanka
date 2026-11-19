package auth_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

// Sobrecarga del paso de sesión con el doble y los dos limitadores frente al
// mismo handler vacío, con el método de httpx/bench_test.go. Para repetirla
// (desde apps/api):
//
//	go test -run '^$' -bench Session -benchmem -count 5 ./internal/platform/auth
//
// La mediana de SessionStep menos la de SessionBaseline debe ser < 0,1 ms. La
// medición con la base real y a 300 req/s es de #196.
func BenchmarkSessionBaseline(b *testing.B) { benchSession(b, false) }

func BenchmarkSessionStep(b *testing.B) { benchSession(b, true) }

func benchSession(b *testing.B, withSession bool) {
	res := &fakeResolver{lookup: func(context.Context, [32]byte) (auth.SessionRecord, bool, error) { return bound(), true, nil }}
	rt := httpx.NewRouter(httpx.RouterOptions{})
	huge := httpx.LimiterConfig{Capacity: 1 << 30, Refill: time.Nanosecond, MaxKeys: 10}
	routes, err := auth.NewRoutes(rt, res, grants(plan()), auth.Options{SessionLimiter: huge, TenantLimiter: huge})
	if err != nil {
		b.Fatal(err)
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	if withSession {
		routes.Handle("GET /v1/x", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:read"}, ok)
	} else {
		rt.Handle("GET /v1/x", ok)
	}
	h := httpx.Chain(rt, logging.New(io.Discard, slog.LevelInfo), nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/x", nil)
	req.Header.Set("Authorization", bearer(token(1)))
	b.ReportAllocs()
	for b.Loop() {
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
}
