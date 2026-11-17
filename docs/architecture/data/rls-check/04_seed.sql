-- Como wanka_migrator (NOBYPASSRLS, dueño con FORCE RLS): datos de prueba, con tenant fijado como haría la app
\set ON_ERROR_STOP on
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
UPDATE iam.memberships SET status = 'activo' WHERE user_id = 'eeeeeeee-0000-0000-0000-000000000001';
INSERT INTO iam.roles (id, tenant_id, name, requires_mfa) VALUES ('a0000000-0000-0000-0000-0000000000a2','aaaaaaaa-0000-0000-0000-000000000001','Cajero', false);
INSERT INTO iam.users (id, email) VALUES ('eeeeeeee-0000-0000-0000-000000000002','cajero@a.pe');
INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status, authz_version)
  VALUES ('3e000000-0000-0000-0000-0000000000c1','aaaaaaaa-0000-0000-0000-000000000001','eeeeeeee-0000-0000-0000-000000000002','a0000000-0000-0000-0000-0000000000a2','activo', 1);
COMMIT;
-- sesión de tenant del cajero, con la versión 1 guardada (iam.sessions no tiene RLS)
INSERT INTO iam.sessions (id, user_id, tenant_id, membership_id, access_hash, access_expires_at, authz_version, mfa_satisfied, mfa_method)
  VALUES ('5e000000-0000-0000-0000-000000000001','eeeeeeee-0000-0000-0000-000000000002','aaaaaaaa-0000-0000-0000-000000000001',
          '3e000000-0000-0000-0000-0000000000c1', sha256('tok-cajero'), now() + interval '15 minutes', 1, true, 'none');
-- ascenso del cajero: authz_version pasa a 2
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
UPDATE iam.memberships SET authz_version = 2 WHERE id = '3e000000-0000-0000-0000-0000000000c1';
COMMIT;
INSERT INTO ops.webhook_routes VALUES ('r-0123456789abcdef', 'aaaaaaaa-0000-0000-0000-000000000001', 'khipu', '\xdeadbeef', NULL);
