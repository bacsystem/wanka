-- Mutacion: quita INSERT sobre platform.tenant_usage a wanka_worker (el job de uso no podria contar CPE).
-- Uso: MUTATION=revoke_worker_usage_write sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
REVOKE INSERT ON platform.tenant_usage FROM wanka_worker;
