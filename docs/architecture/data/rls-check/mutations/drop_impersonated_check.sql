-- Mutacion: quita el CHECK de vigencia de la sesion impersonada (sin refresh, 60 min como maximo).
-- Demuestra que los casos de 05b_superuser_sessions.sql distinguen (wanka_app no llega a estos CHECK: el trigger lo para antes).
-- Uso: MUTATION=drop_impersonated_check sh run.sh   (debe fallar en el paso «superusuario: sesiones impersonadas»)
\set ON_ERROR_STOP on
ALTER TABLE iam.sessions DROP CONSTRAINT sessions_impersonated_short_no_refresh;
