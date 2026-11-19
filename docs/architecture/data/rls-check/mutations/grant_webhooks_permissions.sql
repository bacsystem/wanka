-- Mutacion: da SELECT sobre iam.permissions a wanka_webhooks (que el ADR 0001 deja sin acceso).
-- Demuestra que el caso W2 de 08_webhooks_worker.sql distingue.
-- Uso: MUTATION=grant_webhooks_permissions sh run.sh   (debe fallar en el paso wanka_webhooks)
\set ON_ERROR_STOP on
GRANT USAGE ON SCHEMA iam TO wanka_webhooks;
GRANT SELECT ON iam.permissions TO wanka_webhooks;
