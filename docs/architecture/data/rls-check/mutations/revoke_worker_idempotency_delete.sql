-- Mutacion: wanka_worker pierde el DELETE sobre ops.idempotency_keys: el job horario daria «permission denied».
-- Uso: MUTATION=revoke_worker_idempotency_delete sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
REVOKE DELETE ON ops.idempotency_keys FROM wanka_worker;
