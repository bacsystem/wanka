-- Mutacion: quita el CHECK system = (template_code IS NOT NULL).
-- Uso: MUTATION=drop_roles_system_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE iam.roles DROP CONSTRAINT roles_system_iff_template;
