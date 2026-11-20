-- Mutacion: da SELECT de las plantillas a wanka_platform.
-- Uso: MUTATION=grant_platform_read_role_templates sh run.sh   (debe fallar en el paso wanka_platform)
\set ON_ERROR_STOP on
GRANT SELECT ON iam.role_templates, iam.role_template_permissions TO wanka_platform;
