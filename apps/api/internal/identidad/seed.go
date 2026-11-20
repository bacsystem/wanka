package identidad

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// seedRolesSQL crea una fila de iam.roles por plantilla. Los permisos no se
// copian: salen de iam.role_template_permissions por template_code (ADR 0004,
// nota del 2026-11-19). El conflicto se resuelve solo sobre (tenant_id,
// template_code): una segunda llamada no duplica ni falla, y un rol propio que ya
// use el nombre de una plantilla sí falla (23505) en vez de dejar la empresa sin
// ese rol base.
const seedRolesSQL = `INSERT INTO iam.roles (id, tenant_id, name, system, template_code, requires_mfa, version)
	SELECT gen_random_uuid(), $1, t.name, true, t.code, t.requires_mfa, 1 FROM iam.role_templates t
	ON CONFLICT (tenant_id, template_code) DO NOTHING`

// SeedTenantRoles crea en la empresa los roles base, uno por plantilla de
// iam.role_templates, con system = true, template_code, el requires_mfa de la
// plantilla y version = 1. Es idempotente. tx debe ser una transacción de tenant
// de esa empresa (db.InTenantTx): la RLS de iam.roles rechaza otra. Lo llama el
// alta de la empresa (#31) dentro de su propia transacción.
func SeedTenantRoles(ctx context.Context, tx pgx.Tx, tenantID pgtype.UUID) error {
	if !validUUID(tenantID) {
		return fmt.Errorf("seeding tenant roles: empty tenant id")
	}
	// Sin plantillas no hay roles base: una empresa sin dueño no se da de alta.
	var templates int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM iam.role_templates`).Scan(&templates); err != nil {
		return fmt.Errorf("seeding tenant roles: %w", err)
	}
	if templates == 0 {
		return errors.New("seeding tenant roles: no role templates")
	}
	if _, err := tx.Exec(ctx, seedRolesSQL, tenantID); err != nil {
		return fmt.Errorf("seeding tenant roles: %w", err)
	}
	return nil
}
