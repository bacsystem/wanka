-- Mutacion: se quitan las politicas del worker sobre ops.idempotency_keys: sin tenant fijado FORCE RLS no le deja ver ni
-- una fila y la limpieza horaria borraria 0 filas para siempre (la tabla crece sin limite).
-- Uso: MUTATION=drop_purge_policy sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
DROP POLICY purge_expired ON ops.idempotency_keys;
DROP POLICY purge_expired_read ON ops.idempotency_keys;
