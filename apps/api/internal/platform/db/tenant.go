package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Errores de InTenantTx.
var (
	// ErrNoTenant: el Actor no trae tenant. Una sesión sin empresa (ADR 0011)
	// no puede abrir una transacción de tenant: con app.current_tenant() en NULL
	// se abrirían las ramas por usuario de las políticas (own_memberships,
	// member_read) a filas de otras empresas. Vale para todo Kind, también system.
	ErrNoTenant = errors.New("db: actor has no tenant")
	// ErrInvalidActor es un error de programación: Kind sin definir o
	// ImpersonatorID incoherente con el Kind. No abre transacción.
	ErrInvalidActor = errors.New("db: invalid actor")
	// ErrStatementTimeout: una sentencia superó el statement_timeout del rol
	// (5 s para wanka_app); la capa HTTP lo traduce. Conserva la causa de PostgreSQL.
	ErrStatementTimeout = errors.New("db: statement timeout")
)

// rollbackTimeout acota el ROLLBACK de una transacción abandonada.
const rollbackTimeout = 5 * time.Second

// SessionKind es el tipo de sesión que fija app.session_kind. No tiene valor
// cero válido: un Actor{} sin Kind devuelve ErrInvalidActor y la variable nunca
// queda vacía (ADR 0001, «Cómo lo fija Go»).
type SessionKind uint8

// Tipos de sesión. Los textos son los que comparan las funciones SECURITY DEFINER.
const (
	KindUser SessionKind = iota + 1
	KindImpersonated
	KindSystem
)

// String devuelve el valor de app.session_kind, o "" si el Kind no está definido.
func (k SessionKind) String() string {
	switch k {
	case KindUser:
		return "user"
	case KindImpersonated:
		return "impersonated"
	case KindSystem:
		return "system"
	default:
		return ""
	}
}

// Valid indica si k es uno de los tres tipos definidos.
func (k SessionKind) Valid() bool { return k.String() != "" }

// Actor es quien actúa en una transacción de tenant. Lo arma el middleware de
// autenticación (o el handler de un job) y nunca se llena desde el cuerpo, la
// URL ni una cabecera de la petición.
//
// Los identificadores son pgtype.UUID, de pgx (aprobado en ADR 0002): no hay
// una biblioteca de UUID aprobada y pgx ya codifica y decodifica uuid. Un UUID
// vacío es el que no es Valid o es todo ceros.
type Actor struct {
	TenantID pgtype.UUID
	UserID   pgtype.UUID
	Kind     SessionKind
	// ImpersonatorID es obligatorio si Kind == KindImpersonated y debe estar
	// vacío en los demás.
	ImpersonatorID pgtype.UUID
}

func emptyUUID(u pgtype.UUID) bool { return !u.Valid || u.Bytes == [16]byte{} }

// uuidText devuelve el UUID en texto, o "" si está vacío.
func uuidText(u pgtype.UUID) string {
	if emptyUUID(u) {
		return ""
	}
	return u.String()
}

func (a Actor) validate() error {
	if !a.Kind.Valid() {
		return fmt.Errorf("%w: undefined session kind %d", ErrInvalidActor, a.Kind)
	}
	if emptyUUID(a.TenantID) {
		return ErrNoTenant
	}
	hasImpersonator := !emptyUUID(a.ImpersonatorID)
	switch {
	case a.Kind == KindImpersonated && !hasImpersonator:
		return fmt.Errorf("%w: impersonated session without ImpersonatorID", ErrInvalidActor)
	case a.Kind != KindImpersonated && hasImpersonator:
		return fmt.Errorf("%w: ImpersonatorID with session kind %s", ErrInvalidActor, a.Kind)
	}
	return nil
}

// InTenantTx abre una transacción de lectura y escritura con el contexto de
// a fijado en las variables app.* (local a la transacción) y llama a fn. Hace
// COMMIT si fn devuelve nil y ROLLBACK si devuelve error o entra en pánico (el
// pánico se relanza). Es la única puerta a las tablas de tenant (ADR 0001).
//
// Devuelve ErrNoTenant o ErrInvalidActor sin tocar la base si a no es válido,
// y los errores de Acquire (ErrOverloaded, ErrUnavailable) si no hay conexión.
func (p *Pool) InTenantTx(ctx context.Context, a Actor, fn func(pgx.Tx) error) error {
	return p.inTenantTx(ctx, a, pgx.ReadWrite, fn)
}

// InTenantReadTx es InTenantTx con BEGIN READ ONLY, para las lecturas.
func (p *Pool) InTenantReadTx(ctx context.Context, a Actor, fn func(pgx.Tx) error) error {
	return p.inTenantTx(ctx, a, pgx.ReadOnly, fn)
}

func (p *Pool) inTenantTx(ctx context.Context, a Actor, mode pgx.TxAccessMode, fn func(pgx.Tx) error) (err error) {
	if err := a.validate(); err != nil {
		return err
	}
	conn, err := p.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: mode})
	if err != nil {
		return p.txError(ctx, "beginning transaction", err)
	}
	committed := false
	// También corre si fn entra en pánico: deshace y deja que el pánico siga.
	// El contexto de la petición puede estar cancelado; el ROLLBACK no depende de él.
	defer func() {
		if committed {
			return
		}
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()
		// Si falla, la conexión queda fuera de transacción inactiva y el pool la destruye al soltarla.
		_ = tx.Rollback(rctx)
	}()

	// tenant_id sale solo del Actor. set_config(..., true) es local a la
	// transacción: al terminar, la conexión no conserva nada.
	_, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true), set_config('app.user_id', $2, true),
		set_config('app.session_kind', $3, true), set_config('app.impersonator_id', $4, true)`,
		uuidText(a.TenantID), uuidText(a.UserID), a.Kind.String(), uuidText(a.ImpersonatorID))
	if err != nil {
		return p.txError(ctx, "setting session variables", err)
	}
	if err := fn(tx); err != nil {
		return p.typed(ctx, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return p.txError(ctx, "committing transaction", err)
	}
	committed = true
	return nil
}

func (p *Pool) txError(ctx context.Context, what string, err error) error {
	return p.typed(ctx, fmt.Errorf("%s: %w", what, scrubbed(p.scrub, err)))
}

// typed marca con ErrStatementTimeout el error de una sentencia cancelada por
// statement_timeout. Una cancelación del llamador (ctx) tiene el mismo SQLSTATE
// 57014 pero otro mensaje, y no se marca. La causa original se conserva.
func (p *Pool) typed(ctx context.Context, err error) error {
	var pgErr *pgconn.PgError
	if ctx.Err() == nil && errors.As(err, &pgErr) && pgErr.Code == "57014" &&
		strings.Contains(pgErr.Message, "statement timeout") {
		return fmt.Errorf("%w: %w", ErrStatementTimeout, err)
	}
	return err
}
