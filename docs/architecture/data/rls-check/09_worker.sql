\set ON_ERROR_STOP on
\echo '== 9. (wanka_worker) purge_closed_tenant(B) sin tenant fijado =='
SELECT platform_fn.purge_closed_tenant('bbbbbbbb-0000-0000-0000-000000000002') AS sedes_borradas;
SELECT NULLIF(current_setting('app.tenant_id', true),'') AS tenant_var_despues_no_se_filtra;
\echo '== 10. catalogo: dueño de cada funcion SECURITY DEFINER y atributos de los roles =='
SELECT p.oid::regprocedure AS funcion, pg_get_userbyid(p.proowner) AS dueno, p.prosecdef AS security_definer, p.proconfig AS config
  FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
 WHERE n.nspname IN ('iam','ops','platform_fn') ORDER BY 1;
SELECT rolname, rolbypassrls, rolsuper, rolcanlogin FROM pg_roles WHERE rolname LIKE 'wanka_%' ORDER BY 1;
SELECT c.oid::regclass AS tabla, c.relrowsecurity AS rls, c.relforcerowsecurity AS force_rls, pg_get_userbyid(c.relowner) AS dueno
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
 WHERE n.nspname IN ('iam','org','audit') AND c.relkind = 'r' ORDER BY 1;
