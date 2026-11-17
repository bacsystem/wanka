-- Como wanka_platform: alta, suspension y solicitud de impersonacion solo por funciones acotadas.
-- Cada caso compara contra lo esperado con test.assert / test.expect_error y aborta (exit != 0) si no coincide.
\set ON_ERROR_STOP on
\echo '== P1. provision_tenant (duenio wanka_definer) crea A y B =='
SELECT test.assert(platform_fn.provision_tenant('aaaaaaaa-0000-0000-0000-000000000001','20100000001','Empresa A',
  'eeeeeeee-0000-0000-0000-000000000001','u@a.pe','a0000000-0000-0000-0000-0000000000a1','a5000000-0000-0000-0000-0000000000a1')
  = 'aaaaaaaa-0000-0000-0000-000000000001', 'provision_tenant A');
SELECT test.assert(platform_fn.provision_tenant('bbbbbbbb-0000-0000-0000-000000000002','20100000002','Empresa B',
  'eeeeeeee-0000-0000-0000-000000000009','v@b.pe','b0000000-0000-0000-0000-0000000000b1','b5000000-0000-0000-0000-0000000000b1')
  = 'bbbbbbbb-0000-0000-0000-000000000002', 'provision_tenant B');
\echo '== P2. DML directo de wanka_platform en iam.memberships =='
SELECT test.expect_error($$INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status) VALUES (gen_random_uuid(),
  'aaaaaaaa-0000-0000-0000-000000000001','eeeeeeee-0000-0000-0000-000000000001','a0000000-0000-0000-0000-0000000000a1','activo')$$,
  '42501', 'wanka_platform sin DML en iam');
\echo '== P3. suspend_tenant(B) y lectura via platform_rw =='
SELECT platform_fn.suspend_tenant('bbbbbbbb-0000-0000-0000-000000000002');
SELECT test.assert((SELECT string_agg(name || ':' || status, ',' ORDER BY name) FROM org.tenants) = 'Empresa A:trial,Empresa B:suspendida',
  'B suspendida, A intacta');
\echo '== P4. request_impersonation sobre A y B (quedan solicitado) =='
SELECT test.assert(platform_fn.request_impersonation('99999999-0000-0000-0000-000000000001','5aff0000-0000-0000-0000-000000000001',
  'aaaaaaaa-0000-0000-0000-000000000001','7c000000-0000-0000-0000-000000000001','full','\x01'::bytea) IS NOT NULL, 'solicitud A');
SELECT test.assert(platform_fn.request_impersonation('99999999-0000-0000-0000-000000000002','5aff0000-0000-0000-0000-000000000001',
  'bbbbbbbb-0000-0000-0000-000000000002','7c000000-0000-0000-0000-000000000002','read_only','\x02'::bytea) IS NOT NULL, 'solicitud B');
SELECT test.assert(NULLIF(current_setting('app.tenant_id', true), '') IS NULL, 'las funciones no filtran app.tenant_id al llamador');
