-- +goose Up
-- Kernel: núcleo mínimo de iam y org para la búsqueda de sesión (issue #195, ADR 0001, 0004 y 0011,
-- data-model §6). Corre como wanka_migrator, dueño de todo lo que crea, en UNA transacción: el
-- GRANT CREATE temporal de más abajo no puede quedar aplicado si algo falla, así que este archivo
-- nunca lleva NO TRANSACTION. Desde la ola 1 los dueños de estas tablas son identidad (#24) y
-- empresas (#31), que las amplían solo hacia adelante (expandir); aquí no hay correo, contraseña,
-- RUC, razón social ni rubro.

-- Empresa. La fila propia del tenant se aísla por id. Las políticas extra (member_read, platform_rw)
-- llegan con #27/#187 y con plataforma; ver ADR 0001, lista blanca.
CREATE TABLE org.tenants (
  id uuid PRIMARY KEY,
  status text NOT NULL DEFAULT 'trial' CHECK (status IN ('trial', 'activa', 'suspendida', 'cerrada')),
  closed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (status <> 'cerrada' OR closed_at IS NOT NULL)
);
ALTER TABLE org.tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE org.tenants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON org.tenants
  USING (id = app.current_tenant()) WITH CHECK (id = app.current_tenant());

-- Usuario: identidad global, sin RLS (ADR 0001).
CREATE TABLE iam.users (
  id uuid PRIMARY KEY,
  status text NOT NULL DEFAULT 'activo' CHECK (status IN ('activo', 'bloqueado')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE iam.roles (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL REFERENCES org.tenants,
  name text NOT NULL,
  system boolean NOT NULL DEFAULT false,
  requires_mfa boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, name),
  UNIQUE (tenant_id, id)  -- destino de la FK compuesta de iam.memberships; su prefijo cubre la FK a org.tenants
);

CREATE TABLE iam.memberships (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL REFERENCES org.tenants,
  user_id uuid NOT NULL REFERENCES iam.users,
  role_id uuid NOT NULL,
  all_sites boolean NOT NULL DEFAULT true,
  status text NOT NULL CHECK (status IN ('invitado', 'activo', 'suspendido')),
  authz_version bigint NOT NULL DEFAULT 1 CHECK (authz_version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (tenant_id, role_id) REFERENCES iam.roles (tenant_id, id),
  UNIQUE (tenant_id, user_id),  -- su prefijo cubre la FK a org.tenants
  UNIQUE (tenant_id, id)
);
CREATE INDEX memberships_user_id_idx ON iam.memberships (user_id);
CREATE INDEX memberships_tenant_role_idx ON iam.memberships (tenant_id, role_id);

ALTER TABLE iam.roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.roles FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON iam.roles
  USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());
ALTER TABLE iam.memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.memberships FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON iam.memberships
  USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());

-- Sesión: sin RLS porque se busca por hash ANTES de conocer el tenant (ADR 0004).
-- Sin FK a iam.memberships, org.tenants, org.sites ni org.cash_registers, a propósito: borrar una
-- membresía no debe fallar ni cascadear sobre la sesión. La búsqueda la devuelve sin versión actual
-- y el middleware la desliga sin revocarla (ADR 0011); una FK lo impediría. Por lo mismo, el
-- único vínculo con la empresa es el que fijan los CHECK de abajo.
CREATE TABLE iam.sessions (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES iam.users,
  tenant_id uuid,
  site_id uuid,
  cash_register_id uuid,
  membership_id uuid,
  access_hash bytea NOT NULL UNIQUE CHECK (octet_length(access_hash) = 32),
  access_expires_at timestamptz NOT NULL,
  refresh_hash bytea UNIQUE CHECK (octet_length(refresh_hash) = 32),
  refresh_family uuid,
  prev_refresh_hash bytea,
  rotated_at timestamptz,
  refresh_expires_at timestamptz,
  mfa_satisfied boolean NOT NULL,
  mfa_method text NOT NULL CHECK (mfa_method IN ('totp', 'recovery_code', 'none', 'platform')),
  impersonator_id uuid,
  authz_version bigint,
  ip inet,
  user_agent text,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  -- Con nombre propio para que rls-check/mutations pueda quitarlos uno a uno.
  -- Una sesión con 2FA de plataforma es siempre impersonada, y al revés.
  CONSTRAINT sessions_platform_iff_impersonated CHECK ((mfa_method = 'platform') = (impersonator_id IS NOT NULL)),
  -- Nunca una membresía sin empresa ni una empresa sin membresía (salvo la impersonada).
  CONSTRAINT sessions_membership_needs_tenant CHECK (membership_id IS NULL OR tenant_id IS NOT NULL),
  CONSTRAINT sessions_tenant_needs_membership CHECK (tenant_id IS NULL OR membership_id IS NOT NULL OR impersonator_id IS NOT NULL),
  -- Una sesión desligada o impersonada nunca tiene versión (ADR 0011).
  CONSTRAINT sessions_version_needs_membership CHECK (authz_version IS NULL OR membership_id IS NOT NULL),
  -- Una sesión impersonada no tiene refresh y vive 60 min como máximo (data-model §6, «Vigencia del grant»).
  CONSTRAINT sessions_impersonated_short_no_refresh CHECK (impersonator_id IS NULL
    OR (refresh_hash IS NULL AND access_expires_at <= created_at + interval '60 minutes'))
);
CREATE INDEX sessions_user_id_idx ON iam.sessions (user_id);
CREATE INDEX sessions_created_at_idx ON iam.sessions (created_at);  -- limpieza (data-model §5)

-- Búsqueda de sesión (ADR 0004). La corre wanka_app sin tenant ni usuario. Un JOIN directo contra
-- iam.memberships devolvería NULL (FORCE RLS y un dueño que no se salta las políticas), por eso es plpgsql: lee la
-- sesión (sin RLS), fija el tenant de ESA sesión con set_config(..., true), lee la membresía por
-- tenant_isolation y restaura el valor del llamador. Si falla, la transacción revierte el
-- set_config. La cláusula SET app.tenant_id en CREATE FUNCTION no sirve: un rol sin superusuario
-- recibe «permission denied to set parameter» en PostgreSQL 16.
-- +goose StatementBegin
CREATE FUNCTION iam.session_lookup(p_access_hash bytea)
  RETURNS TABLE (session_id uuid, user_id uuid, tenant_id uuid, site_id uuid, cash_register_id uuid, membership_id uuid,
                 session_authz_version bigint, mfa_satisfied boolean, mfa_method text, impersonator_id uuid,
                 current_authz_version bigint, membership_status text)
  LANGUAGE plpgsql STABLE SECURITY DEFINER
  SET search_path = pg_catalog, pg_temp
AS $$
DECLARE s iam.sessions%ROWTYPE; v bigint; st text; prev text := pg_catalog.current_setting('app.tenant_id', true);
BEGIN
  SELECT * INTO s FROM iam.sessions x
   WHERE x.access_hash = p_access_hash AND x.revoked_at IS NULL AND x.access_expires_at > pg_catalog.now()
     AND x.mfa_satisfied;   -- una sesión a medio login (sin el código TOTP) no autentica peticiones
  IF NOT FOUND THEN RETURN; END IF;
  IF s.membership_id IS NOT NULL THEN
    PERFORM pg_catalog.set_config('app.tenant_id', s.tenant_id::text, true);   -- el tenant de ESA sesión
    SELECT m.authz_version, m.status INTO v, st FROM iam.memberships m WHERE m.id = s.membership_id;  -- vía tenant_isolation
    PERFORM pg_catalog.set_config('app.tenant_id', coalesce(prev, ''), true);  -- restaura el valor del llamador
  END IF;
  RETURN QUERY SELECT s.id, s.user_id, s.tenant_id, s.site_id, s.cash_register_id, s.membership_id, s.authz_version,
                      s.mfa_satisfied, s.mfa_method, s.impersonator_id, v, st;
END $$;
-- +goose StatementEnd

-- Dueño wanka_definer (NOLOGIN, sin tablas propias; ADR 0001). ALTER ... OWNER exige que
-- el nuevo dueño tenga CREATE en el esquema: se concede solo para el cambio y se revoca enseguida,
-- en esta misma transacción.
GRANT CREATE ON SCHEMA iam TO wanka_definer;
ALTER FUNCTION iam.session_lookup(bytea) OWNER TO wanka_definer;
REVOKE CREATE ON SCHEMA iam FROM wanka_definer;
REVOKE ALL ON FUNCTION iam.session_lookup(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION iam.session_lookup(bytea) TO wanka_app;

-- GRANT de la matriz del ADR 0001 para estas tablas, y nada más. wanka_webhooks y wanka_platform no
-- reciben nada aquí (platform_rw llega con plataforma).
GRANT USAGE ON SCHEMA iam, org TO wanka_app, wanka_worker, wanka_definer;
GRANT SELECT, INSERT, UPDATE ON iam.users, iam.sessions, iam.roles, iam.memberships TO wanka_app;
GRANT SELECT ON org.tenants TO wanka_app;
-- Limpieza de sesiones: DELETE, más SELECT por columna sobre lo que usa el WHERE (sin él, el DELETE da
-- «permission denied»). Nunca los hashes ni user_id/tenant_id (aclaración del ADR 0001, PR #197).
GRANT DELETE ON iam.sessions TO wanka_worker;
GRANT SELECT (id, created_at, revoked_at, access_expires_at, refresh_expires_at) ON iam.sessions TO wanka_worker;
GRANT SELECT ON iam.memberships, iam.roles, org.tenants TO wanka_worker;
GRANT SELECT ON iam.sessions, iam.memberships TO wanka_definer;
