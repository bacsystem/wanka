\set ON_ERROR_STOP on
\echo '== A8. (wanka_app) membresia borrada: session_lookup sin version actual -> el middleware desliga la sesion (ADR 0011) =='
SELECT test.assert((SELECT session_authz_version = 1 AND current_authz_version IS NULL AND membership_status IS NULL AND membership_id IS NOT NULL
                      FROM iam.session_lookup(sha256('tok-cajero'))), 'membresia borrada: sin version actual (desligar)');
\echo '== A9. (wanka_app) membresia borrada: el middleware desliga la sesion sin revocarla (ADR 0011) =='
-- Borrar membership_id sin borrar tenant_id viola el CHECK: la sesion no puede quedar en una empresa sin membresia.
SELECT test.expect_error($$UPDATE iam.sessions SET membership_id = NULL WHERE id = '5e000000-0000-0000-0000-000000000001'$$,
  '23514', 'desligar a medias (tenant sin membresia) viola el CHECK');
SELECT test.expect_error($$UPDATE iam.sessions SET tenant_id = NULL WHERE id = '5e000000-0000-0000-0000-000000000001'$$,
  '23514', 'desligar a medias (membresia sin tenant) viola el CHECK');
-- Dentro de UNA transaccion: set_config(..., true) dura hasta el fin de la transaccion, asi que fuera de ella
-- la comprobacion no distingue. La busqueda (sesion aun ligada: fija el tenant de la sesion) y el UPDATE de
-- desligar no deben dejar app.tenant_id fijado para lo que sigue. MUTATION=session_lookup_no_restore lo pone en rojo.
BEGIN;
SELECT test.assert((SELECT count(*) FROM iam.session_lookup(sha256('tok-cajero'))) = 1, 'TX: la sesion aun ligada se encuentra');
SELECT test.assert(NULLIF(current_setting('app.tenant_id', true),'') IS NULL, 'TX: session_lookup restaura app.tenant_id');
-- El UPDATE condicional del middleware: una fila la primera vez y ninguna la segunda (idempotente ante peticiones concurrentes).
WITH u AS (
  UPDATE iam.sessions SET tenant_id = NULL, membership_id = NULL, site_id = NULL, cash_register_id = NULL, authz_version = NULL
   WHERE id = '5e000000-0000-0000-0000-000000000001' AND membership_id = '3e000000-0000-0000-0000-0000000000c1'
     AND impersonator_id IS NULL AND revoked_at IS NULL
  RETURNING 1)
SELECT test.assert(count(*) = 1, 'wanka_app desliga la sesion (1 fila)') FROM u;
SELECT test.assert(NULLIF(current_setting('app.tenant_id', true),'') IS NULL, 'TX: desligar no deja app.tenant_id fijado');
COMMIT;
WITH u AS (
  UPDATE iam.sessions SET tenant_id = NULL, membership_id = NULL, site_id = NULL, cash_register_id = NULL, authz_version = NULL
   WHERE id = '5e000000-0000-0000-0000-000000000001' AND membership_id = '3e000000-0000-0000-0000-0000000000c1'
     AND impersonator_id IS NULL AND revoked_at IS NULL
  RETURNING 1)
SELECT test.assert(count(*) = 0, 'repetir el UPDATE no cambia nada (idempotente)') FROM u;
-- La sesion sigue autenticando, ahora sin tenant: sirve para /v1/me/memberships y session/tenant, no para rutas de tenant.
SELECT test.assert((SELECT tenant_id IS NULL AND membership_id IS NULL AND site_id IS NULL AND cash_register_id IS NULL
                           AND session_authz_version IS NULL AND current_authz_version IS NULL AND membership_status IS NULL
                           AND mfa_satisfied AND mfa_method = 'none' AND impersonator_id IS NULL
                      FROM iam.session_lookup(sha256('tok-cajero'))), 'sesion desligada: sigue viva, sin tenant, conserva la 2FA');
SELECT test.assert((SELECT revoked_at IS NULL FROM iam.sessions WHERE id = '5e000000-0000-0000-0000-000000000001'), 'sesion desligada: no revocada');
