-- Mutacion: da USAGE del esquema iam y SELECT de las plantillas a wanka_webhooks (hoy no tiene acceso a nada de iam).
-- Uso: MUTATION=grant_webhooks_read_role_templates sh run.sh   (debe fallar en el paso wanka_webhooks)
\set ON_ERROR_STOP on
GRANT USAGE ON SCHEMA iam TO wanka_webhooks;
GRANT SELECT ON iam.role_templates, iam.role_template_permissions TO wanka_webhooks;
