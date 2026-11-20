-- Mutacion: quita el indice unico (tenant_id, template_code): otro dueno en la misma empresa.
-- Uso: MUTATION=drop_roles_template_unique sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE iam.roles DROP CONSTRAINT roles_tenant_template_key;
