-- Mutacion: da SELECT, UPDATE y DELETE sobre ops.exchange_rates a wanka_worker (la matriz del ADR 0001 solo le da INSERT).
-- Demuestra que el caso K4 de 09_worker.sql distingue.
-- Uso: MUTATION=grant_worker_read_exchange_rates sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
GRANT SELECT, UPDATE, DELETE ON ops.exchange_rates TO wanka_worker;
