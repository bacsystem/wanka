package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
)

// Valores reservados de x-module y x-permission para «ninguno» (ADR 0009, «Cómo
// se declara ninguno»). Son literales: un string vacío al registrar una ruta
// hace fallar el arranque.
const (
	NoModule     = entitlements.NoModule
	NoPermission = "none"
)

// EntitlementSource es lo que el paso de entitlements necesita de
// entitlements.Service; el test lo reemplaza con un doble.
type EntitlementSource interface {
	For(ctx context.Context, tenant pgtype.UUID) (entitlements.Entitlements, error)
}

// Spec es lo que una ruta declara en el spec (x-module y x-permission) más el
// límite del plan que consume. Registrar una ruta sin Module ni Permission hace
// fallar el arranque; para «ninguno» se usan NoModule y NoPermission.
type Spec struct {
	// Module es el código de módulo de platform.modules, o NoModule.
	Module string
	// Permission es el código del catálogo de resources.md, o NoPermission. Lo
	// aplica el paso de RBAC (#176); aquí solo se exige que esté declarado.
	Permission string
	// Limit es la clave del límite que la ruta consume (entitlements.LimitUsers,
	// LimitSites, LimitCPEPerMonth); vacío si no consume ninguno. Con
	// used >= limit la ruta responde 403 plan_limit_reached.
	Limit string
}

// inactivePass es la lista fija, en el código y no en configuración, de rutas de
// tenant que pasan con la suscripción morosa, suspendida o cancelada (ADR 0005,
// «Suscripción inactiva»). Las rutas sin tenant (/v1/auth/*, GET /v1/me, GET
// /v1/me/memberships) no pasan por entitlements y no están aquí. Cada una se
// declara con NoModule (ADR 0009) y toda ruta Tenant con NoModule está aquí.
var inactivePass = map[string]bool{
	"GET /v1/exports":         true,
	"GET /v1/me/entitlements": true,
}

var knownLimits = map[string]bool{
	entitlements.LimitUsers: true, entitlements.LimitSites: true, entitlements.LimitCPEPerMonth: true,
}

// checkSpec entra en pánico si spec no cumple las reglas del ADR 0009 para esa
// ruta: el arranque falla en vez de dejar una ruta sin módulo o sin permiso.
func checkSpec(pattern string, class RouteClass, spec Spec) {
	switch {
	case spec.Module == "":
		panic(fmt.Sprintf("auth: route %q needs a module: a platform.modules code or auth.NoModule", pattern))
	case spec.Permission == "":
		panic(fmt.Sprintf("auth: route %q needs a permission: a resources.md code or auth.NoPermission", pattern))
	case class != Tenant && spec.Module != NoModule:
		panic(fmt.Sprintf("auth: route %q is %s and does not go through entitlements: its module must be auth.NoModule", pattern, class))
	case class == Tenant && (spec.Module == NoModule) != inactivePass[pattern]:
		panic(fmt.Sprintf("auth: route %q: only GET /v1/exports and GET /v1/me/entitlements are Tenant routes with auth.NoModule, and they must use it", pattern))
	case spec.Limit != "" && !knownLimits[spec.Limit]:
		panic(fmt.Sprintf("auth: route %q consumes the unknown limit %q", pattern, spec.Limit))
	case spec.Limit != "" && class != Tenant:
		panic(fmt.Sprintf("auth: route %q is %s and cannot consume a plan limit", pattern, class))
	}
}

// entitlementsStep aplica a una ruta Tenant, después de los limitadores, las
// reglas de la secuencia 08: empresa cerrada, suscripción, módulo, límite.
func (r *Routes) entitlementsStep(pattern string, spec Spec) httpx.Middleware {
	base := entitlements.Request{Module: spec.Module, Limit: spec.Limit, KeepsRunningWhenInactive: inactivePass[pattern]}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, hr *http.Request) {
			s, ok := FromContext(hr.Context())
			if !ok || !s.HasTenant() {
				// El paso de sesión ya cerró este caso: llegar aquí es un bug, y no se deja pasar.
				httpx.Logger(hr.Context()).ErrorContext(hr.Context(), "entitlements step without a tenant session", "route", httpx.Route(hr))
				httpx.WriteProblem(w, hr, "internal", "")
				return
			}
			ctx := hr.Context()
			if spec.Module != NoModule {
				ctx = httpx.WithModule(ctx, spec.Module)
			}
			// La fase vive solo en el contexto de esta lectura: el handler no la hereda.
			lr := hr.WithContext(httpx.WithPhase(ctx, httpx.PhaseEntitlements))
			e, err := r.ent.For(lr.Context(), s.Actor.TenantID)
			if err != nil {
				r.entitlementsFailed(w, hr, lr, err)
				return
			}
			req := base // copia: el middleware lo comparten todas las peticiones de la ruta
			req.Method = hr.Method
			if d := e.Check(req); d != nil {
				httpx.WriteProblemExt(w, hr, d.Code, d.Detail, httpx.Extensions{Module: d.Module, Limit: d.Limit, Used: d.Used})
				return
			}
			next.ServeHTTP(w, hr.WithContext(ctx))
		})
	}
}

// entitlementsFailed responde cuando no se pudieron leer los entitlements: la
// petición nunca pasa sin ellos.
func (r *Routes) entitlementsFailed(w http.ResponseWriter, hr, lr *http.Request, err error) {
	switch {
	case httpx.WriteDBError(w, lr, err):
	case errors.Is(err, entitlements.ErrTenantNotFound):
		// La sesión apunta a una empresa que ya no existe (purgada): no hay a qué entrar.
		httpx.Logger(hr.Context()).WarnContext(hr.Context(), "session tenant does not exist", "route", httpx.Route(hr), "phase", httpx.PhaseEntitlements)
		httpx.WriteProblem(w, hr, "tenant_access_denied", "the company is not available")
	case isOwnTimeout(err):
		// 57014 sin tipar o plazo vencido con la petición viva: WriteDBError no los reconoce.
		httpx.LogDBFailure(lr, "entitlements read timed out", metrics.DBUnavailable, httpx.PhaseEntitlements, "")
		httpx.WriteProblem(w, hr, "dependency_unavailable", "a dependency is not responding")
	default:
		// Error no tipado: un bug. Solo el tipo va al log, el texto puede venir de la base.
		httpx.Logger(hr.Context()).ErrorContext(hr.Context(), "entitlements read failed with an unexpected error",
			"route", httpx.Route(hr), "error_type", fmt.Sprintf("%T", err))
		httpx.WriteProblem(w, hr, "internal", "")
	}
}
