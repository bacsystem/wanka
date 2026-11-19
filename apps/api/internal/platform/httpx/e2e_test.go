package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// Los siete code que emite esta historia, de punta a punta por httpx.Chain:
// HTTP, Content-Type, campos obligatorios, requestId igual a X-Request-Id y
// Retry-After donde corresponde.
func TestCodesEmittedByTheKernelEndToEnd(t *testing.T) {
	clk := clock.NewFake(t0)
	lim := newLimiter(t, clk, httpx.LimiterConfig{Name: "s", Capacity: 1, Refill: time.Hour, MaxKeys: 10, Key: headerKey("K")})
	h, _, _ := chainWith(t, httpx.RouterOptions{HandlerTimeout: 50 * time.Millisecond}, func(rt *httpx.Router) {
		rt.HandleFunc("POST /json", func(w http.ResponseWriter, r *http.Request) {
			var v struct{ N int }
			if err := httpx.DecodeJSON(r, &v); err != nil {
				httpx.WriteError(w, r, err)
			}
		})
		rt.HandleFunc("GET /limited", func(http.ResponseWriter, *http.Request) {}, httpx.Use(lim.Middleware()))
		rt.HandleFunc("GET /slow", func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
		rt.HandleFunc("GET /db", func(w http.ResponseWriter, r *http.Request) { httpx.WriteError(w, r, db.ErrUnavailable) })
		rt.HandleFunc("GET /panic", func(http.ResponseWriter, *http.Request) { panic("x") })
	})
	post := func(body string, cl int64) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if cl >= 0 {
			req.ContentLength = cl
		}
		return req
	}
	limitedReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.Header.Set("K", "a")
		return req
	}
	do(h, limitedReq()) // agota la única ficha

	cases := []struct {
		code   string
		status int
		req    *http.Request
		retry  bool
	}{
		{"malformed_request", 400, post("{", -1), false},
		{"not_found", 404, httptest.NewRequest(http.MethodGet, "/missing", nil), false},
		{"payload_too_large", 413, post("x", httpx.MaxBodyBytes+1), false},
		{"rate_limited", 429, limitedReq(), true},
		{"overloaded", 503, httptest.NewRequest(http.MethodGet, "/slow", nil), true},
		{"dependency_unavailable", 503, httptest.NewRequest(http.MethodGet, "/db", nil), true},
		{"internal", 500, httptest.NewRequest(http.MethodGet, "/panic", nil), false},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			rec := do(h, tc.req)
			mustProblem(t, rec, tc.status, tc.code)
			if httpx.Codes()[tc.code] != tc.status {
				t.Fatalf("table says %d for %s", httpx.Codes()[tc.code], tc.code)
			}
			ra := rec.Header().Get("Retry-After")
			if tc.retry {
				if n, err := strconv.Atoi(ra); err != nil || n < 1 {
					t.Fatalf("Retry-After = %q", ra)
				}
			} else if ra != "" {
				t.Fatalf("unexpected Retry-After %q", ra)
			}
		})
	}
}
