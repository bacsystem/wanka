// Package audit escribe la bitácora audit.events (ADR 0001, data-model §1):
// quién hizo qué y cuándo, en la misma transacción que el cambio.
//
// # Record
//
// Un caso de uso llama a Writer.Record con el pgx.Tx de su InTenantTx, después
// del cambio de negocio. La fila se inserta en esa transacción: si hace
// ROLLBACK no queda ni el cambio ni el evento, y si el insert falla (por
// ejemplo, no existe la partición del mes) la transacción entera se revierte;
// el evento nunca se descarta en silencio.
//
// El módulo no fija nada de quién actúa. El tenant, el usuario, el
// impersonador y el tipo de actor salen de las variables app.* que InTenantTx
// dejó en la transacción (las mismas que usa la RLS), y request_id e ip salen
// del contexto. Event no tiene campo de tenant.
//
// # Tipo de actor
//
//   - sesión de usuario: actor_type user, con actor_user_id;
//   - sesión impersonada (ADR 0001): actor_type platform_staff, actor_user_id
//     NULL e impersonator_id con el staff; el staff no es un usuario de la empresa;
//   - job o webhook (Kind system): actor_type system, actor_user_id NULL. El
//     origen (por ejemplo el id del job) va en la acción o en el diff, nunca
//     en un usuario inventado.
//
// # Denegaciones
//
// RecordDenied registra un intento rechazado por permiso en una transacción
// propia, porque no hay cambio que acompañar. Si el insert falla devuelve el
// error y el llamador (el middleware) lo registra en el log y responde 403 de
// todos modos: perder el registro no puede convertir un rechazo en un 500.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

// Resultados de un evento.
const (
	ResultOK     = "ok"
	ResultDenied = "denegado"
)

// ErrInvalidEvent: el Event no cumple el formato (acción, tipo de entidad o diff).
// Es un error de programación del módulo; no se reintenta.
var ErrInvalidEvent = errors.New("audit: invalid event")

// actionFormat es modulo.recurso:accion (api/resources.md), en minúsculas.
var actionFormat = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*:[a-z][a-z0-9_]*$`)

// entityTypeFormat es un nombre corto de tabla o recurso.
var entityTypeFormat = regexp.MustCompile(`^[a-z][a-z0-9_.]{0,63}$`)

// Event es lo que un módulo declara de una acción. No lleva tenant, usuario,
// request_id ni ip: salen de la transacción y del contexto.
type Event struct {
	// Action es modulo.recurso:accion, por ejemplo "ventas.documento:void".
	Action string
	// EntityType y EntityID identifican lo afectado; ambos son opcionales.
	EntityType string
	EntityID   pgtype.UUID
	// Diff es el detalle del cambio (típicamente un map o un struct con tags json).
	// Los campos de la lista de prohibidos se enmascaran antes de guardarse.
	Diff any
}

func (e Event) validate() error {
	if !actionFormat.MatchString(e.Action) || len(e.Action) > 120 {
		return fmt.Errorf("%w: action %q is not modulo.recurso:accion", ErrInvalidEvent, e.Action)
	}
	if e.EntityType != "" && !entityTypeFormat.MatchString(e.EntityType) {
		return fmt.Errorf("%w: entity type %q", ErrInvalidEvent, e.EntityType)
	}
	return nil
}

type ctxKey struct{}

// WithClientIP devuelve un contexto que lleva la ip del cliente; el middleware
// la fija una vez por petición y Record la guarda en el evento.
func WithClientIP(ctx context.Context, ip netip.Addr) context.Context {
	return context.WithValue(ctx, ctxKey{}, ip)
}

func clientIP(ctx context.Context) any {
	if ip, ok := ctx.Value(ctxKey{}).(netip.Addr); ok && ip.IsValid() {
		return ip.Unmap()
	}
	return nil
}

// txRunner es lo que el Writer necesita del pool; *db.Pool lo cumple.
type txRunner interface {
	InTenantTx(ctx context.Context, a db.Actor, fn func(pgx.Tx) error) error
}

// Writer escribe eventos. No guarda estado salvo el pool de las denegaciones.
type Writer struct{ db txRunner }

// NewWriter crea un Writer; pool abre la transacción propia de RecordDenied.
func NewWriter(pool txRunner) *Writer { return &Writer{db: pool} }

// insertSQL arma la fila con lo que ya está fijado en la transacción. Si el
// tenant falta, tenant_id es NULL y el insert falla; si session_kind no es uno
// de los tres, actor_type es NULL y también falla. Ningún dato de quién actúa
// viene de un parámetro.
const insertSQL = `
INSERT INTO audit.events (tenant_id, actor_type, actor_user_id, impersonator_id,
                          action, entity_type, entity_id, result, ip, request_id, diff)
SELECT app.current_tenant(),
       CASE s.kind WHEN 'user' THEN 'user' WHEN 'impersonated' THEN 'platform_staff' WHEN 'system' THEN 'system' END,
       CASE s.kind WHEN 'user' THEN app.current_user_id() END,
       NULLIF(current_setting('app.impersonator_id', true), '')::uuid,
       $1, $2, $3, $4, $5, $6, $7
  FROM (SELECT NULLIF(current_setting('app.session_kind', true), '') AS kind) s`

// Record inserta el evento en tx, que debe ser la transacción del cambio
// (InTenantTx): confirma o se revierte con él.
func (w *Writer) Record(ctx context.Context, tx pgx.Tx, ev Event) error {
	return record(ctx, tx, ev, ResultOK)
}

// RecordDenied registra en una transacción propia (InTenantTx con actor) que
// una acción fue denegada por permiso. Sirve aunque no haya cambio.
func (w *Writer) RecordDenied(ctx context.Context, actor db.Actor, ev Event) error {
	return w.db.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		return record(ctx, tx, ev, ResultDenied)
	})
}

func record(ctx context.Context, tx pgx.Tx, ev Event, result string) error {
	if err := ev.validate(); err != nil {
		return err
	}
	diff, err := scrubDiff(ev.Diff)
	if err != nil {
		return err
	}
	var diffArg any
	if diff != nil {
		diffArg = json.RawMessage(diff)
	}
	var entityType, requestID any
	if ev.EntityType != "" {
		entityType = ev.EntityType
	}
	if id := logging.RequestID(ctx); id != "" {
		requestID = truncate(id, 128)
	}
	var entityID any
	if ev.EntityID.Valid {
		entityID = ev.EntityID
	}
	if _, err := tx.Exec(ctx, insertSQL, ev.Action, entityType, entityID, result, clientIP(ctx), requestID, diffArg); err != nil {
		return fmt.Errorf("recording audit event %s: %w", ev.Action, err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
