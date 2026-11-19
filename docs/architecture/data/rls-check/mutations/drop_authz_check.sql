-- Mutacion: quita el CHECK nuevo de iam.sessions (una sesion sin membresia no puede tener authz_version, ADR 0011).
-- Demuestra que los casos A7 de 05_app.sql distinguen.
-- Uso: MUTATION=drop_authz_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE iam.sessions DROP CONSTRAINT sessions_version_needs_membership;
