-- Mutacion: da INSERT, UPDATE, DELETE y TRUNCATE sobre las plantillas de rol a wanka_app (el catalogo dejaria de ser global y de solo lectura).
-- Uso: MUTATION=grant_app_write_role_templates sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT INSERT, UPDATE, DELETE, TRUNCATE ON iam.role_templates, iam.role_template_permissions TO wanka_app;
