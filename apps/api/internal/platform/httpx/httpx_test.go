package httpx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

func newHandler(ready httpx.ReadyFunc, buf *bytes.Buffer) http.Handler {
	log := logging.New(buf, slog.LevelInfo)
	return httpx.Chain(httpx.NewMux(ready), log)
}

func TestHealthzIsOKWithoutDatabase(t *testing.T) {
	var buf bytes.Buffer
	failing := func(context.Context) error { t.Error("healthz must not call readiness"); return nil }
	for _, path := range []string{"/healthz", "/livez"} {
		rec := httptest.NewRecorder()
		newHandler(failing, &buf).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, rec.Code)
		}
	}
}

func TestReadyzStubReturns503(t *testing.T) {
	var buf bytes.Buffer
	rec := httptest.NewRecorder()
	newHandler(httpx.NotReady, &buf).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content type %q", ct)
	}
	var p map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p["status"] != float64(503) {
		t.Fatalf("bad problem body %q (%v)", rec.Body.String(), err)
	}
	// Required by the frozen Problem schema (openapi.yaml, conventions.md §4).
	for _, k := range []string{"type", "title", "status", "code", "requestId"} {
		if _, ok := p[k]; !ok {
			t.Fatalf("problem lacks required field %q: %s", k, rec.Body.String())
		}
	}
	if p["code"] != "dependency_unavailable" {
		t.Fatalf("code %v", p["code"])
	}
	if id := rec.Header().Get("X-Request-Id"); id == "" || p["requestId"] != id {
		t.Fatalf("requestId %v does not match X-Request-Id %q", p["requestId"], id)
	}
	if strings.Contains(rec.Body.String(), httpx.ErrNotReady.Error()) {
		t.Fatalf("detail leaks the internal cause: %s", rec.Body.String())
	}
}

func TestReadyzOKWhenReady(t *testing.T) {
	var buf bytes.Buffer
	rec := httptest.NewRecorder()
	newHandler(func(context.Context) error { return nil }, &buf).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestUnknownRouteIs404(t *testing.T) {
	var buf bytes.Buffer
	rec := httptest.NewRecorder()
	newHandler(httpx.NotReady, &buf).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestRequestIDGeneratedAndLogged(t *testing.T) {
	var buf bytes.Buffer
	rec := httptest.NewRecorder()
	newHandler(httpx.NotReady, &buf).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	id := rec.Header().Get("X-Request-Id")
	if len(id) < 16 {
		t.Fatalf("request id %q", id)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("access log is not JSON: %v: %q", err, buf.String())
	}
	if m["request_id"] != id || m["status"] != float64(200) || m["path"] != "/healthz" {
		t.Fatalf("access log %v", m)
	}
}

func TestIncomingRequestIDKeptOnlyIfSafe(t *testing.T) {
	cases := []struct {
		name, in string
		keep     bool
	}{
		{"safe", "abc-123_DEF", true},
		{"newline", "a\nb", false},
		{"space", "a b", false},
		{"too long", strings.Repeat("a", 129), false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			req.Header["X-Request-Id"] = []string{tc.in}
			rec := httptest.NewRecorder()
			newHandler(httpx.NotReady, &buf).ServeHTTP(rec, req)
			got := rec.Header().Get("X-Request-Id")
			if tc.keep != (got == tc.in) {
				t.Fatalf("in=%q got=%q keep=%v", tc.in, got, tc.keep)
			}
		})
	}
}

func TestAccessLogOmitsQueryAndHeaders(t *testing.T) {
	var buf bytes.Buffer
	req := httptest.NewRequest(http.MethodGet, "/healthz?token=QUERY-SECRET", nil)
	req.Header.Set("Authorization", "Bearer HEADER-SECRET")
	newHandler(httpx.NotReady, &buf).ServeHTTP(httptest.NewRecorder(), req)
	if strings.Contains(buf.String(), "SECRET") {
		t.Fatalf("access log leaks: %s", buf.String())
	}
}

func TestBodyLimit(t *testing.T) {
	var buf bytes.Buffer
	var readErr error
	mux := http.NewServeMux()
	mux.HandleFunc("POST /echo", func(w http.ResponseWriter, r *http.Request) {
		_, readErr = readAll(r)
	})
	h := httpx.Chain(mux, logging.New(&buf, slog.LevelInfo))
	body := strings.NewReader(strings.Repeat("a", httpx.MaxBodyBytes+1))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/echo", body))
	if readErr == nil {
		t.Fatal("body over the limit must fail to read")
	}
}

func TestNewServerTimeouts(t *testing.T) {
	s := httpx.NewServer(":0", http.NewServeMux())
	if s.ReadHeaderTimeout != 5*time.Second || s.IdleTimeout != 120*time.Second || s.ReadTimeout == 0 || s.WriteTimeout == 0 {
		t.Fatalf("timeouts: %+v", s)
	}
}
