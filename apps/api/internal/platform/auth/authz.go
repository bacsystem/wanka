package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/audit"
	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

// Authz es lo que el paso de permisos necesita de una membresía, leído en una
// sola transacción de tenant: la versión, el rol y el alcance de sedes.
type Authz struct {
	// Version es la authz_version de la membresía que se leyó; es la que se
	// guarda en la sesión y con la que se cachea.
	Version int64
	// RequiresMFA es iam.roles.requires_mfa del rol vigente.
	RequiresMFA bool
	// Permissions son los códigos del rol. Un conjunto nil es vacío: todo se niega.
	Permissions map[permission.Code]struct{}
	// AllSites y Sites son el alcance de sedes de la membresía (CheckSite).
	AllSites bool
	Sites    []pgtype.UUID
}

// ErrMembershipLost lo devuelve un PermissionResolver cuando la membresía de la
// sesión ya no da acceso: no existe, no es del usuario ni de la empresa de la
// sesión, o no está activa (invitado, suspendido). Pasa cuando cambia entre la
// búsqueda de la sesión y la resolución. El paso responde 403
// tenant_access_denied sin revocar la sesión (ADR 0011); la siguiente petición
// la desliga la búsqueda de sesión.
var ErrMembershipLost = errors.New("membership no longer grants access")

// PermissionResolver es el puerto de rol y permisos de una membresía. El
// adaptador de PostgreSQL es identidad.NewPermissionResolver (#233); el paso se prueba con un doble.
type PermissionResolver interface {
	// Resolve lee, para la membresía de s y con su Actor, la versión actual, el
	// rol (requires_mfa), los permisos y las sedes. Una membresía que ya no da
	// acceso es ErrMembershipLost. Los errores de la base se devuelven tipados
	// (db.ErrOverloaded, db.ErrUnavailable...).
	Resolve(ctx context.Context, s Session) (Authz, error)
}

// DenyAllPermissions es el resolver que niega todo; serve ya no lo cablea (usa
// identidad.NewPermissionResolver) y quedó para las pruebas: ningún permiso y sin 2FA exigida. Cerrar es negar la petición, no
// destruir estado: por eso no devuelve RequiresMFA, que revocaría sesiones.
type DenyAllPermissions struct{}

// Resolve devuelve el conjunto vacío con la versión que ya tiene la membresía.
func (DenyAllPermissions) Resolve(_ context.Context, s Session) (Authz, error) {
	return Authz{Version: s.CurrentAuthzVersion.Int64}, nil
}

// DenialRecorder registra un intento denegado por permiso; lo implementa
// *audit.Writer.
type DenialRecorder interface {
	RecordDenied(ctx context.Context, actor db.Actor, ev audit.Event) error
}

// Contadores del paso de permisos.
const (
	metricAuthzDecision    = "authz_decision_total"            // outcome: allowed, forbidden, mfa_required, tenant_access_denied, error
	metricAuthzCache       = "authz_cache_total"               // result: hit, miss
	metricAuthzCacheFull   = "authz_cache_full_total"          // sin etiquetas
	metricAuthzAuditFailed = "authz_denied_audit_failed_total" // sin etiquetas
	authzCacheTTL          = 60 * time.Second
	authzCacheMaxEntries   = 50_000
	authzAuditTimeout      = 500 * time.Millisecond
	authzPurgeEvery        = time.Second
	reasonMissingPerm      = "missing_permission"
	reasonImpersonationScp = "impersonation_scope"
)

// noPermissionTenant es la lista fija, en el código, de rutas Tenant que se
// declaran con NoPermission: cualquier usuario con sesión de empresa las ve.
// Agregar una es un PR de contrato.
var noPermissionTenant = map[string]bool{"GET /v1/me/entitlements": true}

// impersonationExcluded son los recursos (y el código suelto) que ninguna
// impersonada alcanza, con cualquier alcance (ADR 0001, «Impersonación de soporte»).
var impersonationExcluded = []string{
	"identidad.support_access:", "identidad.users:", "identidad.roles:", "identidad.sessions:", "empresas.einvoicing:update",
}

// impersonationSet devuelve los permisos del alcance, siempre sin
// impersonationExcluded: read_only solo la acción read (ni read_all ni export);
// full todas las acciones. Un alcance desconocido no tiene conjunto.
func impersonationSet(scope ImpersonationScope) map[permission.Code]struct{} {
	if scope != ScopeReadOnly && scope != ScopeFull {
		return nil
	}
	set := map[permission.Code]struct{}{}
	for _, c := range permission.All() {
		if excludedFromImpersonation(c) { // la exclusión vale en todo alcance (ADR 0001, resources.md)
			continue
		}
		if scope == ScopeReadOnly && !strings.HasSuffix(string(c), ":read") {
			continue
		}
		set[c] = struct{}{}
	}
	return set
}

func excludedFromImpersonation(c permission.Code) bool {
	for _, x := range impersonationExcluded {
		if strings.HasPrefix(string(c), x) {
			return true
		}
	}
	return false
}

// authzCache guarda el Authz por (membresía, versión) con TTL. Es por réplica;
// la clave es la membresía y no el tenant, así nunca se comparte entre las
// membresías de un mismo usuario.
type authzCache struct {
	clk     clock.Clock
	mu      sync.Mutex
	entries map[authzKey]authzEntry
	purged  time.Time
}

type authzKey struct {
	membership [16]byte
	version    int64
}

type authzEntry struct {
	az      Authz
	expires time.Time
}

func newAuthzCache(clk clock.Clock) *authzCache {
	return &authzCache{clk: clk, entries: map[authzKey]authzEntry{}}
}

func (c *authzCache) get(k authzKey) (Authz, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[k]
	if !ok {
		return Authz{}, false
	}
	if !c.clk.Now().Before(e.expires) {
		delete(c.entries, k)
		return Authz{}, false
	}
	return e.az, true
}

// put guarda az y devuelve false si la caché está llena (no cachea).
func (c *authzCache) put(k authzKey, az Authz) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clk.Now()
	if _, exists := c.entries[k]; !exists && len(c.entries) >= authzCacheMaxEntries {
		if now.Sub(c.purged) >= authzPurgeEvery { // barrer los vencidos, a lo sumo una vez por segundo
			c.purged = now
			for key, e := range c.entries {
				if !now.Before(e.expires) {
					delete(c.entries, key)
				}
			}
		}
		if len(c.entries) >= authzCacheMaxEntries {
			return false
		}
	}
	c.entries[k] = authzEntry{az: az, expires: now.Add(authzCacheTTL)}
	return true
}

// lookupAuthz devuelve el Authz de la membresía de s: de la caché si hay una
// entrada para la versión actual, y si no lo resuelve (cached=false). Una entrada
// de otra versión nunca se usa. Quien lo resuelve lo guarda con remember, una
// vez que la versión quedó ligada a la sesión.
func (r *Routes) lookupAuthz(ctx context.Context, s Session, rec metrics.Recorder) (az Authz, cached bool, err error) {
	key := authzKey{membership: s.MembershipID.Bytes, version: s.CurrentAuthzVersion.Int64}
	if az, ok := r.cache.get(key); ok {
		rec.Inc(metricAuthzCache, "result", "hit")
		return az, true, nil
	}
	rec.Inc(metricAuthzCache, "result", "miss")
	az, err = r.perms.Resolve(ctx, s)
	return az, false, err
}

// remember guarda az con la versión que leyó; si la caché está llena no lo
// guarda y lo cuenta.
func (r *Routes) remember(s Session, az Authz, rec metrics.Recorder) {
	if !r.cache.put(authzKey{membership: s.MembershipID.Bytes, version: az.Version}, az) {
		rec.Inc(metricAuthzCacheFull)
	}
}

// authzStep es el paso de permisos de una ruta Tenant: versión de la membresía,
// 2FA del rol y x-permission. Lo monta Routes.Handle después de los
// entitlements y antes de los pasos de la ruta y de la idempotencia; ningún
// módulo lo quita ni lo mueve.
func (r *Routes) authzStep(pattern string, spec Spec) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, hr *http.Request) {
			s, ok := FromContext(hr.Context())
			if !ok || !s.HasTenant() {
				httpx.Logger(hr.Context()).ErrorContext(hr.Context(), "authz step without a tenant session", "route", httpx.Route(hr))
				httpx.WriteProblem(w, hr, "internal", "")
				return
			}
			rec := httpx.Metrics(hr.Context())
			var scope siteScope
			if s.Actor.Kind == db.KindImpersonated {
				if !r.impersonatedAllowed(s, spec) {
					r.deny(w, hr, s, pattern, spec, reasonImpersonationScp)
					return
				}
			} else {
				var allowed bool
				if scope, allowed = r.memberAllowed(w, hr, s, pattern, spec); !allowed {
					return
				}
			}
			rec.Inc(metricAuthzDecision, "outcome", "allowed")
			next.ServeHTTP(w, hr.WithContext(withSiteScope(hr.Context(), scope)))
		})
	}
}

// impersonatedAllowed decide una sesión impersonada: solo el alcance del grant
// (nunca un rol del tenant). Con alcance desconocido niega toda ruta con permiso.
func (r *Routes) impersonatedAllowed(s Session, spec Spec) bool {
	if spec.Permission == NoPermission {
		return true
	}
	_, ok := r.impersonation[s.ImpersonationScope][permission.Code(spec.Permission)]
	return ok
}

// memberAllowed decide una sesión de usuario; si no pasa, ya respondió.
func (r *Routes) memberAllowed(w http.ResponseWriter, hr *http.Request, s Session, pattern string, spec Spec) (siteScope, bool) {
	if emptyUUID(s.MembershipID) || !s.CurrentAuthzVersion.Valid {
		// El paso de sesión ya desliga estos casos: llegar aquí es un bug, y no se deja pasar.
		httpx.Logger(hr.Context()).ErrorContext(hr.Context(), "authz step with a session without a live membership", "route", httpx.Route(hr))
		r.outcome(hr, "tenant_access_denied")
		httpx.WriteProblem(w, hr, "tenant_access_denied", "the company is not available")
		return siteScope{}, false
	}
	ctx, cancel := context.WithTimeout(httpx.WithPhase(hr.Context(), httpx.PhaseAuthz), r.timeout)
	defer cancel()
	lr := hr.WithContext(ctx)
	rec := httpx.Metrics(hr.Context())
	az, cached, err := r.lookupAuthz(ctx, s, rec)
	if err != nil {
		r.authzFailed(w, hr, lr, err)
		return siteScope{}, false
	}
	if !s.SessionAuthzVersion.Valid || s.SessionAuthzVersion.Int64 != az.Version {
		bound, err := r.resolver.BindVersion(ctx, s.ID, s.MembershipID, az.Version)
		if err != nil {
			r.authzFailed(w, hr, lr, err)
			return siteScope{}, false
		}
		if !bound {
			// Otra petición desligó la sesión entre la búsqueda y este UPDATE (ADR 0011).
			r.outcome(hr, "tenant_access_denied")
			httpx.WriteProblem(w, hr, "tenant_access_denied", "membership is no longer active")
			return siteScope{}, false
		}
	}
	if !cached {
		r.remember(s, az, rec)
	}
	if az.RequiresMFA && s.MFAMethod == "none" {
		r.requireMFA(w, hr, s)
		return siteScope{}, false
	}
	if spec.Permission != NoPermission {
		if _, ok := az.Permissions[permission.Code(spec.Permission)]; !ok {
			r.deny(w, hr, s, pattern, spec, reasonMissingPerm)
			return siteScope{}, false
		}
	}
	return siteScope{all: az.AllSites, sites: az.Sites}, true
}

func (r *Routes) outcome(hr *http.Request, outcome string) {
	httpx.Metrics(hr.Context()).Inc(metricAuthzDecision, "outcome", outcome)
}

// requireMFA revoca la sesión de tenant y responde 403 mfa_required. Si la
// revocación falla responde igual: el rechazo no depende de ella.
func (r *Routes) requireMFA(w http.ResponseWriter, hr *http.Request, s Session) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(hr.Context()), r.timeout)
	defer cancel()
	if err := r.resolver.Revoke(ctx, s.ID); err != nil {
		httpx.Logger(hr.Context()).WarnContext(hr.Context(), "session revoke for mfa_required failed",
			"route", httpx.Route(hr), "phase", httpx.PhaseAuthz, "error_type", fmt.Sprintf("%T", err))
	}
	r.outcome(hr, "mfa_required")
	httpx.WriteProblem(w, hr, "mfa_required", "the role requires two-factor authentication")
}

// deny registra el intento denegado y responde 403 forbidden con el permiso que
// falta. Si la auditoría falla, lo registra en el log y responde 403 igual.
func (r *Routes) deny(w http.ResponseWriter, hr *http.Request, s Session, pattern string, spec Spec, reason string) {
	// Sin la cancelación de la petición: un cliente que se va no borra el registro.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(hr.Context()), authzAuditTimeout)
	defer cancel()
	err := r.denials.RecordDenied(ctx, s.Actor, audit.Event{
		Action: spec.Permission,
		Diff:   map[string]string{"route": pattern, "reason": reason},
	})
	if err != nil {
		httpx.Metrics(hr.Context()).Inc(metricAuthzAuditFailed)
		httpx.Logger(hr.Context()).ErrorContext(hr.Context(), "denied attempt could not be audited",
			"route", httpx.Route(hr), "permission", spec.Permission, "reason", reason, "error_type", fmt.Sprintf("%T", err))
	}
	r.outcome(hr, "forbidden")
	httpx.WriteProblemExt(w, hr, "forbidden", "", httpx.Extensions{Permission: spec.Permission})
}

// authzFailed responde cuando no se pudieron resolver los permisos: la petición
// nunca pasa sin ellos.
func (r *Routes) authzFailed(w http.ResponseWriter, hr, lr *http.Request, err error) {
	if errors.Is(err, ErrMembershipLost) {
		r.outcome(hr, "tenant_access_denied")
		httpx.WriteProblem(w, hr, "tenant_access_denied", "membership is no longer active")
		return
	}
	r.outcome(hr, "error")
	switch {
	case httpx.WriteDBError(w, lr, err):
	case hr.Context().Err() != nil:
		// El cliente se fue o venció el plazo del handler: responde otro o nadie.
	case isOwnTimeout(err):
		httpx.LogDBFailure(lr, "authz resolution timed out", metrics.DBUnavailable, httpx.PhaseAuthz, "")
		httpx.WriteProblem(w, hr, "dependency_unavailable", "a dependency is not responding")
	default:
		httpx.Logger(hr.Context()).ErrorContext(hr.Context(), "authz resolution failed with an unexpected error",
			"route", httpx.Route(hr), "error_type", fmt.Sprintf("%T", err))
		httpx.WriteProblem(w, hr, "internal", "")
	}
}

// clientIP es la ip del cliente para la auditoría. Hoy es vacía: la API solo la
// llama el BFF por la red privada (ADR 0007) y RemoteAddr sería el del BFF, así
// que guardarlo sería inventar el dato. Cuando el contrato fije una cabecera de
// confianza del BFF (conventions §3), se lee aquí.
func clientIP(*http.Request) netip.Addr { return netip.Addr{} }

// siteScope es el alcance de sedes de la membresía; CheckSite lo lee.
type siteScope struct {
	all   bool
	sites []pgtype.UUID
}

type siteScopeKey struct{}

func withSiteScope(ctx context.Context, s siteScope) context.Context {
	return context.WithValue(ctx, siteScopeKey{}, s)
}

// CheckSite lo llama el handler, que es quien conoce la sede del recurso. Pasa
// si es la sede de la sesión, si la membresía tiene todas las sedes o si está
// entre las suyas; en cualquier otro caso, con datos ausentes también, devuelve
// el error site_out_of_scope.
func CheckSite(ctx context.Context, siteID pgtype.UUID) error {
	denied := httpx.NewError("site_out_of_scope", "the resource belongs to a site outside the allowed scope")
	s, ok := FromContext(ctx)
	if !ok || emptyUUID(siteID) {
		return denied
	}
	if !emptyUUID(s.SiteID) && s.SiteID == siteID {
		return nil
	}
	sc, _ := ctx.Value(siteScopeKey{}).(siteScope)
	if sc.all {
		return nil
	}
	for _, id := range sc.sites {
		if id == siteID {
			return nil
		}
	}
	return denied
}
