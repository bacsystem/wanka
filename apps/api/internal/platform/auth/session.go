package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

// ErrSessionNotFound es "sin fila": la sesión no existe, venció o se revocó
// (iam.session_lookup no las distingue). El middleware lo responde 401
// session_expired. Cualquier otro error del resolver es un fallo de la base.
var ErrSessionNotFound = errors.New("auth: session not found")

// MembershipActive es el estado de una membresía que da acceso a su empresa.
const MembershipActive = "activo"

// Session es lo que devuelve iam.session_lookup (ADR 0004). Los identificadores
// son pgtype.UUID, como en db.Actor.
type Session struct {
	ID             pgtype.UUID
	UserID         pgtype.UUID
	TenantID       pgtype.UUID
	SiteID         pgtype.UUID
	CashRegisterID pgtype.UUID
	MembershipID   pgtype.UUID
	// AuthzVersion es la versión guardada en la sesión; nil si no hay.
	AuthzVersion   *int64
	MFASatisfied   bool
	MFAMethod      string
	ImpersonatorID pgtype.UUID
	// CurrentAuthzVersion y MembershipStatus son los de la membresía hoy; nil y ""
	// si ya no existe.
	CurrentAuthzVersion *int64
	MembershipStatus    string
}

func emptyUUID(u pgtype.UUID) bool { return !u.Valid || u.Bytes == [16]byte{} }

// Impersonated dice si es una sesión del staff de plataforma dentro de una empresa.
func (s Session) Impersonated() bool { return !emptyUUID(s.ImpersonatorID) }

// HasTenant dice si la sesión está ligada a una empresa.
func (s Session) HasTenant() bool { return !emptyUUID(s.TenantID) }

// LostMembership dice si una sesión no impersonada ligada a una membresía ya no
// tiene derecho a ella: la membresía se borró (sin versión actual) o no está
// activa (ADR 0011). Una impersonada no tiene membresía y nunca la pierde.
func (s Session) LostMembership() bool {
	if s.Impersonated() || emptyUUID(s.MembershipID) {
		return false
	}
	return s.CurrentAuthzVersion == nil || s.MembershipStatus != MembershipActive
}

// WithoutTenant devuelve una copia sin ninguna columna del tenant, el mismo
// estado al que el UPDATE de desligar deja la fila.
func (s Session) WithoutTenant() Session {
	s.TenantID, s.SiteID, s.CashRegisterID, s.MembershipID = pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}
	s.AuthzVersion = nil
	return s
}

// Actor arma el db.Actor de una transacción de tenant. Sale solo de la sesión,
// nunca de la petición.
func (s Session) Actor() db.Actor {
	a := db.Actor{TenantID: s.TenantID, UserID: s.UserID, Kind: db.KindUser}
	if s.Impersonated() {
		a.Kind, a.ImpersonatorID = db.KindImpersonated, s.ImpersonatorID
	}
	return a
}

// SessionResolver es el puerto de la sesión. El adaptador de PostgreSQL lo
// implementa en la historia #196; el middleware se prueba con un doble.
type SessionResolver interface {
	// Lookup busca la sesión por el hash del token. Devuelve ErrSessionNotFound
	// si no hay fila y cualquier otro error si la base falla (db.ErrOverloaded,
	// db.ErrUnavailable, db.ErrStatementTimeout, o el vencimiento de ctx).
	Lookup(ctx context.Context, h TokenHash) (Session, error)
	// Unbind desliga la sesión de su empresa sin revocarla (ADR 0011): UPDATE
	// condicionado a id y membership_id, idempotente (0 filas no es un error).
	Unbind(ctx context.Context, sessionID, membershipID pgtype.UUID) error
}

type ctxKey struct{}

// WithSession deja la sesión autenticada en el contexto de la petición.
func WithSession(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, ctxKey{}, s)
}

// SessionFrom devuelve la sesión que fijó el middleware.
func SessionFrom(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(ctxKey{}).(Session)
	return s, ok
}
