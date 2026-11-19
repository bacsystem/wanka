package httpx_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

const testDeadline = 100 * time.Millisecond

func TestDeadlineExceededIsOverloadedProblemAndDiscardsTheBuffer(t *testing.T) {
	var lateWriteErr error
	var ctxErr error
	finished := make(chan struct{})
	responded := make(chan struct{})
	h, buf, m := chainWith(t, httpx.RouterOptions{HandlerTimeout: testDeadline}, func(rt *httpx.Router) {
		rt.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
			defer close(finished)
			w.Header().Set("X-Leak", "from-handler")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"half":`)
			<-r.Context().Done()
			ctxErr = r.Context().Err()
			<-responded // escribe después de que el cliente ya recibió el 503
			_, lateWriteErr = io.WriteString(w, `"json"}`)
		})
	})
	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	req.Header.Set("X-Request-Id", "req-deadline-1")
	rec := do(h, req)
	close(responded)

	p := mustProblem(t, rec, http.StatusServiceUnavailable, "overloaded")
	if p.RequestID != "req-deadline-1" || rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("requestId %q Retry-After %q", p.RequestID, rec.Header().Get("Retry-After"))
	}
	if rec.Header().Get("X-Leak") != "" || strings.Contains(rec.Body.String(), "half") {
		t.Fatalf("handler output reached the client: %v %q", rec.Header(), rec.Body.String())
	}
	if strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "<") {
		t.Fatalf("body is HTML like http.TimeoutHandler: %s", rec.Body.String())
	}
	<-finished
	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Fatalf("handler ctx error = %v, want DeadlineExceeded", ctxErr)
	}
	if !errors.Is(lateWriteErr, http.ErrHandlerTimeout) {
		t.Fatalf("late write error = %v, want http.ErrHandlerTimeout", lateWriteErr)
	}
	warns := buf.lines(t, "handler deadline exceeded")
	if len(warns) != 1 || warns[0]["level"] != "WARN" || warns[0]["route"] != "GET /slow" ||
		warns[0]["deadline_ms"] != float64(100) || warns[0]["request_id"] != "req-deadline-1" {
		t.Fatalf("warn = %v", warns)
	}
	if el, _ := warns[0]["elapsed_ms"].(float64); el < 100 {
		t.Fatalf("elapsed_ms = %v, want at least the deadline", warns[0]["elapsed_ms"])
	}
	if m.get("http_handler_timeouts_total{route,GET /slow}") != 1 {
		t.Fatalf("counter = %v", m.counts)
	}
	if al := buf.lines(t, "request"); len(al) != 1 || al[0]["status"] != float64(503) {
		t.Fatalf("access log = %v", al)
	}
}

func TestDeadlinePassesACompleteHandlerResponse(t *testing.T) {
	h, _, _ := chainWith(t, httpx.RouterOptions{HandlerTimeout: testDeadline}, func(rt *httpx.Router) {
		rt.HandleFunc("GET /created", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Add("Set-Cookie", "a=1")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"ok":true}`)
		})
		rt.HandleFunc("GET /implicit", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "x") })
		rt.HandleFunc("GET /empty", func(http.ResponseWriter, *http.Request) {})
	})
	rec := do(h, httptest.NewRequest(http.MethodGet, "/created", nil))
	if rec.Code != 201 || rec.Body.String() != `{"ok":true}` || rec.Header().Get("Content-Type") != "application/json" || rec.Header().Get("Set-Cookie") != "a=1" {
		t.Fatalf("created: %d %v %q", rec.Code, rec.Header(), rec.Body.String())
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatal("X-Request-Id lost")
	}
	if rec := do(h, httptest.NewRequest(http.MethodGet, "/implicit", nil)); rec.Code != 200 || rec.Body.String() != "x" {
		t.Fatalf("implicit: %d %q", rec.Code, rec.Body.String())
	}
	if rec := do(h, httptest.NewRequest(http.MethodGet, "/empty", nil)); rec.Code != 200 || rec.Body.Len() != 0 {
		t.Fatalf("empty: %d %q", rec.Code, rec.Body.String())
	}
}

// Un handler que termina justo al vencer el plazo: el cliente recibe una de las
// dos respuestas completa, nunca una mezcla. Con -race también prueba que el
// buffer no se toca desde dos goroutines.
func TestDeadlineNeverMixesResponses(t *testing.T) {
	const deadline = 2 * time.Millisecond
	h, _, _ := chainWith(t, httpx.RouterOptions{HandlerTimeout: deadline}, func(rt *httpx.Router) {
		rt.HandleFunc("GET /edge", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Handler", "yes")
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, "AAAA")
			time.Sleep(deadline - 200*time.Microsecond)
			_, _ = io.WriteString(w, "BBBB")
		})
	})
	var wg sync.WaitGroup
	var mu sync.Mutex
	var full, timedOut int
	for i := 0; i < 400; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := do(h, httptest.NewRequest(http.MethodGet, "/edge", nil))
			body := rec.Body.String()
			mu.Lock()
			defer mu.Unlock()
			switch {
			case rec.Code == http.StatusAccepted && body == "AAAABBBB" && rec.Header().Get("X-Handler") == "yes":
				full++
			case rec.Code == http.StatusServiceUnavailable && strings.Contains(body, `"code":"overloaded"`) &&
				!strings.Contains(body, "AAAA") && rec.Header().Get("X-Handler") == "":
				timedOut++
			default:
				t.Errorf("mixed response: %d %v %q", rec.Code, rec.Header(), body)
			}
		}()
	}
	wg.Wait()
	t.Logf("full=%d timedOut=%d", full, timedOut)
}

func TestDeadlineHandlerPanicIs500(t *testing.T) {
	h, buf, _ := chainWith(t, httpx.RouterOptions{HandlerTimeout: time.Second}, func(rt *httpx.Router) {
		rt.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) { panic("kaboom") })
	})
	rec := do(h, httptest.NewRequest(http.MethodGet, "/boom", nil))
	mustProblem(t, rec, http.StatusInternalServerError, "internal")
	logs := buf.lines(t, "panic recovered")
	// La traza es la de la goroutine del handler, no la de relanzar el panic.
	if len(logs) != 1 || !strings.Contains(fmt.Sprint(logs[0]["stack"]), "deadline_test.go") {
		t.Fatalf("panic log = %v", logs)
	}
}

func TestDeadlineErrAbortHandlerCutsTheConnection(t *testing.T) {
	h, buf, _ := chainWith(t, httpx.RouterOptions{HandlerTimeout: time.Second}, func(rt *httpx.Router) {
		rt.HandleFunc("GET /abort", func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })
	})
	srv := httptest.NewServer(h)
	defer srv.Close()
	if resp, err := http.Get(srv.URL + "/abort"); err == nil {
		_ = resp.Body.Close()
		t.Fatal("the connection must be cut")
	}
	if strings.Contains(buf.String(), "stack") {
		t.Fatalf("no trace expected: %s", buf.String())
	}
}

func TestDeadlineClientGoneWritesNothing(t *testing.T) {
	h, buf, _ := chainWith(t, httpx.RouterOptions{HandlerTimeout: time.Second}, func(rt *httpx.Router) {
		rt.HandleFunc("GET /wait", func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	})
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/wait", nil).WithContext(ctx)
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	rec := do(h, req)
	if rec.Body.Len() != 0 {
		t.Fatalf("wrote to a client that left: %q", rec.Body.String())
	}
	if len(buf.lines(t, "handler deadline exceeded")) != 0 {
		t.Fatal("a client that left is not a deadline")
	}
	if al := buf.lines(t, "request"); len(al) != 1 || al[0]["status"] != float64(499) {
		t.Fatalf("access log = %v, want status 499", al)
	}
}

func TestFlushUnderTheDeadlineSendsNothingBeforeTheEnd(t *testing.T) {
	release := make(chan struct{})
	flushErrs := make(chan error, 2)
	h, _, _ := chainWith(t, httpx.RouterOptions{HandlerTimeout: 5 * time.Second}, func(rt *httpx.Router) {
		rt.HandleFunc("GET /buffered", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "first")
			flushErrs <- http.NewResponseController(w).Flush()
			<-release
			_, _ = io.WriteString(w, "-second")
		})
	})
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := &http.Client{Timeout: 300 * time.Millisecond}
	if resp, err := client.Get(srv.URL + "/buffered"); err == nil {
		_ = resp.Body.Close()
		t.Fatal("the client got bytes before the handler finished")
	}
	close(release)
	time.Sleep(50 * time.Millisecond)
	resp, err := http.Get(srv.URL + "/buffered")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "first-second" {
		t.Fatalf("body = %q", body)
	}
	if flushErr := <-flushErrs; !errors.Is(flushErr, http.ErrNotSupported) {
		t.Fatalf("Flush error = %v, want ErrNotSupported", flushErr)
	}
}

func TestStreamingRouteSkipsTheHandlerDeadlineAndCanFlush(t *testing.T) {
	proceed := make(chan struct{})
	h, buf, _ := chainWith(t, httpx.RouterOptions{HandlerTimeout: 50 * time.Millisecond, StreamTimeout: 5 * time.Second}, func(rt *httpx.Router) {
		rt.HandleFunc("GET /export", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "row1\n")
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Errorf("Flush: %v", err)
			}
			<-proceed
			time.Sleep(120 * time.Millisecond) // más que el plazo de 10 s del handler (aquí 50 ms)
			_, _ = io.WriteString(w, "row2\n")
		}, httpx.Streaming())
	})
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/export")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("X-Request-Id") == "" {
		t.Fatal("streaming route must keep the request id")
	}
	first := make([]byte, 5)
	if _, err := io.ReadFull(resp.Body, first); err != nil || string(first) != "row1\n" {
		t.Fatalf("first chunk = %q (%v): Flush must send it before the handler ends", first, err)
	}
	close(proceed)
	rest, err := io.ReadAll(resp.Body)
	if err != nil || string(rest) != "row2\n" {
		t.Fatalf("rest = %q (%v)", rest, err)
	}
	if al := buf.lines(t, "request"); len(al) != 1 || al[0]["status"] != float64(200) {
		t.Fatalf("access log = %v", al)
	}
}

func TestStreamingContextExpiresAtTheBudgetFromArrival(t *testing.T) {
	const budget = 3 * time.Second
	var deadline time.Time
	h, _, _ := chainWith(t, httpx.RouterOptions{StreamTimeout: budget}, func(rt *httpx.Router) {
		rt.HandleFunc("GET /export", func(_ http.ResponseWriter, r *http.Request) {
			deadline, _ = r.Context().Deadline()
		}, httpx.Streaming())
	})
	before := time.Now()
	do(h, httptest.NewRequest(http.MethodGet, "/export", nil))
	after := time.Now()
	if deadline.Before(before.Add(budget)) || deadline.After(after.Add(budget)) {
		t.Fatalf("deadline %v not within [%v, %v] + %v", deadline, before, after, budget)
	}
}

func TestStreamingBudgetBeforeFirstByteIsAProblem(t *testing.T) {
	h, buf, _ := chainWith(t, httpx.RouterOptions{StreamTimeout: 100 * time.Millisecond}, func(rt *httpx.Router) {
		rt.HandleFunc("GET /export", func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }, httpx.Streaming())
	})
	rec := do(h, httptest.NewRequest(http.MethodGet, "/export", nil))
	mustProblem(t, rec, http.StatusServiceUnavailable, "overloaded")
	if rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	if len(buf.lines(t, "stream budget exceeded")) != 1 {
		t.Fatalf("expected one warn: %s", buf.String())
	}
}

func TestStreamingBudgetMidStreamCutsTheConnection(t *testing.T) {
	h, _, _ := chainWith(t, httpx.RouterOptions{StreamTimeout: 150 * time.Millisecond}, func(rt *httpx.Router) {
		rt.HandleFunc("GET /export", func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "row1\n")
			_ = http.NewResponseController(w).Flush()
			<-r.Context().Done()
		}, httpx.Streaming())
	})
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/export")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("read error = %v (body %q), want unexpected EOF: the final chunk must not be sent", err, body)
	}
}

func TestStreamingStillRecoversPanicsAndLimitsTheBody(t *testing.T) {
	h, _, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) { panic("x") }, httpx.Streaming())
		rt.HandleFunc("POST /upload", func(http.ResponseWriter, *http.Request) { t.Error("must not run") }, httpx.Streaming())
	})
	mustProblem(t, do(h, httptest.NewRequest(http.MethodGet, "/boom", nil)), http.StatusInternalServerError, "internal")
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("x"))
	req.ContentLength = httpx.MaxBodyBytes + 1
	mustProblem(t, do(h, req), http.StatusRequestEntityTooLarge, "payload_too_large")
}

func TestSlowHeadersAreCut(t *testing.T) {
	srv := httpx.NewServer("127.0.0.1:0", http.NewServeMux())
	srv.ReadHeaderTimeout = 150 * time.Millisecond // el valor real (5 s) se comprueba en TestNewServerTimeouts
	hs := httptest.NewUnstartedServer(srv.Handler)
	hs.Config = srv
	hs.Start()
	defer hs.Close()
	conn, err := (&netDialer{}).dial(hs.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n")) // sin la línea en blanco final
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	// El servidor responde 408 o cierra: lo que no puede es esperar sin fin.
	if err == nil && !strings.Contains(string(buf[:n]), "408") && n > 0 {
		t.Logf("server answered %q", buf[:n])
	}
	if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
		t.Fatal("the server kept the connection open past ReadHeaderTimeout")
	}
}
