package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
)

// Class es la clase de una ruta /v1 frente a la sesión (api/conventions.md §2).
// Se declara al registrarla y Router.Handle entra en pánico si no coincide con
// las reglas, así que el arranque falla en vez de dejar una ruta sin guarda.
type Class uint8

// Clases de ruta. El cero es "sin clase": solo vale fuera de /v1.
const (
	classNone Class = iota
	// ClassPublic: sin búsqueda de sesión; solo bajo /v1/auth/ (login, refresh...).
	ClassPublic
	// ClassNoTenant: con sesión y sin tenant: /v1/auth/*, GET /v1/me y GET /v1/me/memberships.
	ClassNoTenant
	// ClassTenant: con sesión ligada a una empresa; todas las demás rutas /v1.
	ClassTenant
)

func (c Class) String() string {
	switch c {
	case ClassPublic:
		return "Public"
	case ClassNoTenant:
		return "NoTenant"
	case ClassTenant:
		return "Tenant"
	default:
		return "none"
	}
}

// Public declara una ruta sin sesión (solo /v1/auth/*).
func Public() RouteOption { return withClass(ClassPublic) }

// NoTenant declara una ruta que exige sesión pero no empresa.
func NoTenant() RouteOption { return withClass(ClassNoTenant) }

// Tenant declara una ruta que exige una sesión ligada a una empresa.
func Tenant() RouteOption { return withClass(ClassTenant) }

func withClass(c Class) RouteOption {
	return func(r *route) {
		r.classes = append(r.classes, c)
	}
}

// Valores por defecto del limitador (api/conventions.md §7). Son supuestos que
// ajusta la prueba de carga, no mediciones.
const (
	// DefaultSessionBurst y DefaultSessionRefill: 10/s con ráfaga de 20 por sesión.
	DefaultSessionBurst  = 20
	DefaultSessionRefill = 100 * time.Millisecond
	// DefaultSessionMaxKeys: unas 5 veces el pico supuesto del año 1, por réplica.
	DefaultSessionMaxKeys = 50_000
	// DefaultTenantBurst y DefaultTenantRefill: 100/s con ráfaga de 200 por tenant.
	DefaultTenantBurst  = 200
	DefaultTenantRefill = 10 * time.Millisecond
	// DefaultTenantMaxKeys es el tope de empresas con bucket por réplica.
	DefaultTenantMaxKeys = 10_000
	// DefaultLookupTimeout es el plazo de la búsqueda de sesión (ADR 0004).
	DefaultLookupTimeout = time.Second
)

// NewSessionLimiter devuelve el limitador por sesión con los valores del
// contrato. La clave es el id de la sesión, nunca el token ni su hash.
func NewSessionLimiter(clk clock.Clock) *Limiter {
	return mustLimiter(LimiterConfig{Name: "session", Capacity: DefaultSessionBurst, Refill: DefaultSessionRefill,
		MaxKeys: DefaultSessionMaxKeys, Key: sessionKey, Clock: clk})
}

// NewTenantLimiter devuelve el limitador por tenant con los valores del contrato.
func NewTenantLimiter(clk clock.Clock) *Limiter {
	return mustLimiter(LimiterConfig{Name: "tenant", Capacity: DefaultTenantBurst, Refill: DefaultTenantRefill,
		MaxKeys: DefaultTenantMaxKeys, Key: tenantKey, Clock: clk})
}

func mustLimiter(cfg LimiterConfig) *Limiter {
	l, err := NewLimiter(cfg)
	if err != nil {
		panic("httpx: " + err.Error()) // los valores por defecto son constantes válidas
	}
	return l
}

func sessionKey(r *http.Request) (string, bool) {
	s, ok := auth.SessionFrom(r.Context())
	if !ok {
		return "", false
	}
	return s.ID.String(), true
}

func tenantKey(r *http.Request) (string, bool) {
	s, ok := auth.SessionFrom(r.Context())
	if !ok || !s.HasTenant() {
		return "", false
	}
	return s.TenantID.String(), true
}

// SessionOptions activa el paso de sesión del Router. Los campos en cero salvo
// Resolver toman los valores del contrato.
type SessionOptions struct {
	Resolver auth.SessionResolver
	// SessionLimiter corre en toda ruta con sesión; TenantLimiter solo en las de tenant.
	SessionLimiter *Limiter
	TenantLimiter  *Limiter
	// LookupTimeout acota la búsqueda de la sesión (1 s) y el desligue.
	LookupTimeout time.Duration
}

// withDefaults devuelve una copia con los valores por defecto. Entra en pánico
// sin Resolver: es un error de programación al armar el Router.
func (o SessionOptions) withDefaults() SessionOptions {
	if o.Resolver == nil {
		panic("httpx: SessionOptions needs a Resolver")
	}
	if o.SessionLimiter == nil {
		o.SessionLimiter = NewSessionLimiter(clock.System{})
	}
	if o.TenantLimiter == nil {
		o.TenantLimiter = NewTenantLimiter(clock.System{})
	}
	if o.LookupTimeout <= 0 {
		o.LookupTimeout = DefaultLookupTimeout
	}
	return o
}

// Rutas sin tenant que no cuelgan de /v1/auth/ (conventions.md §2).
var noTenantRoutes = map[string]bool{"GET /v1/me": true, "GET /v1/me/memberships": true}

// authPath dice si path cuelga de /v1/auth/. Los patrones con comodines siguen las mismas reglas.
func authPath(path string) bool { return strings.HasPrefix(path, "/v1/auth/") }

func isV1(path string) bool { return path == "/v1" || strings.HasPrefix(path, "/v1/") }

// checkClass entra en pánico si la clase declarada no cumple las reglas de §2.
func checkClass(pattern, method, path string, classes []Class, hasSession bool) Class {
	if len(classes) > 1 {
		panic(fmt.Sprintf("httpx: route %q declares more than one class", pattern))
	}
	var c Class
	if len(classes) == 1 {
		c = classes[0]
	}
	if !isV1(path) {
		if c != classNone {
			panic(fmt.Sprintf("httpx: route %q is outside /v1 and cannot declare class %s", pattern, c))
		}
		return c
	}
	if c == classNone {
		panic(fmt.Sprintf("httpx: route %q must declare Public(), NoTenant() or Tenant()", pattern))
	}
	noTenant := authPath(path) || noTenantRoutes[method+" "+path]
	switch {
	case c == ClassPublic && !authPath(path):
		panic(fmt.Sprintf("httpx: route %q is Public but only /v1/auth/* may be", pattern))
	case c == ClassNoTenant && !noTenant:
		panic(fmt.Sprintf("httpx: route %q is NoTenant but only /v1/auth/*, GET /v1/me and GET /v1/me/memberships may be", pattern))
	case c == ClassTenant && noTenant:
		panic(fmt.Sprintf("httpx: route %q is Tenant but it is one of the routes without a tenant", pattern))
	case c != ClassPublic && !hasSession:
		panic(fmt.Sprintf("httpx: route %q needs a session step: set RouterOptions.Session", pattern))
	}
	return c
}

// sessionMiddlewares devuelve el paso de sesión y los limitadores de la clase,
// en el orden de la secuencia 08: sesión, límite por sesión y, solo en rutas de tenant, por tenant.
func (rt *Router) sessionMiddlewares(c Class) []Middleware {
	switch c {
	case ClassNoTenant:
		return []Middleware{rt.sessionStep(false), rt.session.SessionLimiter.Middleware()}
	case ClassTenant:
		return []Middleware{rt.sessionStep(true), rt.session.SessionLimiter.Middleware(), rt.session.TenantLimiter.Middleware()}
	default:
		return nil
	}
}

// sessionStep autentica la petición: valida el token sin consultar la base,
// busca la sesión, desliga la que perdió su membresía (ADR 0011), y la deja en
// el contexto. Con needTenant responde 403 tenant_access_denied si la sesión no
// está ligada a una empresa. Termina la petición con 401 (unauthenticated o
// session_expired) o 503 cuando falla la búsqueda: nunca 401 ni 500 por un fallo de la base.
func (rt *Router) sessionStep(needTenant bool) Middleware {
	so := rt.session
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hash, err := auth.ParseAuthorization(r.Header.Values("Authorization"))
			if err != nil {
				WriteProblem(w, r, "unauthenticated", "a valid access token is required")
				return
			}
			s, ok := lookupSession(w, r, so, hash)
			if !ok {
				return
			}
			denied := ""
			if s.LostMembership() {
				unbindSession(r, so, s)
				s = s.WithoutTenant()
				denied = "you no longer have access to that company"
			}
			if needTenant && !s.HasTenant() {
				if denied == "" {
					denied = "select a company to continue"
				}
				WriteProblem(w, r, "tenant_access_denied", denied)
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithSession(r.Context(), s)))
		})
	}
}

// lookupSession busca la sesión con la fase session_lookup y el plazo propio.
// Si no devuelve una sesión, ya respondió (o decidió no responder).
func lookupSession(w http.ResponseWriter, r *http.Request, so SessionOptions, hash auth.TokenHash) (auth.Session, bool) {
	lr := r.WithContext(WithPhase(r.Context(), PhaseSessionLookup))
	ctx, cancel := context.WithTimeout(lr.Context(), so.LookupTimeout)
	defer cancel()
	s, err := so.Resolver.Lookup(ctx, hash)
	switch {
	case err == nil:
		return s, true
	case errors.Is(err, auth.ErrSessionNotFound):
		WriteProblem(w, r, "session_expired", "the session is not valid")
	case r.Context().Err() != nil:
		// El cliente se fue o venció el plazo del handler: no hay a quién responder.
	case WriteDBError(w, lr, err):
	default:
		// Plazo propio vencido u otro fallo del resolver: no se sabe si la sesión
		// vale, así que ni 401 (el BFF refrescaría contra la misma base caída) ni 500.
		// El texto del error no se registra: puede venir de la base.
		msg := "session lookup failed"
		if errors.Is(err, context.DeadlineExceeded) {
			msg = "session lookup timed out"
		}
		logDBFailure(lr, msg, metrics.DBUnavailable, PhaseSessionLookup, "")
		WriteProblem(w, r, "dependency_unavailable", "a dependency is not responding")
	}
	return auth.Session{}, false
}

// unbindSession desliga la sesión de su empresa. Si falla, la decisión no
// cambia y la siguiente búsqueda lo vuelve a detectar (ADR 0011).
func unbindSession(r *http.Request, so SessionOptions, s auth.Session) {
	ctx, cancel := context.WithTimeout(r.Context(), so.LookupTimeout)
	defer cancel()
	if err := so.Resolver.Unbind(ctx, s.ID, s.MembershipID); err != nil && r.Context().Err() == nil {
		depsFrom(r.Context()).log.WarnContext(r.Context(), "session unbind failed",
			"route", routeOf(r), "error_type", fmt.Sprintf("%T", err))
	}
}
