-- Mutacion: UPDATE de toda la fila de iam.sessions para wanka_app (permitiria mover created_at, H13).
-- Uso: MUTATION=grant_app_update_all_sessions sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT UPDATE ON iam.sessions TO wanka_app;
