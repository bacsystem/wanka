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
	"github.com/jackc/pgx/v5/pgxpool"
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
	// Solo system puede ir sin usuario (ADR 0001: webhooks, «Workers»; ADR 0003: activación de facturación): no existe un
	// usuario de sistema, y app.current_user_id() queda en NULL. Una sesión de usuario sin UserID haría lo mismo.
	if a.Kind != KindSystem && emptyUUID(a.UserID) {
		return fmt.Errorf("%w: session kind %s without UserID", ErrInvalidActor, a.Kind)
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
// y los de adquisición de conexión (ErrOverloaded, ErrUnavailable) si no hay conexión.
func (p *Pool) InTenantTx(ctx context.Context, a Actor, fn func(pgx.Tx) error) error {
	return p.inTenantTx(ctx, a, pgx.ReadWrite, fn)
}

// InTenantReadTx es InTenantTx con BEGIN READ ONLY, para las lecturas.
func (p *Pool) InTenantReadTx(ctx context.Context, a Actor, fn func(pgx.Tx) error) error {
	return p.inTenantTx(ctx, a, pgx.ReadOnly, fn)
}

func (p *Pool) inTenantTx(ctx context.Context, a Actor, mode pgx.TxAccessMode, fn func(pgx.Tx) error) error {
	if err := a.validate(); err != nil {
		return err
	}
	// tenant_id sale solo del Actor.
	return p.runTx(ctx, mode, setTenantVarsSQL,
		[]any{uuidText(a.TenantID), uuidText(a.UserID), a.Kind.String(), uuidText(a.ImpersonatorID)}, fn)
}

// setTenantVarsSQL fija las cuatro variables app.* de una transacción de tenant.
// set_config(..., true) es local a la transacción: al terminar, la conexión no
// conserva nada.
const setTenantVarsSQL = `SELECT set_config('app.tenant_id', $1, true), set_config('app.user_id', $2, true),
		set_config('app.session_kind', $3, true), set_config('app.impersonator_id', $4, true)`

// runTx es el ciclo común de las puertas de transacción (tenant y sin tenant):
// toma una conexión, hace BEGIN, ejecuta setSQL con args como primera sentencia
// (fija las variables app.*), llama a fn y hace COMMIT, o ROLLBACK si fn falla
// o entra en pánico (el pánico se relanza). Los errores salen tipados con typedErr.
func (p *Pool) runTx(ctx context.Context, mode pgx.TxAccessMode, setSQL string, args []any, fn func(pgx.Tx) error) (err error) {
	conn, err := p.acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: mode})
	if err != nil {
		return p.txError(ctx, conn, "beginning transaction", err)
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

	_, err = tx.Exec(ctx, setSQL, args...)
	if err != nil {
		return p.txError(ctx, conn, "setting session variables", err)
	}
	if err := fn(tx); err != nil {
		return typedErr(ctx, conn.Conn().PgConn().IsClosed(), err)
	}
	if err := tx.Commit(ctx); err != nil {
		err = p.txError(ctx, conn, "committing transaction", err)
		if errors.Is(err, ErrUnavailable) {
			// La conexión se perdió con el COMMIT en vuelo: pudo confirmarse.
			err = fmt.Errorf("%w: %w", ErrCommitUncertain, err)
		}
		return err
	}
	committed = true
	return nil
}

func (p *Pool) txError(ctx context.Context, conn *pgxpool.Conn, what string, err error) error {
	return typedErr(ctx, conn.Conn().PgConn().IsClosed(), fmt.Errorf("%s: %w", what, scrubbed(p.scrub, err)))
}

// typedErr marca con ErrStatementTimeout el error de una sentencia cancelada por
// el servidor. Se reconoce por SQLSTATE 57014 (query_canceled) con el contexto
// del llamador todavía vivo: si el llamador canceló, el ctx ya terminó antes de
// que llegue el error (pgx envía el cancel por eso). Nunca por el texto del
// mensaje, que PostgreSQL traduce según lc_messages. Un pg_cancel_backend() de
// un administrador también cae aquí; para la petición es el mismo "la base
// canceló tu sentencia, reintenta". La causa original se conserva.
func typedErr(ctx context.Context, connClosed bool, err error) error {
	if ctx.Err() != nil {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "57014" {
		return fmt.Errorf("%w: %w", ErrStatementTimeout, err)
	}
	if connectionLost(connClosed, err) {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return err
}

// connectionLost dice si err es la base que se fue (reinicio, failover,
// pg_terminate_backend, red caída) y no un error de la sentencia o del código
// que la rodea. Decide por el estado de la conexión (connClosed: pgx la da por
// cerrada) o por el SQLSTATE (clase 08 y 57P01 a 57P03), nunca por la forma del
// error ni por su texto: un io.EOF de un CSV, un error de otra API o un
// "conn busy" (un bug de la aplicación) dejan la conexión viva y siguen siendo
// un 500 con su causa en el log.
//
// Si la conexión se pierde durante el COMMIT el resultado es incierto (la
// transacción pudo confirmarse), y reintentar solo es seguro con la misma
// Idempotency-Key o con If-Match. El BFF no reintenta un 503 por su cuenta
// (conventions.md §3 y §4).
func connectionLost(connClosed bool, err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false // el plazo propio del llamador no es una base caída
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "57P01", "57P02", "57P03":
			return true
		}
		if strings.HasPrefix(pgErr.Code, "08") {
			return true
		}
	}
	return connClosed
}
