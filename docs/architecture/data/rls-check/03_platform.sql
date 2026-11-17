-- Como wanka_platform: alta de dos tenants por la función acotada (sin DML directo en org ni iam)
\set ON_ERROR_STOP on
\echo '== provision_tenant como wanka_platform (dueño wanka_definer) =='
SELECT platform_fn.provision_tenant('aaaaaaaa-0000-0000-0000-000000000001','20100000001','Empresa A',
  'eeeeeeee-0000-0000-0000-000000000001','u@a.pe','a0000000-0000-0000-0000-0000000000a1','a5000000-0000-0000-0000-0000000000a1') AS tenant_a;
SELECT platform_fn.provision_tenant('bbbbbbbb-0000-0000-0000-000000000002','20100000002','Empresa B',
  'eeeeeeee-0000-0000-0000-000000000009','v@b.pe','b0000000-0000-0000-0000-0000000000b1','b5000000-0000-0000-0000-0000000000b1') AS tenant_b;
\echo '== DML directo de wanka_platform en iam.memberships (debe fallar: sin GRANT) =='
\set ON_ERROR_STOP off
INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status) VALUES (gen_random_uuid(),'aaaaaaaa-0000-0000-0000-000000000001','eeeeeeee-0000-0000-0000-000000000001','a0000000-0000-0000-0000-0000000000a1','activo');
\set ON_ERROR_STOP on
\echo '== suspend_tenant(B) y lectura via platform_rw =='
SELECT platform_fn.suspend_tenant('bbbbbbbb-0000-0000-0000-000000000002');
SELECT name, status FROM org.tenants ORDER BY name;
\echo '== request_impersonation sobre A (queda solicitado) =='
SELECT platform_fn.request_impersonation('99999999-0000-0000-0000-000000000001','5aff0000-0000-0000-0000-000000000001',
  'aaaaaaaa-0000-0000-0000-000000000001','7c000000-0000-0000-0000-000000000001','full','\x01'::bytea) AS grant_id;
SELECT platform_fn.request_impersonation('99999999-0000-0000-0000-000000000002','5aff0000-0000-0000-0000-000000000001',
  'bbbbbbbb-0000-0000-0000-000000000002','7c000000-0000-0000-0000-000000000002','read_only','\x02'::bytea) AS grant_id_b;
SHOW app.tenant_id;
