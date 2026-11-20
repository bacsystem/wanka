-- Mutacion: da DELETE sobre org.cash_registers a wanka_app.
-- Uso: MUTATION=grant_app_delete_cash_registers sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT DELETE ON org.cash_registers TO wanka_app;
