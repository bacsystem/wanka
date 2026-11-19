package httpx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
)

// handlerDeadline aplica el plazo del handler con la semántica de
// http.TimeoutHandler, pero con un 503 overloaded RFC 9457 (conventions.md §4,
// «Plazo del handler»; el 503 de TimeoutHandler es HTML sin code ni requestId).
//
// El handler corre en su goroutine con un contexto con plazo y escribe en un
// buffer. Si termina a tiempo, el buffer y las cabeceras pasan al cliente; si
// vence, el contexto se cancela (pgx cancela la sentencia e InTenantTx hace
// ROLLBACK y suelta la conexión), lo escrito se descarta —cabeceras incluidas,
// salvo las que ya puso un middleware externo como X-Request-Id— y se envía el
// problema. Las escrituras posteriores del handler devuelven
// http.ErrHandlerTimeout. El cliente recibe la respuesta del handler completa o
// el 503, nunca una mezcla. Un panic del handler se relanza en la goroutine de
// la petición, donde lo recoge recoverPanic.
//
// El buffer vive en memoria: las respuestas largas (descargas, exportes) van por
// rutas Streaming, que no pasan por aquí.
func handlerDeadline(next http.Handler, d time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), d)
		defer cancel()
		hr := r.WithContext(ctx)

		tw := &timeoutWriter{h: make(http.Header)}
		done := make(chan struct{})
		panicked := make(chan *handlerPanic, 1)
		go func() {
			defer func() {
				if v := recover(); v != nil {
					hp := &handlerPanic{value: v, stack: debug.Stack()}
					if tw.timedOutNow() {
						// Nadie la recogerá: la petición ya respondió el 503.
						depsFrom(r.Context()).log.ErrorContext(r.Context(), "panic after the handler deadline",
							"route", routeOf(hr), "panic", fmt.Sprint(v), "stack", string(hp.stack))
					}
					panicked <- hp
				}
			}()
			next.ServeHTTP(tw, hr)
			close(done)
		}()

		select {
		case hp := <-panicked:
			if hp.value == http.ErrAbortHandler {
				panic(http.ErrAbortHandler)
			}
			panic(hp)
		case <-done:
			tw.mu.Lock()
			defer tw.mu.Unlock()
			dst := w.Header()
			for k, vv := range tw.h {
				dst[k] = vv
			}
			code := tw.code
			if code == 0 {
				code = http.StatusOK
			}
			w.WriteHeader(code)
			_, _ = w.Write(tw.buf.Bytes())
		case <-ctx.Done():
			tw.mu.Lock()
			defer tw.mu.Unlock()
			tw.timedOut = true
			if r.Context().Err() != nil {
				return // el cliente se fue: no hay a quién responder
			}
			dp := depsFrom(r.Context())
			route := routeOf(hr)
			dp.log.WarnContext(r.Context(), "handler deadline exceeded",
				"route", route, "deadline_ms", d.Milliseconds(), "elapsed_ms", time.Since(dp.start).Milliseconds())
			dp.metrics.Inc(metrics.HandlerTimeouts, "route", route)
			WriteProblem(w, r, "overloaded", "the server is busy, retry shortly")
		}
	})
}

// timeoutWriter guarda en memoria la respuesta del handler hasta saber si llegó a tiempo.
type timeoutWriter struct {
	mu       sync.Mutex
	h        http.Header
	buf      bytes.Buffer
	code     int
	timedOut bool
}

func (tw *timeoutWriter) Header() http.Header { return tw.h }

func (tw *timeoutWriter) Write(p []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return 0, http.ErrHandlerTimeout
	}
	if tw.code == 0 {
		tw.code = http.StatusOK
	}
	return tw.buf.Write(p)
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut || tw.code != 0 {
		return
	}
	if code < 100 || code > 999 {
		panic(fmt.Sprintf("invalid WriteHeader code %v", code))
	}
	tw.code = code
}

func (tw *timeoutWriter) timedOutNow() bool {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return tw.timedOut
}

// streamBudget da a una ruta de streaming su presupuesto, contado desde la
// llegada de la petición. No guarda nada: el handler escribe directo y puede
// hacer Flush. Si el presupuesto vence antes del primer byte, responde 503
// overloaded; si vence con la respuesta empezada, corta la conexión para que el
// cliente no tome por completo un archivo truncado (ADR 0003).
func streamBudget(next http.Handler, d time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dp := depsFrom(r.Context())
		ctx, cancel := context.WithDeadline(r.Context(), dp.start.Add(d))
		defer cancel()
		sw := track(w)
		hr := r.WithContext(ctx)
		next.ServeHTTP(sw, hr)
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) || r.Context().Err() != nil {
			return
		}
		route := routeOf(hr)
		dp.log.WarnContext(r.Context(), "stream budget exceeded",
			"route", route, "budget_ms", d.Milliseconds(), "elapsed_ms", time.Since(dp.start).Milliseconds(), "started", sw.wrote)
		dp.metrics.Inc(metrics.HandlerTimeouts, "route", route)
		if sw.wrote {
			panic(http.ErrAbortHandler)
		}
		WriteProblem(sw, r, "overloaded", "the server is busy, retry shortly")
	})
}
