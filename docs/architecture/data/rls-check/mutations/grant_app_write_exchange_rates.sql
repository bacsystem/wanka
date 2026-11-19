-- Mutacion: da INSERT, UPDATE y DELETE sobre ops.exchange_rates a wanka_app (la matriz del ADR 0001 solo le da SELECT).
-- Demuestra que el caso A11 de 05_app.sql distingue.
-- Uso: MUTATION=grant_app_write_exchange_rates sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT INSERT, UPDATE, DELETE ON ops.exchange_rates TO wanka_app;
