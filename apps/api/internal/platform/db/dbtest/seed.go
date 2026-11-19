package dbtest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"
)

// SessionKind es el estado de la sesión que siembra SeedSession.
type SessionKind int

// Estados de sesión. El valor cero es SessionBound.
const (
	// SessionBound está ligada a la empresa y a su membresía, con sede y caja.
	SessionBound SessionKind = iota
	// SessionNoTenant ya pasó el login pero no eligió empresa.
	SessionNoTenant
	// SessionHalfLogin falta el segundo factor: mfa_satisfied = false.
	SessionHalfLogin
	// SessionExpired está ligada y su access_expires_at ya pasó.
	SessionExpired
	// SessionRevoked está ligada y revocada.
	SessionRevoked
	// SessionImpersonated es de staff de plataforma: mfa_method = 'platform', sin user_id, membresía, versión ni refresh, y vence a los 60 min.
	SessionImpersonated
)

// SessionSpec elige lo que SeedSession siembra. El valor cero es una sesión
// ligada, con membresía activa y versión 1 en ambas.
type SessionSpec struct {
	Kind SessionKind
	// MembershipStatus es 'invitado', 'activo' (por defecto) o 'suspendido'.
	MembershipStatus string
	// MembershipVersion es la authz_version de la membresía (por defecto 1).
	MembershipVersion int64
	// SessionVersion es la authz_version guardada en una sesión ligada (por defecto 1).
	SessionVersion int64
}

// Seeded son el token y los ids que devuelve SeedSession.
type Seeded struct {
	// Token es el token de acceso en claro: wk_at_ y 43 caracteres base64url.
	// La base guarda solo su sha256.
	Token                                  string
	TenantID, UserID, RoleID               string
	MembershipID, SessionID                string
	SiteID, CashRegisterID, ImpersonatorID string
}

// SeedSession crea una empresa, un usuario, un rol, una membresía y una sesión
// del estado pedido, como superusuario (que no pasa por RLS), en una base ya
// migrada. Cada llamada crea su propia empresa, así que puede repetirse. Solo
// existe en código de prueba. Los ids de sede y caja son uuid sin fila: iam.sessions
// no tiene FK a org.sites ni a org.cash_registers.
func (d *DB) SeedSession(t testing.TB, spec SessionSpec) Seeded {
	t.Helper()
	if spec.MembershipStatus == "" {
		spec.MembershipStatus = "activo"
	}
	if spec.MembershipVersion == 0 {
		spec.MembershipVersion = 1
	}
	if spec.SessionVersion == 0 {
		spec.SessionVersion = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatal(err)
	}
	token := "wk_at_" + base64.RawURLEncoding.EncodeToString(raw[:])
	hash := sha256.Sum256([]byte(token))

	conn := d.Admin(ctx, t)
	var s Seeded
	s.Token = token
	err := conn.QueryRow(ctx, `
		WITH t AS (INSERT INTO org.tenants (id) VALUES (gen_random_uuid()) RETURNING id),
		     u AS (INSERT INTO iam.users (id) VALUES (gen_random_uuid()) RETURNING id),
		     r AS (INSERT INTO iam.roles (id, tenant_id, name) SELECT gen_random_uuid(), t.id, 'Rol' FROM t RETURNING id),
		     m AS (INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status, authz_version)
		           SELECT gen_random_uuid(), t.id, u.id, r.id, $1, $2 FROM t, u, r RETURNING id)
		SELECT t.id::text, u.id::text, r.id::text, m.id::text FROM t, u, r, m`,
		spec.MembershipStatus, spec.MembershipVersion).Scan(&s.TenantID, &s.UserID, &s.RoleID, &s.MembershipID)
	if err != nil {
		t.Fatalf("seeding the tenant, user, role and membership: %v", err)
	}

	var (
		userID                                       any = s.UserID
		tenant, membership, site, cash, impersonator any // nil → NULL
		version                                      any
		mfaSatisfied                                 = true
		mfaMethod                                    = "none"
		expires                                      = "1 hour"
		revoked                                      = false
	)
	switch spec.Kind {
	case SessionBound, SessionExpired, SessionRevoked:
		s.SiteID, s.CashRegisterID = newUUID(t), newUUID(t)
		tenant, membership, site, cash, version = s.TenantID, s.MembershipID, s.SiteID, s.CashRegisterID, spec.SessionVersion
		if spec.Kind == SessionExpired {
			expires = "-1 minute"
		}
		revoked = spec.Kind == SessionRevoked
	case SessionNoTenant:
	case SessionHalfLogin:
		mfaSatisfied = false
	case SessionImpersonated:
		// El staff vive en platform.platform_users (ola 6): la sesión no tiene user_id y el staff va en impersonator_id.
		s.ImpersonatorID, s.UserID = newUUID(t), ""
		userID, impersonator, tenant, mfaMethod = nil, s.ImpersonatorID, s.TenantID, "platform"
		expires = "60 minutes" // sin refresh y con vigencia máxima de 60 min (CHECK de iam.sessions)
	default:
		t.Fatalf("unknown session kind %d", spec.Kind)
	}
	err = conn.QueryRow(ctx, `
		INSERT INTO iam.sessions (id, user_id, tenant_id, site_id, cash_register_id, membership_id, access_hash, access_expires_at,
		       authz_version, mfa_satisfied, mfa_method, impersonator_id, revoked_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, now() + $7::interval, $8, $9, $10, $11, CASE WHEN $12 THEN now() END)
		RETURNING id::text`,
		userID, tenant, site, cash, membership, hash[:], expires, version, mfaSatisfied, mfaMethod, impersonator, revoked).Scan(&s.SessionID)
	if err != nil {
		t.Fatalf("seeding the session: %v", err)
	}
	return s
}

func newUUID(t testing.TB) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	b[6] = b[6]&0x0f | 0x40 // versión 4
	b[8] = b[8]&0x3f | 0x80 // variante RFC 4122
	h := make([]byte, 0, 36)
	const hex = "0123456789abcdef"
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			h = append(h, '-')
		}
		h = append(h, hex[v>>4], hex[v&0x0f])
	}
	return string(h)
}
