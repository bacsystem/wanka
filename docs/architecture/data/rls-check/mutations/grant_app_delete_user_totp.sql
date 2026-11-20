-- Mutacion: wanka_app puede borrar filas de iam.user_totp (la matriz del ADR 0001 solo le da SELECT, INSERT y UPDATE, #231).
-- Uso: MUTATION=grant_app_delete_user_totp sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT DELETE ON iam.user_totp TO wanka_app;
