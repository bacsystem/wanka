-- Mutacion: wanka_app puede cambiar request_hash y created_at de una clave (UPDATE de toda la fila). Con el hash movible
-- una clave aceptaria otro cuerpo; con created_at movible escaparia de la limpieza.
-- Uso: MUTATION=grant_app_update_idempotency_hash sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT UPDATE ON ops.idempotency_keys TO wanka_app;
