import { argon2Sync, randomBytes } from "node:crypto";
import { execFileSync } from "node:child_process";
import { E2E_KINDS, E2E_PROJECTS, e2eEmail } from "./login-users";

const b64 = (buf: Buffer) => buf.toString("base64").split("=").join("");

/** PHC de argon2id con los parámetros que lee la API (formato de `internal/identidad/adapters/argon2`). */
function phc(password: string): string {
  const salt = randomBytes(16);
  const memory = 19456;
  const passes = 2;
  const parallelism = 1;
  const key = argon2Sync("argon2id", { message: password, nonce: salt, memory, passes, parallelism, tagLength: 32 });
  return `$argon2id$v=19$m=${memory},t=${passes},p=${parallelism}$${b64(salt)}$${b64(key)}`;
}

/**
 * SQL idempotente: una empresa de prueba y, por proyecto, cuatro usuarios con la contraseña dada (hasheada aquí, nunca
 * guardada). `nomembership` tiene su única membresía suspendida (403); `mfa` tiene TOTP confirmado (`mfa_required`).
 * Reinicia los contadores de bloqueo de las corridas anteriores.
 */
export function seedSql(password: string): string {
  const hash = phc(password);
  const rows: string[] = [];
  for (const project of E2E_PROJECTS) {
    for (const kind of E2E_KINDS) {
      const email = e2eEmail(kind, project);
      const status = kind === "nomembership" ? "suspendido" : "activo";
      rows.push(`
INSERT INTO iam.users (id, email, password_hash) VALUES (gen_random_uuid(), '${email}', '${hash}')
  ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, failed_logins = 0, locked_until = NULL,
    login_lockouts = 0, login_window_started_at = NULL, mfa_failed_attempts = 0, mfa_locked_until = NULL, mfa_lockouts = 0;
INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status, all_sites)
  SELECT gen_random_uuid(), t.id, u.id, r.id, '${status}', true
  FROM iam.users u, e2e_tenant t, e2e_role r WHERE u.email = '${email}'
  ON CONFLICT (tenant_id, user_id) DO NOTHING;
${kind === "mfa" ? `INSERT INTO iam.user_totp (user_id, secret_enc, confirmed_at) SELECT id, '\\x01', now() FROM iam.users WHERE email = '${email}' ON CONFLICT (user_id) DO NOTHING;` : ""}`);
    }
  }
  return `
BEGIN;
CREATE TEMP TABLE e2e_tenant (id uuid);
CREATE TEMP TABLE e2e_role (id uuid);
WITH t AS (SELECT id FROM org.tenants WHERE trade_name = 'Empresa E2E' LIMIT 1),
     n AS (INSERT INTO org.tenants (id, status, trade_name, industry) SELECT gen_random_uuid(), 'activa', 'Empresa E2E', 'retail' WHERE NOT EXISTS (SELECT 1 FROM t) RETURNING id)
INSERT INTO e2e_tenant SELECT id FROM t UNION ALL SELECT id FROM n;
INSERT INTO iam.roles (id, tenant_id, name, requires_mfa) SELECT gen_random_uuid(), id, 'Rol E2E', false FROM e2e_tenant ON CONFLICT (tenant_id, name) DO NOTHING;
INSERT INTO e2e_role SELECT r.id FROM iam.roles r JOIN e2e_tenant t ON t.id = r.tenant_id WHERE r.name = 'Rol E2E';
${rows.join("\n")}
COMMIT;`;
}

/** Siembra los usuarios en la base local vía `docker exec <contenedor> psql` (superusuario del contenedor de desarrollo). */
export function seedLoginUsers(password: string, container: string): void {
  execFileSync("docker", ["exec", "-i", container, "psql", "-q", "-U", "postgres", "-d", "wanka", "-v", "ON_ERROR_STOP=1"], {
    input: seedSql(password),
    stdio: ["pipe", "ignore", "pipe"],
  });
}
