-- Mutacion: wanka_worker lee iam.user_totp (el ADR 0001 le niega todo, #231).
-- Uso: MUTATION=grant_worker_user_totp sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
GRANT SELECT ON iam.user_totp TO wanka_worker;
