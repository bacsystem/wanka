package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// NoTenantScope es lo único que una transacción sin tenant puede declarar: el
// usuario, y solo si la función que se va a llamar lo exige (por ejemplo
// iam.list_my_memberships). No hay campo de tenant ni de Kind: por esta puerta
// no se puede fijar app.tenant_id, app.session_kind ni app.impersonator_id.
type NoTenantScope struct {
	// UserID es opcional; vacío (no Valid o todo ceros) deja app.user_id en ''.
	UserID pgtype.UUID
}

// setNoTenantVarsSQL limpia las cuatro variables app.* en una sola sentencia
// (la misma ida y vuelta que InTenantTx): lo que haya dejado en la conexión
// una petición anterior con set_config(..., false) queda tapado durante la
// transacción, y las funciones que guardan y restauran app.tenant_id (como
// iam.session_lookup) restauran la cadena vacía.
const setNoTenantVarsSQL = `SELECT set_config('app.tenant_id', '', true), set_config('app.user_id', $1, true),
		set_config('app.session_kind', '', true), set_config('app.impersonator_id', '', true)`

// InNoTenantTx abre una transacción de lectura y escritura sin tenant: limpia
// las variables app.* (solo app.user_id puede traer el UserID del scope) y
// llama a fn. Hace COMMIT si fn devuelve nil y ROLLBACK si devuelve error o
// entra en pánico (el pánico se relanza). Es la puerta de todo acceso a la base
// sin tenant fijado, de cualquier servicio (data-model.md, «Puerta sin tenant»):
// en api, la sesión, el login, iam.list_my_memberships y el canje de
// impersonación; en webhooks, ops.resolve_webhook_route; en platform-api, las
// platform_fn.* y platform.*; en worker, la limpieza de iam.sessions. Las
// tablas de tenant no se ven: app.current_tenant() es NULL. Los errores son los
// de InTenantTx.
func (p *Pool) InNoTenantTx(ctx context.Context, s NoTenantScope, fn func(pgx.Tx) error) error {
	return p.runTx(ctx, pgx.ReadWrite, setNoTenantVarsSQL, []any{uuidText(s.UserID)}, fn)
}

// InNoTenantReadTx es InNoTenantTx con BEGIN READ ONLY.
func (p *Pool) InNoTenantReadTx(ctx context.Context, s NoTenantScope, fn func(pgx.Tx) error) error {
	return p.runTx(ctx, pgx.ReadOnly, setNoTenantVarsSQL, []any{uuidText(s.UserID)}, fn)
}
