-- Mutacion: la plantilla administrador conserva empresas.einvoicing:update, que solo tiene el dueno (#233).
-- Uso: MUTATION=seed_admin_keeps_einvoicing sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
INSERT INTO iam.role_template_permissions (template_code, permission_code) VALUES ('administrador', 'empresas.einvoicing:update');
