-- Mutacion: se quita el CHECK status_code BETWEEN 200 AND 299: una respuesta 4xx podria quedar guardada y bloquear la
-- operacion 24 h en ese error (conventions.md §3).
-- Uso: MUTATION=drop_idempotency_2xx_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE ops.idempotency_keys DROP CONSTRAINT idempotency_keys_2xx;
