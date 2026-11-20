-- Mutacion: UPDATE de toda la fila de iam.user_tokens para wanka_app (alargaria expires_at o movería el token a otro usuario, #231).
-- Uso: MUTATION=grant_app_update_all_user_tokens sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT UPDATE ON iam.user_tokens TO wanka_app;
