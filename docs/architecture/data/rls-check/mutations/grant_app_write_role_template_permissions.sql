-- Mutacion: da INSERT y DELETE sobre iam.role_template_permissions a wanka_app (los permisos de un rol base cambiarian desde la API).
-- Uso: MUTATION=grant_app_write_role_template_permissions sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT INSERT, DELETE ON iam.role_template_permissions TO wanka_app;
