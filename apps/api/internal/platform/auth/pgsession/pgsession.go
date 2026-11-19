// Package pgsession implementa auth.SessionResolver sobre PostgreSQL (ADR 0004,
// ADR 0011): la búsqueda de sesión con iam.session_lookup y el desligue de la
// membresía perdida, más las dos escrituras del paso de permisos (#176): guardar
// la versión resuelta (BindVersion) y revocar por 2FA (Revoke).
//
// Son las únicas puertas a la base de la sesión: Lookup corre en
// db.InNoTenantReadTx y Detach, BindVersion y Revoke en db.InNoTenantTx. No hay otra consulta ni caché:
// la búsqueda se hace en cada petición, para que revocar o suspender surta efecto
// en la siguiente.
package pgsession

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

// lookupSQL nombra las columnas en el orden de scanTargets. Si iam.session_lookup
// cambia de forma, TestLookupColumnsMatchTheFunction falla antes que esta consulta.
const lookupSQL = `SELECT session_id, user_id, tenant_id, site_id, cash_register_id, membership_id,
	session_authz_version, mfa_satisfied, mfa_method, impersonator_id, current_authz_version, membership_status
	FROM iam.session_lookup($1)`

// detachSQL es el UPDATE condicional del ADR 0011: conserva tokens, familia de
// refresh y 2FA, y deja la fila en el único estado desligado que admiten los CHECK.
const detachSQL = `UPDATE iam.sessions
	SET tenant_id = NULL, membership_id = NULL, site_id = NULL, cash_register_id = NULL, authz_version = NULL
	WHERE id = $1 AND membership_id = $2 AND impersonator_id IS NULL AND revoked_at IS NULL`

// Resolver es el auth.SessionResolver de PostgreSQL.
type Resolver struct{ pool *db.Pool }

var _ auth.SessionResolver = (*Resolver)(nil)

// New devuelve el resolver sobre pool, que debe ser el de wanka_app. Devuelve el
// puerto y no *Resolver para que el cableado de cmd pase al kernel un
// auth.SessionResolver, sin que go-arch-lint vea a platform dependiendo de este
// paquete (el análisis de inyección sigue el tipo concreto).
func New(pool *db.Pool) auth.SessionResolver { return &Resolver{pool: pool} }

// scanTargets devuelve los destinos del SELECT de lookupSQL, en su orden.
func scanTargets(r *auth.SessionRecord) []any {
	return []any{&r.SessionID, &r.UserID, &r.TenantID, &r.SiteID, &r.CashRegisterID, &r.MembershipID,
		&r.SessionAuthzVersion, &r.MFASatisfied, &r.MFAMethod, &r.ImpersonatorID, &r.CurrentAuthzVersion, &r.MembershipStatus}
}

// Lookup busca la sesión por el sha256 del token. Sin fila (inexistente,
// vencida, revocada o a medio login: la función no las distingue) devuelve
// found=false. Los errores de la base se devuelven con su tipo (db.ErrOverloaded,
// db.ErrUnavailable, db.ErrStatementTimeout o el del contexto) para que la capa
// HTTP los responda.
func (r *Resolver) Lookup(ctx context.Context, accessHash [32]byte) (auth.SessionRecord, bool, error) {
	var rec auth.SessionRecord
	found := true
	err := r.pool.InNoTenantReadTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, lookupSQL, accessHash[:]).Scan(scanTargets(&rec)...)
		if errors.Is(err, pgx.ErrNoRows) {
			found = false
			return nil
		}
		return err
	})
	if err != nil {
		return auth.SessionRecord{}, false, fmt.Errorf("looking up session: %w", err)
	}
	if !found {
		return auth.SessionRecord{}, false, nil
	}
	return rec, true, nil
}

// Detach desliga la sesión de su empresa sin revocarla. Es idempotente: con
// dos peticiones concurrentes, una actualiza 1 fila y la otra 0.
func (r *Resolver) Detach(ctx context.Context, sessionID, membershipID pgtype.UUID) (bool, error) {
	var detached bool
	err := r.pool.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, detachSQL, sessionID, membershipID)
		detached = err == nil && tag.RowsAffected() == 1
		return err
	})
	if err != nil {
		return false, fmt.Errorf("detaching session: %w", err)
	}
	return detached, nil
}

// bindSQL guarda la versión resuelta solo si la sesión sigue ligada a esa
// membresía y viva: 0 filas significa únicamente «desligada» o revocada, por eso
// no lleva condición sobre la versión.
const bindSQL = `UPDATE iam.sessions SET authz_version = $3
	WHERE id = $1 AND membership_id = $2 AND revoked_at IS NULL`

// revokeSQL revoca la sesión; el instante lo pone la base. Revocar otra vez es
// un no-op (el trigger conserva el instante viejo).
const revokeSQL = `UPDATE iam.sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`

// BindVersion guarda en la sesión la versión de permisos resuelta. bound=false
// con err=nil es que otra petición la desligó (ADR 0011) o está revocada.
func (r *Resolver) BindVersion(ctx context.Context, sessionID, membershipID pgtype.UUID, version int64) (bool, error) {
	var bound bool
	err := r.pool.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, bindSQL, sessionID, membershipID, version)
		bound = err == nil && tag.RowsAffected() == 1
		return err
	})
	if err != nil {
		return false, fmt.Errorf("binding session authz version: %w", err)
	}
	return bound, nil
}

// Revoke revoca la sesión. Es idempotente.
func (r *Resolver) Revoke(ctx context.Context, sessionID pgtype.UUID) error {
	err := r.pool.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, revokeSQL, sessionID)
		return err
	})
	if err != nil {
		return fmt.Errorf("revoking session: %w", err)
	}
	return nil
}
