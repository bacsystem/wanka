// Package identidad es la fachada del módulo de identidad (ADR 0002): el
// adaptador de PostgreSQL del puerto auth.PermissionResolver y la siembra de los
// roles base de una empresa. Los demás módulos y cmd solo importan este paquete.
package identidad

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

// resolveSQL lee, en una sola consulta, la membresía activa del usuario en la
// empresa de la sesión con su rol. Un rol base (system) toma los permisos de su
// plantilla y no tiene filas propias; un rol propio, las de iam.role_permissions.
// Las sedes son las de iam.membership_sites (vacías si la membresía tiene todas).
// ARRAY(...) devuelve '{}' y nunca NULL: un rol sin permisos niega todo.
const resolveSQL = `SELECT m.authz_version, m.all_sites, r.requires_mfa, r.system, r.template_code,
	(r.template_code IS NOT NULL AND EXISTS (SELECT 1 FROM iam.role_templates t WHERE t.code = r.template_code)) AS template_found,
	CASE WHEN r.system
	     THEN ARRAY(SELECT tp.permission_code FROM iam.role_template_permissions tp WHERE tp.template_code = r.template_code)
	     ELSE ARRAY(SELECT rp.permission_code FROM iam.role_permissions rp WHERE rp.tenant_id = r.tenant_id AND rp.role_id = r.id)
	END AS permissions,
	ARRAY(SELECT ms.site_id FROM iam.membership_sites ms WHERE ms.tenant_id = m.tenant_id AND ms.membership_id = m.id) AS sites
	FROM iam.memberships m
	JOIN iam.roles r ON r.tenant_id = m.tenant_id AND r.id = m.role_id
	WHERE m.tenant_id = $1 AND m.id = $2 AND m.user_id = $3 AND m.status = 'activo'`

// readRunner es lo que el resolver necesita de db.Pool: una transacción de
// tenant de solo lectura.
type readRunner interface {
	InTenantReadTx(ctx context.Context, a db.Actor, fn func(pgx.Tx) error) error
}

type permissionResolver struct{ tx readRunner }

// NewPermissionResolver devuelve el auth.PermissionResolver de PostgreSQL. pool
// debe ser el de wanka_app. Devuelve el puerto y no el tipo concreto, como
// pgsession.New, para que el cableado de cmd no dependa del adaptador.
func NewPermissionResolver(pool *db.Pool) auth.PermissionResolver {
	return permissionResolver{tx: pool}
}

// Resolve cumple auth.PermissionResolver con una consulta en una transacción de
// tenant de solo lectura y el Actor de la sesión. Falla cerrado: una membresía
// que no existe, no es del usuario ni de la empresa de la sesión, o no está
// activa es auth.ErrMembershipLost; un rol base sin plantilla, o un rol sin
// permisos, devuelve el conjunto vacío y niega todo.
func (p permissionResolver) Resolve(ctx context.Context, s auth.Session) (auth.Authz, error) {
	a := s.Actor
	if a.Kind != db.KindUser || !validUUID(a.TenantID) || !validUUID(a.UserID) || !validUUID(s.MembershipID) {
		// Solo una sesión de usuario con membresía pasa por aquí; cualquier otra no tiene permisos propios.
		return auth.Authz{}, auth.ErrMembershipLost
	}
	var (
		az            auth.Authz
		system, found bool
		template      pgtype.Text
		codes         []string
	)
	lost := false
	err := p.tx.InTenantReadTx(ctx, a, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, resolveSQL, a.TenantID, s.MembershipID, a.UserID).
			Scan(&az.Version, &az.AllSites, &az.RequiresMFA, &system, &template, &found, &codes, &az.Sites)
		if errors.Is(err, pgx.ErrNoRows) {
			lost = true
			return nil
		}
		return err
	})
	if err != nil {
		return auth.Authz{}, fmt.Errorf("resolving permissions: %w", err)
	}
	if lost {
		return auth.Authz{}, auth.ErrMembershipLost
	}
	if system && !found {
		// La FK de iam.roles lo impide; si pasa, niega todo y deja rastro con el código de plantilla.
		httpx.Logger(ctx).ErrorContext(ctx, "system role without its template, denying everything", "template_code", template.String)
		codes = nil
	}
	az.Permissions = make(map[permission.Code]struct{}, len(codes))
	for _, c := range codes {
		az.Permissions[permission.Code(c)] = struct{}{}
	}
	return az, nil
}

func validUUID(u pgtype.UUID) bool { return u.Valid && u.Bytes != [16]byte{} }
