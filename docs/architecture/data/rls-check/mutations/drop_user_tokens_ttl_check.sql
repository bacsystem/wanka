-- Mutacion: quita el CHECK de vigencia maxima por proposito de iam.user_tokens (#231).
-- Uso: MUTATION=drop_user_tokens_ttl_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE iam.user_tokens DROP CONSTRAINT user_tokens_ttl;
