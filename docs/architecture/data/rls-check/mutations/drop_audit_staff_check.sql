-- Mutacion: quita el CHECK que exige impersonador (y ningun usuario) en una fila platform_staff.
-- Uso: MUTATION=drop_audit_staff_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE audit.events DROP CONSTRAINT events_staff_has_staff;
