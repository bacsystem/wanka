-- Mutacion: quita el CHECK que ata membership_id al proposito invitation (#231).
-- Uso: MUTATION=drop_user_tokens_membership_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE iam.user_tokens DROP CONSTRAINT user_tokens_membership_iff_invitation;
