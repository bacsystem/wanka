package ventas_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/apigen/ventas"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// Ejemplo de la regla de apps/api/README.md («Servidor generado»): el código generado nunca
// responde con su http.Error de texto plano. Todo ServerInterfaceWrapper y todo
// NewStrictHandlerWithOptions se construyen con manejadores que escriben RFC 9457 con httpx.
// Cada ola copia esta forma para su módulo.

type stub struct{ called bool }

func (s *stub) CreateSale(context.Context, ventas.CreateSaleRequestObject) (ventas.CreateSaleResponseObject, error) {
	s.called = true
	return nil, errors.New("boom")
}

// problemErrors es el manejador de errores de petición del generado (parámetros y cuerpo).
func problemErrors(w http.ResponseWriter, r *http.Request, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		httpx.WriteProblem(w, r, "payload_too_large", "request body is too large")
		return
	}
	httpx.WriteProblem(w, r, "malformed_request", "the request is malformed")
}

func newHandler(s *stub) http.HandlerFunc {
	strict := ventas.NewStrictHandlerWithOptions(s, nil, ventas.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  problemErrors,
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) { httpx.WriteError(w, r, err) },
	})
	w := ventas.ServerInterfaceWrapper{Handler: strict, ErrorHandlerFunc: problemErrors}
	return w.CreateSale
}

func TestGeneratedErrorsAreProblemJSON(t *testing.T) {
	const key = "0b8f4f5e-3a8e-4c0e-9b7e-1d2f3a4b5c6d"
	cases := []struct {
		name       string
		idemKey    string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"idempotency key is not a uuid", "nope", `{}`, 400, "malformed_request"},
		{"idempotency key missing", "", `{}`, 400, "malformed_request"},
		{"body is not JSON", key, `{not json`, 400, "malformed_request"},
		{"body too large", key, `{"shiftId":"` + strings.Repeat("a", 2048) + `"}`, 413, "payload_too_large"},
		{"handler error", key, `{}`, 500, "internal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &stub{}
			req := httptest.NewRequest(http.MethodPost, "/v1/sales", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if tc.idemKey != "" {
				req.Header.Set("Idempotency-Key", tc.idemKey)
			}
			rec := httptest.NewRecorder()
			if tc.wantStatus == 413 {
				req.Body = http.MaxBytesReader(rec, req.Body, 1024)
			}
			newHandler(s).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body %q", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
				t.Errorf("Content-Type = %q, want application/problem+json (not the generated text/plain)", ct)
			}
			var p struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Code != tc.wantCode {
				t.Errorf("body = %q (%v), want code %q", rec.Body.String(), err, tc.wantCode)
			}
			if tc.wantStatus != 500 && s.called {
				t.Error("the handler ran with a request that does not validate")
			}
		})
	}
}
