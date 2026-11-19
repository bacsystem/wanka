package httpx_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
)

// logBuf es un bytes.Buffer seguro para las goroutines del servidor y del test.
type logBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *logBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// lines devuelve las líneas de log JSON cuyo msg es msg.
func (l *logBuf) lines(t *testing.T, msg string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, ln := range strings.Split(strings.TrimSpace(l.String()), "\n") {
		if ln == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("log line is not JSON: %q", ln)
		}
		if m["msg"] == msg {
			out = append(out, m)
		}
	}
	return out
}

// fakeMetrics cuenta los eventos que recibe.
type fakeMetrics struct {
	mu     sync.Mutex
	counts map[string]int
}

func (f *fakeMetrics) Inc(name string, labels ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts == nil {
		f.counts = map[string]int{}
	}
	f.counts[name+"{"+strings.Join(labels, ",")+"}"]++
}

func (f *fakeMetrics) get(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[key]
}

var _ metrics.Recorder = (*fakeMetrics)(nil)

// chain arma Chain sobre un router con las rutas que registra setup.
func chain(t *testing.T, setup func(rt *httpx.Router)) (http.Handler, *logBuf, *fakeMetrics) {
	t.Helper()
	return chainWith(t, httpx.RouterOptions{}, setup)
}

func chainWith(t *testing.T, o httpx.RouterOptions, setup func(rt *httpx.Router)) (http.Handler, *logBuf, *fakeMetrics) {
	t.Helper()
	buf := &logBuf{}
	m := &fakeMetrics{}
	rt := httpx.NewRouter(o)
	setup(rt)
	return httpx.Chain(rt, logging.New(buf, slog.LevelDebug), m), buf, m
}

func do(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Code      string `json:"code"`
	Detail    string `json:"detail"`
	RequestID string `json:"requestId"`
	Errors    []struct {
		Field string `json:"field"`
		Code  string `json:"code"`
	} `json:"errors"`
}

// mustProblem comprueba el contrato común de todo error: application/problem+json,
// campos obligatorios y requestId igual a X-Request-Id.
func mustProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) problem {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, status, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("body is not JSON: %q", rec.Body.String())
	}
	for _, k := range []string{"type", "title", "status", "code", "requestId"} {
		if _, ok := raw[k]; !ok {
			t.Fatalf("problem lacks %q: %s", k, rec.Body.String())
		}
	}
	var p problem
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if p.Code != code || p.Status != status {
		t.Fatalf("problem = %+v, want code %q status %d", p, code, status)
	}
	if id := rec.Header().Get("X-Request-Id"); id == "" || p.RequestID != id {
		t.Fatalf("requestId %q != X-Request-Id %q", p.RequestID, id)
	}
	return p
}

func TestChainRequestIDEverywhere(t *testing.T) {
	cases := []struct {
		name, in string
		keep     bool
	}{
		{"missing", "", false},
		{"valid", "abc-123_DEF", true},
		{"newline", "a\nb", false},
		{"space", "a b", false},
		{"slash", "a/b", false},
		{"129 chars", strings.Repeat("a", 129), false},
		{"128 chars", strings.Repeat("a", 128), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, buf, _ := chain(t, func(rt *httpx.Router) {
				rt.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) { panic("boom") })
				rt.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {
					httpx.WriteError(w, r, httpx.NewError("conflict", ""))
				})
			})
			for _, path := range []string{"/ok", "/boom", "/missing"} {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				if tc.in != "" {
					req.Header["X-Request-Id"] = []string{tc.in}
				}
				rec := do(h, req)
				got := rec.Header().Get("X-Request-Id")
				if tc.keep && got != tc.in || !tc.keep && (got == tc.in || len(got) != 32) {
					t.Fatalf("%s: in=%q got=%q keep=%v", path, tc.in, got, tc.keep)
				}
				var p problem
				_ = json.Unmarshal(rec.Body.Bytes(), &p)
				if p.RequestID != got {
					t.Fatalf("%s: requestId %q != header %q", path, p.RequestID, got)
				}
				if rec.Code == http.StatusBadRequest {
					t.Fatalf("%s: request id must never cause 400", path)
				}
				for _, ln := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
					if ln != "" && !strings.Contains(ln, `"request_id":"`+got+`"`) {
						t.Fatalf("%s: log line without request_id %q: %s", path, got, ln)
					}
				}
				buf.mu.Lock()
				buf.b.Reset()
				buf.mu.Unlock()
			}
		})
	}
}

func TestUndeclaredRouteOrMethodIs404Problem(t *testing.T) {
	h, _, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /things", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
		rt.HandleFunc("GET /things/{id}", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	})
	cases := []struct{ method, path string }{
		{http.MethodGet, "/nope"},
		{http.MethodPost, "/things"},
		{http.MethodDelete, "/things/1"},
		{http.MethodOptions, "/things"},
		{http.MethodGet, "//things"},
		{http.MethodGet, "/things/../things"},
		{http.MethodGet, "/things/"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := do(h, httptest.NewRequest(tc.method, tc.path, nil))
			mustProblem(t, rec, http.StatusNotFound, "not_found")
			if rec.Header().Get("Allow") != "" {
				t.Fatalf("Allow header leaked from ServeMux: %q", rec.Header().Get("Allow"))
			}
		})
	}
	if rec := do(h, httptest.NewRequest(http.MethodGet, "/things/7", nil)); rec.Code != http.StatusOK {
		t.Fatalf("declared route: %d", rec.Code)
	}
}

func TestNoCORSHeaders(t *testing.T) {
	h, _, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /things", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	})
	for _, method := range []string{http.MethodGet, http.MethodOptions} {
		req := httptest.NewRequest(method, "/things", nil)
		req.Header.Set("Origin", "https://evil.example")
		req.Header.Set("Access-Control-Request-Method", "POST")
		rec := do(h, req)
		for k := range rec.Header() {
			if strings.HasPrefix(strings.ToLower(k), "access-control-") {
				t.Fatalf("%s: unexpected CORS header %s", method, k)
			}
		}
		if method == http.MethodOptions {
			mustProblem(t, rec, http.StatusNotFound, "not_found")
		}
	}
}

func TestRouterRejectsBadPatterns(t *testing.T) {
	for _, pattern := range []string{"/no-method", "GET /subtree/", "GET", "GET /v1/files/{p...}", "GET /{rest...}"} {
		t.Run(pattern, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("pattern %q must panic at registration", pattern)
				}
			}()
			httpx.NewRouter(httpx.RouterOptions{}).HandleFunc(pattern, func(http.ResponseWriter, *http.Request) {})
		})
	}
}

func TestPanicIs500WithoutLeaks(t *testing.T) {
	h, buf, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) { panic("SECRET-INTERNAL select * from users") })
		rt.HandleFunc("GET /late", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "partial")
			panic("late")
		})
	})
	rec := do(h, httptest.NewRequest(http.MethodGet, "/boom", nil))
	p := mustProblem(t, rec, http.StatusInternalServerError, "internal")
	if p.Detail != "" {
		t.Fatalf("500 must not carry detail: %q", p.Detail)
	}
	if strings.Contains(rec.Body.String(), "SECRET") || strings.Contains(rec.Body.String(), "goroutine") {
		t.Fatalf("500 leaks internals: %s", rec.Body.String())
	}
	errs := buf.lines(t, "panic recovered")
	if len(errs) != 1 || errs[0]["level"] != "ERROR" || !strings.Contains(fmt.Sprint(errs[0]["stack"]), "goroutine") ||
		!strings.Contains(fmt.Sprint(errs[0]["panic"]), "SECRET-INTERNAL") {
		t.Fatalf("panic log = %v", errs)
	}
	if al := buf.lines(t, "request"); len(al) != 1 || al[0]["status"] != float64(500) {
		t.Fatalf("access log of a panic = %v", al)
	}
}

func TestPanicAfterPartialWriteIs500OnBufferedRoutes(t *testing.T) {
	h, _, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /late", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Set-Cookie", "leak=1")
			_, _ = io.WriteString(w, "partial")
			panic("late")
		})
	})
	rec := do(h, httptest.NewRequest(http.MethodGet, "/late", nil))
	mustProblem(t, rec, http.StatusInternalServerError, "internal")
	if strings.Contains(rec.Body.String(), "partial") || rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("buffered output of the panicking handler reached the client: %v %s", rec.Header(), rec.Body.String())
	}
}

func TestPanicAfterStreamingStartedAbortsTheConnection(t *testing.T) {
	h, buf, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /late", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "partial")
			panic("late")
		}, httpx.Streaming())
	})
	defer func() {
		if v := recover(); v != http.ErrAbortHandler {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", v)
		}
		if len(buf.lines(t, "panic recovered")) != 1 {
			t.Fatalf("the panic must still be logged once: %s", buf.String())
		}
	}()
	do(h, httptest.NewRequest(http.MethodGet, "/late", nil))
	t.Fatal("expected the connection to be aborted")
}

func TestErrAbortHandlerCutsTheConnectionQuietly(t *testing.T) {
	h, buf, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /abort", func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })
		rt.HandleFunc("GET /ok", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	})
	srv := httptest.NewServer(h)
	defer srv.Close()
	srv.Config.ErrorLog = nil
	resp, err := http.Get(srv.URL + "/abort")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("the connection must be cut without a response")
	}
	if strings.Contains(buf.String(), `"level":"ERROR"`) || strings.Contains(buf.String(), "stack") {
		t.Fatalf("ErrAbortHandler must not log a trace: %s", buf.String())
	}
	// El proceso sigue sirviendo.
	resp, err = http.Get(srv.URL + "/ok")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestContentLengthOverLimitIs413WithoutReadingTheBody(t *testing.T) {
	called := false
	h, _, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("POST /echo", func(http.ResponseWriter, *http.Request) { called = true })
	})
	body := &failingReader{t: t}
	req := httptest.NewRequest(http.MethodPost, "/echo", body)
	req.ContentLength = httpx.MaxBodyBytes + 1
	mustProblem(t, do(h, req), http.StatusRequestEntityTooLarge, "payload_too_large")
	if called {
		t.Fatal("handler must not run")
	}

	// Justo en el límite pasa.
	req = httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("x"))
	req.ContentLength = httpx.MaxBodyBytes
	if rec := do(h, req); rec.Code != http.StatusOK || !called {
		t.Fatalf("body of exactly MaxBodyBytes must pass: %d", rec.Code)
	}
}

type failingReader struct{ t *testing.T }

func (f *failingReader) Read([]byte) (int, error) {
	f.t.Error("the body must not be read")
	return 0, io.EOF
}

func TestChunkedBodyOverLimitStopsAtLimitPlusOne(t *testing.T) {
	var read int
	var decodeErr error
	h, _, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("POST /echo", func(w http.ResponseWriter, r *http.Request) {
			var v map[string]any
			decodeErr = httpx.DecodeJSON(r, &v)
			httpx.WriteError(w, r, decodeErr)
		})
	})
	// {"a":"xxxx... con 3 MB y sin Content-Length.
	src := io.MultiReader(strings.NewReader(`{"a":"`), io.LimitReader(repeatReader('x'), 3<<20))
	counted := &countingReader{r: src, n: &read}
	req := httptest.NewRequest(http.MethodPost, "/echo", counted)
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json")
	mustProblem(t, do(h, req), http.StatusRequestEntityTooLarge, "payload_too_large")
	if read > httpx.MaxBodyBytes+1 {
		t.Fatalf("read %d bytes from the client, want at most %d", read, httpx.MaxBodyBytes+1)
	}
}

type repeatReader byte

func (r repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

type countingReader struct {
	r io.Reader
	n *int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	*c.n += n
	return n, err
}

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Qty  int    `json:"qty"`
		Name string `json:"name"`
	}
	cases := []struct {
		name, ctype, body string
		wantCode          string // "" = decoded fine
		wantField         string
	}{
		{"ok", "application/json", `{"qty":2,"name":"a"}`, "", ""},
		{"ok with charset", "application/json; charset=utf-8", `{"qty":2}`, "", ""},
		{"invalid syntax", "application/json", `{"qty":`, "malformed_request", ""},
		{"empty", "application/json", ``, "malformed_request", ""},
		{"wrong type", "application/json", `{"qty":"two"}`, "malformed_request", "qty"},
		{"trailing data", "application/json", `{"qty":2} {"qty":3}`, "malformed_request", ""},
		{"trailing garbage", "application/json", `{"qty":2}x`, "malformed_request", ""},
		{"text/plain", "text/plain", `{"qty":2}`, "malformed_request", ""},
		{"no content type", "", `{"qty":2}`, "malformed_request", ""},
		{"form", "application/x-www-form-urlencoded", `qty=2`, "malformed_request", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _, _ := chain(t, func(rt *httpx.Router) {
				rt.HandleFunc("POST /x", func(w http.ResponseWriter, r *http.Request) {
					var p payload
					if err := httpx.DecodeJSON(r, &p); err != nil {
						httpx.WriteError(w, r, err)
						return
					}
					w.WriteHeader(http.StatusNoContent)
				})
			})
			req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(tc.body))
			if tc.ctype != "" {
				req.Header.Set("Content-Type", tc.ctype)
			}
			rec := do(h, req)
			if tc.wantCode == "" {
				if rec.Code != http.StatusNoContent {
					t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
				}
				return
			}
			p := mustProblem(t, rec, http.StatusBadRequest, tc.wantCode)
			if tc.wantField != "" && (len(p.Errors) != 1 || p.Errors[0].Field != tc.wantField) {
				t.Fatalf("errors = %+v, want field %q", p.Errors, tc.wantField)
			}
			if tc.wantField == "" && len(p.Errors) != 0 {
				t.Fatalf("unexpected errors %+v", p.Errors)
			}
		})
	}
}

func TestAccessLogOmitsQueryAndHeadersThroughRouter(t *testing.T) {
	h, buf, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /x", func(http.ResponseWriter, *http.Request) {})
	})
	req := httptest.NewRequest(http.MethodGet, "/x?token=QUERY-SECRET", nil)
	req.Header.Set("Authorization", "Bearer HEADER-SECRET")
	do(h, req)
	if strings.Contains(buf.String(), "SECRET") {
		t.Fatalf("access log leaks: %s", buf.String())
	}
}

// Ninguna petición puede recibir una redirección ni un cuerpo que no sea RFC 9457
// o del handler: ServeMux redirige (301 HTML) cuando un patrón termina en "/" o "...}".
func TestNoRequestGetsAServeMuxResponse(t *testing.T) {
	h, _, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /v1/files", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "list") })
		rt.HandleFunc("GET /v1/files/{id}", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "one") })
		rt.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "root") })
	})
	paths := []string{"/", "/v1", "/v1/", "/v1/files", "/v1/files/", "/v1/files/a/b", "/v1/files/a/", "//v1/files", "/v1//files",
		"/v1/./files", "/v1/files/..", "/v1/files/%2e%2e", "/*", "/v1/files?x=1", "/V1/FILES"}
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodOptions, http.MethodDelete} {
		for _, p := range paths {
			req := httptest.NewRequest(method, "http://example.com/", nil)
			req.URL.Path, req.URL.RawQuery = strings.SplitN(p, "?", 2)[0], ""
			req.RequestURI = p
			rec := do(h, req)
			ct := rec.Header().Get("Content-Type")
			if rec.Code >= 300 && rec.Code < 400 || rec.Header().Get("Location") != "" || rec.Header().Get("Allow") != "" {
				t.Fatalf("%s %s: redirect or ServeMux response %d %v", method, p, rec.Code, rec.Header())
			}
			if rec.Code >= 400 && ct != "application/problem+json" {
				t.Fatalf("%s %s: error %d with Content-Type %q: %s", method, p, rec.Code, ct, rec.Body.String())
			}
		}
	}
}

// Un OPTIONS * lo atiende net/http antes de Chain salvo que se desactive.
func TestOptionsStarGoesThroughTheChain(t *testing.T) {
	buf := &logBuf{}
	h := httpx.Chain(httpx.NewMux(func(context.Context) error { return nil }), logging.New(buf, slog.LevelInfo), nil)
	srv := httptest.NewUnstartedServer(h)
	srv.Config = httpx.NewServer("", h)
	srv.Start()
	defer srv.Close()
	conn, err := net.DialTimeout("tcp", srv.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = io.WriteString(conn, "OPTIONS * HTTP/1.1\r\nHost: x\r\nOrigin: https://evil.example\r\nConnection: close\r\n\r\n")
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNotFound || resp.Header.Get("Content-Type") != "application/problem+json" ||
		resp.Header.Get("X-Request-Id") == "" || !strings.Contains(string(body), `"code":"not_found"`) {
		t.Fatalf("OPTIONS *: %d %v %s", resp.StatusCode, resp.Header, body)
	}
	if len(buf.lines(t, "request")) != 1 {
		t.Fatalf("the access log must record it: %s", buf.String())
	}
}

// Lo que el handler dejó en las cabeceras antes de entrar en panic en una ruta
// Streaming (donde no hay buffer) no llega al 500.
func TestPanicOnAStreamingRouteDropsTheHandlerHeaders(t *testing.T) {
	h, _, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /boom", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Set-Cookie", "session=leak")
			w.Header().Set("Content-Disposition", "attachment; filename=x.csv")
			w.Header().Set("Content-Type", "text/csv")
			panic("boom")
		}, httpx.Streaming())
	})
	rec := do(h, httptest.NewRequest(http.MethodGet, "/boom", nil))
	mustProblem(t, rec, http.StatusInternalServerError, "internal")
	if rec.Header().Get("Set-Cookie") != "" || rec.Header().Get("Content-Disposition") != "" {
		t.Fatalf("handler headers reached the 500: %v", rec.Header())
	}
}

// El log de acceso de una conexión cortada no dice 200: dice que se abortó.
func TestAccessLogMarksAbortedRequests(t *testing.T) {
	h, buf, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /abort", func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })
		rt.HandleFunc("GET /mid", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "partial")
			panic(http.ErrAbortHandler)
		}, httpx.Streaming())
		rt.HandleFunc("GET /ok", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	})
	for _, path := range []string{"/abort", "/mid", "/ok"} {
		func() {
			defer func() { _ = recover() }()
			do(h, httptest.NewRequest(http.MethodGet, path, nil))
		}()
	}
	lines := buf.lines(t, "request")
	if len(lines) != 3 {
		t.Fatalf("access log lines = %d: %s", len(lines), buf.String())
	}
	byPath := map[string]map[string]any{}
	for _, l := range lines {
		byPath[fmt.Sprint(l["path"])] = l
	}
	if l := byPath["/abort"]; l["aborted"] != true || l["status"] != float64(0) {
		t.Fatalf("nothing was sent, so status 0 and aborted: %v", l)
	}
	if l := byPath["/mid"]; l["aborted"] != true || l["status"] != float64(200) {
		t.Fatalf("200 was sent and the rest cut: %v", l)
	}
	if l := byPath["/ok"]; l["aborted"] != nil || l["status"] != float64(200) {
		t.Fatalf("a normal request has no aborted field: %v", l)
	}
}
