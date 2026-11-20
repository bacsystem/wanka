-- Como wanka_migrator (NOBYPASSRLS, duenio con FORCE RLS): datos de prueba con el tenant fijado, como la app.
\set ON_ERROR_STOP on
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
UPDATE iam.memberships SET status = 'activo' WHERE user_id = 'eeeeeeee-0000-0000-0000-000000000001';
-- ADR 0013: configuracion de A con una serie de boleta ya sincronizada con Khipu y una proforma (interna)
UPDATE org.sites SET sunat_annex = '0000' WHERE id = 'a5000000-0000-0000-0000-0000000000a1';
INSERT INTO org.tenant_settings (tenant_id, ubigeo, fiscal_address) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', '150122', 'Av. Prueba 123');
INSERT INTO org.document_series (id, tenant_id, site_id, doc_type, series, last_number, provider_synced_at) VALUES
  ('d5000000-0000-0000-0000-0000000000a1','aaaaaaaa-0000-0000-0000-000000000001','a5000000-0000-0000-0000-0000000000a1','03','B001', 10, now()),
  ('d5000000-0000-0000-0000-0000000000a2','aaaaaaaa-0000-0000-0000-000000000001','a5000000-0000-0000-0000-0000000000a1','PRO','P001', 0, NULL);
INSERT INTO iam.roles (id, tenant_id, name, requires_mfa) VALUES ('a0000000-0000-0000-0000-0000000000a2','aaaaaaaa-0000-0000-0000-000000000001','Cajero', false);
INSERT INTO iam.users (id, email) VALUES ('eeeeeeee-0000-0000-0000-000000000002','cajero@a.pe');
INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status, authz_version)
  VALUES ('3e000000-0000-0000-0000-0000000000c1','aaaaaaaa-0000-0000-0000-000000000001','eeeeeeee-0000-0000-0000-000000000002','a0000000-0000-0000-0000-0000000000a2','activo', 1);
COMMIT;
-- sesion de tenant del cajero (version 1, con sede y caja) y una sesion a medio login (mfa_satisfied = false)
INSERT INTO iam.sessions (id, user_id, tenant_id, site_id, cash_register_id, membership_id, access_hash, access_expires_at, authz_version, mfa_satisfied, mfa_method)
  VALUES ('5e000000-0000-0000-0000-000000000001','eeeeeeee-0000-0000-0000-000000000002','aaaaaaaa-0000-0000-0000-000000000001',
          'a5000000-0000-0000-0000-0000000000a1','ca000000-0000-0000-0000-0000000000a1','3e000000-0000-0000-0000-0000000000c1',
          sha256('tok-cajero'), now() + interval '15 minutes', 1, true, 'none'),
         ('5e000000-0000-0000-0000-000000000002','eeeeeeee-0000-0000-0000-000000000001', NULL, NULL, NULL, NULL,
          sha256('tok-a-medio-login'), now() + interval '15 minutes', NULL, false, 'totp');
-- ascenso del cajero: authz_version pasa a 2
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
UPDATE iam.memberships SET authz_version = 2 WHERE id = '3e000000-0000-0000-0000-0000000000c1';
COMMIT;
-- ex miembro de B: su membresia esta suspendida (issue #187). Sin tenant fijado no debe ver nada de B.
BEGIN;
SELECT set_config('app.tenant_id','bbbbbbbb-0000-0000-0000-000000000002', true);
INSERT INTO iam.users (id, email) VALUES ('eeeeeeee-0000-0000-0000-000000000003','ex@b.pe');
INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status)
  VALUES ('3e000000-0000-0000-0000-0000000000d1','bbbbbbbb-0000-0000-0000-000000000002','eeeeeeee-0000-0000-0000-000000000003','b0000000-0000-0000-0000-0000000000b1','suspendido');
COMMIT;
-- Identidad de la ola 1, F2 (#232): un rol base, sedes, cajas y el alcance de las membresias. Las plantillas (catalogo global;
-- solo el duenio las escribe) las siembra la migracion real 20261120120000 (#233): aqui no se insertan otras. El cajero solo opera en la sede 0000; 'multi' es miembro de A y de B (prueba de dos tenants).
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
INSERT INTO iam.roles (id, tenant_id, name, system, template_code, requires_mfa)
  VALUES ('a0000000-0000-0000-0000-0000000000a3','aaaaaaaa-0000-0000-0000-000000000001','Dueno', true, 'dueno', true);
INSERT INTO org.sites (id, tenant_id, code, name) VALUES ('a5000000-0000-0000-0000-0000000000a2','aaaaaaaa-0000-0000-0000-000000000001','S2','Sede 2');
INSERT INTO org.cash_registers (id, tenant_id, site_id, code) VALUES
  ('ca000000-0000-0000-0000-0000000000a1','aaaaaaaa-0000-0000-0000-000000000001','a5000000-0000-0000-0000-0000000000a1','CAJA-01'),
  ('ca000000-0000-0000-0000-0000000000a2','aaaaaaaa-0000-0000-0000-000000000001','a5000000-0000-0000-0000-0000000000a1','CAJA-02'),
  ('ca000000-0000-0000-0000-0000000000a3','aaaaaaaa-0000-0000-0000-000000000001','a5000000-0000-0000-0000-0000000000a2','CAJA-03');
INSERT INTO iam.role_permissions VALUES ('aaaaaaaa-0000-0000-0000-000000000001','a0000000-0000-0000-0000-0000000000a2','ventas.pos:read');
INSERT INTO iam.users (id, email) VALUES ('eeeeeeee-0000-0000-0000-00000000000a','multi@ab.pe');
INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status)
  VALUES ('3e000000-0000-0000-0000-0000000000e1','aaaaaaaa-0000-0000-0000-000000000001','eeeeeeee-0000-0000-0000-00000000000a','a0000000-0000-0000-0000-0000000000a2','activo');
COMMIT;
BEGIN;
SELECT set_config('app.tenant_id','bbbbbbbb-0000-0000-0000-000000000002', true);
INSERT INTO org.cash_registers (id, tenant_id, site_id, code) VALUES ('cb000000-0000-0000-0000-0000000000b1','bbbbbbbb-0000-0000-0000-000000000002','b5000000-0000-0000-0000-0000000000b1','CAJA-01');
INSERT INTO iam.role_permissions VALUES ('bbbbbbbb-0000-0000-0000-000000000002','b0000000-0000-0000-0000-0000000000b1','ventas.pos:read');
INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status)
  VALUES ('3e000000-0000-0000-0000-0000000000e2','bbbbbbbb-0000-0000-0000-000000000002','eeeeeeee-0000-0000-0000-00000000000a','b0000000-0000-0000-0000-0000000000b1','activo');
COMMIT;
-- El cajero solo opera en la sede 0000 (all_sites = false); la otra sede de A no es de su alcance.
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
UPDATE iam.memberships SET all_sites = false WHERE id = '3e000000-0000-0000-0000-0000000000c1';
INSERT INTO iam.membership_sites VALUES ('aaaaaaaa-0000-0000-0000-000000000001','3e000000-0000-0000-0000-0000000000c1','a5000000-0000-0000-0000-0000000000a1');
COMMIT;
-- El ex miembro de B (suspendido) tenia alcance parcial: su fila de iam.membership_sites no debe leerse sin tenant.
BEGIN;
SELECT set_config('app.tenant_id','bbbbbbbb-0000-0000-0000-000000000002', true);
UPDATE iam.memberships SET all_sites = false WHERE id = '3e000000-0000-0000-0000-0000000000d1';
INSERT INTO iam.membership_sites VALUES ('bbbbbbbb-0000-0000-0000-000000000002','3e000000-0000-0000-0000-0000000000d1','b5000000-0000-0000-0000-0000000000b1');
COMMIT;
INSERT INTO ops.webhook_routes VALUES ('r-0123456789abcdef', 'aaaaaaaa-0000-0000-0000-000000000001', 'khipu', '\xdeadbeef', NULL);
INSERT INTO ops.exchange_rates VALUES ('2026-11-19', 'USD', 3.7, 3.8, 'official');
-- ops.idempotency_keys (issue #177): claves de A y B, vencidas (mas de 24 h) y vigentes. created_at explicito: lo puede fijar el
-- duenio, no wanka_app. Cada insercion con su tenant fijado.
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
INSERT INTO ops.idempotency_keys (tenant_id, principal_id, key, request_hash, status_code, response, created_at) VALUES
  ('aaaaaaaa-0000-0000-0000-000000000001', 'eeeeeeee-0000-0000-0000-000000000001', 'old-a', sha256('h-old-a'), 201, '\x7b7d', now() - interval '25 hours'),
  ('aaaaaaaa-0000-0000-0000-000000000001', 'eeeeeeee-0000-0000-0000-000000000001', 'recent-a', sha256('h-recent-a'), 201, '\x7b7d', now() - interval '23 hours');
COMMIT;
BEGIN;
SELECT set_config('app.tenant_id','bbbbbbbb-0000-0000-0000-000000000002', true);
INSERT INTO ops.idempotency_keys (tenant_id, principal_id, key, request_hash, status_code, response, created_at) VALUES
  ('bbbbbbbb-0000-0000-0000-000000000002', 'eeeeeeee-0000-0000-0000-0000000000b1', 'old-b', sha256('h-old-b'), 201, '\x7b7d', now() - interval '30 hours'),
  ('bbbbbbbb-0000-0000-0000-000000000002', 'eeeeeeee-0000-0000-0000-0000000000b1', 'recent-b', sha256('h-recent-b'), 201, '\x7b7d', now() - interval '1 hour');
COMMIT;
