package pgsession_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/idempotency"
)

// unbindBeforeBind desliga la sesión justo antes del UPDATE de la versión: la
// otra petición del criterio 4 ganó la carrera.
type unbindBeforeBind struct{ auth.SessionResolver }

func (u unbindBeforeBind) BindVersion(ctx context.Context, sid, mid pgtype.UUID, v int64) (bool, error) {
	if _, err := u.SessionResolver.Detach(ctx, sid, mid); err != nil {
		return false, err
	}
	return u.SessionResolver.BindVersion(ctx, sid, mid, v)
}

// Criterio 4 (integración): entre la búsqueda y el UPDATE otra petición desligó
// la sesión -> 0 filas, 403 tenant_access_denied, el handler no corre y la sesión
// queda sin tenant ni versión.
func TestUnboundInParallelBeforeTheVersionUpdateIs403(t *testing.T) {
	e := newEnv(t, envConfig{wrap: func(r auth.SessionResolver) auth.SessionResolver { return unbindBeforeBind{r} }})
	s := e.seed(dbtest.SessionSpec{SessionVersion: 1, MembershipVersion: 2}) // distinta: obliga al UPDATE
	res := e.get("/v1/items", s.Token)
	if res.status != 403 || res.problemCode(t) != "tenant_access_denied" {
		t.Fatalf("status = %d: %s", res.status, res.body)
	}
	if e.ran.Load() != 0 {
		t.Fatal("the handler ran on a session unbound in parallel")
	}
	wantDetached(t, e.row(s.SessionID))
}

// La versión nueva se guarda en la sesión antes de decidir, y se cumple la
// invariante: una sesión ligada nunca queda sin versión.
func TestStaleSessionVersionIsRewrittenBeforeDeciding(t *testing.T) {
	e := newEnv(t, envConfig{})
	s := e.seed(dbtest.SessionSpec{SessionVersion: 1, MembershipVersion: 4})
	if res := e.get("/v1/items", s.Token); res.status != 200 {
		t.Fatalf("status = %d: %s", res.status, res.body)
	}
	if r := e.row(s.SessionID); r.version != "4" || r.membership == "<null>" {
		t.Fatalf("row = %+v, want authz_version 4 and still bound", r)
	}
}

// 2FA exigida por el rol: la sesión de tenant queda revocada en la base.
func TestRoleRequiringMFARevokesTheSessionInTheDatabase(t *testing.T) {
	e := newEnv(t, envConfig{perms: mfaRole{}})
	s := e.seed(dbtest.SessionSpec{}) // mfa_method none
	res := e.get("/v1/items", s.Token)
	if res.status != 403 || res.problemCode(t) != "mfa_required" {
		t.Fatalf("status = %d: %s", res.status, res.body)
	}
	if !e.row(s.SessionID).revoked {
		t.Fatal("the session was not revoked")
	}
	e.get("/v1/items", s.Token).wantUnauthorized(t)
	if e.ran.Load() != 0 {
		t.Fatal("the handler ran without the required MFA")
	}
}

type mfaRole struct{}

func (mfaRole) Resolve(ctx context.Context, s auth.Session) (auth.Authz, error) {
	az, err := allowAll{}.Resolve(ctx, s)
	az.RequiresMFA = true
	return az, err
}

// Criterio 1 (integración): la denegación queda en audit.events con result
// denegado, el actor real y el diff intacto tras la ida y vuelta por JSON, y la
// idempotencia (que va después) no llega a correr.
func TestDeniedAttemptIsAuditedAndNeverReachesIdempotencyOrTheHandler(t *testing.T) {
	var handler atomic.Int64
	e := newEnv(t, envConfig{perms: auth.DenyAllPermissions{}, mount: func(r *auth.Routes, pool *db.Pool) {
		idem := idempotency.New(pool, idempotency.Config{})
		r.HandleFunc("POST /v1/orders", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:create"},
			func(w http.ResponseWriter, _ *http.Request) { handler.Add(1) }, idem.Required())
	}})
	s := e.seed(dbtest.SessionSpec{})
	req, _ := http.NewRequest(http.MethodPost, e.srv.URL+"/v1/orders", bytes.NewReader([]byte(`{"a":1}`)))
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "k-176")
	res := e.do(req)
	if res.status != 403 || res.problemCode(t) != "forbidden" {
		t.Fatalf("status = %d: %s", res.status, res.body)
	}
	var p struct{ Permission string }
	if err := json.Unmarshal(res.body, &p); err != nil || p.Permission != "ventas.pos:create" {
		t.Fatalf("permission = %q (%v)", p.Permission, err)
	}
	if handler.Load() != 0 {
		t.Fatal("the handler ran")
	}
	ctx := context.Background()
	admin := e.d.Admin(ctx, t)
	var keys int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM ops.idempotency_keys`).Scan(&keys); err != nil || keys != 0 {
		t.Fatalf("idempotency rows = %d (%v)", keys, err)
	}
	var tenant, actorType, user, action, result string
	var diff []byte
	err := admin.QueryRow(ctx, `SELECT tenant_id::text, actor_type, actor_user_id::text, action, result, diff FROM audit.events`).
		Scan(&tenant, &actorType, &user, &action, &result, &diff)
	if err != nil {
		t.Fatalf("audit.events: %v", err)
	}
	if tenant != s.TenantID || actorType != "user" || user != s.UserID || action != "ventas.pos:create" || result != "denegado" {
		t.Fatalf("event = %s %s %s %s %s", tenant, actorType, user, action, result)
	}
	var got map[string]string
	if err := json.Unmarshal(diff, &got); err != nil || got["route"] != "POST /v1/orders" || got["reason"] != "missing_permission" || len(got) != 2 {
		t.Fatalf("diff = %s (%v)", diff, err)
	}
}

// Una denegación a la impersonada deja el actor de plataforma y su impersonador.
func TestImpersonatedDenialIsAuditedAsPlatformStaff(t *testing.T) {
	e := newEnv(t, envConfig{wrap: func(r auth.SessionResolver) auth.SessionResolver { return withScope{r, auth.ScopeReadOnly} }, mount: func(r *auth.Routes, _ *db.Pool) {
		r.HandleFunc("POST /v1/orders", auth.Tenant, auth.Spec{Module: "ventas", Permission: "ventas.pos:create"}, func(http.ResponseWriter, *http.Request) {})
	}})
	s := e.seed(dbtest.SessionSpec{Kind: dbtest.SessionImpersonated})
	if res := e.request(http.MethodPost, "/v1/orders", s.Token); res.status != 403 || res.problemCode(t) != "forbidden" {
		t.Fatalf("status = %d: %s", res.status, res.body)
	}
	ctx := context.Background()
	var actorType, impersonator string
	var user *string
	err := e.d.Admin(ctx, t).QueryRow(ctx, `SELECT actor_type, actor_user_id::text, impersonator_id::text FROM audit.events WHERE result = 'denegado'`).
		Scan(&actorType, &user, &impersonator)
	if err != nil {
		t.Fatal(err)
	}
	if actorType != "platform_staff" || user != nil || impersonator != s.ImpersonatorID {
		t.Fatalf("event = %s %v %s", actorType, user, impersonator)
	}
}

// Criterio 9: un usuario de la empresa A que pide un recurso de la empresa B
// recibe 404 not_found (RLS da 0 filas), con un handler de ejemplo detrás del
// middleware de permisos.
func TestResourceOfAnotherTenantIs404(t *testing.T) {
	e := newEnv(t, envConfig{mount: func(r *auth.Routes, pool *db.Pool) {
		r.HandleFunc("GET /v1/roles/{id}", auth.Tenant, auth.Spec{Module: "ventas", Permission: "identidad.roles:read"}, func(w http.ResponseWriter, req *http.Request) {
			s, _ := auth.FromContext(req.Context())
			var name string
			err := pool.InTenantReadTx(req.Context(), s.Actor, func(tx pgx.Tx) error {
				err := tx.QueryRow(req.Context(), `SELECT name FROM iam.roles WHERE id = $1`, req.PathValue("id")).Scan(&name)
				if err == pgx.ErrNoRows {
					return httpx.NewError("not_found", "the resource does not exist")
				}
				return err
			})
			if err != nil {
				httpx.WriteError(w, req, err)
			}
		})
	}})
	a := e.seed(dbtest.SessionSpec{})
	b := e.seed(dbtest.SessionSpec{})
	ctx := context.Background()
	var ownRole, otherRole string
	admin := e.d.Admin(ctx, t)
	if err := admin.QueryRow(ctx, `SELECT id::text FROM iam.roles WHERE tenant_id = $1`, a.TenantID).Scan(&ownRole); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(ctx, `SELECT id::text FROM iam.roles WHERE tenant_id = $1`, b.TenantID).Scan(&otherRole); err != nil {
		t.Fatal(err)
	}
	if res := e.get("/v1/roles/"+ownRole, a.Token); res.status != 200 {
		t.Fatalf("own resource = %d: %s", res.status, res.body)
	}
	if res := e.get("/v1/roles/"+otherRole, a.Token); res.status != 404 || res.problemCode(t) != "not_found" {
		t.Fatalf("another tenant's resource = %d: %s", res.status, res.body)
	}
}
