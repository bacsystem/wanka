-- Mutacion: da SELECT sobre iam.user_tokens a wanka_platform, que el ADR 0001 deja sin acceso (#231).
-- Uso: MUTATION=grant_platform_user_tokens sh run.sh   (debe fallar en el paso wanka_platform)
\set ON_ERROR_STOP on
GRANT SELECT ON iam.user_tokens, iam.user_totp, iam.mfa_enrollments TO wanka_platform;
