-- Mutacion: da SELECT de org.cash_registers a wanka_worker.
-- Uso: MUTATION=grant_worker_read_cash_registers sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
GRANT SELECT ON org.cash_registers TO wanka_worker;
