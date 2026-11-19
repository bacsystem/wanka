-- Mutacion: da INSERT sobre platform.subscriptions a wanka_app (un GRANT de mas respecto de la matriz del ADR 0001).
-- Demuestra que los casos E1 de 05_app.sql distinguen.
-- Uso: MUTATION=grant_app_write_subscriptions sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT INSERT ON platform.subscriptions TO wanka_app;
