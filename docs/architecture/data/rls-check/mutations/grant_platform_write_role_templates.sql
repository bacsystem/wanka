-- Mutacion: da INSERT, UPDATE y DELETE sobre las plantillas a wanka_platform.
-- Uso: MUTATION=grant_platform_write_role_templates sh run.sh   (debe fallar en el paso wanka_platform)
\set ON_ERROR_STOP on
GRANT INSERT, UPDATE, DELETE ON iam.role_templates TO wanka_platform;
