-- Mutacion: wanka_worker puede escribir ops.river_migration (el esquema de River solo lo cambia el migrador).
-- Uso: MUTATION=grant_worker_river_migration sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
GRANT SELECT, DELETE ON ops.river_migration TO wanka_worker;
