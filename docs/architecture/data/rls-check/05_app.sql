-- Como wanka_app (NOBYPASSRLS), sin app.tenant_id ni app.user_id: lo que hace el middleware en cada peticion.
-- Cada caso compara contra lo esperado con test.assert / test.expect_error y aborta (exit != 0) si no coincide.
\set ON_ERROR_STOP on
\echo '== A1. session_lookup sin variables: sesion en version 1, membresia en version 2 y activa =='
SELECT test.assert(current_user = 'wanka_app' AND NULLIF(current_setting('app.tenant_id', true),'') IS NULL, 'rol wanka_app y sin tenant');
SELECT test.assert(current_setting('statement_timeout') = '5s', 'wanka_app hereda statement_timeout 5s del bootstrap');
SELECT test.assert((SELECT session_authz_version = 1 AND current_authz_version = 2 AND membership_status = 'activo'
                           AND site_id = 'a5000000-0000-0000-0000-0000000000a1' AND cash_register_id = 'ca000000-0000-0000-0000-0000000000a1'
                           AND mfa_satisfied AND mfa_method = 'none'
                      FROM iam.session_lookup(sha256('tok-cajero'))), 'session_lookup: version 1 -> 2/activo, con sede y caja');
SELECT test.assert(NULLIF(current_setting('app.tenant_id', true),'') IS NULL, 'session_lookup no filtra app.tenant_id');
SELECT test.assert((SELECT count(*) FROM iam.session_lookup(sha256('tok-a-medio-login'))) = 0, 'session_lookup ignora sesiones con mfa_satisfied = false');
\echo '== A2. lectura directa de iam.memberships sin tenant =='
SELECT test.assert((SELECT count(*) FROM iam.memberships) = 0, 'RLS: 0 membresias sin tenant');
SELECT test.assert((SELECT count(*) FROM iam.roles) = 0, 'RLS: 0 roles sin tenant');
SELECT test.assert((SELECT count(*) FROM org.tenants) = 0, 'RLS: 0 empresas sin tenant');
\echo '== A3. list_my_memberships (security_invoker) con solo app.user_id del cajero =='
BEGIN;
SELECT set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000002', true);
SELECT test.assert((SELECT string_agg(tenant || '|' || role || '|' || requires_mfa || '|' || site || '|' || status, ',') FROM iam.list_my_memberships)
                   = 'Empresa A|Cajero|false|Sede principal|activo', 'list_my_memberships del cajero');
COMMIT;
\echo '== A4. list_support_requests =='
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
SELECT test.assert((SELECT string_agg(scope || ':' || status, ',') FROM iam.list_support_requests()) = 'full:solicitado', 'solo la solicitud de A');
COMMIT;
SELECT test.assert((SELECT count(*) FROM iam.list_support_requests()) = 0, 'sin tenant: 0 solicitudes');
SELECT test.expect_error('SELECT count(*) FROM platform.impersonation_grants', '42501', 'wanka_app sin acceso a platform');
\echo '== A5. decide_impersonation falla cerrada =='
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true), set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000001', true);
SELECT test.expect_error($$SELECT iam.decide_impersonation('99999999-0000-0000-0000-000000000001', true)$$, '42501', 'sin app.session_kind: rechazo');
SELECT set_config('app.session_kind','impersonated', true);
SELECT test.expect_error($$SELECT iam.decide_impersonation('99999999-0000-0000-0000-000000000001', true)$$, '42501', 'session_kind impersonated: rechazo');
SELECT set_config('app.session_kind','system', true);
SELECT test.expect_error($$SELECT iam.decide_impersonation('99999999-0000-0000-0000-000000000001', true)$$, '42501', 'session_kind system: rechazo');
SELECT set_config('app.session_kind','user', true);
SELECT test.expect_error($$SELECT iam.decide_impersonation('99999999-0000-0000-0000-000000000002', true)$$, 'P0002', 'grant de B desde A: no encontrado');
SELECT test.assert(iam.decide_impersonation('99999999-0000-0000-0000-000000000001', true) = 'aprobado', 'session_kind user aprueba el de A');
COMMIT;
\echo '== A6. redeem_impersonation sin tenant fijado =='
SELECT test.expect_error($$SELECT iam.redeem_impersonation('\x02'::bytea, gen_random_uuid(), sha256('tok-imp-b'))$$, '42501', 'grant de B no aprobado: rechazo');
SELECT test.assert(iam.redeem_impersonation('\x01'::bytea, '5e000000-0000-0000-0000-0000000000f1', sha256('tok-imp-a')) = 'aaaaaaaa-0000-0000-0000-000000000001', 'canje del grant aprobado de A');
SELECT test.assert((SELECT mfa_method = 'platform' AND impersonator_id IS NOT NULL AND membership_id IS NULL
                      FROM iam.session_lookup(sha256('tok-imp-a'))), 'sesion impersonada: platform, sin membresia');
SELECT test.assert(NULLIF(current_setting('app.tenant_id', true),'') IS NULL, 'redeem no filtra app.tenant_id');
-- Revocada, la sesion impersonada ya no se encuentra (dentro de una transaccion que se deshace); la vencida, en 05b.
BEGIN;
UPDATE iam.sessions SET revoked_at = now() WHERE access_hash = sha256('tok-imp-a');
SELECT test.assert((SELECT count(*) FROM iam.session_lookup(sha256('tok-imp-a'))) = 0, 'impersonada revocada: 0 filas');
ROLLBACK;
\echo '== A7. CHECK y NOT NULL de iam.sessions =='
SELECT test.expect_error($$INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, mfa_satisfied, mfa_method)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', sha256('x'), now(), true, 'platform')$$, '23514', 'platform sin impersonator_id');
SELECT test.expect_error($$INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, mfa_satisfied, mfa_method)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', sha256('y'), now(), true, NULL)$$, '23502', 'mfa_method NULL');
SELECT test.expect_error($$INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, mfa_satisfied, mfa_method)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', sha256('z'), now(), true, 'sms')$$, '23514', 'mfa_method fuera de la lista');
-- Una sesion desligada o impersonada nunca tiene version (ADR 0011, CHECK sessions_version_needs_membership).
SELECT test.expect_error($$INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, authz_version, mfa_satisfied, mfa_method)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', sha256('v1'), now(), 1, true, 'none')$$, '23514', 'authz_version sin membresia');
SELECT test.expect_error($$INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, mfa_satisfied, mfa_method)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', '\x01', now(), true, 'none')$$, '23514', 'access_hash de 1 byte');
SELECT test.expect_error($$INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, mfa_satisfied, mfa_method)
  VALUES (gen_random_uuid(), gen_random_uuid(), sha256('v3'), now(), true, 'none')$$, '23503', 'sesion de un usuario inexistente');
-- GRANT del ADR 0001: wanka_app escribe sesiones pero no empresas.
SELECT test.expect_error($$UPDATE org.tenants SET status = 'activa'$$, '42501', 'wanka_app no escribe org.tenants');
-- H13: GRANT por columna y trigger iam.guard_impersonated_session (sondeo de data-model §6, regla 2).
-- (a) columnas fuera del GRANT
SELECT test.expect_error($$UPDATE iam.sessions SET created_at = created_at + interval '30 days' WHERE access_hash = sha256('tok-cajero')$$, '42501', '(a) wanka_app no mueve created_at');
SELECT test.expect_error($$UPDATE iam.sessions SET created_at = created_at + interval '30 days' WHERE access_hash = sha256('tok-imp-a')$$, '42501', '(a) ni en la impersonada');
SELECT test.expect_error($$UPDATE iam.sessions SET impersonator_id = NULL WHERE access_hash = sha256('tok-imp-a')$$, '42501', '(a) wanka_app no mueve impersonator_id');
SELECT test.expect_error($$UPDATE iam.sessions SET user_id = NULL WHERE access_hash = sha256('tok-cajero')$$, '42501', '(a) wanka_app no mueve user_id');
-- (b) columnas del GRANT sobre una impersonada: las para el trigger
SELECT test.expect_error($$UPDATE iam.sessions SET access_expires_at = access_expires_at + interval '30 days' WHERE access_hash = sha256('tok-imp-a')$$, '42501', '(b) no alarga la impersonada');
SELECT test.expect_error($$UPDATE iam.sessions SET tenant_id = 'bbbbbbbb-0000-0000-0000-000000000002' WHERE access_hash = sha256('tok-imp-a')$$, '42501', '(b) no cambia su tenant');
-- (c) INSERT de una impersonada
SELECT test.expect_error($$INSERT INTO iam.sessions (id, tenant_id, access_hash, access_expires_at, mfa_satisfied, mfa_method, impersonator_id)
  VALUES (gen_random_uuid(), 'aaaaaaaa-0000-0000-0000-000000000001', sha256('p2'), now() + interval '30 days', true, 'platform', gen_random_uuid())$$, '42501', '(c) wanka_app no inserta impersonadas');
-- (d) revocar si; la rotacion del refresh de una sesion normal tambien
BEGIN;
UPDATE iam.sessions SET revoked_at = now() WHERE access_hash = sha256('tok-imp-a');
SELECT test.assert((SELECT revoked_at IS NOT NULL FROM iam.sessions WHERE access_hash = sha256('tok-imp-a')), '(d) revocar la impersonada funciona');
ROLLBACK;
BEGIN;
UPDATE iam.sessions SET access_hash = sha256('rot-a'), refresh_hash = sha256('rot-r'), prev_refresh_hash = sha256('old-r'), rotated_at = now(),
  access_expires_at = now() + interval '15 minutes', refresh_expires_at = now() + interval '30 days' WHERE access_hash = sha256('tok-cajero');
SELECT test.assert((SELECT count(*) FROM iam.sessions WHERE access_hash = sha256('rot-a')) = 1, 'rotacion del refresh de una sesion normal');
ROLLBACK;
-- user_id solo es NULL en la impersonada
SELECT test.expect_error($$INSERT INTO iam.sessions (id, access_hash, access_expires_at, mfa_satisfied, mfa_method) VALUES (gen_random_uuid(), sha256('nu'), now(), true, 'none')$$,
  '23514', 'sesion normal sin user_id');
