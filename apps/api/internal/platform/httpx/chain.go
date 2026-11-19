package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
)

// MaxBodyBytes es el cuerpo más grande que acepta la API (api/conventions.md §7).
const MaxBodyBytes = 1 << 20

// Chain envuelve h con, de afuera hacia adentro: request id, log de acceso,
// recuperación de panic y límite de cuerpo. m puede ser nil. El logger y m
// quedan en el contexto para WriteProblem, WriteDBError y los plazos.
//
// El límite de cuerpo responde 413 payload_too_large sin leer el cuerpo si
// Content-Length lo excede, y corta la lectura en MaxBodyBytes+1 si no hay
// Content-Length (DecodeJSON lo traduce al mismo 413).
func Chain(h http.Handler, log *slog.Logger, m metrics.Recorder) http.Handler {
	if m == nil {
		m = metrics.Nop{}
	}
	d := deps{log: log, metrics: m}
	inner := requestID(accessLog(log, recoverPanic(limitBody(h))))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rd := d // copia por petición: start no se comparte entre goroutines
		rd.start = time.Now()
		inner.ServeHTTP(w, r.WithContext(withDeps(r.Context(), rd)))
	})
}

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > MaxBodyBytes {
			WriteProblem(w, r, "payload_too_large", "request body exceeds the 1 MB limit")
			return
		}
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

// validRequestID acepta solo tokens cortos y simples para que quien llama no
// pueda inyectar líneas de log ni valores enormes (conventions.md §3).
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
		panic("httpx: crypto/rand failed: " + err.Error()) // no ocurre en las plataformas soportadas
	}
	return hex.EncodeToString(b[:])
}

// statusWriter recuerda el status y si ya se escribió algo.
type statusWriter struct {
	http.ResponseWriter
	status  int
	wrote   bool
	problem bool // lo escrito es un problema RFC 9457 completo, no un envío a medias
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wrote {
		w.status, w.wrote = code, true
		w.problem = w.Header().Get("Content-Type") == "application/problem+json"
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	w.wrote = true
	return w.ResponseWriter.Write(p)
}

// Unwrap deja que http.ResponseController llegue al escritor real (Flush).
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// track devuelve w como *statusWriter, envolviéndolo si hace falta.
func track(w http.ResponseWriter) *statusWriter {
	if sw, ok := w.(*statusWriter); ok {
		return sw
	}
	return &statusWriter{ResponseWriter: w, status: http.StatusOK}
}

// accessLog registra una línea por petición, también si el handler aborta la
// conexión. Guarda la ruta pero nunca la query ni las cabeceras, que pueden
// llevar tokens. Si el cliente se fue sin que se escribiera nada, el status del
// log es 499 (solo en el log; no es un código del contrato).
func accessLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		finished := false
		defer func() {
			attrs := []any{"method", r.Method, "path", r.URL.Path}
			status := sw.status
			switch {
			case !finished:
				// El handler abortó la conexión (http.ErrAbortHandler): no es un 200.
				// Con nada enviado el status es 0 (no hubo línea de estado); con la
				// respuesta empezada queda el que se envió. Se distingue del 499,
				// que es el cliente que se fue, por el campo aborted.
				if !sw.wrote {
					status = 0
				}
				attrs = append(attrs, "status", status, "aborted", true)
			case !sw.wrote && errors.Is(r.Context().Err(), context.Canceled):
				attrs = append(attrs, "status", 499)
			default:
				attrs = append(attrs, "status", status)
			}
			attrs = append(attrs, "duration_ms", time.Since(start).Milliseconds())
			log.InfoContext(r.Context(), "request", attrs...)
		}()
		next.ServeHTTP(sw, r)
		finished = true
	})
}

// recoverPanic convierte un panic del handler en 500 internal: sin detalle ni
// traza para el cliente y con la traza en una línea error del log. Si ya se
// escribió parte de la respuesta no se puede cambiar el status: se corta la
// conexión. http.ErrAbortHandler sigue su camino sin log (convención de net/http).
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := track(w)
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			value, stack := v, debug.Stack()
			if hp, ok := v.(*handlerPanic); ok {
				value, stack = hp.value, hp.stack
			}
			if value == http.ErrAbortHandler {
				panic(http.ErrAbortHandler)
			}
			depsFrom(r.Context()).log.ErrorContext(r.Context(), "panic recovered",
				"panic", fmt.Sprint(value), "stack", string(stack))
			if sw.wrote {
				panic(http.ErrAbortHandler)
			}
			// Lo que el handler dejó en las cabeceras (Set-Cookie, Content-Type) no va en el 500.
			for k := range sw.Header() {
				if k != "X-Request-Id" {
					sw.Header().Del(k)
				}
			}
			WriteProblem(sw, r, "internal", "")
		}()
		next.ServeHTTP(sw, r)
	})
}

// handlerPanic lleva un panic de la goroutine del handler (deadline.go) a la
// goroutine de la petición, con la traza original.
type handlerPanic struct {
	value any
	stack []byte
}
