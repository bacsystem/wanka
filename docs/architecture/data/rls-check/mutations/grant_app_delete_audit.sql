-- Mutacion: da DELETE sobre audit.events a wanka_app.
-- Uso: MUTATION=grant_app_delete_audit sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT DELETE ON audit.events TO wanka_app;
