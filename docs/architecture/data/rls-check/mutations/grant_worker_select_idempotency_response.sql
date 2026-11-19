-- Mutacion: wanka_worker puede leer la respuesta guardada (datos de venta de todos los tenants).
-- Uso: MUTATION=grant_worker_select_idempotency_response sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
GRANT SELECT (response, request_hash) ON ops.idempotency_keys TO wanka_worker;
