-- +goose Up
-- Kernel: tipo de cambio diario (issue #170, ADR 0001 y 0008, data-model §2). Tabla global, sin tenant y
-- sin RLS: la matriz del ADR 0001 da SELECT a wanka_app y a wanka_platform e INSERT a wanka_worker (el job
-- de ingesta, fuera de #170). Nadie hace UPDATE ni DELETE: una tasa mal cargada se corrige con una fila
-- nueva en otra fecha o a mano como superusuario, nunca desde un servicio. Corre como wanka_migrator.
--
-- El único redondeo del tipo de cambio ocurre al ingerir (half-up a 3 decimales, fx.Save); esta tabla
-- solo guarda el numeric(6,3) ya redondeado y el documento y el envío a Khipu lo usan tal cual.
CREATE TABLE ops.exchange_rates (
  rate_date date NOT NULL,
  currency text NOT NULL CONSTRAINT exchange_rates_currency_check CHECK (currency IN ('USD')),
  buy numeric(6,3) NOT NULL CONSTRAINT exchange_rates_buy_check CHECK (buy > 0),
  sell numeric(6,3) NOT NULL CONSTRAINT exchange_rates_sell_check CHECK (sell > 0),
  source text NOT NULL CONSTRAINT exchange_rates_source_check CHECK (source IN ('official', 'manual', 'fallback')),
  PRIMARY KEY (rate_date, currency)
);

GRANT SELECT ON ops.exchange_rates TO wanka_app, wanka_platform;
GRANT INSERT ON ops.exchange_rates TO wanka_worker;
