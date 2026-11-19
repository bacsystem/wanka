-- Mutacion: cualquiera ejecuta audit.ensure_partitions (DDL con los privilegios del dueno).
-- Uso: MUTATION=ensure_partitions_to_public sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT EXECUTE ON FUNCTION audit.ensure_partitions(date, integer) TO PUBLIC;
