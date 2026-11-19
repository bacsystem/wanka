package httpx

import "net/http"

// RejectIdempotencyKey es el paso de las rutas donde la clave de idempotencia no
// vale (/v1/auth/*, api/conventions.md §3): una petición con la cabecera
// Idempotency-Key responde 400 malformed_request y no ejecuta nada. El registrador
// de rutas lo agrega con Use donde corresponda; el middleware de idempotencia
// (internal/platform/idempotency) solo se monta en rutas Tenant.
func RejectIdempotencyKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.Header.Values("Idempotency-Key")) > 0 {
			WriteProblem(w, r, "malformed_request", "the Idempotency-Key header is not accepted on this route",
				FieldError{Field: "Idempotency-Key", Code: "not_allowed"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
