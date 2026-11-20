package identidad

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/adapters/argon2"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/passwordpolicy"
)

// Datos inventados de la empresa de prueba (solo local). El RUC tiene formato válido (11 dígitos, dígito verificador
// correcto) pero no es de una empresa real.
const (
	DevTenantRUC        = "20999999990"
	DevTenantLegalName  = "Empresa Demo SAC"
	DevTenantTradeName  = "Empresa Demo"
	DevEmailDomain      = "demo.local" // no se resuelve: ningún correo sale de aquí
	DevSiteCode         = "PRINCIPAL"
	DevCashRegisterCode = "CAJA-01"
)

// DevRoles son los códigos de las plantillas base; cada uno tiene un usuario <código>@demo.local.
var DevRoles = []string{"dueno", "administrador", "contador", "cajero", "vendedor", "almacenero"}

// DevSeedUser es un usuario sembrado.
type DevSeedUser struct {
	Role  string
	Email string
}

// DevSeed crea (o repone) la empresa de prueba con sus roles base, un local con su caja y un usuario activo por rol
// con la contraseña password. Es idempotente: una segunda corrida deja los mismos registros y repone la contraseña y
// el desbloqueo de cada usuario. pool debe conectarse con un rol que cree empresas y usuarios (en local, el
// superusuario del PostgreSQL de desarrollo): ningún rol de servicio puede. Quien llama decide si el entorno es local.
func DevSeed(ctx context.Context, pool *db.Pool, hashParams argon2.Params, password string) ([]DevSeedUser, error) {
	policy, err := passwordpolicy.NewEmbedded()
	if err != nil {
		return nil, err
	}
	if err := policy.Check(password); err != nil || len(password) < 10 {
		return nil, errors.New("dev seed: the test password is too short or appears in the breached list")
	}
	hasher, err := argon2.New(hashParams)
	if err != nil {
		return nil, fmt.Errorf("dev seed: creating the password hasher: %w", err)
	}
	hashes := make(map[string]string, len(DevRoles))
	for _, r := range DevRoles {
		if hashes[r], err = hasher.Hash(password); err != nil {
			return nil, fmt.Errorf("dev seed: hashing: %w", err)
		}
	}

	var tenantID pgtype.UUID
	users := make(map[string]pgtype.UUID, len(DevRoles))
	err = pool.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT id FROM org.tenants WHERE ruc = $1`, DevTenantRUC).Scan(&tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `INSERT INTO org.tenants (id, status, ruc, legal_name, trade_name, industry)
				VALUES (gen_random_uuid(), 'activa', $1, $2, $3, 'retail') RETURNING id`,
				DevTenantRUC, DevTenantLegalName, DevTenantTradeName).Scan(&tenantID)
		}
		if err != nil {
			return fmt.Errorf("tenant: %w", err)
		}
		for _, r := range DevRoles {
			var id pgtype.UUID
			// Repone contraseña y desbloqueo: la semilla sirve también para dejar el entorno como nuevo.
			err := tx.QueryRow(ctx, `INSERT INTO iam.users (id, email, full_name, password_hash, email_verified, password_changed_at)
				VALUES (gen_random_uuid(), $1, $2, $3, true, now())
				ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, status = 'activo', email_verified = true,
					failed_logins = 0, login_window_started_at = NULL, locked_until = NULL, login_lockouts = 0,
					mfa_failed_attempts = 0, mfa_locked_until = NULL, mfa_lockouts = 0, updated_at = now()
				RETURNING id`, r+"@"+DevEmailDomain, "Demo "+r, hashes[r]).Scan(&id)
			if err != nil {
				return fmt.Errorf("user %s: %w", r, err)
			}
			users[r] = id
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("dev seed: %w", err)
	}

	actor := db.Actor{TenantID: tenantID, Kind: db.KindSystem}
	err = pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		if err := SeedTenantRoles(ctx, tx, tenantID); err != nil {
			return err
		}
		var siteID pgtype.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO org.sites (id, tenant_id, code, name) VALUES (gen_random_uuid(), $1, $2, 'Local principal')
			ON CONFLICT (tenant_id, code) DO UPDATE SET active = true RETURNING id`, tenantID, DevSiteCode).Scan(&siteID); err != nil {
			return fmt.Errorf("site: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO org.cash_registers (id, tenant_id, site_id, code) VALUES (gen_random_uuid(), $1, $2, $3)
			ON CONFLICT (tenant_id, code) DO NOTHING`, tenantID, siteID, DevCashRegisterCode); err != nil {
			return fmt.Errorf("cash register: %w", err)
		}
		for _, r := range DevRoles {
			if _, err := tx.Exec(ctx, `INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, all_sites, status)
				SELECT gen_random_uuid(), $1, $2, ro.id, true, 'activo' FROM iam.roles ro WHERE ro.tenant_id = $1 AND ro.template_code = $3
				ON CONFLICT (tenant_id, user_id) DO UPDATE SET role_id = EXCLUDED.role_id, status = 'activo', updated_at = now()`,
				tenantID, users[r], r); err != nil {
				return fmt.Errorf("membership %s: %w", r, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("dev seed: %w", err)
	}
	out := make([]DevSeedUser, 0, len(DevRoles))
	for _, r := range DevRoles {
		out = append(out, DevSeedUser{Role: r, Email: r + "@" + DevEmailDomain})
	}
	return out, nil
}
