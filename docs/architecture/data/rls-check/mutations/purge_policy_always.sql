-- Mutacion: la politica de borrado del worker ignora la antiguedad (USING true): una limpieza defectuosa borraria claves
-- vigentes y reabriria su efecto.
-- Uso: MUTATION=purge_policy_always sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
DROP POLICY purge_expired ON ops.idempotency_keys;
DROP POLICY purge_expired_read ON ops.idempotency_keys;
CREATE POLICY purge_expired_read ON ops.idempotency_keys FOR SELECT TO wanka_worker USING (true);
CREATE POLICY purge_expired ON ops.idempotency_keys FOR DELETE TO wanka_worker USING (true);
