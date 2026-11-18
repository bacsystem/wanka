// Package httpx holds the HTTP plumbing shared by every plane: server
// timeouts, health endpoints, request ids, access log and the body limit.
package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

// MaxBodyBytes is the largest request body accepted (api/conventions.md §7).
const MaxBodyBytes = 1 << 20

// ReadyFunc reports whether the process can serve traffic; a non-nil error means not ready.
type ReadyFunc func(ctx context.Context) error

// ErrNotReady is returned by NotReady.
var ErrNotReady = errors.New("database not connected")

// NotReady es una ReadyFunc que siempre falla.
func NotReady(context.Context) error { return ErrNotReady }

// NewServer returns an http.Server with the timeouts of api/conventions.md §7.
// IdleTimeout (120 s) is above the proxy keep-alive. The write timeout leaves
// room for the 10 s handler timeout that arrives with the first routes.
func NewServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// NewMux returns the routes every plane mounts: /healthz (and its alias
// /livez) never touches dependencies; /readyz does, through ready.
func NewMux(ready ReadyFunc) *http.ServeMux {
	mux := http.NewServeMux()
	live := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
	}
	mux.HandleFunc("GET /healthz", live)
	mux.HandleFunc("GET /livez", live)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := ready(r.Context()); err != nil {
			// Un pool sin conexión libre es 503 overloaded y una base que no
			// responde es 503 dependency_unavailable (conventions.md §4).
			if WriteDBError(w, r, err) {
				return
			}
			// The cause is not returned to the caller: it may name internals.
			w.Header().Set("Retry-After", "1")
			WriteProblem(w, r, http.StatusServiceUnavailable, "dependency_unavailable", "Service Unavailable", "dependency not ready")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready"}` + "\n"))
	})
	return mux
}

// WriteProblem writes an RFC 9457 problem document with the fields the API
// contract requires (openapi.yaml Problem): code comes from the closed list in
// conventions.md §4 and requestId from the request context.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "about:blank", "title": title, "status": status, "code": code,
		"detail": detail, "requestId": logging.RequestID(r.Context()),
	})
}

// WriteDBError responde el error de base de datos que es contrapresión o base
// caída: 503 overloaded si el pool no tiene conexión libre, 503
// dependency_unavailable si la base no responde, ambos con Retry-After: 1
// (api/conventions.md §4). Informa si escribió la respuesta; cualquier otro
// error lo resuelve quien llama. La causa nunca se devuelve.
func WriteDBError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, db.ErrOverloaded):
		w.Header().Set("Retry-After", "1")
		WriteProblem(w, r, http.StatusServiceUnavailable, "overloaded", "Service Unavailable", "the server is busy, retry shortly")
	case errors.Is(err, db.ErrUnavailable):
		w.Header().Set("Retry-After", "1")
		WriteProblem(w, r, http.StatusServiceUnavailable, "dependency_unavailable", "Service Unavailable", "a dependency is not responding")
	default:
		return false
	}
	return true
}

// Chain wraps h with request id, body limit and access log, in that order.
func Chain(h http.Handler, log *slog.Logger) http.Handler {
	return requestID(accessLog(log, limitBody(h)))
}

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if !validRequestID(id) {
			id = newRequestID()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(logging.WithRequestID(r.Context(), id)))
	})
}

// validRequestID accepts only short, plain tokens so a caller cannot inject
// log lines or oversized values.
func validRequestID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("httpx: crypto/rand failed: " + err.Error()) // does not happen on supported platforms
	}
	return hex.EncodeToString(b[:])
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the real writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// accessLog logs one line per request. It records the path but never the query
// string or headers, which may carry tokens.
func accessLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.InfoContext(r.Context(), "request",
			"method", r.Method, "path", r.URL.Path, "status", sw.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
}
