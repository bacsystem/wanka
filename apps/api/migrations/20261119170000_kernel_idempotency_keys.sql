-- +goose Up
-- Kernel: ops.idempotency_keys (issue #177, ADR 0001, api/conventions.md §3, data-model §5). Corre como
-- wanka_migrator, dueño de la tabla.
--
-- Una fila por (tenant, clave). El middleware la inserta PRIMERO, en la misma transacción que el efecto, con
-- status_code y response vacíos: la fila no confirmada ocupa la clave y una segunda petición con la misma
-- clave espera en el índice único hasta que la primera confirme o aborte. Antes del UPDATE final la fila
-- solo existe dentro de esa transacción: ninguna fila confirmada tiene status_code NULL.
-- Solo se guardan respuestas 2xx (conventions.md §3); un 4xx o 5xx hace ROLLBACK y con él se va la clave.
CREATE TABLE ops.idempotency_keys (
  tenant_id uuid NOT NULL REFERENCES org.tenants,
  key text NOT NULL CHECK (length(key) BETWEEN 1 AND 64),
  -- sha256 de método, ruta y cuerpo canónico (JSON normalizado); la misma clave con otro hash es idempotency_key_reused.
  request_hash bytea NOT NULL CONSTRAINT idempotency_keys_hash_len CHECK (octet_length(request_hash) = 32),
  -- Con nombre propio para que rls-check/mutations pueda quitarlo.
  status_code smallint CONSTRAINT idempotency_keys_2xx CHECK (status_code BETWEEN 200 AND 299),
  -- Cuerpo de la respuesta tal como salió (bytes exactos, vacío en un 204) y las cabeceras que se repiten
  -- (Content-Type, Location, ETag).
  response bytea,
  response_headers jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, key)  -- su prefijo cubre la FK a org.tenants
);
-- Limpieza horaria por antigüedad (data-model §5).
CREATE INDEX idempotency_keys_created_at_idx ON ops.idempotency_keys (created_at);

ALTER TABLE ops.idempotency_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE ops.idempotency_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ops.idempotency_keys
  USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());

-- Limpieza cross-tenant. El job horario corre sin tenant (el worker no puede listar los tenants: org.tenants
-- tiene RLS), así que tenant_isolation no le deja ver ninguna fila. Estas dos políticas, limitadas a
-- wanka_worker (como platform_rw en org.tenants), le dejan leer y borrar SOLO filas de más de 24 h: la
-- retención es un mínimo y un worker defectuoso no puede borrar claves vivas. No dan INSERT ni UPDATE (y el
-- GRANT tampoco). La lectura es necesaria porque el WHERE del DELETE pasa por las políticas de SELECT.
CREATE POLICY purge_expired_read ON ops.idempotency_keys FOR SELECT TO wanka_worker
  USING (created_at < now() - interval '24 hours');
CREATE POLICY purge_expired ON ops.idempotency_keys FOR DELETE TO wanka_worker
  USING (created_at < now() - interval '24 hours');

-- GRANT (ADR 0001, «Roles de base de datos y permisos»).
-- wanka_app: INSERT solo de la clave y el hash (created_at lo pone el servidor: una clave no puede escapar de la
-- limpieza con una fecha futura), UPDATE solo de la respuesta (el hash y la fecha no se mueven), y nunca DELETE:
-- un 4xx no deja fila que borrar.
GRANT SELECT ON ops.idempotency_keys TO wanka_app;
GRANT INSERT (tenant_id, key, request_hash) ON ops.idempotency_keys TO wanka_app;
GRANT UPDATE (status_code, response, response_headers) ON ops.idempotency_keys TO wanka_app;
-- wanka_worker: DELETE, y SELECT solo de created_at (lo que filtra el job); nunca el hash ni la respuesta.
GRANT SELECT (created_at) ON ops.idempotency_keys TO wanka_worker;
GRANT DELETE ON ops.idempotency_keys TO wanka_worker;
-- wanka_platform queda sin acceso: sin tenant fijado no vería ninguna fila (el ADR 0001 le asignaba SELECT; ver el PR #177).
