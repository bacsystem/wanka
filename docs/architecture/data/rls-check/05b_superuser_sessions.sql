-- Como superusuario (el trigger iam.guard_session_writes lo admite, como a las semillas de prueba): los CHECK de una
-- sesion impersonada, que wanka_app no puede alcanzar porque el trigger lo para antes. Cada INSERT va en su subtransaccion.
\set ON_ERROR_STOP on
\echo '== S1. CHECK de la sesion impersonada (sin refresh, 60 min como maximo, sin user_id ni version) =='
SELECT test.assert((SELECT rolsuper FROM pg_roles WHERE rolname = current_user), 'superusuario');
SELECT test.expect_error($$INSERT INTO iam.sessions (id, tenant_id, access_hash, access_expires_at, refresh_hash, mfa_satisfied, mfa_method, impersonator_id)
  VALUES (gen_random_uuid(), 'aaaaaaaa-0000-0000-0000-000000000001', sha256('s1'), now() + interval '60 minutes', sha256('s1r'), true, 'platform', gen_random_uuid())$$,
  '23514', 'impersonada con refresh_hash');
SELECT test.expect_error($$INSERT INTO iam.sessions (id, tenant_id, access_hash, access_expires_at, mfa_satisfied, mfa_method, impersonator_id)
  VALUES (gen_random_uuid(), 'aaaaaaaa-0000-0000-0000-000000000001', sha256('s2'), now() + interval '61 minutes', true, 'platform', gen_random_uuid())$$,
  '23514', 'impersonada que vive 61 minutos');
SELECT test.expect_error($$INSERT INTO iam.sessions (id, user_id, tenant_id, access_hash, access_expires_at, mfa_satisfied, mfa_method, impersonator_id)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'aaaaaaaa-0000-0000-0000-000000000001', sha256('s3'), now(), true, 'platform', gen_random_uuid())$$,
  '23514', 'impersonada con user_id');
SELECT test.expect_error($$INSERT INTO iam.sessions (id, tenant_id, access_hash, access_expires_at, authz_version, mfa_satisfied, mfa_method, impersonator_id)
  VALUES (gen_random_uuid(), 'aaaaaaaa-0000-0000-0000-000000000001', sha256('s4'), now(), 1, true, 'platform', gen_random_uuid())$$,
  '23514', 'impersonada con authz_version');
\echo '== S2. impersonada vencida: session_lookup no la encuentra =='
BEGIN;
UPDATE iam.sessions SET access_expires_at = now() - interval '1 second' WHERE access_hash = sha256('tok-imp-a');
SELECT test.assert((SELECT count(*) FROM iam.session_lookup(sha256('tok-imp-a'))) = 0, 'impersonada vencida: 0 filas');
ROLLBACK;
\echo '== S3. impersonada con membership_id y sin user_id: la membresia se ve ausente =='
BEGIN;
UPDATE iam.sessions SET membership_id = (SELECT id FROM iam.memberships WHERE user_id = 'eeeeeeee-0000-0000-0000-000000000001' AND tenant_id = 'aaaaaaaa-0000-0000-0000-000000000001')
 WHERE access_hash = sha256('tok-imp-a');
SELECT test.assert((SELECT user_id IS NULL AND membership_id IS NOT NULL FROM iam.sessions WHERE access_hash = sha256('tok-imp-a')), 'impersonada con membership_id y sin user_id');
SELECT test.assert((SELECT current_authz_version IS NULL AND membership_status IS NULL AND membership_id IS NOT NULL
                      FROM iam.session_lookup(sha256('tok-imp-a'))), 'membresia ausente: NULL no coincide con ningun usuario');
ROLLBACK;
