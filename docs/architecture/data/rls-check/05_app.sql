-- Como wanka_app (NOBYPASSRLS), sin app.tenant_id ni app.user_id: lo que hace el middleware en cada peticion.
-- Cada caso compara contra lo esperado con test.assert / test.expect_error y aborta (exit != 0) si no coincide.
\set ON_ERROR_STOP on
\echo '== A1. session_lookup sin variables: sesion en version 1, membresia en version 2 y activa =='
SELECT test.assert(current_user = 'wanka_app' AND NULLIF(current_setting('app.tenant_id', true),'') IS NULL, 'rol wanka_app y sin tenant');
SELECT test.assert(current_setting('statement_timeout') = '5s', 'wanka_app hereda statement_timeout 5s del bootstrap');
-- La busqueda y la comprobacion de app.tenant_id van en UNA transaccion: set_config(..., true) dura hasta su fin, asi que
-- fuera de ella el valor ya se borro y la asercion no distingue (mismo defecto que H6 en 07_after_delete.sql).
BEGIN;
SELECT test.assert((SELECT session_authz_version = 1 AND current_authz_version = 2 AND membership_status = 'activo'
                           AND site_id = 'a5000000-0000-0000-0000-0000000000a1' AND cash_register_id = 'ca000000-0000-0000-0000-0000000000a1'
                           AND mfa_satisfied AND mfa_method = 'none'
                      FROM iam.session_lookup(sha256('tok-cajero'))), 'session_lookup: version 1 -> 2/activo, con sede y caja');
SELECT test.assert(NULLIF(current_setting('app.tenant_id', true),'') IS NULL, 'TX: session_lookup no filtra app.tenant_id');
COMMIT;
SELECT test.assert((SELECT count(*) FROM iam.session_lookup(sha256('tok-a-medio-login'))) = 0, 'session_lookup ignora sesiones con mfa_satisfied = false');
\echo '== A2. lectura directa de iam.memberships sin tenant =='
SELECT test.assert((SELECT count(*) FROM iam.memberships) = 0, 'RLS: 0 membresias sin tenant');
SELECT test.assert((SELECT count(*) FROM iam.roles) = 0, 'RLS: 0 roles sin tenant');
SELECT test.assert((SELECT count(*) FROM org.tenants) = 0, 'RLS: 0 empresas sin tenant');
\echo '== A2b. ex miembro (membresia suspendida) sin tenant: la base no le da ninguna fila de la empresa (issue #187) =='
-- Sin app.tenant_id y solo con app.user_id, como GET /v1/me/memberships. No depende de que un handler filtre.
BEGIN;
SELECT set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000003', true);
SELECT test.assert((SELECT count(*) FROM org.tenants) = 0, 'ex miembro: 0 filas de org.tenants');
SELECT test.assert((SELECT count(*) FROM iam.memberships) = 0, 'ex miembro: 0 membresias (la suspendida no se lee)');
SELECT test.assert((SELECT count(*) FROM iam.roles) = 0 AND (SELECT count(*) FROM org.sites) = 0, 'ex miembro: 0 roles y 0 sedes');
SELECT test.assert((SELECT count(*) FROM iam.list_my_memberships) = 0, 'ex miembro: list_my_memberships vacia');
COMMIT;
-- Las membresias invitado/activo siguen visibles: el filtro no cierra de mas.
BEGIN;
SELECT set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000002', true);   -- cajero: activo en A
SELECT test.assert((SELECT string_agg(t.name || ':' || m.status, ',') FROM org.tenants t JOIN iam.memberships m ON m.tenant_id = t.id) = 'Empresa A:activo', 'activo: ve su empresa y su membresia');
COMMIT;
BEGIN;
SELECT set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000009', true);   -- propietario de B: invitado
SELECT test.assert((SELECT string_agg(t.name || ':' || m.status, ',') FROM org.tenants t JOIN iam.memberships m ON m.tenant_id = t.id) = 'Empresa B:invitado', 'invitado: ve su empresa y su membresia');
COMMIT;
-- Catalogo: el filtro de estado esta escrito en las dos politicas. Con own_memberships filtrando, la subconsulta de member_read
-- ya no ve la suspendida, asi que quitarlo de member_read no cambia ninguna fila: solo el catalogo lo distingue.
-- El conjunto EXACTO de estados: todo literal entre comillas de la politica (solo hay estados) debe ser {activo,invitado}; un
-- 'suspendido' de mas, o uno de menos, falla. Se mira tambien que la rama por usuario siga bajo app.current_tenant() IS NULL.
SELECT test.assert((SELECT qual ~ 'status' AND qual ~ 'current_tenant'
                           AND (SELECT array_agg(DISTINCT m[1] ORDER BY m[1]) FROM regexp_matches(qual, '''([^'']*)''', 'g') AS m) = ARRAY['activo','invitado']
                      FROM pg_policies WHERE schemaname = 'org' AND tablename = 'tenants' AND policyname = 'member_read'),
                   'org.tenants member_read exige exactamente invitado/activo');
SELECT test.assert((SELECT qual ~ 'status'
                           AND (SELECT array_agg(DISTINCT m[1] ORDER BY m[1]) FROM regexp_matches(qual, '''([^'']*)''', 'g') AS m) = ARRAY['activo','invitado']
                      FROM pg_policies WHERE schemaname = 'iam' AND tablename = 'memberships' AND policyname = 'own_memberships'),
                   'iam.memberships own_memberships exige exactamente invitado/activo');
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
-- H13: GRANT por columna y trigger iam.guard_session_writes (sondeo de data-model §6, regla 2).
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
-- H18: ningun privilegio de UPDATE sobre estas columnas (el sondeo (a) de arriba corre sobre una sesion normal, tok-cajero).
SELECT test.assert(NOT has_column_privilege('wanka_app', 'iam.sessions', c, 'UPDATE'), 'wanka_app sin UPDATE en iam.sessions.' || c)
  FROM unnest(ARRAY['created_at','id','user_id','impersonator_id','refresh_family']) AS c;
-- H19: refresh_family se fija en el INSERT; login a medio camino, verify y cambio de familia.
BEGIN;
INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, refresh_family, mfa_satisfied, mfa_method)
  VALUES ('5e000000-0000-0000-0000-0000000000e1', 'eeeeeeee-0000-0000-0000-000000000002', sha256('half'), now() + interval '10 minutes', gen_random_uuid(), false, 'totp');
UPDATE iam.sessions SET mfa_satisfied = true, mfa_method = 'totp', access_hash = sha256('v-a'), refresh_hash = sha256('v-r'),
  access_expires_at = now() + interval '15 minutes', refresh_expires_at = now() + interval '30 days' WHERE id = '5e000000-0000-0000-0000-0000000000e1';
SELECT test.assert((SELECT mfa_satisfied AND refresh_family IS NOT NULL FROM iam.sessions WHERE id = '5e000000-0000-0000-0000-0000000000e1'), 'insert a medio login y verify funcionan');
COMMIT;
SELECT test.expect_error($$UPDATE iam.sessions SET refresh_family = gen_random_uuid() WHERE id = '5e000000-0000-0000-0000-0000000000e1'$$, '42501', 'wanka_app no cambia refresh_family');
\echo '== A10. sesion cruzada, revocacion irreversible y created_at del servidor =='
-- H1: la sesion del cajero con la membresia de OTRO usuario de la misma empresa (la app puede escribir membership_id):
-- session_lookup ve la membresia ausente (lo trata el desligue del ADR 0011), nunca la ajena como vigente.
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
UPDATE iam.sessions SET membership_id = (SELECT id FROM iam.memberships WHERE user_id = 'eeeeeeee-0000-0000-0000-000000000001')
 WHERE access_hash = sha256('tok-cajero');
SELECT set_config('app.tenant_id','', true);
SELECT test.assert((SELECT membership_id IS NOT NULL AND current_authz_version IS NULL AND membership_status IS NULL
                      FROM iam.session_lookup(sha256('tok-cajero'))), 'sesion cruzada: membresia ajena = ausente, no vigente');
ROLLBACK;
-- La revocacion no se deshace ni se repite, en una sesion normal y en una impersonada.
BEGIN;
UPDATE iam.sessions SET revoked_at = now() WHERE access_hash = sha256('tok-cajero');
SELECT test.expect_error($$UPDATE iam.sessions SET revoked_at = NULL WHERE access_hash = sha256('tok-cajero')$$, '42501', 'una sesion normal revocada no se reactiva');
UPDATE iam.sessions SET revoked_at = now() WHERE access_hash = sha256('tok-imp-a');
SELECT test.expect_error($$UPDATE iam.sessions SET revoked_at = NULL WHERE access_hash = sha256('tok-imp-a')$$, '42501', 'una impersonada revocada no se reactiva');
ROLLBACK;
-- Revocar otra vez es un no-op que conserva el instante viejo, y un UPDATE masivo revoca las vivas aunque alguna ya este revocada.
BEGIN;
UPDATE iam.sessions SET revoked_at = '2026-01-01 00:00:00+00' WHERE access_hash = sha256('tok-imp-a');
UPDATE iam.sessions SET revoked_at = now() WHERE user_id = 'eeeeeeee-0000-0000-0000-000000000002' OR impersonator_id IS NOT NULL;
SELECT test.assert((SELECT revoked_at = '2026-01-01 00:00:00+00' FROM iam.sessions WHERE access_hash = sha256('tok-imp-a')), 're-revocar conserva el instante viejo');
SELECT test.assert((SELECT revoked_at IS NOT NULL FROM iam.sessions WHERE access_hash = sha256('tok-cajero')), 'el UPDATE masivo revoca la sesion viva del cajero');
ROLLBACK;
-- created_at lo pone el servidor aunque la app mande otro.
BEGIN;
INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, mfa_satisfied, mfa_method, created_at)
  VALUES ('5e000000-0000-0000-0000-0000000000e2', 'eeeeeeee-0000-0000-0000-000000000002', sha256('old-created'), now(), true, 'none', now() - interval '400 days');
SELECT test.assert((SELECT created_at = now() FROM iam.sessions WHERE id = '5e000000-0000-0000-0000-0000000000e2'), 'created_at del INSERT lo fija el servidor');
ROLLBACK;
\echo '== A10. (wanka_app) tablas de River: solo INSERT en ops.river_job =='
SELECT test.river_enqueue_only();
\echo '== A11. (wanka_app) ops.idempotency_keys: aislada por tenant, la respuesta se guarda en UPDATE, sin DELETE =='
SELECT test.assert((SELECT count(*) FROM ops.idempotency_keys) = 0, 'RLS: 0 claves sin tenant');
SELECT test.expect_error($$INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'sin-tenant', sha256('x'))$$,
  '42501', 'sin tenant fijado no se inserta una clave');
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true), set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000001', true);
SELECT test.assert((SELECT count(*) FROM ops.idempotency_keys) = 2 AND (SELECT count(*) FROM ops.idempotency_keys WHERE key IN ('old-b', 'recent-b')) = 0,
  'tenant A ve sus 2 claves (la vencida incluida) y ninguna de B');
-- INSERT primero (sin respuesta), UPDATE despues; created_at lo pone el servidor.
INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'k-new', sha256('body'));
SELECT test.assert((SELECT status_code IS NULL AND response IS NULL AND created_at = now() FROM ops.idempotency_keys WHERE key = 'k-new'), 'la clave nace sin respuesta y con created_at del servidor');
DO $$ DECLARE n int; BEGIN
  INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'k-new', sha256('otro')) ON CONFLICT DO NOTHING;
  GET DIAGNOSTICS n = ROW_COUNT;
  PERFORM test.assert(n = 0, 'ON CONFLICT DO NOTHING no inserta una clave repetida');
END $$;
UPDATE ops.idempotency_keys SET status_code = 201, response = '\x7b226964223a317d', response_headers = '{"Location": "/v1/x/1"}' WHERE key = 'k-new';
SELECT test.assert((SELECT status_code = 201 AND response = '\x7b226964223a317d' FROM ops.idempotency_keys WHERE key = 'k-new'), 'el UPDATE guarda la respuesta');
SELECT test.expect_error($$INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash) VALUES ('bbbbbbbb-0000-0000-0000-000000000002', 'k-b', sha256('x'))$$,
  '42501', 'tenant A no inserta una clave de B (WITH CHECK)');
SELECT test.expect_error($$UPDATE ops.idempotency_keys SET status_code = 302 WHERE key = 'k-new'$$, '23514', 'solo 2xx: un 302 viola el CHECK');
SELECT test.expect_error($$UPDATE ops.idempotency_keys SET status_code = 409 WHERE key = 'k-new'$$, '23514', 'solo 2xx: un 409 viola el CHECK');
SELECT test.expect_error($$INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'k-short', '\x00')$$,
  '23514', 'request_hash de 32 bytes');
SELECT test.expect_error($$UPDATE ops.idempotency_keys SET request_hash = sha256('otro') WHERE key = 'k-new'$$, '42501', 'el hash no se mueve');
SELECT test.expect_error($$UPDATE ops.idempotency_keys SET created_at = now() + interval '1 year' WHERE key = 'k-new'$$, '42501', 'created_at no se mueve');
SELECT test.expect_error($$INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash, created_at) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'k-future', sha256('x'), now() + interval '1 year')$$,
  '42501', 'created_at no se elige en el INSERT');
SELECT test.expect_error($$INSERT INTO ops.idempotency_keys (tenant_id, principal_id, key, request_hash) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'eeeeeeee-0000-0000-0000-000000000002', 'k-imp', sha256('x'))$$,
  '42501', 'principal_id lo fija el servidor: wanka_app no lo elige');
SELECT test.assert((SELECT principal_id FROM ops.idempotency_keys WHERE key = 'k-new') = 'eeeeeeee-0000-0000-0000-000000000001', 'principal_id es app.user_id');
SELECT test.expect_error($$UPDATE ops.idempotency_keys SET principal_id = 'eeeeeeee-0000-0000-0000-000000000002' WHERE key = 'k-new'$$, '42501', 'principal_id no se mueve');
SELECT test.expect_error($$DELETE FROM ops.idempotency_keys WHERE key = 'k-new'$$, '42501', 'wanka_app no borra claves');
SELECT test.expect_error($$TRUNCATE ops.idempotency_keys$$, '42501', 'wanka_app no hace TRUNCATE');
ROLLBACK;
-- La misma clave en dos tenants no interfiere (PK tenant_id, key).
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true), set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000001', true);
INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'compartida', sha256('a'));
SELECT set_config('app.tenant_id','bbbbbbbb-0000-0000-0000-000000000002', true), set_config('app.user_id','eeeeeeee-0000-0000-0000-0000000000b1', true);
INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash) VALUES ('bbbbbbbb-0000-0000-0000-000000000002', 'compartida', sha256('b'));
SELECT test.assert((SELECT count(*) FROM ops.idempotency_keys WHERE key = 'compartida') = 1
                   AND (SELECT request_hash FROM ops.idempotency_keys WHERE key = 'compartida') = sha256('b'), 'B ve solo su clave compartida, con su hash');
ROLLBACK;
-- La clave es del principal: dos usuarios del mismo tenant con la misma clave dejan dos filas, sin conflicto.
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true), set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000001', true);
INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'dos-usuarios', sha256('u1'));
SELECT set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000002', true);
DO $$ DECLARE n int; BEGIN
  INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'dos-usuarios', sha256('u2')) ON CONFLICT DO NOTHING;
  GET DIAGNOSTICS n = ROW_COUNT;
  PERFORM test.assert(n = 1, 'U2 inserta la misma clave que U1: es otra fila');
END $$;
SELECT test.assert((SELECT count(*) FROM ops.idempotency_keys WHERE key = 'dos-usuarios') = 2, 'U1 y U2 con la misma clave dejan dos filas');
\echo '== A11. (wanka_app) audit.events: aislamiento por tenant, solo insercion, sin DDL de particiones (#173) =='
-- Como Record: el tenant, el usuario y el tipo de sesion salen de las variables app.* de la transaccion.
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true), set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000001', true),
       set_config('app.session_kind','user', true);
INSERT INTO audit.events (tenant_id, actor_type, actor_user_id, action) VALUES (app.current_tenant(), 'user', app.current_user_id(), 'rls.check:a');
COMMIT;
BEGIN;
SELECT set_config('app.tenant_id','bbbbbbbb-0000-0000-0000-000000000002', true), set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000009', true),
       set_config('app.session_kind','user', true);
INSERT INTO audit.events (tenant_id, actor_type, actor_user_id, action) VALUES (app.current_tenant(), 'user', app.current_user_id(), 'rls.check:b');
COMMIT;
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
SELECT test.assert((SELECT count(*) FROM audit.events WHERE action = 'rls.check:a') = 1 AND (SELECT count(*) FROM audit.events WHERE action = 'rls.check:b') = 0,
                   'A ve su evento y no el de B');
SELECT test.assert((SELECT count(*) > 0 AND bool_and(tenant_id = 'aaaaaaaa-0000-0000-0000-000000000001') FROM audit.events),
                   'todo lo que ve A es de A (tambien los eventos de plataforma sobre A)');
SELECT test.expect_error($$INSERT INTO audit.events (tenant_id, actor_type, actor_user_id, action)
                           VALUES ('bbbbbbbb-0000-0000-0000-000000000002', 'user', 'eeeeeeee-0000-0000-0000-000000000001', 'rls.check:x')$$,
                         '42501', 'A no inserta un evento a nombre de B (WITH CHECK)');
SELECT test.expect_error($$INSERT INTO audit.events (tenant_id, actor_type, actor_user_id, action)
                           VALUES (app.current_tenant(), 'user', NULL, 'rls.check:x')$$,
                         '23514', 'una fila user sin usuario es imposible');
SELECT test.expect_error($$INSERT INTO audit.events (tenant_id, actor_type, actor_user_id, action)
                           VALUES (app.current_tenant(), 'system', 'eeeeeeee-0000-0000-0000-000000000001', 'rls.check:x')$$,
                         '23514', 'una fila system no lleva usuario');
SELECT test.expect_error($$INSERT INTO audit.events (tenant_id, actor_type, action) VALUES (app.current_tenant(), 'platform_staff', 'rls.check:x')$$,
                         '23514', 'una fila platform_staff sin impersonador es imposible');
SELECT test.expect_error($$INSERT INTO audit.events (tenant_id, actor_type, actor_user_id, impersonator_id, action)
                           VALUES (app.current_tenant(), 'platform_staff', 'eeeeeeee-0000-0000-0000-000000000001', 'cccccccc-0000-0000-0000-000000000009', 'rls.check:x')$$,
                         '23514', 'una fila platform_staff no lleva usuario de la empresa');
SELECT test.expect_error($$INSERT INTO audit.events (tenant_id, actor_type, actor_user_id, impersonator_id, action)
                           VALUES (app.current_tenant(), 'user', 'eeeeeeee-0000-0000-0000-000000000001', 'cccccccc-0000-0000-0000-000000000009', 'rls.check:x')$$,
                         '23514', 'solo el staff lleva impersonador');
SELECT test.expect_error($$INSERT INTO audit.events (tenant_id, actor_type, actor_user_id, action) VALUES (app.current_tenant(), 'user', 'eeeeeeee-0000-0000-0000-000000000001', 'tenants:create')$$,
                         '23514', 'una accion sin modulo.recurso:accion se rechaza');
SELECT test.expect_error($$UPDATE audit.events SET action = 'rls.check:z'$$, '42501', 'wanka_app no actualiza audit.events');
SELECT test.expect_error('DELETE FROM audit.events', '42501', 'wanka_app no borra audit.events');
SELECT test.expect_error('TRUNCATE audit.events', '42501', 'wanka_app no vacia audit.events');
SELECT test.expect_error($$SELECT audit.ensure_partitions(current_date, 0)$$, '42501', 'wanka_app no crea particiones');
SELECT test.expect_error('SELECT count(*) FROM audit.events_y' || to_char(now() AT TIME ZONE 'UTC', 'YYYY') || 'm' || to_char(now() AT TIME ZONE 'UTC', 'MM'), '42501',
                         'wanka_app no lee una particion directo');
COMMIT;
SELECT test.assert((SELECT count(*) FROM audit.events) = 0, 'sin tenant fijado: 0 eventos');
\echo '== A11. ops.exchange_rates (global, sin RLS; issue #170): wanka_app solo lee =='
SELECT test.expect_error($$INSERT INTO ops.exchange_rates VALUES ('2026-11-19', 'USD', 3.7, 3.8, 'official')$$, '42501', 'wanka_app no inserta tipos de cambio');
SELECT test.expect_error($$UPDATE ops.exchange_rates SET buy = 1$$, '42501', 'wanka_app no actualiza tipos de cambio');
SELECT test.expect_error($$DELETE FROM ops.exchange_rates$$, '42501', 'wanka_app no borra tipos de cambio');
SELECT test.assert((SELECT count(*) FROM ops.exchange_rates) = 1, 'wanka_app lee la tasa sembrada sin tenant fijado (tabla global)');
\echo '== E1. entitlements (#171): wanka_app solo LEE las tablas de platform de entitlements, que no tienen RLS =='
SELECT test.assert((SELECT count(*) FROM platform.modules) >= 21 AND (SELECT count(*) FROM platform.plan_modules) >= 21,
                   'wanka_app lee el catalogo de modulos y los modulos del plan');
SELECT test.assert((SELECT count(*) FROM platform.plans WHERE code = 'semilla' AND non_production) = 1, 'wanka_app lee el plan semilla, marcado no productivo');
SELECT test.assert((SELECT count(*) FROM platform.subscriptions) = 0 AND (SELECT count(*) FROM platform.tenant_module_overrides) = 0
                   AND (SELECT count(*) FROM platform.feature_flags) = 0 AND (SELECT count(*) FROM platform.tenant_flag_overrides) = 0
                   AND (SELECT count(*) FROM platform.tenant_usage) = 0, 'wanka_app lee el resto de las tablas de entitlements (vacias)');
SELECT test.assert((SELECT count(*) = 8 AND NOT bool_or(c.relrowsecurity OR c.relforcerowsecurity) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                     WHERE n.nspname = 'platform' AND c.relname IN ('modules','plans','plan_modules','subscriptions','tenant_module_overrides',
                                                                   'feature_flags','tenant_flag_overrides','tenant_usage')),
                   'las 8 tablas de entitlements existen y no tienen RLS');
SELECT test.expect_error($$INSERT INTO platform.subscriptions (id, tenant_id, plan_id, status)
  VALUES (gen_random_uuid(), 'aaaaaaaa-0000-0000-0000-000000000001', (SELECT id FROM platform.plans LIMIT 1), 'activa')$$, '42501', 'wanka_app no inserta suscripciones');
SELECT test.expect_error($$UPDATE platform.subscriptions SET status = 'activa'$$, '42501', 'wanka_app no actualiza suscripciones');
SELECT test.expect_error($$DELETE FROM platform.subscriptions$$, '42501', 'wanka_app no borra suscripciones');
SELECT test.expect_error($$INSERT INTO platform.tenant_module_overrides (tenant_id, module_code, enabled) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'ventas', true)$$,
                         '42501', 'wanka_app no escribe overrides de modulos');
SELECT test.expect_error($$INSERT INTO platform.tenant_flag_overrides (tenant_id, flag_key, enabled) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'f', true)$$,
                         '42501', 'wanka_app no escribe overrides de flags');
SELECT test.expect_error($$INSERT INTO platform.tenant_usage (tenant_id, period) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', '2026-11-01')$$,
                         '42501', 'wanka_app no escribe el uso (lo escribe wanka_worker)');
SELECT test.expect_error($$UPDATE platform.plans SET name = 'x'$$, '42501', 'wanka_app no edita planes');
SELECT test.expect_error($$INSERT INTO platform.modules (code, kind, description) VALUES ('x', 'core', 'x')$$, '42501', 'wanka_app no edita modulos');
SELECT test.expect_error($$INSERT INTO platform.plan_modules (plan_id, module_code) VALUES ((SELECT id FROM platform.plans LIMIT 1), 'x')$$, '42501', 'wanka_app no edita plan_modules');
SELECT test.expect_error($$INSERT INTO platform.feature_flags (key) VALUES ('x')$$, '42501', 'wanka_app no edita flags');
-- Catalogo de permisos (issue #172): wanka_app lo lee y no lo escribe, sin importar el tenant fijado.
\echo '== C1. wanka_app: iam.permissions es de solo lectura =='
SELECT test.assert((SELECT count(*) FROM iam.permissions) > 100
                   AND EXISTS (SELECT 1 FROM iam.permissions WHERE code = 'ventas.pos:create'), 'wanka_app lee el catalogo de permisos');
SELECT test.expect_error($$INSERT INTO iam.permissions (code, description) VALUES ('ventas.pos:void', 'Anular ventas')$$, '42501', 'wanka_app no inserta permisos');
SELECT test.expect_error($$UPDATE iam.permissions SET description = 'x'$$, '42501', 'wanka_app no actualiza permisos');
SELECT test.expect_error($$DELETE FROM iam.permissions$$, '42501', 'wanka_app no borra permisos');
SELECT test.expect_error($$TRUNCATE iam.permissions$$, '42501', 'wanka_app no vacia permisos');

\echo '== E1. iam.mfa_enrollments (#25): grants por columna, CHECK y trigger del token de enrolamiento =='
-- e1: token consumido con dos envios; e2: token vivo con el secreto pendiente fijado.
BEGIN;
INSERT INTO iam.mfa_enrollments (id, user_id, token_hash, email_code_hmac, expires_at)
  VALUES ('e0000000-0000-0000-0000-0000000000e1', 'eeeeeeee-0000-0000-0000-000000000002', sha256('wk_en_prueba'),
          sha256('hmac-prueba'), now() + interval '10 minutes'),
         ('e0000000-0000-0000-0000-0000000000e2', 'eeeeeeee-0000-0000-0000-000000000002', sha256('wk_en_vivo'),
          sha256('hmac-vivo'), now() + interval '10 minutes');
-- reenvio, secreto pendiente y consumo: lo que el contrato permite
UPDATE iam.mfa_enrollments SET email_code_hmac = sha256('hmac-2'), email_sends = 2, totp_secret_enc = '\x00', consumed_at = now()
 WHERE id = 'e0000000-0000-0000-0000-0000000000e1';
UPDATE iam.mfa_enrollments SET totp_secret_enc = '\x01' WHERE id = 'e0000000-0000-0000-0000-0000000000e2';
SELECT test.assert((SELECT email_sends = 2 AND consumed_at IS NOT NULL FROM iam.mfa_enrollments WHERE id = 'e0000000-0000-0000-0000-0000000000e1'),
                   'wanka_app reenvia, guarda el secreto pendiente y consume el token');
COMMIT;
SELECT test.expect_error($$UPDATE iam.mfa_enrollments SET expires_at = expires_at + interval '1 hour'$$, '42501', 'wanka_app no alarga un token de enrolamiento');
SELECT test.expect_error($$UPDATE iam.mfa_enrollments SET user_id = 'eeeeeeee-0000-0000-0000-000000000001'$$, '42501', 'wanka_app no pasa el token a otro usuario');
SELECT test.expect_error($$UPDATE iam.mfa_enrollments SET token_hash = sha256('otro')$$, '42501', 'wanka_app no cambia token_hash');
SELECT test.expect_error($$DELETE FROM iam.mfa_enrollments$$, '42501', 'wanka_app no borra tokens de enrolamiento');
SELECT test.assert(NOT has_column_privilege('wanka_app', 'iam.mfa_enrollments', c, 'UPDATE'), 'wanka_app sin UPDATE en iam.mfa_enrollments.' || c)
  FROM unnest(ARRAY['id','user_id','token_hash','expires_at','created_at']) AS c;
SELECT test.expect_error($$INSERT INTO iam.mfa_enrollments (id, user_id, token_hash, email_code_hmac, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', sha256('t2'), sha256('h2'), now() + interval '11 minutes')$$,
  '23514', 'token de enrolamiento de mas de 10 min');
SELECT test.expect_error($$INSERT INTO iam.mfa_enrollments (id, user_id, token_hash, email_code_hmac, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', sha256('t3'), '\x00', now() + interval '5 minutes')$$,
  '23514', 'email_code_hmac de 32 bytes');
SELECT test.expect_error($$UPDATE iam.users SET mfa_failed_attempts = 5 WHERE id = 'eeeeeeee-0000-0000-0000-000000000002'$$, '23514', 'el 5o fallo bloquea, no se guarda');
SELECT test.expect_error($$UPDATE iam.users SET mfa_lockouts = 4 WHERE id = 'eeeeeeee-0000-0000-0000-000000000002'$$, '23514', 'con 3 bloqueos el segundo paso ya esta cerrado');
-- Trigger, INSERT: created_at lo fija el servidor. Un created_at futuro no hace pasar un TTL largo por el CHECK.
SELECT test.expect_error($$INSERT INTO iam.mfa_enrollments (id, user_id, token_hash, email_code_hmac, created_at, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', sha256('t4'), sha256('h4'), now() + interval '30 days' - interval '5 minutes', now() + interval '30 days')$$,
  '23514', 'un created_at futuro no deja un token vigente semanas');
BEGIN;
INSERT INTO iam.mfa_enrollments (id, user_id, token_hash, email_code_hmac, created_at, email_sent_at, email_sends, consumed_at, expires_at)
  VALUES ('e0000000-0000-0000-0000-0000000000e3', 'eeeeeeee-0000-0000-0000-000000000002', sha256('t5'), sha256('h5'),
          now() - interval '1 day', now() - interval '1 day', 3, now(), now() + interval '5 minutes');
SELECT test.assert((SELECT created_at = now() AND email_sent_at = now() AND email_sends = 1 AND consumed_at IS NULL
                      FROM iam.mfa_enrollments WHERE id = 'e0000000-0000-0000-0000-0000000000e3'),
                   'el INSERT fija created_at, email_sent_at, email_sends = 1 y consumed_at NULL');
ROLLBACK;
-- Trigger, UPDATE de un token consumido: no se reabre ni cambia; consumir otra vez conserva el instante viejo.
SELECT test.expect_error($$UPDATE iam.mfa_enrollments SET consumed_at = NULL WHERE id = 'e0000000-0000-0000-0000-0000000000e1'$$, '42501', 'un token consumido no se reabre');
SELECT test.expect_error($$UPDATE iam.mfa_enrollments SET email_sends = 3, email_code_hmac = sha256('h-otra') WHERE id = 'e0000000-0000-0000-0000-0000000000e1'$$,
  '42501', 'un token consumido no recibe reenvios');
BEGIN;
UPDATE iam.mfa_enrollments SET consumed_at = now() + interval '1 hour' WHERE user_id = 'eeeeeeee-0000-0000-0000-000000000002';
SELECT test.assert((SELECT consumed_at < now() FROM iam.mfa_enrollments WHERE id = 'e0000000-0000-0000-0000-0000000000e1'),
                   'consumir otra vez (UPDATE masivo del restablecimiento) conserva el instante viejo');
SELECT test.assert((SELECT consumed_at = now() FROM iam.mfa_enrollments WHERE id = 'e0000000-0000-0000-0000-0000000000e2'),
                   'consumed_at lo fija el servidor');
ROLLBACK;
-- Trigger, UPDATE de un token vivo: envios solo suben, el codigo cambia solo con un envio, el secreto no rota, email_sent_at no se finge.
SELECT test.expect_error($$UPDATE iam.mfa_enrollments SET email_code_hmac = sha256('h-sin-envio') WHERE id = 'e0000000-0000-0000-0000-0000000000e2'$$,
  '42501', 'un codigo nuevo exige un envio nuevo');
SELECT test.expect_error($$UPDATE iam.mfa_enrollments SET totp_secret_enc = '\x02' WHERE id = 'e0000000-0000-0000-0000-0000000000e2'$$,
  '42501', 'el secreto pendiente no rota');
BEGIN;
UPDATE iam.mfa_enrollments SET email_code_hmac = sha256('h-3'), email_sends = 2 WHERE id = 'e0000000-0000-0000-0000-0000000000e2';
SELECT test.expect_error($$UPDATE iam.mfa_enrollments SET email_sends = 1 WHERE id = 'e0000000-0000-0000-0000-0000000000e2'$$,
  '42501', 'email_sends no baja');
UPDATE iam.mfa_enrollments SET email_code_hmac = sha256('h-4'), email_sends = 3 WHERE id = 'e0000000-0000-0000-0000-0000000000e2';
SELECT test.expect_error($$UPDATE iam.mfa_enrollments SET email_sends = 4, email_code_hmac = sha256('h-5') WHERE id = 'e0000000-0000-0000-0000-0000000000e2'$$,
  '23514', 'mas de 3 envios por token');
UPDATE iam.mfa_enrollments SET email_sent_at = now() - interval '1 hour' WHERE id = 'e0000000-0000-0000-0000-0000000000e2';
SELECT test.assert((SELECT email_sent_at = now() FROM iam.mfa_enrollments WHERE id = 'e0000000-0000-0000-0000-0000000000e2'),
                   'email_sent_at no se finge: lo fija el servidor en cada envio');
ROLLBACK;
\echo '== I1. iam.user_totp (#231): wanka_app lee, inserta y actualiza; una fila por usuario; sin DELETE ni TRUNCATE =='
INSERT INTO iam.user_totp (user_id, secret_enc, recovery_codes_hash) VALUES ('eeeeeeee-0000-0000-0000-000000000002', '\x01', ARRAY['v1:' || repeat('A', 43),'v2:' || repeat('b', 43)]);
UPDATE iam.user_totp SET confirmed_at = now(), last_used_step = 57000000, recovery_codes_hash = ARRAY['v2:' || repeat('b', 43)] WHERE user_id = 'eeeeeeee-0000-0000-0000-000000000002';
SELECT test.assert((SELECT confirmed_at IS NOT NULL AND last_used_step = 57000000 AND recovery_codes_hash = ARRAY['v2:' || repeat('b', 43)] FROM iam.user_totp
                     WHERE user_id = 'eeeeeeee-0000-0000-0000-000000000002'), 'wanka_app confirma el TOTP, avanza last_used_step y quita el codigo usado');
SELECT test.expect_error($$INSERT INTO iam.user_totp (user_id, secret_enc) VALUES ('eeeeeeee-0000-0000-0000-000000000002', '\x02')$$, '23505', 'a lo sumo una fila TOTP por usuario');
SELECT test.expect_error($$INSERT INTO iam.user_totp (user_id, secret_enc) VALUES (gen_random_uuid(), '\x02')$$, '23503', 'user_totp.user_id es FK a iam.users');
SELECT test.expect_error($$UPDATE iam.user_totp SET recovery_codes_hash = ARRAY['ABCD-EFGH']$$, '23514', 'un codigo de recuperacion en claro no entra');
SELECT test.expect_error($$UPDATE iam.user_totp SET recovery_codes_hash = ARRAY['v1:' || repeat('A', 42) || '=']$$, '23514', 'el relleno = no entra');
SELECT test.expect_error($$UPDATE iam.user_totp SET recovery_codes_hash = ARRAY['v1:' || repeat('A', 43), '']$$, '23514', 'un elemento vacio no entra');
SELECT test.expect_error($$UPDATE iam.user_totp SET recovery_codes_hash = ARRAY['v1:aa']$$, '23514', 'un codigo de recuperacion que no es un HMAC de 43 caracteres no entra');
SELECT test.expect_error($$UPDATE iam.user_totp SET recovery_codes_hash = ARRAY['v1:' || repeat('A', 43) || ',' || 'v1:' || repeat('B', 43)]$$, '23514', 'un elemento con coma no entra');
UPDATE iam.user_totp SET recovery_codes_hash = '{}';
SELECT test.expect_error($$DELETE FROM iam.user_totp$$, '42501', 'wanka_app no borra user_totp');
SELECT test.expect_error($$TRUNCATE iam.user_totp$$, '42501', 'wanka_app no vacia user_totp');

\echo '== I2. iam.user_tokens (#231): CHECK por proposito, trigger, UPDATE solo de consumed_at y consumo condicional =='
-- u1: restablecimiento vivo; u2: invitacion viva; u3: cambio de correo vivo. Quedan guardados para la limpieza del worker (K1e).
BEGIN;
INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, expires_at) VALUES
  ('70000000-0000-0000-0000-0000000000a1', 'eeeeeeee-0000-0000-0000-000000000002', 'password_reset', sha256('wk_pr_prueba'), now() + interval '15 minutes');
INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, membership_id, expires_at) VALUES
  ('70000000-0000-0000-0000-0000000000a2', 'eeeeeeee-0000-0000-0000-000000000002', 'invitation', sha256('wk_iv_prueba'),
   '3e000000-0000-0000-0000-0000000000c1', now() + interval '7 days');
INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, new_email, expires_at) VALUES
  ('70000000-0000-0000-0000-0000000000a3', 'eeeeeeee-0000-0000-0000-000000000002', 'email_change', sha256('wk_ec_prueba'),
   'nuevo@a.pe', now() + interval '24 hours');
COMMIT;
-- propositos y sus columnas
SELECT test.expect_error($$INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'otro', sha256('x1'), now() + interval '5 minutes')$$, '23514', 'purpose fuera de la lista');
SELECT test.expect_error($$INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'invitation', sha256('x2'), now() + interval '5 minutes')$$, '23514', 'invitation sin membership_id');
SELECT test.expect_error($$INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, membership_id, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'password_reset', sha256('x3'), gen_random_uuid(), now() + interval '5 minutes')$$,
  '23514', 'membership_id solo en invitation');
SELECT test.expect_error($$INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'email_change', sha256('x4'), now() + interval '5 minutes')$$, '23514', 'email_change sin new_email');
SELECT test.expect_error($$INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, new_email, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'password_reset', sha256('x5'), 'a@a.pe', now() + interval '5 minutes')$$,
  '23514', 'new_email solo en email_change');
SELECT test.expect_error($$INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, new_email, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'email_change', sha256('x6'), 'Nuevo@A.pe', now() + interval '5 minutes')$$,
  '23514', 'new_email con mayusculas');
-- vigencia maxima por proposito: el borde pasa y el siguiente falla
SELECT test.expect_error($$INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'password_reset', sha256('x7'), now() + interval '16 minutes')$$, '23514', 'password_reset de mas de 15 min');
SELECT test.expect_error($$INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, membership_id, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'invitation', sha256('x8'), gen_random_uuid(), now() + interval '169 hours')$$, '23514', 'invitation de mas de 7 dias');
SELECT test.expect_error($$INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, new_email, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'email_change', sha256('x9'), 'a@a.pe', now() + interval '25 hours')$$, '23514', 'email_change de mas de 24 h');
SELECT test.expect_error($$INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'password_reset', sha256('xa'), now())$$, '23514', 'token ya vencido al nacer');
SELECT test.expect_error($$INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'password_reset', '\x00', now() + interval '5 minutes')$$, '23514', 'token_hash de 32 bytes');
SELECT test.expect_error($$INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'password_reset', sha256('wk_pr_prueba'), now() + interval '5 minutes')$$, '23505', 'token_hash unico');
-- el borde exacto pasa
BEGIN;
INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, expires_at) VALUES
  (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'password_reset', sha256('b1'), now() + interval '15 minutes'),
  (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'password_reset', sha256('b2'), now() + interval '1 microsecond');
INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, membership_id, expires_at) VALUES
  (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'invitation', sha256('b3'), gen_random_uuid(), now() + interval '168 hours');
ROLLBACK;
-- trigger, INSERT: created_at lo fija el servidor y consumed_at nace NULL
SELECT test.expect_error($$INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, created_at, expires_at)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', 'password_reset', sha256('xb'), now() + interval '30 days' - interval '5 minutes', now() + interval '30 days')$$,
  '23514', 'un created_at futuro no deja un token vigente semanas');
BEGIN;
INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, created_at, consumed_at, expires_at)
  VALUES ('70000000-0000-0000-0000-0000000000a4', 'eeeeeeee-0000-0000-0000-000000000002', 'password_reset', sha256('xc'),
          now() - interval '1 day', now(), now() + interval '5 minutes');
SELECT test.assert((SELECT created_at = now() AND consumed_at IS NULL FROM iam.user_tokens WHERE id = '70000000-0000-0000-0000-0000000000a4'),
                   'el INSERT fija created_at y deja consumed_at en NULL');
ROLLBACK;
-- grants: solo consumed_at se actualiza; ni DELETE ni TRUNCATE
SELECT test.assert(NOT has_column_privilege('wanka_app', 'iam.user_tokens', c, 'UPDATE'), 'wanka_app sin UPDATE en iam.user_tokens.' || c)
  FROM unnest(ARRAY['id','user_id','purpose','token_hash','membership_id','new_email','expires_at','created_at']) AS c;
SELECT test.assert(has_column_privilege('wanka_app', 'iam.user_tokens', 'consumed_at', 'UPDATE'), 'wanka_app actualiza consumed_at');
SELECT test.expect_error($$UPDATE iam.user_tokens SET expires_at = expires_at + interval '1 hour'$$, '42501', 'wanka_app no alarga un token');
SELECT test.expect_error($$UPDATE iam.user_tokens SET token_hash = sha256('otro')$$, '42501', 'wanka_app no cambia token_hash');
SELECT test.expect_error($$UPDATE iam.user_tokens SET user_id = 'eeeeeeee-0000-0000-0000-000000000001'$$, '42501', 'wanka_app no pasa el token a otro usuario');
SELECT test.expect_error($$UPDATE iam.user_tokens SET created_at = now() + interval '1 day'$$, '42501', 'wanka_app no mueve created_at');
SELECT test.expect_error($$DELETE FROM iam.user_tokens$$, '42501', 'wanka_app no borra tokens');
SELECT test.expect_error($$TRUNCATE iam.user_tokens$$, '42501', 'wanka_app no vacia tokens');
-- consumo condicional: dos consumos del mismo token dejan pasar a uno; el instante lo fija el servidor
BEGIN;
DO $$ DECLARE n int; BEGIN
  UPDATE iam.user_tokens SET consumed_at = now() + interval '1 hour'
   WHERE token_hash = sha256('wk_pr_prueba') AND purpose = 'password_reset' AND consumed_at IS NULL AND expires_at > now();
  GET DIAGNOSTICS n = ROW_COUNT;
  PERFORM test.assert(n = 1, 'el primer consumo condicional pasa');
  UPDATE iam.user_tokens SET consumed_at = now()
   WHERE token_hash = sha256('wk_pr_prueba') AND purpose = 'password_reset' AND consumed_at IS NULL AND expires_at > now();
  GET DIAGNOSTICS n = ROW_COUNT;
  PERFORM test.assert(n = 0, 'el segundo consumo condicional del mismo token no pasa');
END $$;
SELECT test.assert((SELECT consumed_at = now() FROM iam.user_tokens WHERE id = '70000000-0000-0000-0000-0000000000a1'), 'consumed_at lo fija el servidor');
ROLLBACK;
-- trigger, UPDATE: un token consumido (en otra transaccion) no se reabre y consumir otra vez conserva el instante viejo
UPDATE iam.user_tokens SET consumed_at = now() WHERE id = '70000000-0000-0000-0000-0000000000a3';
SELECT test.expect_error($$UPDATE iam.user_tokens SET consumed_at = NULL WHERE id = '70000000-0000-0000-0000-0000000000a3'$$, '42501', 'un token consumido no se reabre');
BEGIN;
UPDATE iam.user_tokens SET consumed_at = now() + interval '1 hour' WHERE id = '70000000-0000-0000-0000-0000000000a3';
SELECT test.assert((SELECT consumed_at < now() FROM iam.user_tokens WHERE id = '70000000-0000-0000-0000-0000000000a3'),
                   'consumir otra vez conserva el instante viejo');
ROLLBACK;
\echo '== A12. (wanka_app) org.document_series, org.sites y org.tenant_settings: numera y edita, pero no marca la sincronizacion con Khipu (ADR 0013) =='
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
SELECT test.assert((SELECT count(*) FROM org.document_series) = 2, 'wanka_app ve las series de A');
UPDATE org.document_series SET last_number = last_number + 1 WHERE id = 'd5000000-0000-0000-0000-0000000000a1'
   AND active AND provider_conflict_at IS NULL AND (doc_type NOT IN ('01','03','07','08','09') OR provider_synced_at IS NOT NULL);
SELECT test.assert((SELECT last_number FROM org.document_series WHERE id = 'd5000000-0000-0000-0000-0000000000a1') = 11, 'NextNumber: N+1 en una serie sincronizada');
UPDATE org.tenant_settings SET fiscal_address = 'Av. Nueva 456', version = version + 1, provider_sync_seq = provider_sync_seq + 1;
SELECT test.assert((SELECT provider_sync_seq FROM org.tenant_settings) = 1, 'wanka_app pide la sincronizacion (provider_sync_seq)');
SELECT test.expect_error($$UPDATE org.document_series SET provider_synced_at = now()$$, '42501', 'wanka_app no marca una serie como sincronizada');
UPDATE org.document_series SET provider_conflict_at = '2000-01-01' WHERE id = 'd5000000-0000-0000-0000-0000000000a1';
SELECT test.assert((SELECT provider_conflict_at = now() FROM org.document_series WHERE id = 'd5000000-0000-0000-0000-0000000000a1'),
                   'wanka_app congela al rechazar una reactivacion, y el instante lo fija el servidor');
SELECT test.expect_error($$UPDATE org.document_series SET provider_conflict_at = NULL WHERE id = 'd5000000-0000-0000-0000-0000000000a1'$$,
                         '42501', 'wanka_app no levanta un congelado');
SELECT test.expect_error($$SELECT * FROM einv.company_sync_state$$, '42501', 'wanka_app no ve el arrendamiento de company_sync');
SELECT test.expect_error($$INSERT INTO org.document_series (id, tenant_id, site_id, doc_type, series, provider_synced_at)
                           VALUES (gen_random_uuid(), app.current_tenant(), 'a5000000-0000-0000-0000-0000000000a1', '01', 'F001', now())$$,
                         '42501', 'wanka_app no crea una serie ya sincronizada');
SELECT test.expect_error($$UPDATE org.sites SET provider_synced_version = 99$$, '42501', 'wanka_app no marca un anexo como sincronizado');
SELECT test.expect_error($$UPDATE org.tenant_settings SET provider_synced_version = 99$$, '42501', 'wanka_app no marca los datos fiscales como sincronizados');
SELECT test.expect_error($$DELETE FROM org.document_series$$, '42501', 'ninguna serie se borra');
ROLLBACK;
BEGIN;
SELECT set_config('app.tenant_id','bbbbbbbb-0000-0000-0000-000000000002', true);
SELECT test.assert((SELECT count(*) FROM org.document_series) = 0 AND (SELECT count(*) FROM org.tenant_settings) = 0, 'wanka_app con tenant B no ve la configuracion de A');
ROLLBACK;
