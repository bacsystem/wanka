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
