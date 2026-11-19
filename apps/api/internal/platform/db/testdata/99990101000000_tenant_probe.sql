-- +goose Up
-- Solo para tests: una tabla de tenant con la política estándar (ADR 0001, «Política estándar»)
-- para probar InTenantTx contra PostgreSQL real. Nunca va en apps/api/migrations.

CREATE SCHEMA rlstest;
GRANT USAGE ON SCHEMA rlstest TO wanka_app, wanka_worker, wanka_platform;

CREATE TABLE rlstest.items (
  id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id uuid NOT NULL,
  name      text NOT NULL
);
ALTER TABLE rlstest.items ENABLE ROW LEVEL SECURITY;
ALTER TABLE rlstest.items FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON rlstest.items
  USING (tenant_id = app.current_tenant())
  WITH CHECK (tenant_id = app.current_tenant());
GRANT SELECT, INSERT, UPDATE, DELETE ON rlstest.items TO wanka_app, wanka_worker, wanka_platform;
GRANT USAGE ON SEQUENCE rlstest.items_id_seq TO wanka_app, wanka_worker, wanka_platform;
