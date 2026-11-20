-- +goose Up
-- Identidad de la ola 1, F2: tablas con RLS (issue #232, épica #3; ADR 0001, nota del 2026-11-19; data-model §6,
-- «DDL de identidad de la ola 1»; er-01). Corre como wanka_migrator, dueño de todo lo que crea, y solo hacia adelante
-- (expandir): ninguna migración anterior se edita. No siembra nada: las plantillas y los roles base son de #29/#233, y
-- las columnas de org.tenants las llena el alta (#31). El núcleo de org.sites y org.cash_registers nace aquí para cortar
-- el ciclo con la épica #4; empresas (#33) lo amplía con el anexo SUNAT y la sincronización con Khipu (ADR 0013).
-- Los triggers no son SECURITY DEFINER (corren con el rol que escribe, que es lo que se quiere guardar), así que esta
-- migración no crea funciones definer ni cambia dueños.

-- ===== Catálogo global de plantillas de rol (sin RLS, como iam.permissions) =====
-- Los seis roles base de toda empresa salen de aquí. Nacen vacías; las siembra #233 con un test de paridad contra
-- resources.md. Ningún rol de aplicación escribe (ver los GRANT al final).
CREATE TABLE iam.role_templates (
  code text PRIMARY KEY CONSTRAINT role_templates_code_format CHECK (code ~ '^[a-z][a-z_]{0,39}$'),
  name text NOT NULL CONSTRAINT role_templates_name_len CHECK (char_length(btrim(name)) BETWEEN 1 AND 60),
  requires_mfa boolean NOT NULL
);
CREATE TABLE iam.role_template_permissions (
  template_code text NOT NULL REFERENCES iam.role_templates,
  permission_code text NOT NULL REFERENCES iam.permissions,
  PRIMARY KEY (template_code, permission_code)  -- su prefijo cubre la FK a iam.role_templates
);
CREATE INDEX role_template_permissions_permission_idx ON iam.role_template_permissions (permission_code);

-- ===== Columnas de empresa que la lista de membresías muestra (las llena #31 en el alta) =====
-- Nullable para las filas existentes. El RUC es único en toda la plataforma a propósito (ADR 0013, er-01): un RUC, una
-- empresa; no es una UNIQUE de tenant porque org.tenants no tiene tenant_id.
ALTER TABLE org.tenants
  ADD COLUMN ruc text CONSTRAINT tenants_ruc_key UNIQUE CONSTRAINT tenants_ruc_format CHECK (ruc ~ '^[0-9]{11}$'),
  ADD COLUMN legal_name text CONSTRAINT tenants_legal_name_len CHECK (char_length(btrim(legal_name)) BETWEEN 1 AND 200),
  ADD COLUMN trade_name text CONSTRAINT tenants_trade_name_len CHECK (char_length(btrim(trade_name)) BETWEEN 1 AND 200),
  ADD COLUMN industry text CONSTRAINT tenants_industry_values
    CHECK (industry IN ('lubricentro', 'libreria', 'restaurante', 'veterinaria', 'odontologia', 'retail'));

-- ===== Núcleo de sedes y cajas (#33 lo amplía; #31 crea las filas) =====
CREATE TABLE org.sites (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL REFERENCES org.tenants,
  code text NOT NULL CONSTRAINT sites_code_format CHECK (code ~ '^[A-Za-z0-9_-]{1,20}$'),
  name text NOT NULL CONSTRAINT sites_name_len CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, code),  -- su prefijo cubre la FK a org.tenants
  UNIQUE (tenant_id, id)     -- destino de las FK compuestas de cajas, membresías y (con #33) series
);
CREATE TABLE org.cash_registers (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL,
  site_id uuid NOT NULL,
  code text NOT NULL CONSTRAINT cash_registers_code_format CHECK (code ~ '^[A-Za-z0-9_-]{1,20}$'),
  active boolean NOT NULL DEFAULT true,
  UNIQUE (tenant_id, code),
  FOREIGN KEY (tenant_id, site_id) REFERENCES org.sites (tenant_id, id)  -- una caja de otra empresa no se referencia
);
CREATE INDEX cash_registers_site_idx ON org.cash_registers (tenant_id, site_id);

-- ===== Roles: plantilla, versión y nombre único sin distinguir mayúsculas =====
-- UNIQUE (tenant_id, name) del kernel se conserva: el índice nuevo (tenant_id, lower(name)) lo refuerza.
-- Si ya hubiera un rol system, el CHECK hace fallar la migración (falla cerrada): hoy ninguna empresa tiene roles base.
ALTER TABLE iam.roles
  ADD COLUMN template_code text REFERENCES iam.role_templates,
  ADD COLUMN version bigint NOT NULL DEFAULT 1 CONSTRAINT roles_version_positive CHECK (version > 0),
  ADD CONSTRAINT roles_system_iff_template CHECK (system = (template_code IS NOT NULL)),
  ADD CONSTRAINT roles_tenant_template_key UNIQUE (tenant_id, template_code);  -- nadie crea otro dueno, ni con system = true
CREATE UNIQUE INDEX roles_tenant_lower_name_key ON iam.roles (tenant_id, lower(name));
CREATE INDEX roles_template_code_idx ON iam.roles (template_code) WHERE template_code IS NOT NULL;  -- índice de la FK

-- Permisos de un rol propio. Los de un rol base no tienen filas aquí: salen de su plantilla.
CREATE TABLE iam.role_permissions (
  tenant_id uuid NOT NULL,
  role_id uuid NOT NULL,
  permission_code text NOT NULL REFERENCES iam.permissions,
  PRIMARY KEY (tenant_id, role_id, permission_code),  -- su prefijo cubre la FK compuesta
  FOREIGN KEY (tenant_id, role_id) REFERENCES iam.roles (tenant_id, id) ON DELETE CASCADE  -- un rol de otra empresa se rechaza
);
CREATE INDEX role_permissions_permission_idx ON iam.role_permissions (permission_code);

-- Sedes de una membresía con all_sites = false.
CREATE TABLE iam.membership_sites (
  tenant_id uuid NOT NULL,
  membership_id uuid NOT NULL,
  site_id uuid NOT NULL,
  PRIMARY KEY (tenant_id, membership_id, site_id),  -- su prefijo cubre la FK a iam.memberships
  FOREIGN KEY (tenant_id, membership_id) REFERENCES iam.memberships (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, site_id) REFERENCES org.sites (tenant_id, id)  -- una sede de otra empresa se rechaza
);
CREATE INDEX membership_sites_site_idx ON iam.membership_sites (tenant_id, site_id);

-- ===== RLS: habilitada y forzada, con tenant_isolation, en las cuatro tablas nuevas de tenant =====
ALTER TABLE org.sites ENABLE ROW LEVEL SECURITY;
ALTER TABLE org.sites FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON org.sites
  USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());
ALTER TABLE org.cash_registers ENABLE ROW LEVEL SECURITY;
ALTER TABLE org.cash_registers FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON org.cash_registers
  USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());
ALTER TABLE iam.role_permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.role_permissions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON iam.role_permissions
  USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());
ALTER TABLE iam.membership_sites ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.membership_sites FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON iam.membership_sites
  USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());

-- ===== Políticas por usuario (ADR 0001, «Caminos que no tienen un tenant fijado», 1) =====
-- Solo SELECT y solo sin tenant fijado (app.current_tenant() IS NULL): dentro de InTenantTx(A) un miembro de A y B no
-- ve nada de B. Solo las membresías invitado/activo dan lectura (ADR 0011): una suspendida o borrada no ve su empresa.
-- Texto exacto del ADR; catalog_test.go lo compara contra pg_policies.qual.
CREATE POLICY own_memberships ON iam.memberships FOR SELECT USING (
  app.current_tenant() IS NULL AND user_id = app.current_user_id() AND status IN ('invitado','activo'));
-- El ADR no escribe este USING todavía; lleva el mismo filtro de estado, vía la membresía propia.
CREATE POLICY own_memberships ON iam.membership_sites FOR SELECT USING (
  app.current_tenant() IS NULL AND membership_id IN (SELECT id FROM iam.memberships
            WHERE user_id = app.current_user_id() AND status IN ('invitado','activo')));
CREATE POLICY member_read ON org.tenants FOR SELECT USING (id = app.current_tenant() OR (app.current_tenant() IS NULL AND id IN (SELECT tenant_id FROM iam.memberships WHERE user_id = app.current_user_id() AND status IN ('invitado','activo'))));
CREATE POLICY member_read ON iam.roles FOR SELECT USING (
  tenant_id = app.current_tenant()
  OR (app.current_tenant() IS NULL AND id IN (SELECT role_id FROM iam.memberships
            WHERE user_id = app.current_user_id() AND status IN ('invitado','activo'))));
-- OJO (revisión del PR #245): PostgreSQL comprueba los privilegios de TODA la política con el rol que consulta, aunque la rama
-- por usuario no se ejecute. Quien lea o borre sedes necesita SELECT en iam.memberships e iam.membership_sites, y quien lea
-- cajas, además SELECT en org.sites. Hoy solo wanka_app lee sedes y cajas; el GRANT para wanka_worker (sincronización con
-- Khipu, #33) y wanka_definer (purga de empresa cerrada, #246) lo agrega la historia que los haga leer, no esta.
CREATE POLICY member_read ON org.sites FOR SELECT USING (
  tenant_id = app.current_tenant()
  OR (app.current_tenant() IS NULL AND EXISTS (SELECT 1 FROM iam.memberships m
             WHERE m.user_id = app.current_user_id() AND m.status = 'activo' AND m.tenant_id = sites.tenant_id
               AND (m.all_sites OR EXISTS (SELECT 1 FROM iam.membership_sites ms
                                           WHERE ms.membership_id = m.id AND ms.site_id = sites.id)))));
CREATE POLICY member_read ON org.cash_registers FOR SELECT USING (
  tenant_id = app.current_tenant()
  OR (app.current_tenant() IS NULL AND site_id IN (SELECT id FROM org.sites)));  -- org.sites ya filtrado por su propia política

-- ===== Triggers de guarda (ADR 0001, nota del 2026-11-19) =====
-- wanka_app tiene UPDATE de fila completa en iam.roles, así que un GRANT por columna no alcanza. Sin SECURITY DEFINER
-- (corren con el rol que escribe) y con search_path fijado; solo un superusuario (las semillas de prueba) los salta.
-- Todo rechazo es 42501. Leen iam.role_templates con el privilegio de quien escribe: por eso wanka_app y wanka_definer
-- tienen SELECT sobre las plantillas.
-- +goose StatementBegin
CREATE FUNCTION iam.guard_role_writes() RETURNS trigger
  LANGUAGE plpgsql
  SET search_path = pg_catalog, pg_temp
AS $$
DECLARE template_mfa boolean;
BEGIN
  IF coalesce((SELECT r.rolsuper FROM pg_catalog.pg_roles r WHERE r.rolname = current_user), false) THEN
    IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
  END IF;
  IF TG_OP = 'DELETE' THEN
    IF OLD.system THEN
      RAISE EXCEPTION 'a system role is never deleted' USING ERRCODE = 'insufficient_privilege';
    END IF;
    RETURN OLD;
  END IF;
  IF TG_OP = 'UPDATE' THEN
    -- Un rol propio no pasa a base ni al revés, y ninguno cambia de plantilla.
    IF NEW.system IS DISTINCT FROM OLD.system OR NEW.template_code IS DISTINCT FROM OLD.template_code THEN
      RAISE EXCEPTION 'system and template_code never change' USING ERRCODE = 'insufficient_privilege';
    END IF;
    IF OLD.system AND NEW.name IS DISTINCT FROM OLD.name THEN
      RAISE EXCEPTION 'a system role is not renamed' USING ERRCODE = 'insufficient_privilege';
    END IF;
    -- Un rol base no se edita en nada más que requires_mfa (subirlo), version y updated_at: ni id ni created_at.
    IF OLD.system AND pg_catalog.to_jsonb(NEW) - 'requires_mfa' - 'version' - 'updated_at'
                      <> pg_catalog.to_jsonb(OLD) - 'requires_mfa' - 'version' - 'updated_at' THEN
      RAISE EXCEPTION 'a system role is only edited in requires_mfa' USING ERRCODE = 'insufficient_privilege';
    END IF;
  END IF;
  -- INSERT o UPDATE de un rol base: requires_mfa no queda por debajo del de su plantilla (subirlo sí se puede).
  IF NEW.template_code IS NOT NULL THEN
    SELECT t.requires_mfa INTO template_mfa FROM iam.role_templates t WHERE t.code = NEW.template_code;
    IF FOUND AND template_mfa AND NOT NEW.requires_mfa THEN
      RAISE EXCEPTION 'requires_mfa of a system role cannot be lower than its template' USING ERRCODE = 'insufficient_privilege';
    END IF;
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER roles_guard_writes BEFORE INSERT OR UPDATE OR DELETE ON iam.roles
  FOR EACH ROW EXECUTE FUNCTION iam.guard_role_writes();

-- Solo se borra una membresía invitada (anular una invitación). Borrar una activa o suspendida no es una operación de la
-- API: se suspende (ADR 0011). Borrar una membresía no toca sus sesiones.
-- +goose StatementBegin
CREATE FUNCTION iam.guard_membership_deletes() RETURNS trigger
  LANGUAGE plpgsql
  SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
  IF OLD.status <> 'invitado'
     AND NOT coalesce((SELECT r.rolsuper FROM pg_catalog.pg_roles r WHERE r.rolname = current_user), false) THEN
    RAISE EXCEPTION 'only an invited membership is deleted' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN OLD;
END $$;
-- +goose StatementEnd
CREATE TRIGGER memberships_guard_deletes BEFORE DELETE ON iam.memberships
  FOR EACH ROW EXECUTE FUNCTION iam.guard_membership_deletes();

-- wanka_app tiene UPDATE de fila completa en iam.memberships: sin esta guarda, activo/suspendido -> invitado -> DELETE
-- saltaría la de arriba (revisión del PR #245). Una membresía que dejó de ser invitada no vuelve a serlo.
-- +goose StatementBegin
CREATE FUNCTION iam.guard_membership_updates() RETURNS trigger
  LANGUAGE plpgsql
  SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
  IF NEW.status = 'invitado' AND OLD.status <> 'invitado'
     AND NOT coalesce((SELECT r.rolsuper FROM pg_catalog.pg_roles r WHERE r.rolname = current_user), false) THEN
    RAISE EXCEPTION 'a membership never goes back to invited' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER memberships_guard_status BEFORE UPDATE OF status ON iam.memberships
  FOR EACH ROW EXECUTE FUNCTION iam.guard_membership_updates();

-- Un rol base (system) no tiene filas en iam.role_permissions: sus permisos salen de su plantilla (data-model §6). Un bug de
-- la app no le agrega permisos propios a un rol base (revisión del PR #245).
-- +goose StatementBegin
CREATE FUNCTION iam.guard_role_permission_writes() RETURNS trigger
  LANGUAGE plpgsql
  SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM iam.roles r WHERE r.tenant_id = NEW.tenant_id AND r.id = NEW.role_id AND r.system)
     AND NOT coalesce((SELECT r.rolsuper FROM pg_catalog.pg_roles r WHERE r.rolname = current_user), false) THEN
    RAISE EXCEPTION 'a system role takes its permissions from its template' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER role_permissions_guard_writes BEFORE INSERT ON iam.role_permissions
  FOR EACH ROW EXECUTE FUNCTION iam.guard_role_permission_writes();

-- ===== Vista de membresías propias (ADR 0001, «Caminos que no tienen un tenant fijado», 1) =====
-- security_invoker: corre con los permisos y la RLS de quien llama, no es SECURITY DEFINER. Filtra por app.user_id de la
-- transacción, nunca por un parámetro, y lo repite en el WHERE (no solo en las políticas) para que con un tenant fijado
-- tampoco devuelva las membresías de otros. Una membresía suspendida o borrada no aparece de ninguna forma (ADR 0011).
-- Sedes: solo las activas del alcance de una membresía activa (una invitada aún no elige sede); cajas: las activas de
-- esas sedes. Sin nulos: sin sedes o sin cajas, '[]'.
CREATE VIEW iam.list_my_memberships WITH (security_invoker = true) AS
SELECT m.id AS membership_id,
       m.tenant_id,
       coalesce(t.trade_name, t.legal_name) AS tenant_name,
       t.industry,
       t.status AS tenant_status,
       m.status AS membership_status,
       r.id AS role_id,
       r.name AS role_name,
       r.requires_mfa,
       m.all_sites,
       coalesce(s.sites, '[]'::jsonb) AS sites,
       coalesce(c.cash_registers, '[]'::jsonb) AS cash_registers
  FROM iam.memberships m
  JOIN org.tenants t ON t.id = m.tenant_id
  JOIN iam.roles r ON r.tenant_id = m.tenant_id AND r.id = m.role_id
  LEFT JOIN LATERAL (
    SELECT jsonb_agg(jsonb_build_object('id', x.id, 'code', x.code, 'name', x.name) ORDER BY x.code, x.id) AS sites
      FROM org.sites x
     WHERE x.tenant_id = m.tenant_id AND x.active AND m.status = 'activo'
       AND (m.all_sites OR EXISTS (SELECT 1 FROM iam.membership_sites ms
                                    WHERE ms.tenant_id = m.tenant_id AND ms.membership_id = m.id AND ms.site_id = x.id))
  ) s ON true
  LEFT JOIN LATERAL (
    SELECT jsonb_agg(jsonb_build_object('id', k.id, 'site_id', k.site_id, 'code', k.code) ORDER BY k.code, k.id) AS cash_registers
      FROM org.cash_registers k
      JOIN org.sites x ON x.tenant_id = k.tenant_id AND x.id = k.site_id
     WHERE k.tenant_id = m.tenant_id AND k.active AND x.active AND m.status = 'activo'
       AND (m.all_sites OR EXISTS (SELECT 1 FROM iam.membership_sites ms
                                    WHERE ms.tenant_id = m.tenant_id AND ms.membership_id = m.id AND ms.site_id = x.id))
  ) c ON true
 WHERE m.user_id = app.current_user_id() AND m.status IN ('invitado', 'activo');

-- ===== GRANT de la matriz del ADR 0001 para estas tablas, y nada más =====
-- Plantillas: catálogo global. SELECT para wanka_app (resolver de permisos y alta por autoservicio) y wanka_definer (el
-- alta desde la consola crea las filas de roles; su trigger lee la plantilla). Ningún INSERT, UPDATE, DELETE ni TRUNCATE
-- para nadie más que el dueño, y wanka_worker, wanka_webhooks y wanka_platform no reciben nada: ningún job resuelve permisos.
GRANT SELECT ON iam.role_templates, iam.role_template_permissions TO wanka_app, wanka_definer;
-- Editar los permisos de un rol propio y las sedes de una membresía; siempre bajo tenant_isolation.
GRANT SELECT, INSERT, DELETE ON iam.role_permissions, iam.membership_sites TO wanka_app;
-- Borrar un rol propio (la FK de iam.memberships rechaza uno en uso) y anular una invitación (el trigger rechaza el resto).
GRANT DELETE ON iam.roles, iam.memberships TO wanka_app;
-- Sedes y cajas: GRANT por columna, para que las columnas que #33 agrega (anexo, versión y marcas de Khipu, ADR 0013) no
-- queden dentro de un GRANT de tabla. created_at lo pone el servidor.
GRANT SELECT ON org.sites, org.cash_registers TO wanka_app;
GRANT INSERT (id, tenant_id, code, name, active) ON org.sites TO wanka_app;
GRANT UPDATE (code, name, active, updated_at) ON org.sites TO wanka_app;
GRANT INSERT (id, tenant_id, site_id, code, active) ON org.cash_registers TO wanka_app;
GRANT UPDATE (code, active) ON org.cash_registers TO wanka_app;
GRANT SELECT ON iam.list_my_memberships TO wanka_app;
