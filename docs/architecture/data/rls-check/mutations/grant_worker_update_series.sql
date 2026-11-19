-- Mutacion: da UPDATE de toda org.document_series a wanka_worker (lo que daria la fila generica de la matriz del ADR 0001).
-- Demuestra que K5 distingue: con este GRANT, el worker podria mover last_number.
-- Uso: MUTATION=grant_worker_update_series sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
GRANT UPDATE ON org.document_series TO wanka_worker;
