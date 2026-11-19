-- Mutacion: quita el CHECK (user_id IS NULL) = (impersonator_id IS NOT NULL) de iam.sessions.
-- Uso: MUTATION=drop_user_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE iam.sessions DROP CONSTRAINT sessions_user_iff_not_impersonated;
