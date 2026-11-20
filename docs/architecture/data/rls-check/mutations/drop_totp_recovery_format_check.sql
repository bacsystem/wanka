-- Mutacion: quita el CHECK de formato de los codigos de recuperacion de iam.user_totp (entrarian en claro, #231).
-- Uso: MUTATION=drop_totp_recovery_format_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE iam.user_totp DROP CONSTRAINT user_totp_recovery_codes_format;
