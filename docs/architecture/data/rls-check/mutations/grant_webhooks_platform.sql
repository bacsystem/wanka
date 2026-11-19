-- Mutacion: da a wanka_webhooks acceso de lectura a platform.modules (la matriz del ADR 0001 no le da nada en platform).
-- Uso: MUTATION=grant_webhooks_platform sh run.sh   (debe fallar en el paso wanka_webhooks)
\set ON_ERROR_STOP on
GRANT USAGE ON SCHEMA platform TO wanka_webhooks;
GRANT SELECT ON platform.modules TO wanka_webhooks;
