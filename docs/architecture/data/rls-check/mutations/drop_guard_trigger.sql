-- Mutacion: quita el trigger iam.guard_impersonated_session. wanka_app volveria a poder insertar impersonadas y alargarlas.
-- Uso: MUTATION=drop_guard_trigger sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP TRIGGER sessions_guard_impersonated ON iam.sessions;
