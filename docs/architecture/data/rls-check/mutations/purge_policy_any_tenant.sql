-- Mutacion: las politicas de limpieza pierden app.current_tenant() IS NULL: dentro de InTenantTx(A) el worker
-- alcanzaria las claves vencidas de B.
-- Uso: MUTATION=purge_policy_any_tenant sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
DROP POLICY purge_expired ON ops.idempotency_keys;
DROP POLICY purge_expired_read ON ops.idempotency_keys;
CREATE POLICY purge_expired_read ON ops.idempotency_keys FOR SELECT TO wanka_worker
  USING (created_at < now() - interval '24 hours');
CREATE POLICY purge_expired ON ops.idempotency_keys FOR DELETE TO wanka_worker
  USING (created_at < now() - interval '24 hours');
