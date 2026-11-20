-- Mutacion: wanka_worker lee token_hash de iam.user_tokens (el ADR 0001 le deja solo id y expires_at, #231).
-- Uso: MUTATION=grant_worker_select_user_token_hash sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
GRANT SELECT (token_hash) ON iam.user_tokens TO wanka_worker;
