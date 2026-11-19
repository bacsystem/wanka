package auth

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

// MembershipActive es el estado de una membresía que da acceso a su empresa.
const MembershipActive = "activo"

// SessionRecord es la fila que devuelve iam.session_lookup (ADR 0004), tal cual.
type SessionRecord struct {
	SessionID, UserID, TenantID, SiteID, CashRegisterID, MembershipID, ImpersonatorID pgtype.UUID
	// SessionAuthzVersion es la versión guardada en la sesión; CurrentAuthzVersion
	// y MembershipStatus son los de la membresía hoy (nulos si ya no existe).
	SessionAuthzVersion, CurrentAuthzVersion pgtype.Int8
	MembershipStatus                         pgtype.Text
	MFASatisfied                             bool
	MFAMethod                                string
}

// SessionResolver es el puerto de la sesión. El adaptador de PostgreSQL lo
// implementa en la historia #196; el middleware se prueba con un doble.
type SessionResolver interface {
	// Lookup busca la sesión por sha256 del token completo. found=false con
	// err=nil es «sin fila» (401 session_expired). err!=nil es infraestructura
	// (503: db.ErrOverloaded, db.ErrUnavailable, db.ErrStatementTimeout o el
	// vencimiento de ctx) o un bug (500).
	//
	// Contrato: nunca devuelve una sesión revocada, vencida o a medio login. Una
	// impersonada cuyo grant se revocó o venció está revocada o vencida en la base
	// (la revocación escribe revoked_at en la misma transacción, data-model §6),
	// así que tampoco se devuelve; el middleware no consulta el grant y acepta
	// toda impersonada que llegue.
	Lookup(ctx context.Context, accessHash [32]byte) (rec SessionRecord, found bool, err error)
	// Detach desliga la sesión de su empresa sin revocarla (ADR 0011): UPDATE
	// condicionado a id y membership_id, idempotente. detached=false con err=nil
	// es que no actualizó filas.
	Detach(ctx context.Context, sessionID, membershipID pgtype.UUID) (detached bool, err error)
}

// Session es la sesión autenticada que el handler lee con FromContext. Sus
// valores salen solo de la base, nunca de la petición.
type Session struct {
	ID                                       pgtype.UUID
	Actor                                    db.Actor
	SiteID, CashRegisterID, MembershipID     pgtype.UUID
	SessionAuthzVersion, CurrentAuthzVersion pgtype.Int8
	MFAMethod                                string
}

// HasTenant dice si la sesión está ligada a una empresa.
func (s Session) HasTenant() bool { return !emptyUUID(s.Actor.TenantID) }

func emptyUUID(u pgtype.UUID) bool { return !u.Valid || u.Bytes == [16]byte{} }

func (r SessionRecord) impersonated() bool { return !emptyUUID(r.ImpersonatorID) }

// coherent comprueba que la fila tenga una forma posible: sesión; usuario si no
// es impersonada (y vacío si lo es); y tenant si y solo si hay membresía (o
// impersonador, que no tiene membresía pero sí tenant). Una fila que no la cumple es un bug del resolver.
func (r SessionRecord) coherent() bool {
	if emptyUUID(r.SessionID) {
		return false
	}
	hasTenant := !emptyUUID(r.TenantID)
	if r.impersonated() {
		// El staff no está en iam.users: UserID viene vacío y el actor es el impersonador.
		return hasTenant && emptyUUID(r.UserID)
	}
	if emptyUUID(r.UserID) {
		return false
	}
	return hasTenant == !emptyUUID(r.MembershipID)
}

// lostMembership dice si una sesión no impersonada ligada a una membresía ya no
// tiene derecho a ella: se borró (sin versión actual) o no está activa (ADR 0011).
func (r SessionRecord) lostMembership() bool {
	if r.impersonated() || emptyUUID(r.MembershipID) {
		return false
	}
	return !r.CurrentAuthzVersion.Valid || !r.MembershipStatus.Valid || r.MembershipStatus.String != MembershipActive
}

// session arma la Session; detached la deja sin ninguna columna del tenant, el
// mismo estado al que el UPDATE de desligar deja la fila.
func (r SessionRecord) session(detached bool) Session {
	s := Session{ID: r.SessionID, SiteID: r.SiteID, CashRegisterID: r.CashRegisterID, MembershipID: r.MembershipID,
		SessionAuthzVersion: r.SessionAuthzVersion, CurrentAuthzVersion: r.CurrentAuthzVersion, MFAMethod: r.MFAMethod,
		Actor: db.Actor{TenantID: r.TenantID, UserID: r.UserID, Kind: db.KindUser}}
	if r.impersonated() {
		// El user_id del actor es el del staff, como el app.user_id que fija el canje.
		s.Actor.Kind, s.Actor.UserID, s.Actor.ImpersonatorID = db.KindImpersonated, r.ImpersonatorID, r.ImpersonatorID
	}
	if detached {
		s.Actor.TenantID = pgtype.UUID{}
		s.SiteID, s.CashRegisterID, s.MembershipID = pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}
		s.SessionAuthzVersion, s.CurrentAuthzVersion = pgtype.Int8{}, pgtype.Int8{}
	}
	return s
}

type ctxKey struct{}

// WithSession deja la sesión autenticada en el contexto de la petición.
func WithSession(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, ctxKey{}, s)
}

// FromContext devuelve la sesión que fijó el middleware.
func FromContext(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(ctxKey{}).(Session)
	return s, ok
}
