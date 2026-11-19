-- Como wanka_migrator (NOBYPASSRLS, duenio con FORCE RLS): datos de prueba con el tenant fijado, como la app.
\set ON_ERROR_STOP on
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
UPDATE iam.memberships SET status = 'activo' WHERE user_id = 'eeeeeeee-0000-0000-0000-000000000001';
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
INSERT INTO ops.webhook_routes VALUES ('r-0123456789abcdef', 'aaaaaaaa-0000-0000-0000-000000000001', 'khipu', '\xdeadbeef', NULL);
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
