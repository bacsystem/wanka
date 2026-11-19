-- Mutacion: wanka_worker pierde el DELETE sobre ops.river_job: el JobCleaner no podria limpiar completados (7 dias) ni descartados (30).
-- Uso: MUTATION=revoke_worker_river_delete sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
REVOKE DELETE ON ops.river_job FROM wanka_worker;
