-- Mutacion: da INSERT sobre audit.events a wanka_webhooks (la matriz del ADR 0001 no se lo da).
-- Uso: MUTATION=grant_webhooks_insert_audit sh run.sh   (debe fallar en el paso wanka_webhooks)
\set ON_ERROR_STOP on
GRANT USAGE ON SCHEMA audit TO wanka_webhooks;
GRANT INSERT ON audit.events TO wanka_webhooks;
