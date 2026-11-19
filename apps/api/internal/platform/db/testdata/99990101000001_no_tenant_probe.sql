-- +goose Up
-- Solo para tests: apoyo para probar InNoTenantTx. Nunca va en apps/api/migrations.

-- Con el mismo patrón que iam.session_lookup (ADR 0004): guarda app.tenant_id,
-- lo cambia para mirar al tenant de la sesión y lo restaura al salir. Si la
-- conexión traía un tenant viejo, la transacción termina con ese tenant.
-- +goose StatementBegin
CREATE FUNCTION rlstest.lookup_like(p_tenant text) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
  saved text := current_setting('app.tenant_id', true);
BEGIN
  PERFORM set_config('app.tenant_id', p_tenant, true);
  PERFORM set_config('app.tenant_id', coalesce(saved, ''), true);
END $$;
-- +goose StatementEnd
GRANT EXECUTE ON FUNCTION rlstest.lookup_like(text) TO wanka_app, wanka_worker, wanka_platform;

-- Tabla sin RLS para comprobar que un ROLLBACK no deja filas escritas.
CREATE TABLE rlstest.writes (msg text NOT NULL);
GRANT SELECT, INSERT ON rlstest.writes TO wanka_app, wanka_worker, wanka_platform;
