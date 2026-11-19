package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

// statusByCode es la única tabla code -> HTTP del kernel: los 31 code de la
// lista cerrada de api/conventions.md §4. Un handler elige el code, nunca el
// HTTP. TestCodeTableMatchesConventions la compara con el archivo.
var statusByCode = map[string]int{
	"malformed_request":           http.StatusBadRequest,
	"unauthenticated":             http.StatusUnauthorized,
	"session_expired":             http.StatusUnauthorized,
	"invalid_credentials":         http.StatusUnauthorized,
	"subscription_inactive":       http.StatusPaymentRequired,
	"forbidden":                   http.StatusForbidden,
	"module_not_enabled":          http.StatusForbidden,
	"plan_limit_reached":          http.StatusForbidden,
	"tenant_read_only":            http.StatusForbidden,
	"mfa_required":                http.StatusForbidden,
	"tenant_access_denied":        http.StatusForbidden,
	"site_out_of_scope":           http.StatusForbidden,
	"not_found":                   http.StatusNotFound,
	"conflict":                    http.StatusConflict,
	"duplicate":                   http.StatusConflict,
	"insufficient_stock":          http.StatusConflict,
	"shift_not_open":              http.StatusConflict,
	"invalid_state_transition":    http.StatusConflict,
	"request_in_progress":         http.StatusConflict,
	"version_mismatch":            http.StatusPreconditionFailed,
	"payload_too_large":           http.StatusRequestEntityTooLarge,
	"validation_failed":           http.StatusUnprocessableEntity,
	"idempotency_key_reused":      http.StatusUnprocessableEntity,
	"totals_mismatch":             http.StatusUnprocessableEntity,
	"void_window_expired":         http.StatusUnprocessableEntity,
	"exchange_rate_required":      http.StatusUnprocessableEntity,
	"discount_exceeds_role_limit": http.StatusUnprocessableEntity,
	"rate_limited":                http.StatusTooManyRequests,
	"overloaded":                  http.StatusServiceUnavailable,
	"dependency_unavailable":      http.StatusServiceUnavailable,
	"internal":                    http.StatusInternalServerError,
}

// Codes devuelve una copia de la tabla code -> HTTP.
func Codes() map[string]int {
	out := make(map[string]int, len(statusByCode))
	for k, v := range statusByCode {
		out[k] = v
	}
	return out
}

// retryAfterCodes son los code que llevan Retry-After (conventions.md §3).
// Quien necesita otro valor lo fija antes de escribir el problema.
var retryAfterCodes = map[string]bool{
	"rate_limited": true, "overloaded": true, "dependency_unavailable": true, "request_in_progress": true,
}

// wwwAuthenticateCodes son los 401 del kernel, que llevan WWW-Authenticate: Bearer.
var wwwAuthenticateCodes = map[string]bool{"unauthenticated": true, "session_expired": true}

// FieldError es un elemento de errors[] del problema (openapi.yaml Problem).
type FieldError struct {
	Field string         `json:"field"`
	Code  string         `json:"code"`
	Meta  map[string]any `json:"meta,omitempty"`
}

// Error es un error de dominio tipado con un code del catálogo. Los handlers y
// los casos de uso lo devuelven; WriteError lo traduce con el HTTP de la tabla.
type Error struct {
	Code   string
	Detail string
	Fields []FieldError
}

// NewError construye un Error. detail es el texto para la persona; no debe
// contener datos internos ni de otro tenant.
func NewError(code, detail string, fields ...FieldError) *Error {
	return &Error{Code: code, Detail: detail, Fields: fields}
}

func (e *Error) Error() string { return e.Code + ": " + e.Detail }

type problemBody struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Code      string       `json:"code"`
	Detail    string       `json:"detail,omitempty"`
	RequestID string       `json:"requestId"`
	Errors    []FieldError `json:"errors,omitempty"`
	// Miembros de extensión (RFC 9457 §3.2) de los code del kernel de entitlements.
	Module string `json:"module,omitempty"`
	Limit  *int64 `json:"limit,omitempty"`
	Used   *int64 `json:"used,omitempty"`
}

// Extensions son los miembros extra del problema que llevan module_not_enabled
// (Module) y plan_limit_reached (Limit y Used, conventions.md §4). Limit y Used
// son punteros para que un 0 se envíe.
type Extensions struct {
	Module string
	Limit  *int64
	Used   *int64
}

// WriteProblem escribe un problema RFC 9457 con los campos que exige el
// esquema Problem de openapi.yaml. El HTTP sale de la tabla de code; un code
// fuera del catálogo es un error de programación: se registra y se responde
// 500 internal sin detalle. Los code de 429 y 503 llevan Retry-After: 1 si
// quien llama no fijó otro.
func WriteProblem(w http.ResponseWriter, r *http.Request, code, detail string, fields ...FieldError) {
	writeProblem(w, r, code, detail, Extensions{}, fields)
}

// WriteProblemExt es WriteProblem con los miembros de extensión de ext.
func WriteProblemExt(w http.ResponseWriter, r *http.Request, code, detail string, ext Extensions) {
	writeProblem(w, r, code, detail, ext, nil)
}

func writeProblem(w http.ResponseWriter, r *http.Request, code, detail string, ext Extensions, fields []FieldError) {
	status, ok := statusByCode[code]
	if !ok {
		depsFrom(r.Context()).log.ErrorContext(r.Context(), "problem code not in catalog", "code", code)
		code, status, detail, ext, fields = "internal", http.StatusInternalServerError, "", Extensions{}, nil
	}
	h := w.Header()
	if retryAfterCodes[code] && h.Get("Retry-After") == "" {
		h.Set("Retry-After", "1")
	}
	if wwwAuthenticateCodes[code] {
		h.Set("WWW-Authenticate", "Bearer") // sin error ni error_description: el motivo va en el code (conventions.md §3)
	}
	h.Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problemBody{
		Type: "about:blank", Title: http.StatusText(status), Status: status, Code: code,
		Detail: detail, RequestID: logging.RequestID(r.Context()), Errors: fields,
		Module: ext.Module, Limit: ext.Limit, Used: ext.Used,
	})
}

// WriteError responde err: un *Error (también envuelto con %w) con su code; un
// error de base de datos que es contrapresión o base caída (WriteDBError); y
// cualquier otro como 500 internal, con la causa solo en el log.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var pe *Error
	switch {
	case errors.As(err, &pe):
		WriteProblem(w, r, pe.Code, pe.Detail, pe.Fields...)
	case WriteDBError(w, r, err):
	case r.Context().Err() != nil:
		// El cliente se fue o venció el plazo: la respuesta no llegaría y el
		// error es consecuencia de eso, no un 500 que registrar.
	default:
		depsFrom(r.Context()).log.ErrorContext(r.Context(), "unhandled error", "err", err)
		WriteProblem(w, r, "internal", "")
	}
}

// DecodeJSON lee el cuerpo JSON de r en dst. Devuelve un *Error con
// malformed_request si el Content-Type no es application/json, el JSON es
// inválido, un campo tiene el tipo incorrecto (errors[] lleva su field) o hay
// datos después del objeto; y payload_too_large si el cuerpo pasa de
// MaxBodyBytes (la lectura se corta en MaxBodyBytes+1, ver Chain).
func DecodeJSON(r *http.Request, dst any) error {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return NewError("malformed_request", "Content-Type must be application/json")
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return bodyError(err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return NewError("malformed_request", "unexpected data after the JSON value")
		}
		return bodyError(err)
	}
	return nil
}

func bodyError(err error) error {
	var tooBig *http.MaxBytesError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &tooBig):
		return NewError("payload_too_large", "request body exceeds "+strconv.Itoa(MaxBodyBytes)+" bytes")
	case errors.As(err, &typeErr) && typeErr.Field != "":
		return NewError("malformed_request", "a field has the wrong type", FieldError{Field: typeErr.Field, Code: "invalid_type"})
	default:
		return NewError("malformed_request", "the request body is not valid JSON")
	}
}
