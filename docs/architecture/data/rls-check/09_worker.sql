\set ON_ERROR_STOP on
\echo '== K1. (wanka_worker) purge_closed_tenant(B) sin tenant fijado =='
SELECT test.assert(platform_fn.purge_closed_tenant('bbbbbbbb-0000-0000-0000-000000000002') = 1, 'purge borra la sede de B');
SELECT test.assert(NULLIF(current_setting('app.tenant_id', true),'') IS NULL, 'purge no filtra app.tenant_id');
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
