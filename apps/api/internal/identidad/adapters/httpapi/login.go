// Package httpapi es el adaptador HTTP de identidad: el handler de POST /v1/auth/login.
package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/app"
	"github.com/bacsystem/wanka/apps/api/internal/identidad/domain"
	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// Cuotas de POST /v1/auth/login (conventions.md §7). Son por réplica y en memoria, como las demás.
const (
	// 5/min por IP+email: un bucket de 5 que recupera una ficha cada 12 s.
	EmailQuotaBurst  = 5
	EmailQuotaRefill = 12 * time.Second
	// 30/min por IP: un bucket de 30 que recupera una ficha cada 2 s.
	IPQuotaBurst  = 30
	IPQuotaRefill = 2 * time.Second
	// MaxQuotaKeys acota la memoria de cada limitador por réplica.
	MaxQuotaKeys = 50_000
	// RetryAfterEnrollment es el Retry-After de la rama de enrolamiento mientras la implementa #25.
	RetryAfterEnrollment = 60
)

// unknownIP es la clave de todas las IP desconocidas: comparten un solo bucket (conventions.md §3).
const unknownIP = "unknown"

// LoginUseCase es lo que el handler necesita del caso de uso.
type LoginUseCase interface {
	Login(ctx context.Context, req app.Request) (app.Result, error)
}

// Options arma el handler.
type Options struct {
	// ClientIP saca la IP de la petición (auth.ClientIP con la configuración de confianza).
	ClientIP func(*http.Request) netip.Addr
	Clock    clock.Clock
	Log      *slog.Logger
	// Los limitadores con cero toman las cuotas del contrato.
	EmailLimiter httpx.LimiterConfig
	IPLimiter    httpx.LimiterConfig
}

// Login es el handler de POST /v1/auth/login.
type Login struct {
	uc       LoginUseCase
	clientIP func(*http.Request) netip.Addr
	log      *slog.Logger
	quotas   func(http.Handler) http.Handler
}

type quotaKey struct{}

// quotaIdentity es lo que las cuotas leen de la petición: se fija después de validar el cuerpo.
type quotaIdentity struct {
	ip    string
	email string
}

// NewLogin arma el handler y sus dos limitadores.
func NewLogin(uc LoginUseCase, o Options) (*Login, error) {
	if o.Clock == nil {
		o.Clock = clock.System{}
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	ipCfg := withDefaults(o.IPLimiter, "login_ip", IPQuotaBurst, IPQuotaRefill, o.Clock, func(r *http.Request) (string, bool) {
		return identityOf(r).ip, true
	})
	emailCfg := withDefaults(o.EmailLimiter, "login_ip_email", EmailQuotaBurst, EmailQuotaRefill, o.Clock, func(r *http.Request) (string, bool) {
		id := identityOf(r)
		// El correo se resume: la clave no crece con el largo de lo que mande quien llama.
		sum := sha256.Sum256([]byte(id.email))
		return id.ip + "|" + hex.EncodeToString(sum[:16]), true
	})
	ipL, err := httpx.NewLimiter(ipCfg)
	if err != nil {
		return nil, err
	}
	emailL, err := httpx.NewLimiter(emailCfg)
	if err != nil {
		return nil, err
	}
	if o.ClientIP == nil {
		return nil, errors.New("httpapi: Login needs a ClientIP function")
	}
	return &Login{uc: uc, clientIP: o.ClientIP, log: o.Log, quotas: func(next http.Handler) http.Handler {
		// Primero la IP y luego IP+email: si el segundo rechaza, el primero devuelve su ficha.
		return ipL.Middleware()(emailL.Middleware()(next))
	}}, nil
}

func withDefaults(c httpx.LimiterConfig, name string, burst int, refill time.Duration, clk clock.Clock, key httpx.KeyFunc) httpx.LimiterConfig {
	if c.Capacity == 0 {
		c = httpx.LimiterConfig{Capacity: burst, Refill: refill, MaxKeys: MaxQuotaKeys}
	}
	if c.Name == "" {
		c.Name = name
	}
	if c.Key == nil {
		c.Key = key
	}
	if c.Clock == nil {
		c.Clock = clk
	}
	return c
}

func identityOf(r *http.Request) quotaIdentity {
	id, _ := r.Context().Value(quotaKey{}).(quotaIdentity)
	return id
}

// loginBody es el cuerpo de LoginRequest. Guarda los campos tal como llegaron para distinguir un campo
// ausente de uno vacío y detectar los que el contrato no permite (additionalProperties: false).
type loginBody struct {
	email, password string
	hasEmail        bool
	hasPassword     bool
	unknown         []string
}

// UnmarshalJSON lo llama httpx.DecodeJSON: un campo de tipo incorrecto es 400 malformed_request con su
// nombre; todo lo demás se valida después, como 422.
func (b *loginBody) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for k, v := range raw {
		switch k {
		case "email":
			if err := decodeString(v, &b.email, &b.hasEmail, k); err != nil {
				return err
			}
		case "password":
			if err := decodeString(v, &b.password, &b.hasPassword, k); err != nil {
				return err
			}
		default:
			b.unknown = append(b.unknown, k)
		}
	}
	return nil
}

func decodeString(raw json.RawMessage, dst *string, has *bool, field string) error {
	if string(raw) == "null" {
		return nil // se trata como ausente: 422 required
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return &json.UnmarshalTypeError{Field: field}
	}
	*has = true
	return nil
}

// validate devuelve los errores de esquema (422) y el correo ya normalizado.
func (b *loginBody) validate() (email string, fields []httpx.FieldError) {
	email = domain.NormalizeEmail(b.email)
	switch {
	case !b.hasEmail:
		fields = append(fields, httpx.FieldError{Field: "email", Code: "required"})
	case !domain.EmailLenOK(email):
		fields = append(fields, httpx.FieldError{Field: "email", Code: "invalid_length",
			Meta: map[string]any{"min": domain.MinEmailLen, "max": domain.MaxEmailLen}})
	case domain.EmailHasNUL(email):
		fields = append(fields, httpx.FieldError{Field: "email", Code: "invalid_format"})
	}
	switch {
	case !b.hasPassword:
		fields = append(fields, httpx.FieldError{Field: "password", Code: "required"})
	case !domain.PasswordLenOK(b.password):
		fields = append(fields, httpx.FieldError{Field: "password", Code: "invalid_length",
			Meta: map[string]any{"min": domain.MinPasswordLen, "max": domain.MaxPasswordLen}})
	}
	for _, k := range b.unknown {
		fields = append(fields, httpx.FieldError{Field: k, Code: "unknown_field"})
	}
	return email, fields
}

// ServeHTTP responde POST /v1/auth/login. Orden: cuerpo (400), esquema (422), cuotas (429), caso de uso.
func (h *Login) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	email, fields := body.validate()
	if len(fields) > 0 {
		httpx.WriteProblem(w, r, "validation_failed", "the request does not meet the schema", fields...)
		return
	}
	ip := unknownIP
	addr := h.clientIP(r)
	if addr.IsValid() {
		ip = addr.String()
	}
	ctx := context.WithValue(r.Context(), quotaKey{}, quotaIdentity{ip: ip, email: email})
	h.quotas(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.login(w, r, app.Request{Email: email, Password: body.password, IP: addr, UserAgent: r.UserAgent()})
	})).ServeHTTP(w, r.WithContext(ctx))
}

func (h *Login) login(w http.ResponseWriter, r *http.Request, req app.Request) {
	res, err := h.uc.Login(r.Context(), req)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	// La respuesta lleva tokens: ninguna caché la guarda.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	switch res.Outcome {
	case app.MFARequired:
		writeJSON(w, mfaRequiredBody{Status: "mfa_required", ChallengeToken: res.ChallengeToken, ExpiresAt: res.ExpiresAt})
	default:
		writeJSON(w, authenticatedBody{Status: "authenticated", AccessToken: res.AccessToken, RefreshToken: res.RefreshToken,
			ExpiresAt: res.ExpiresAt, Memberships: membershipBodies(res.Memberships)})
	}
}

func (h *Login) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var locked *app.MFALockedError
	switch {
	case errors.Is(err, app.ErrInvalidCredentials):
		httpx.WriteProblem(w, r, "invalid_credentials", "the email or the password is incorrect")
	case errors.Is(err, app.ErrTenantAccessDenied):
		httpx.WriteProblem(w, r, "tenant_access_denied", "the user has no company to enter")
	case errors.Is(err, app.ErrOverloaded):
		httpx.WriteProblem(w, r, "overloaded", "the server is busy, retry shortly")
	case errors.Is(err, app.ErrEnrollmentUnavailable):
		w.Header().Set("Retry-After", strconv.Itoa(RetryAfterEnrollment))
		httpx.WriteProblem(w, r, "dependency_unavailable", "a dependency is not responding")
	case errors.As(err, &locked):
		secs := int((locked.RetryAfter + time.Second - 1) / time.Second)
		w.Header().Set("Retry-After", strconv.Itoa(max(secs, 1)))
		httpx.WriteProblem(w, r, "rate_limited", "too many attempts, retry later")
	default:
		httpx.WriteError(w, r, err)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	// Un fallo al escribir es del cliente (se fue): no hay otra respuesta posible.
	_ = json.NewEncoder(w).Encode(v)
}

type authenticatedBody struct {
	Status       string           `json:"status"`
	AccessToken  string           `json:"accessToken"`
	RefreshToken string           `json:"refreshToken"`
	ExpiresAt    time.Time        `json:"expiresAt"`
	Memberships  []membershipBody `json:"memberships"`
}

type mfaRequiredBody struct {
	Status         string    `json:"status"`
	ChallengeToken string    `json:"challengeToken"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

type membershipBody struct {
	MembershipID  string     `json:"membershipId"`
	Tenant        tenantBody `json:"tenant"`
	Role          roleBody   `json:"role"`
	Selectable    bool       `json:"selectable"`
	BlockedReason string     `json:"blockedReason,omitempty"`
	ReadOnly      bool       `json:"readOnly"`
	Sites         []siteBody `json:"sites"`
}

type tenantBody struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Industry string `json:"industry"`
	Status   string `json:"status"`
}

type roleBody struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RequiresMFA bool   `json:"requiresMfa"`
}

type siteBody struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	CashRegisters []cashRegister `json:"cashRegisters"`
}

type cashRegister struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}

func membershipBodies(ms []app.Membership) []membershipBody {
	out := make([]membershipBody, 0, len(ms))
	for _, m := range ms {
		sites := make([]siteBody, 0, len(m.Sites))
		for _, s := range m.Sites {
			regs := make([]cashRegister, 0, len(s.CashRegisters))
			for _, c := range s.CashRegisters {
				regs = append(regs, cashRegister{ID: c.ID, Code: c.Code})
			}
			sites = append(sites, siteBody{ID: s.ID, Name: s.Name, CashRegisters: regs})
		}
		out = append(out, membershipBody{
			MembershipID: m.ID,
			Tenant:       tenantBody{ID: m.TenantID, Name: m.TenantName, Industry: m.Industry, Status: m.TenantStatus},
			Role:         roleBody{ID: m.RoleID, Name: m.RoleName, RequiresMFA: m.RequiresMFA},
			Selectable:   m.Selectable, BlockedReason: m.BlockedReason, ReadOnly: m.ReadOnly, Sites: sites,
		})
	}
	return out
}
