package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
)

// RouteClass es la clase de una ruta /v1 frente a la sesión (api/conventions.md
// §2). No tiene valor cero válido, como db.SessionKind: una clase omitida entra
// en pánico al registrar.
type RouteClass uint8

// Clases de ruta.
const (
	// Public: sin búsqueda de sesión; solo bajo /v1/auth/ (login, refresh...).
	Public RouteClass = iota + 1
	// NoTenant: con sesión y sin tenant: /v1/auth/*, GET /v1/me y GET /v1/me/memberships.
	NoTenant
	// Tenant: con sesión ligada a una empresa; todas las demás rutas /v1.
	Tenant
)

func (c RouteClass) String() string {
	switch c {
	case Public:
		return "Public"
	case NoTenant:
		return "NoTenant"
	case Tenant:
		return "Tenant"
	default:
		return ""
	}
}

// Valores por defecto (conventions.md §7). Son supuestos que ajusta la prueba
// de carga de #196, no mediciones.
const (
	DefaultSessionBurst   = 20
	DefaultSessionRefill  = 100 * time.Millisecond // 10/s
	DefaultSessionMaxKeys = 50_000
	DefaultTenantBurst    = 200
	DefaultTenantRefill   = 10 * time.Millisecond // 100/s
	DefaultTenantMaxKeys  = 10_000
	// DefaultLookupTimeout es el plazo de la búsqueda de sesión y del desligue (ADR 0004).
	DefaultLookupTimeout = time.Second
)

// Contador de desligues por resultado: detached, already o failed.
const metricDetach = "auth_session_detach_total"

// Options reemplaza los valores por defecto, para los tests. Una LimiterConfig
// con Capacity 0 toma los valores del contrato; en las demás, Name, Key y Clock
// se completan si faltan.
type Options struct {
	SessionLimiter httpx.LimiterConfig
	TenantLimiter  httpx.LimiterConfig
	Clock          clock.Clock
	LookupTimeout  time.Duration
}

// Routes es el registrador de rutas /v1: arma la cadena sesión -> limitadores ->
// pasos de la ruta -> handler.
//
// Los módulos registran sus rutas /v1 solo por Routes. httpx.Router.Handle sigue
// siendo público (httpx lo necesita para /healthz y los tests) y acepta /v1 sin
// clase, así que el cableado de serve (#196) no expone el *httpx.Router fuera
// de cmd/wanka.
type Routes struct {
	rt       *httpx.Router
	resolver SessionResolver
	session  *httpx.Limiter
	tenant   *httpx.Limiter
	timeout  time.Duration
}

// NewRoutes valida las opciones y devuelve el registrador sobre rt.
func NewRoutes(rt *httpx.Router, resolver SessionResolver, o Options) (*Routes, error) {
	if rt == nil || resolver == nil {
		return nil, errors.New("auth: NewRoutes needs a router and a resolver")
	}
	if o.Clock == nil {
		o.Clock = clock.System{}
	}
	if o.LookupTimeout <= 0 {
		o.LookupTimeout = DefaultLookupTimeout
	}
	sc := withDefaults(o.SessionLimiter, "session", DefaultSessionBurst, DefaultSessionRefill, DefaultSessionMaxKeys, sessionKey, o.Clock)
	tc := withDefaults(o.TenantLimiter, "tenant", DefaultTenantBurst, DefaultTenantRefill, DefaultTenantMaxKeys, tenantKey, o.Clock)
	sl, err := httpx.NewLimiter(sc)
	if err != nil {
		return nil, fmt.Errorf("session limiter: %w", err)
	}
	tl, err := httpx.NewLimiter(tc)
	if err != nil {
		return nil, fmt.Errorf("tenant limiter: %w", err)
	}
	return &Routes{rt: rt, resolver: resolver, session: sl, tenant: tl, timeout: o.LookupTimeout}, nil
}

func withDefaults(c httpx.LimiterConfig, name string, burst int, refill time.Duration, maxKeys int, key httpx.KeyFunc, clk clock.Clock) httpx.LimiterConfig {
	if c.Capacity == 0 {
		c = httpx.LimiterConfig{Capacity: burst, Refill: refill, MaxKeys: maxKeys}
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

// Las claves son el id de la sesión y el del tenant, nunca el token ni su hash.
func sessionKey(r *http.Request) (string, bool) {
	s, ok := FromContext(r.Context())
	if !ok {
		return "", false
	}
	return s.ID.String(), true
}

func tenantKey(r *http.Request) (string, bool) {
	s, ok := FromContext(r.Context())
	if !ok || !s.HasTenant() {
		return "", false
	}
	return s.Actor.TenantID.String(), true
}

// Sweep elimina los buckets llenos de los dos limitadores y devuelve cuántos.
// Quien arranca el servidor lo llama cada cierto tiempo (#196).
func (r *Routes) Sweep() int { return r.session.Sweep() + r.tenant.Sweep() }

// Len es el número de buckets en memoria de los dos limitadores.
func (r *Routes) Len() int { return r.session.Len() + r.tenant.Len() }

// Rutas sin tenant que no cuelgan de /v1/auth/ (conventions.md §2).
var noTenantRoutes = map[string]bool{"GET /v1/me": true, "GET /v1/me/memberships": true}

func authPath(path string) bool { return strings.HasPrefix(path, "/v1/auth/") }

// checkRoute entra en pánico si pattern o class no cumplen las reglas de §2:
// el arranque falla en vez de dejar una ruta sin guarda.
func checkRoute(pattern string, class RouteClass) {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok || !strings.HasPrefix(path, "/v1/") {
		panic(fmt.Sprintf("auth: route %q must be \"METHOD /v1/...\"", pattern))
	}
	noTenant := authPath(path) || noTenantRoutes[method+" "+path]
	switch {
	case class.String() == "":
		panic(fmt.Sprintf("auth: route %q needs a class: Public, NoTenant or Tenant", pattern))
	case class == Public && !authPath(path):
		panic(fmt.Sprintf("auth: route %q is Public but only /v1/auth/* may be", pattern))
	case class == NoTenant && !noTenant:
		panic(fmt.Sprintf("auth: route %q is NoTenant but only /v1/auth/*, GET /v1/me and GET /v1/me/memberships may be", pattern))
	case class == Tenant && noTenant:
		panic(fmt.Sprintf("auth: route %q is Tenant but it is a route without a tenant", pattern))
	}
}

// Handle registra h para pattern ("MÉTODO /v1/camino") con su clase. Los
// pasos de ruta de opts (httpx.Use) corren después de la sesión y los
// limitadores. Entra en pánico si la clase no cumple las reglas.
func (r *Routes) Handle(pattern string, class RouteClass, h http.Handler, opts ...httpx.RouteOption) {
	checkRoute(pattern, class)
	if class == Public {
		r.rt.Handle(pattern, h, opts...)
		return
	}
	mws := []httpx.Middleware{r.sessionStep(class == Tenant), r.session.Middleware()}
	if class == Tenant {
		mws = append(mws, r.tenant.Middleware())
	}
	r.rt.Handle(pattern, h, append([]httpx.RouteOption{httpx.Use(mws...)}, opts...)...)
}

// HandleFunc es Handle con una función.
func (r *Routes) HandleFunc(pattern string, class RouteClass, f http.HandlerFunc, opts ...httpx.RouteOption) {
	r.Handle(pattern, class, f, opts...)
}

// sessionStep autentica la petición: valida el token sin consultar la base,
// busca la sesión, desliga la que perdió su membresía (ADR 0011) y la deja en
// el contexto. Con needTenant responde 403 tenant_access_denied si la sesión no
// está ligada a una empresa.
func (r *Routes) sessionStep(needTenant bool) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			hash, err := ParseAuthorization(req.Header.Values("Authorization"))
			if err != nil {
				httpx.WriteProblem(w, req, "unauthenticated", "a valid access token is required")
				return
			}
			rec, ok := r.lookup(w, req, hash)
			if !ok {
				return
			}
			detail := ""
			detached := false
			if rec.lostMembership() {
				r.detach(req, rec)
				detached, detail = true, "membership is no longer active"
			}
			s := rec.session(detached)
			if needTenant && !s.HasTenant() {
				if detail == "" {
					detail = "session has no company selected"
				}
				httpx.WriteProblem(w, req, "tenant_access_denied", detail)
				return
			}
			next.ServeHTTP(w, req.WithContext(WithSession(req.Context(), s)))
		})
	}
}

// lookup busca la sesión en la fase session_lookup y con el plazo propio. Si no
// devuelve una sesión válida, ya respondió (o decidió no responder).
func (r *Routes) lookup(w http.ResponseWriter, req *http.Request, hash TokenHash) (SessionRecord, bool) {
	// La fase vive solo en este contexto: el handler no la hereda.
	lr := req.WithContext(httpx.WithPhase(req.Context(), httpx.PhaseSessionLookup))
	ctx, cancel := context.WithTimeout(lr.Context(), r.timeout)
	defer cancel()
	rec, found, err := r.resolver.Lookup(ctx, hash)
	if req.Context().Err() != nil {
		return SessionRecord{}, false // el cliente se fue o venció el plazo del handler: responde otro o nadie
	}
	switch {
	case err == nil && found && !rec.MFASatisfied:
		// El resolver ya filtra la sesión a medio login; esto es defensa en profundidad.
		httpx.WriteProblem(w, req, "session_expired", "the session is not valid")
	case err == nil && !found:
		httpx.WriteProblem(w, req, "session_expired", "the session is not valid")
	case err == nil && !rec.coherent():
		httpx.Logger(req.Context()).ErrorContext(req.Context(), "session record is inconsistent", "route", httpx.Route(req))
		httpx.WriteProblem(w, req, "internal", "")
	case err == nil:
		return rec, true
	case httpx.WriteDBError(w, lr, err):
	case isOwnTimeout(err):
		// Plazo propio vencido o 57014 sin tipar: WriteDBError no los reconoce con
		// la petición viva. Sin la búsqueda no se sabe si la sesión vale: ni 401
		// (el BFF refrescaría contra la misma base caída) ni 500.
		httpx.LogDBFailure(lr, "session lookup timed out", metrics.DBUnavailable, httpx.PhaseSessionLookup, "")
		httpx.WriteProblem(w, req, "dependency_unavailable", "a dependency is not responding")
	default:
		// Error no tipado: un bug. Solo el tipo va al log, el texto puede venir de la base.
		httpx.Logger(req.Context()).ErrorContext(req.Context(), "session lookup failed with an unexpected error",
			"route", httpx.Route(req), "error_type", fmt.Sprintf("%T", err))
		httpx.WriteProblem(w, req, "internal", "")
	}
	return SessionRecord{}, false
}

func isOwnTimeout(err error) bool {
	var pgErr *pgconn.PgError
	return errors.Is(err, context.DeadlineExceeded) || errors.As(err, &pgErr) && pgErr.Code == "57014"
}

// detach desliga la sesión de su empresa, con la fase session_lookup. Si falla o
// no actualiza filas, la decisión no cambia y la siguiente búsqueda lo vuelve a detectar.
func (r *Routes) detach(req *http.Request, rec SessionRecord) {
	ctx, cancel := context.WithTimeout(httpx.WithPhase(req.Context(), httpx.PhaseSessionLookup), r.timeout)
	defer cancel()
	ok, err := r.resolver.Detach(ctx, rec.SessionID, rec.MembershipID)
	outcome := "detached"
	switch {
	case err != nil:
		outcome = "failed"
		if req.Context().Err() == nil {
			httpx.Logger(req.Context()).WarnContext(req.Context(), "session detach failed",
				"route", httpx.Route(req), "phase", httpx.PhaseSessionLookup, "error_type", fmt.Sprintf("%T", err))
		}
	case !ok:
		outcome = "already"
	}
	httpx.Metrics(req.Context()).Inc(metricDetach, "outcome", outcome)
}
