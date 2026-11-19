-- Mutacion: quita el CHECK del architect (una fila user sin usuario deja de ser imposible).
-- Uso: MUTATION=drop_audit_user_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE audit.events DROP CONSTRAINT events_user_has_user;
