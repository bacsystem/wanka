-- Mutacion: da INSERT, UPDATE y DELETE sobre iam.permissions a wanka_worker.
-- Demuestra que el caso K3 de 09_worker.sql distingue.
-- Uso: MUTATION=grant_worker_write_permissions sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
GRANT INSERT, UPDATE, DELETE ON iam.permissions TO wanka_worker;
