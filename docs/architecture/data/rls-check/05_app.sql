-- Como wanka_app (NOBYPASSRLS), sin app.tenant_id ni app.user_id: lo que hace el middleware en cada petición
\set ON_ERROR_STOP on
\echo '== 1. session_lookup sin variables: sesion en version 1, membresia en version 2 y activa =='
SELECT current_user, NULLIF(current_setting('app.tenant_id', true),'') AS tenant_var_antes;
SELECT session_authz_version, current_authz_version, membership_status, mfa_method,
       CASE WHEN membership_id IS NOT NULL AND (current_authz_version IS NULL OR membership_status <> 'activo') THEN 'revocar'
            WHEN current_authz_version <> session_authz_version THEN 're-resolver rol'
            ELSE 'sin cambios' END AS decision_middleware
  FROM iam.session_lookup(sha256('tok-cajero'));
SELECT NULLIF(current_setting('app.tenant_id', true),'') AS tenant_var_despues_no_se_filtra;

\echo '== 2. lectura directa de iam.memberships por wanka_app sin tenant (RLS: 0 filas) =='
SELECT count(*) AS memberships_visibles FROM iam.memberships;

\echo '== 3. list_my_memberships (vista security_invoker) con solo app.user_id del cajero =='
BEGIN;
SELECT set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000002', true);
SELECT tenant, role, requires_mfa, site, status FROM iam.list_my_memberships;
COMMIT;

\echo '== 4. list_support_requests con app.tenant_id = A (solo las de A) y sin tenant (0) =='
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
SELECT scope, status FROM iam.list_support_requests();
COMMIT;
SELECT count(*) AS sin_tenant FROM iam.list_support_requests();
\set ON_ERROR_STOP off
SELECT count(*) FROM platform.impersonation_grants;
\set ON_ERROR_STOP on

\echo '== 5. decide_impersonation: sin app.session_kind (rechazo), impersonated (rechazo), user (aprueba) =='
\set ON_ERROR_STOP off
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true), set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000001', true);
SELECT iam.decide_impersonation('99999999-0000-0000-0000-000000000001', true);
ROLLBACK;
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true), set_config('app.session_kind','impersonated', true);
SELECT iam.decide_impersonation('99999999-0000-0000-0000-000000000001', true);
ROLLBACK;
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true), set_config('app.session_kind','user', true);
SELECT iam.decide_impersonation('99999999-0000-0000-0000-000000000002', true) AS grant_de_B_desde_A;
ROLLBACK;
\set ON_ERROR_STOP on
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true), set_config('app.user_id','eeeeeeee-0000-0000-0000-000000000001', true), set_config('app.session_kind','user', true);
SELECT iam.decide_impersonation('99999999-0000-0000-0000-000000000001', true) AS decision;
COMMIT;

\echo '== 6. redeem_impersonation sin tenant fijado: grant de B no aprobado (rechazo), grant de A aprobado (crea sesion platform) =='
\set ON_ERROR_STOP off
SELECT iam.redeem_impersonation('\x02'::bytea, gen_random_uuid(), sha256('tok-imp-b'));
\set ON_ERROR_STOP on
SELECT iam.redeem_impersonation('\x01'::bytea, '5e000000-0000-0000-0000-0000000000f1', sha256('tok-imp-a')) AS tenant_impersonado;
SELECT mfa_method, impersonator_id IS NOT NULL AS impersonada, membership_id, current_authz_version FROM iam.session_lookup(sha256('tok-imp-a'));
SELECT NULLIF(current_setting('app.tenant_id', true),'') AS tenant_var_despues_no_se_filtra;
\echo '== 6b. CHECK: sesion platform sin impersonator_id (debe violar) =='
\set ON_ERROR_STOP off
INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, mfa_satisfied, mfa_method)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', sha256('x'), now(), true, 'platform');
INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, mfa_satisfied, mfa_method)
  VALUES (gen_random_uuid(), 'eeeeeeee-0000-0000-0000-000000000002', sha256('y'), now(), true, NULL);
\set ON_ERROR_STOP on
