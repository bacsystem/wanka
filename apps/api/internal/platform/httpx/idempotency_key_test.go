package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// El 400 de /v1/auth/*: con la cabecera no se ejecuta nada; sin ella la ruta corre.
func TestRejectIdempotencyKey(t *testing.T) {
	ran := 0
	h, _, _ := chain(t, func(rt *httpx.Router) {
		// HandleV1 es lo que auth.Routes llama tras validar la clase (Router.Handle no admite /v1).
		rt.HandleV1("POST /v1/auth/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ran++
			w.WriteHeader(http.StatusNoContent)
		}), httpx.Use(httpx.RejectIdempotencyKey))
	})
	cases := []struct {
		name   string
		values []string
		want   int
		ran    int
	}{
		{"without the header", nil, http.StatusNoContent, 1},
		{"with a key", []string{"0190c1a2-0000-7000-8000-000000000001"}, http.StatusBadRequest, 0},
		{"with an empty value", []string{""}, http.StatusBadRequest, 0},
		{"with two values", []string{"a", "b"}, http.StatusBadRequest, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ran = 0
			req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
			for _, v := range tc.values {
				req.Header.Add("Idempotency-Key", v)
			}
			rec := do(h, req)
			if rec.Code != tc.want || ran != tc.ran {
				t.Fatalf("status %d (want %d), handler runs %d (want %d): %s", rec.Code, tc.want, ran, tc.ran, rec.Body.String())
			}
			if tc.want == http.StatusBadRequest {
				mustProblem(t, rec, http.StatusBadRequest, "malformed_request")
			}
		})
	}
}

// Innermost corre más cerca del handler que todo Use, aunque se declare antes.
func TestInnermostRunsAfterEveryUse(t *testing.T) {
	var order []string
	step := func(name string) httpx.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	h, _, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("POST /x", func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "handler")
			w.WriteHeader(http.StatusNoContent)
		}, httpx.Innermost(step("inner1"), step("inner2")), httpx.Use(step("use1")), httpx.Use(step("use2")))
	})
	do(h, httptest.NewRequest(http.MethodPost, "/x", nil))
	if got := strings.Join(order, ","); got != "use1,use2,inner1,inner2,handler" {
		t.Fatalf("order = %s", got)
	}
}
