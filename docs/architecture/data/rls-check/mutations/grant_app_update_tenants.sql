-- Mutacion: da UPDATE sobre org.tenants a wanka_app (un GRANT de mas respecto de la matriz del ADR 0001).
-- Demuestra que el caso «wanka_app no escribe org.tenants» de 05_app.sql distingue.
-- Uso: MUTATION=grant_app_update_tenants sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT UPDATE ON org.tenants TO wanka_app;
