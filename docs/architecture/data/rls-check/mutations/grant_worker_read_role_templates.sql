-- Mutacion: da SELECT de las plantillas a wanka_worker (ningun job resuelve permisos).
-- Uso: MUTATION=grant_worker_read_role_templates sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
GRANT SELECT ON iam.role_templates, iam.role_template_permissions TO wanka_worker;
