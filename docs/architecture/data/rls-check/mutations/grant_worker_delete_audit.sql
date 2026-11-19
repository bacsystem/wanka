-- Mutacion: da DELETE sobre audit.events a wanka_worker.
-- Uso: MUTATION=grant_worker_delete_audit sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
GRANT DELETE ON audit.events TO wanka_worker;
