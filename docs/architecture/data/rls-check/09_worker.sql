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
SELECT test.assert((SELECT count(*) = 5 AND bool_and(c.relrowsecurity AND c.relforcerowsecurity) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                     WHERE (n.nspname, c.relname) IN (('org','tenants'), ('org','sites'), ('iam','roles'), ('iam','memberships'), ('audit','events'))),
                   'tablas de tenant con RLS y FORCE RLS');
SELECT test.assert(NOT has_schema_privilege('wanka_definer', 'iam', 'CREATE')
                   AND NOT has_schema_privilege('wanka_definer', 'ops', 'CREATE')
                   AND NOT has_schema_privilege('wanka_definer', 'platform_fn', 'CREATE'),
                   'wanka_definer sin CREATE tras el cambio de duenio');
