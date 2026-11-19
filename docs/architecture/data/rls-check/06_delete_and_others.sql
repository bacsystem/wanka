\set ON_ERROR_STOP on
-- (como wanka_migrator, dueño de las tablas) FORCE ROW LEVEL SECURITY: sin tenant fijado ni el dueño ve filas.
SELECT test.assert((SELECT count(*) FROM iam.memberships) = 0 AND (SELECT count(*) FROM iam.roles) = 0 AND (SELECT count(*) FROM org.tenants) = 0,
                   'FORCE RLS: el duenio sin tenant ve 0 filas de memberships, roles y tenants');
-- (como wanka_migrator) borrar la membresía del cajero
BEGIN;
SELECT set_config('app.tenant_id','aaaaaaaa-0000-0000-0000-000000000001', true);
DELETE FROM iam.memberships WHERE id = '3e000000-0000-0000-0000-0000000000c1';
COMMIT;
