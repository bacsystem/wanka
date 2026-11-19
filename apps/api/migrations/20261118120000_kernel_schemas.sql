-- +goose Up
-- Kernel: esquemas, funciones de tenant y los primeros GRANT (ADR 0001, data-model §1 y §6).
-- Corre como wanka_migrator, que es dueño de todo lo que se crea aquí. NO crea roles:
-- vienen de bootstrap/roles.sql, y si falta uno su GRANT falla.

CREATE SCHEMA app;
CREATE SCHEMA ops;

-- Nombres reservados para los contextos de data-model §1 (un esquema por módulo).
CREATE SCHEMA iam;
CREATE SCHEMA org;
CREATE SCHEMA crm;
CREATE SCHEMA catalog;
CREATE SCHEMA inv;
CREATE SCHEMA sales;
CREATE SCHEMA einv;
CREATE SCHEMA purch;
CREATE SCHEMA fin;
CREATE SCHEMA rpt;
CREATE SCHEMA agenda;
CREATE SCHEMA integ;
CREATE SCHEMA audit;
CREATE SCHEMA notify;
CREATE SCHEMA lube;
CREATE SCHEMA books;
CREATE SCHEMA resto;
CREATE SCHEMA vet;
CREATE SCHEMA dental;
CREATE SCHEMA platform;

-- El NULLIF es obligatorio: en una conexión del pool que ya corrió
-- set_config(..., true) el parámetro queda definido como '' y el cast fallaría.
-- +goose StatementBegin
CREATE FUNCTION app.current_tenant() RETURNS uuid LANGUAGE sql STABLE AS
  $$ SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION app.current_user_id() RETURNS uuid LANGUAGE sql STABLE AS
  $$ SELECT NULLIF(current_setting('app.user_id', true), '')::uuid $$;
-- +goose StatementEnd

-- Toda política RLS llama a app.*; webhooks resuelve su ruta con ops.*.
-- Los GRANT de tablas y funciones llegan con las migraciones que las crean.
GRANT USAGE ON SCHEMA app, ops TO wanka_app, wanka_worker, wanka_webhooks, wanka_platform;
