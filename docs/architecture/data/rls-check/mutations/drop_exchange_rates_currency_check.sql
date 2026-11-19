-- Mutacion: quita el CHECK de moneda de ops.exchange_rates (solo USD).
-- Demuestra que el caso K3 de 09_worker.sql distingue.
-- Uso: MUTATION=drop_exchange_rates_currency_check sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
ALTER TABLE ops.exchange_rates DROP CONSTRAINT exchange_rates_currency_check;
