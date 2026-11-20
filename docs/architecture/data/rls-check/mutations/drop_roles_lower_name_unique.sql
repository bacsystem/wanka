-- Mutacion: quita el indice unico (tenant_id, lower(name)): 'CAJERO' y 'Cajero' conviven.
-- Uso: MUTATION=drop_roles_lower_name_unique sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP INDEX iam.roles_tenant_lower_name_key;
