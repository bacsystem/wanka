\set ON_ERROR_STOP on
-- (como wanka_migrator) borrar la membresía del cajero
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
DELETE FROM iam.memberships WHERE id = '3e000000-0000-0000-0000-0000000000c1';
COMMIT;
