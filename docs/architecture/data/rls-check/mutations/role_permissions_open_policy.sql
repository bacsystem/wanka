-- Mutacion: tenant_isolation de iam.role_permissions deja de filtrar por tenant: el DELETE de A alcanza los permisos de B.
-- Uso: MUTATION=role_permissions_open_policy sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP POLICY tenant_isolation ON iam.role_permissions;
CREATE POLICY tenant_isolation ON iam.role_permissions USING (true) WITH CHECK (true);
