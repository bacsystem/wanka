// Servidor de medición de la sobrecarga de la cadena HTTP (#165). Sirve el mismo
// handler vacío en dos puertos: sin cadena (:18081) y detrás de Chain + Router +
// un limitador (:18082). Lo carga el generador separado de load.js:
//
//	go run ./testdata/loadtest &
//	RATE=300  URL=http://127.0.0.1:18081/x k6 run testdata/loadtest/load.js   # vacío
//	RATE=300  URL=http://127.0.0.1:18082/x k6 run testdata/loadtest/load.js   # cadena
//	(y lo mismo con RATE=1000)
//
// La sobrecarga es la diferencia de p(95) entre los dos puertos.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

func main() {
	body := []byte(`{"ok":true}` + "\n")
	h := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
	base := http.NewServeMux()
	base.HandleFunc("GET /x", h)

	lim, err := httpx.NewLimiter(httpx.LimiterConfig{Name: "session", Capacity: 1_000_000, Refill: time.Microsecond, MaxKeys: 1000,
		Key: func(*http.Request) (string, bool) { return "k", true }, Clock: clock.System{}})
	if err != nil {
		panic(err)
	}
	rt := httpx.NewRouter(httpx.RouterOptions{})
	rt.HandleFunc("GET /x", h, httpx.Use(lim.Middleware()))
	log := logging.New(os.Stderr, slog.LevelInfo) // el log de acceso real; redirigir stderr a /dev/null

	go func() { _ = http.ListenAndServe("127.0.0.1:18081", base) }()
	_ = httpx.NewServer("127.0.0.1:18082", httpx.Chain(rt, log, nil)).ListenAndServe()
}
