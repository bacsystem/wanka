-- Mutacion: quita el CHECK que impide un usuario inventado en una fila system.
-- Uso: MUTATION=drop_audit_system_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE audit.events DROP CONSTRAINT events_system_has_no_user;
