package httpx_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

// Sobrecarga de la cadena de #165 (request id, log de acceso, recuperación de
// panic, límite de cuerpo, plazo con buffer y un limitador) frente al mismo
// handler vacío. Para repetirla (desde apps/api):
//
//	go test -run '^$' -bench 'Overhead' -benchmem ./internal/platform/httpx
//
// La medición con carga real y un generador separado está en testdata/loadtest.
func BenchmarkChainOverheadBaseline(b *testing.B) {
	benchHandler(b, emptyHandler())
}

func BenchmarkChainOverhead(b *testing.B) {
	lim, err := httpx.NewLimiter(httpx.LimiterConfig{Name: "bench", Capacity: 1 << 30, Refill: time.Nanosecond, MaxKeys: 10,
		Key: func(*http.Request) (string, bool) { return "k", true }, Clock: clock.System{}})
	if err != nil {
		b.Fatal(err)
	}
	rt := httpx.NewRouter(httpx.RouterOptions{})
	rt.Handle("GET /x", emptyHandler(), httpx.Use(lim.Middleware()))
	benchHandler(b, httpx.Chain(rt, logging.New(io.Discard, slog.LevelInfo), nil))
}

func emptyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
}

func benchHandler(b *testing.B, h http.Handler) {
	b.ReportAllocs()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	for b.Loop() {
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
}
