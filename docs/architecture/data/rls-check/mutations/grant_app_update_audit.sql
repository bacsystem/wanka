-- Mutacion: da UPDATE sobre audit.events a wanka_app (los eventos dejarian de ser de solo insercion).
-- Demuestra que el caso «wanka_app no actualiza audit.events» (A11) distingue.
-- Uso: MUTATION=grant_app_update_audit sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT UPDATE ON audit.events TO wanka_app;
