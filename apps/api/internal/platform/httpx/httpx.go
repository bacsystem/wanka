// Package httpx es la base HTTP común a todos los planos: timeouts del
// servidor, rutas de salud, request id, log de acceso, recuperación de panic,
// límite de cuerpo, problemas RFC 9457, plazos del handler, enrutado y limitador.
//
// Chain es la capa externa (request id, log de acceso, recuperación, límite de
// cuerpo) y Router la interna (rutas no declaradas, plazo del handler,
// middleware por ruta). Toda respuesta de error pasa por WriteProblem,
// WriteError o WriteDBError, así que todas son application/problem+json con un
// code de la lista cerrada de api/conventions.md §4.
package httpx

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// ReadyFunc informa si el proceso puede atender tráfico; un error no nulo significa que no.
type ReadyFunc func(ctx context.Context) error

// ErrNotReady es lo que devuelve NotReady.
var ErrNotReady = errors.New("database not connected")

// NotReady es una ReadyFunc que siempre falla.
func NotReady(context.Context) error { return ErrNotReady }

// Timeouts del servidor (api/conventions.md §7).
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	// writeTimeout debe ser mayor que el presupuesto de streaming (DefaultStreamTimeout).
	writeTimeout = 30 * time.Second
	// idleTimeout es mayor que el keep-alive del proxy (60 s, supuesto).
	idleTimeout = 120 * time.Second
	// maxHeaderBytes: cabeceras de más de 64 KiB reciben 431 de net/http (conventions.md §4);
	// el BFF envía unos pocos KB y el valor por defecto de Go (1 MB) es excesivo por conexión.
	maxHeaderBytes = 64 << 10
)

// NewServer devuelve un http.Server con los timeouts de api/conventions.md §7.
func NewServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		// Sin esto net/http responde "OPTIONS *" con un 200 vacío antes de Chain:
		// sin X-Request-Id, sin log y sin RFC 9457. Así llega al Router (404).
		DisableGeneralOptionsHandler: true,
	}
}

// NewMux devuelve el router con las rutas que monta todo plano: /healthz (y su
// alias /livez) no consulta dependencias; /readyz sí, mediante ready.
func NewMux(ready ReadyFunc) *Router {
	rt := NewRouter(RouterOptions{})
	live := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
	}
	rt.HandleFunc("GET /healthz", live)
	rt.HandleFunc("GET /livez", live)
	rt.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := ready(r.Context()); err != nil {
			// Un pool sin conexión libre es 503 overloaded y una base que no
			// responde es 503 dependency_unavailable (conventions.md §4).
			if WriteDBError(w, r, err) {
				return
			}
			// La causa no se devuelve: puede nombrar detalles internos.
			WriteProblem(w, r, "dependency_unavailable", "dependency not ready")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready"}` + "\n"))
	})
	return rt
}
