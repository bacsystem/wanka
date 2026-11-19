// Package httpx holds the HTTP plumbing shared by every plane: server
// timeouts, health endpoints, request ids, access log, panic recovery, the body
// limit, RFC 9457 problems, handler deadlines, routing and the rate limiter.
//
// Chain is the outer layer (request id, access log, recovery, body limit) and
// Router the inner one (undeclared routes, handler deadline, per-route
// middleware). Every error response goes through WriteProblem, WriteError or
// WriteDBError, so all of them are application/problem+json with a code of the
// closed list in api/conventions.md §4.
package httpx

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// ReadyFunc reports whether the process can serve traffic; a non-nil error means not ready.
type ReadyFunc func(ctx context.Context) error

// ErrNotReady is returned by NotReady.
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
)

// NewServer returns an http.Server with the timeouts of api/conventions.md §7.
func NewServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// NewMux returns the router with the routes every plane mounts: /healthz (and
// its alias /livez) never touches dependencies; /readyz does, through ready.
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
			// The cause is not returned to the caller: it may name internals.
			WriteProblem(w, r, "dependency_unavailable", "dependency not ready")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready"}` + "\n"))
	})
	return rt
}
