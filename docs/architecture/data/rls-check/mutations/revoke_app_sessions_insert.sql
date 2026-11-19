-- Mutacion: quita el INSERT de wanka_app sobre iam.sessions (GRANT de la matriz del ADR 0001).
-- Demuestra que los casos A7 de 05_app.sql, que insertan como wanka_app, distinguen.
-- Uso: MUTATION=revoke_app_sessions_insert sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
REVOKE INSERT ON iam.sessions FROM wanka_app;
