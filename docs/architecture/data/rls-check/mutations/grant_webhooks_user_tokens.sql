-- Mutacion: da SELECT sobre iam.user_tokens a wanka_webhooks, que el ADR 0001 deja sin acceso (#231).
-- Uso: MUTATION=grant_webhooks_user_tokens sh run.sh   (debe fallar en el paso wanka_webhooks)
\set ON_ERROR_STOP on
GRANT USAGE ON SCHEMA iam TO wanka_webhooks;
GRANT SELECT ON iam.user_tokens, iam.user_totp, iam.mfa_enrollments TO wanka_webhooks;
