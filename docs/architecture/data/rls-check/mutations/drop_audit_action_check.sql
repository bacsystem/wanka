-- Mutacion: quita el CHECK del formato modulo.recurso:accion de audit.events.
-- Uso: MUTATION=drop_audit_action_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE audit.events DROP CONSTRAINT events_action_format;
