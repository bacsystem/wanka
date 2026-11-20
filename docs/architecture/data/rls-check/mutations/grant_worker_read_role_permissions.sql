-- Mutacion: da SELECT de iam.role_permissions a wanka_worker.
-- Uso: MUTATION=grant_worker_read_role_permissions sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
GRANT SELECT ON iam.role_permissions TO wanka_worker;
