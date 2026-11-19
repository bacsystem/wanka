-- Mutacion: quita el CHECK que limita el impersonador al staff de plataforma.
-- Uso: MUTATION=drop_audit_impersonator_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE audit.events DROP CONSTRAINT events_impersonator_is_staff;
