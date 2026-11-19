-- Mutacion: wanka_webhooks puede actualizar ops.river_job (podria reescribir o cancelar los jobs de otros servicios).
-- Uso: MUTATION=grant_webhooks_update_river_job sh run.sh   (debe fallar en el paso wanka_webhooks)
\set ON_ERROR_STOP on
GRANT UPDATE ON ops.river_job TO wanka_webhooks;
