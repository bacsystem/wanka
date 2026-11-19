-- Mutacion: da INSERT, UPDATE y DELETE sobre iam.permissions a wanka_app (el catalogo dejaria de estar congelado).
-- Demuestra que el caso C1 de 05_app.sql distingue.
-- Uso: MUTATION=grant_app_write_permissions sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT INSERT, UPDATE, DELETE ON iam.permissions TO wanka_app;
