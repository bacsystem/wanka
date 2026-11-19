-- Mutacion: audit.ensure_partitions pierde su search_path fijo.
-- Uso: MUTATION=ensure_partitions_no_search_path sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
ALTER FUNCTION audit.ensure_partitions(date, integer) RESET search_path;
