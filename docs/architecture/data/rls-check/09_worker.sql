\set ON_ERROR_STOP on
\echo '== K1. (wanka_worker) purge_closed_tenant(B) sin tenant fijado =='
SELECT test.assert(platform_fn.purge_closed_tenant('bbbbbbbb-0000-0000-0000-000000000002') = 1, 'purge borra la sede de B');
SELECT test.assert(NULLIF(current_setting('app.tenant_id', true),'') IS NULL, 'purge no filtra app.tenant_id');
\echo '== K1b. (wanka_worker) iam.sessions: solo DELETE (limpieza); tablas de tenant solo SELECT y con RLS =='
SELECT test.expect_error('SELECT access_hash FROM iam.sessions', '42501', 'wanka_worker no lee los hashes de iam.sessions');
SELECT test.expect_error('SELECT user_id FROM iam.sessions', '42501', 'wanka_worker no lee user_id de iam.sessions');
DO $$ DECLARE n int; BEGIN DELETE FROM iam.sessions WHERE created_at < now() + interval '1 day'; GET DIAGNOSTICS n = ROW_COUNT;
  PERFORM test.assert(n > 0, 'wanka_worker borra sesiones por created_at (SELECT por columna)'); RAISE EXCEPTION 'rollback' USING ERRCODE = 'P0001';
  EXCEPTION WHEN SQLSTATE 'P0001' THEN NULL; END $$;
SELECT test.expect_error('INSERT INTO iam.sessions DEFAULT VALUES', '42501', 'wanka_worker no inserta sesiones');
SELECT test.expect_error($$UPDATE iam.memberships SET status = 'activo'$$, '42501', 'wanka_worker no escribe membresias');
SELECT test.assert((SELECT count(*) FROM iam.memberships) = 0 AND (SELECT count(*) FROM org.tenants) = 0, 'worker sin tenant: 0 filas (RLS)');
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
SELECT test.assert((SELECT count(*) FROM iam.memberships) = 1 AND (SELECT count(*) FROM iam.roles) = 2 AND (SELECT count(*) FROM org.tenants) = 1,
                   'worker con tenant A: lee sus roles y su empresa (solo la membresia del propietario: la del cajero ya se borro)');
COMMIT;
DO $$ DECLARE n int; BEGIN DELETE FROM iam.sessions; GET DIAGNOSTICS n = ROW_COUNT; PERFORM test.assert(n > 0, 'wanka_worker borra sesiones'); RAISE EXCEPTION 'rollback' USING ERRCODE = 'P0001'; EXCEPTION WHEN SQLSTATE 'P0001' THEN NULL; END $$;
\echo '== K1c. (wanka_worker) tablas de River: todo el DML de la cola, sin river_migration =='
SELECT test.river_worker_full();
\echo '== K1d. (wanka_worker) audit.events: escribe como system sin usuario, solo insercion; crea particiones (#173) =='
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true), set_config('app.session_kind','system', true);
INSERT INTO audit.events (tenant_id, actor_type, action) VALUES (app.current_tenant(), 'system', 'rls.check:job');
SELECT test.assert((SELECT count(*) FROM audit.events WHERE action = 'rls.check:job') = 1, 'wanka_worker escribe y lee sus eventos system con tenant fijado');
SELECT test.expect_error($$INSERT INTO audit.events (tenant_id, actor_type, actor_user_id, action)
                           VALUES (app.current_tenant(), 'system', 'eeeeeeee-0000-0000-0000-000000000001', 'rls.check:x')$$,
                         '23514', 'un job no inventa un usuario');
SELECT test.expect_error($$INSERT INTO audit.events (tenant_id, actor_type, action) VALUES (app.current_tenant(), 'user', 'rls.check:x')$$,
                         '23514', 'una fila user sin usuario es imposible tampoco desde el worker');
SELECT test.expect_error($$UPDATE audit.events SET action = 'rls.check:z'$$, '42501', 'wanka_worker no actualiza audit.events');
SELECT test.expect_error('DELETE FROM audit.events', '42501', 'wanka_worker no borra audit.events');
SELECT test.expect_error('TRUNCATE audit.events', '42501', 'wanka_worker no vacia audit.events');
ROLLBACK;
SELECT test.assert(audit.ensure_partitions((now() AT TIME ZONE 'UTC')::date, 3) = 0, 'ensure_partitions es idempotente: la migracion ya dejo el mes actual y los 3 siguientes');
SELECT test.assert(audit.ensure_partitions((now() AT TIME ZONE 'UTC')::date + 130, 0) >= 0, 'ensure_partitions acepta una fecha futura cercana');
SELECT test.expect_error($$SELECT audit.ensure_partitions(current_date, 13)$$, '22023', 'ensure_partitions rechaza mas de 12 meses por delante');
SELECT test.expect_error($$SELECT audit.ensure_partitions(date '2025-01-01', 0)$$, '22023', 'ensure_partitions rechaza fechas anteriores a 2026');
SELECT test.assert((SELECT p.prosecdef AND pg_get_userbyid(p.proowner) = 'wanka_migrator' AND p.proconfig @> ARRAY['search_path=pg_catalog, pg_temp']
                      FROM pg_proc p WHERE p.oid = 'audit.ensure_partitions(date,integer)'::regprocedure),
                   'audit.ensure_partitions: SECURITY DEFINER, dueno wanka_migrator (el de la tabla) y search_path fijo');
SELECT test.assert(NOT has_function_privilege('public', 'audit.ensure_partitions(date,integer)', 'EXECUTE')
                   AND NOT has_function_privilege('wanka_app', 'audit.ensure_partitions(date,integer)', 'EXECUTE')
                   AND NOT has_function_privilege('wanka_webhooks', 'audit.ensure_partitions(date,integer)', 'EXECUTE')
                   AND NOT has_function_privilege('wanka_platform', 'audit.ensure_partitions(date,integer)', 'EXECUTE')
                   AND has_function_privilege('wanka_worker', 'audit.ensure_partitions(date,integer)', 'EXECUTE'),
                   'solo wanka_worker ejecuta audit.ensure_partitions');
\echo '== K1c. (wanka_worker) iam.mfa_enrollments: limpieza por expires_at sin leer hashes ni secretos (#25) =='
SELECT test.expect_error('SELECT token_hash FROM iam.mfa_enrollments', '42501', 'wanka_worker no lee token_hash');
SELECT test.expect_error('SELECT email_code_hmac FROM iam.mfa_enrollments', '42501', 'wanka_worker no lee el HMAC del emailCode');
SELECT test.expect_error('SELECT totp_secret_enc FROM iam.mfa_enrollments', '42501', 'wanka_worker no lee el secreto pendiente');
DO $$ DECLARE n int; BEGIN DELETE FROM iam.mfa_enrollments WHERE expires_at < now() + interval '1 day'; GET DIAGNOSTICS n = ROW_COUNT;
  PERFORM test.assert(n > 0, 'wanka_worker borra tokens de enrolamiento por expires_at'); RAISE EXCEPTION 'rollback' USING ERRCODE = 'P0001';
  EXCEPTION WHEN SQLSTATE 'P0001' THEN NULL; END $$;
SELECT test.expect_error('INSERT INTO iam.mfa_enrollments DEFAULT VALUES', '42501', 'wanka_worker no inserta tokens de enrolamiento');
\echo '== K2. catalogo: duenio y search_path de las funciones SECURITY DEFINER, roles y FORCE RLS =='
SELECT test.assert((SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
                     WHERE p.prosecdef AND n.nspname IN ('iam','ops','platform_fn')
                       AND pg_get_userbyid(p.proowner) = 'wanka_definer'
                       AND p.proconfig @> ARRAY['search_path=pg_catalog, pg_temp']) = 9,
                   '9 funciones SECURITY DEFINER con duenio wanka_definer y search_path fijo');
SELECT test.assert((SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
                     WHERE p.prosecdef AND n.nspname IN ('iam','ops','platform_fn')
                       AND pg_get_userbyid(p.proowner) <> 'wanka_definer') = 0, 'ninguna SECURITY DEFINER con otro duenio');
SELECT test.assert((SELECT count(*) FROM pg_roles WHERE rolname LIKE 'wanka_%' AND (rolbypassrls OR rolsuper)) = 0,
                   'ningun rol wanka_* con BYPASSRLS ni superusuario');
SELECT test.assert((SELECT count(*) = 6 AND bool_and(c.relrowsecurity AND c.relforcerowsecurity) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                     WHERE (n.nspname, c.relname) IN (('org','tenants'), ('org','sites'), ('iam','roles'), ('iam','memberships'), ('audit','events'), ('ops','idempotency_keys'))),
                   'tablas de tenant con RLS y FORCE RLS');
SELECT test.assert(NOT has_schema_privilege('wanka_definer', 'iam', 'CREATE')
                   AND NOT has_schema_privilege('wanka_definer', 'ops', 'CREATE')
                   AND NOT has_schema_privilege('wanka_definer', 'platform_fn', 'CREATE'),
                   'wanka_definer sin CREATE tras el cambio de duenio');
\echo '== K1d. (wanka_worker) ops.idempotency_keys: limpia cross-tenant solo lo de mas de 24 h, sin leer hash ni respuesta =='
SELECT test.expect_error('SELECT request_hash FROM ops.idempotency_keys', '42501', 'wanka_worker no lee request_hash');
SELECT test.expect_error('SELECT response FROM ops.idempotency_keys', '42501', 'wanka_worker no lee response');
SELECT test.expect_error('SELECT key FROM ops.idempotency_keys', '42501', 'wanka_worker no lee las claves');
SELECT test.expect_error($$INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'w', sha256('x'))$$,
  '42501', 'wanka_worker no inserta claves');
SELECT test.expect_error($$UPDATE ops.idempotency_keys SET status_code = 200$$, '42501', 'wanka_worker no actualiza claves');
DO $$ DECLARE n int; BEGIN
  -- Sin tenant fijado: una limpieza que se pasa de largo no borra las vigentes (la politica purge_expired lo impide).
  DELETE FROM ops.idempotency_keys WHERE created_at > now() - interval '24 hours'; GET DIAGNOSTICS n = ROW_COUNT;
  PERFORM test.assert(n = 0, 'wanka_worker no borra claves de menos de 24 h');
  DELETE FROM ops.idempotency_keys; GET DIAGNOSTICS n = ROW_COUNT;
  PERFORM test.assert(n = 2, 'wanka_worker borra las 2 vencidas (una de A y una de B) y ninguna vigente, sin tenant fijado');
  RAISE EXCEPTION 'rollback' USING ERRCODE = 'RR177';
  EXCEPTION WHEN SQLSTATE 'RR177' THEN NULL; END $$;
DO $$ DECLARE n int; BEGIN
  DELETE FROM ops.idempotency_keys WHERE created_at < now() - interval '24 hours'; GET DIAGNOSTICS n = ROW_COUNT;
  PERFORM test.assert(n = 2, 'el filtro de created_at (SELECT por columna) alcanza para el DELETE por antigüedad');
  RAISE EXCEPTION 'rollback' USING ERRCODE = 'RR177';
  EXCEPTION WHEN SQLSTATE 'RR177' THEN NULL; END $$;
DO $$ DECLARE n int; BEGIN
  -- Con tenant fijado la rama cross-tenant no aplica (app.current_tenant() IS NULL): solo la vencida de A.
  PERFORM set_config('app.tenant_id', 'aaaaaaaa-0000-0000-0000-000000000001', true);
  DELETE FROM ops.idempotency_keys WHERE created_at < now() - interval '24 hours'; GET DIAGNOSTICS n = ROW_COUNT;
  PERFORM test.assert(n = 1, 'con tenant A fijado, la limpieza no alcanza las vencidas de B');
  RAISE EXCEPTION 'rollback' USING ERRCODE = 'RR177';
  EXCEPTION WHEN SQLSTATE 'RR177' THEN NULL; END $$;
\echo '== K4. ops.exchange_rates (issue #170): wanka_worker solo inserta, y la base valida moneda, fuente y valores =='
SELECT test.expect_error('SELECT buy FROM ops.exchange_rates', '42501', 'wanka_worker no lee los tipos de cambio');
SELECT test.expect_error($$UPDATE ops.exchange_rates SET buy = 1$$, '42501', 'wanka_worker no actualiza tipos de cambio');
SELECT test.expect_error($$DELETE FROM ops.exchange_rates$$, '42501', 'wanka_worker no borra tipos de cambio');
SELECT test.expect_error($$INSERT INTO ops.exchange_rates VALUES ('2026-11-20', 'EUR', 3.7, 3.8, 'official')$$, '23514', 'moneda distinta de USD');
SELECT test.expect_error($$INSERT INTO ops.exchange_rates VALUES ('2026-11-20', 'USD', 0, 3.8, 'official')$$, '23514', 'compra en cero');
SELECT test.expect_error($$INSERT INTO ops.exchange_rates VALUES ('2026-11-20', 'USD', 3.7, -3.8, 'official')$$, '23514', 'venta negativa');
SELECT test.expect_error($$INSERT INTO ops.exchange_rates VALUES ('2026-11-20', 'USD', 3.7, 3.8, 'sbs')$$, '23514', 'fuente desconocida');
INSERT INTO ops.exchange_rates VALUES ('2026-11-20', 'USD', 3.7, 3.8, 'manual');
\echo '== E2. entitlements (#171): wanka_worker lee las tablas de entitlements y escribe SOLO platform.tenant_usage =='
SELECT test.assert((SELECT count(*) FROM platform.modules) >= 21 AND (SELECT count(*) FROM platform.plans) >= 1
                   AND (SELECT count(*) FROM platform.subscriptions) = 0, 'wanka_worker lee los entitlements');
BEGIN;
INSERT INTO platform.tenant_usage (tenant_id, period, cpe_issued) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', '2026-11-01', 1);
INSERT INTO platform.tenant_usage (tenant_id, period, cpe_issued) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', '2026-11-01', 2)
  ON CONFLICT (tenant_id, period) DO UPDATE SET cpe_issued = platform.tenant_usage.cpe_issued + EXCLUDED.cpe_issued;
SELECT test.assert((SELECT cpe_issued FROM platform.tenant_usage WHERE tenant_id = 'aaaaaaaa-0000-0000-0000-000000000001') = 3, 'wanka_worker inserta y actualiza el uso');
ROLLBACK;
SELECT test.expect_error($$DELETE FROM platform.tenant_usage$$, '42501', 'wanka_worker no borra uso');
SELECT test.expect_error($$INSERT INTO platform.subscriptions (id, tenant_id, plan_id, status)
  VALUES (gen_random_uuid(), 'aaaaaaaa-0000-0000-0000-000000000001', (SELECT id FROM platform.plans LIMIT 1), 'activa')$$, '42501', 'wanka_worker no inserta suscripciones');
SELECT test.expect_error($$UPDATE platform.subscriptions SET status = 'activa'$$, '42501', 'wanka_worker no actualiza suscripciones');
SELECT test.expect_error($$UPDATE platform.plans SET name = 'x'$$, '42501', 'wanka_worker no edita planes');
SELECT test.expect_error($$INSERT INTO platform.tenant_module_overrides (tenant_id, module_code, enabled) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'ventas', true)$$,
                         '42501', 'wanka_worker no escribe overrides');
\echo '== K3. (wanka_worker) iam.permissions es de solo lectura (issue #172) =='
SELECT test.assert((SELECT count(*) FROM iam.permissions) > 100, 'wanka_worker lee el catalogo de permisos');
SELECT test.expect_error($$INSERT INTO iam.permissions (code, description) VALUES ('ventas.pos:void', 'Anular ventas')$$, '42501', 'wanka_worker no inserta permisos');
SELECT test.expect_error($$UPDATE iam.permissions SET description = 'x'$$, '42501', 'wanka_worker no actualiza permisos');
SELECT test.expect_error($$DELETE FROM iam.permissions$$, '42501', 'wanka_worker no borra permisos');
\echo '== K5. (wanka_worker) sincronizacion con Khipu (ADR 0013): solo marca sincronizado o en conflicto, nunca numera ni edita org =='
SELECT test.assert((SELECT count(*) FROM org.document_series) = 0, 'worker sin tenant: 0 series (RLS)');
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
UPDATE org.document_series SET provider_conflict_at = now() WHERE id = 'd5000000-0000-0000-0000-0000000000a1';
UPDATE org.document_series SET provider_synced_at = now() WHERE id = 'd5000000-0000-0000-0000-0000000000a1' AND last_number = 10;
UPDATE org.sites SET provider_synced_version = version WHERE id = 'a5000000-0000-0000-0000-0000000000a1';
UPDATE org.tenant_settings SET provider_synced_version = version;
SELECT test.assert((SELECT provider_conflict_at IS NOT NULL FROM org.document_series WHERE id = 'd5000000-0000-0000-0000-0000000000a1')
                   AND (SELECT provider_synced_version = 1 FROM org.tenant_settings), 'wanka_worker marca conflicto y sincronizacion');
SELECT test.expect_error($$UPDATE org.document_series SET last_number = 99$$, '42501', 'wanka_worker no numera');
SELECT test.expect_error($$UPDATE org.document_series SET active = true$$, '42501', 'wanka_worker no reactiva series');
SELECT test.expect_error($$UPDATE org.document_series SET series = 'B999'$$, '42501', 'wanka_worker no renombra series');
SELECT test.expect_error($$INSERT INTO org.document_series (id, tenant_id, site_id, doc_type, series)
                           VALUES (gen_random_uuid(), app.current_tenant(), 'a5000000-0000-0000-0000-0000000000a1', '01', 'F009')$$,
                         '42501', 'wanka_worker no crea series');
SELECT test.expect_error($$DELETE FROM org.document_series$$, '42501', 'wanka_worker no borra series');
SELECT test.expect_error($$UPDATE org.sites SET sunat_annex = '0002'$$, '42501', 'wanka_worker no cambia el anexo');
SELECT test.expect_error($$UPDATE org.tenant_settings SET fiscal_address = 'x'$$, '42501', 'wanka_worker no edita los datos fiscales');
SELECT test.expect_error($$UPDATE org.document_series SET provider_synced_at = now() WHERE doc_type = 'PRO'$$, '23514', 'una serie interna nunca se sincroniza con Khipu');
ROLLBACK;
