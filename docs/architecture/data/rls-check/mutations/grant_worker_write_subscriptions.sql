-- Mutacion: da UPDATE sobre platform.subscriptions a wanka_worker (la matriz solo le da escritura en tenant_usage).
-- Uso: MUTATION=grant_worker_write_subscriptions sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
GRANT UPDATE ON platform.subscriptions TO wanka_worker;
