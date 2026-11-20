-- Mutacion: quita el SELECT por columna de wanka_worker en iam.user_tokens: sin el, el WHERE del DELETE de limpieza da permission denied (#231).
-- Uso: MUTATION=revoke_worker_user_token_columns sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
REVOKE SELECT (id, expires_at) ON iam.user_tokens FROM wanka_worker;
