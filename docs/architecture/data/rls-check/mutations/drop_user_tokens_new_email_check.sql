-- Mutacion: quita el CHECK que ata new_email al proposito email_change (#231).
-- Uso: MUTATION=drop_user_tokens_new_email_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE iam.user_tokens DROP CONSTRAINT user_tokens_new_email_iff_email_change;
