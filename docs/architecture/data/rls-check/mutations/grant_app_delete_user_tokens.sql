-- Mutacion: wanka_app puede borrar tokens de iam.user_tokens (solo el worker limpia, #231).
-- Uso: MUTATION=grant_app_delete_user_tokens sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT DELETE ON iam.user_tokens TO wanka_app;
