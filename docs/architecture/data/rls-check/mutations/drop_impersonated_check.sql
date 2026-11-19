-- Mutacion: quita el CHECK de vigencia de la sesion impersonada (sin refresh, 60 min como maximo).
-- Demuestra que los casos de 05_app.sql, despues de A6, distinguen.
-- Uso: MUTATION=drop_impersonated_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
ALTER TABLE iam.sessions DROP CONSTRAINT sessions_impersonated_short_no_refresh;
