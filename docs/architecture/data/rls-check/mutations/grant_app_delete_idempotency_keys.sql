-- Mutacion: wanka_app puede borrar claves de idempotencia. Un 4xx no deja fila que borrar y borrar una confirmada
-- reabriria el efecto de un reintento. Demuestra que el caso A11 de 05_app.sql lo distingue.
-- Uso: MUTATION=grant_app_delete_idempotency_keys sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT DELETE ON ops.idempotency_keys TO wanka_app;
