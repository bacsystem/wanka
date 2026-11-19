-- +goose Up
-- Kernel: tablas de entitlements de platform (issue #171, ADR 0005 y ADR 0001, er-05). Corre como
-- wanka_migrator, dueño de todo lo que crea. Son tablas SIN RLS (esquema platform): el aislamiento
-- entre tenants lo da la lectura de internal/platform/entitlements, que filtra siempre por
-- app.current_tenant(). Aquí no hay datos de ninguna empresa; la suscripción de cada empresa la crea
-- empresas (#31) y los overrides y el catálogo los gestiona plataforma (ola 6). Solo hacia adelante.

-- Catálogo de módulos (ADR 0005): core, rubro y addon. Una ruta declara su módulo por este código.
CREATE TABLE platform.modules (
  code text PRIMARY KEY CHECK (code ~ '^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)*$'),
  kind text NOT NULL CHECK (kind IN ('core', 'rubro', 'addon')),
  description text NOT NULL
);

-- limits: topes numéricos por clave (users, sites, cpe_per_month). Una clave ausente es «sin tope».
-- non_production marca el plan que solo existe para desarrollo y pruebas: asignarlo a una empresa real
-- es una decisión del dueño (issue #171, Notas) y lo hace empresas (#31), nunca un valor por defecto.
CREATE TABLE platform.plans (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE,
  name text NOT NULL,
  limits jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(limits) = 'object'),
  active boolean NOT NULL DEFAULT true,
  non_production boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE platform.plan_modules (
  plan_id uuid NOT NULL REFERENCES platform.plans,
  module_code text NOT NULL REFERENCES platform.modules,
  PRIMARY KEY (plan_id, module_code)
);
CREATE INDEX plan_modules_module_code_idx ON platform.plan_modules (module_code);

-- Sin FK a org.tenants, como el resto de platform.* (er-05): el historial de cobro no debe impedir
-- ni cascadear la purga de una empresa cerrada.
CREATE TABLE platform.subscriptions (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL,
  plan_id uuid NOT NULL REFERENCES platform.plans,
  status text NOT NULL CHECK (status IN ('trial', 'activa', 'morosa', 'suspendida', 'cancelada')),
  period_start date NOT NULL DEFAULT current_date,
  period_end date,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (period_end IS NULL OR period_end >= period_start)
);
-- Una suscripción viva por empresa: las canceladas son historial y pueden ser varias.
CREATE UNIQUE INDEX subscriptions_one_live_per_tenant ON platform.subscriptions (tenant_id) WHERE status <> 'cancelada';
CREATE INDEX subscriptions_tenant_created_idx ON platform.subscriptions (tenant_id, created_at DESC);
CREATE INDEX subscriptions_plan_id_idx ON platform.subscriptions (plan_id);

CREATE TABLE platform.tenant_module_overrides (
  tenant_id uuid NOT NULL,
  module_code text NOT NULL REFERENCES platform.modules,
  enabled boolean NOT NULL,
  reason text,
  set_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, module_code)
);
CREATE INDEX tenant_module_overrides_module_code_idx ON platform.tenant_module_overrides (module_code);

-- default_on = true enciende el flag para todos; si no, rollout_pct es el porcentaje de empresas que lo
-- tienen, repartidas por hash(tenant_id, key) (lo calcula Go, igual en cada réplica). Un override del
-- tenant gana sobre ambos.
CREATE TABLE platform.feature_flags (
  key text PRIMARY KEY CHECK (key ~ '^[a-z][a-z0-9_.-]*$'),
  description text NOT NULL DEFAULT '',
  rollout_pct integer NOT NULL DEFAULT 0 CHECK (rollout_pct BETWEEN 0 AND 100),
  default_on boolean NOT NULL DEFAULT false
);

CREATE TABLE platform.tenant_flag_overrides (
  tenant_id uuid NOT NULL,
  flag_key text NOT NULL REFERENCES platform.feature_flags,
  enabled boolean NOT NULL,
  PRIMARY KEY (tenant_id, flag_key)
);
CREATE INDEX tenant_flag_overrides_flag_key_idx ON platform.tenant_flag_overrides (flag_key);

-- Uso por mes (primer día del mes, hora de Lima). Lo escribe wanka_worker; lo lee el límite del plan.
CREATE TABLE platform.tenant_usage (
  tenant_id uuid NOT NULL,
  period date NOT NULL CHECK (period = date_trunc('month', period)::date),
  cpe_issued integer NOT NULL DEFAULT 0 CHECK (cpe_issued >= 0),
  active_users integer NOT NULL DEFAULT 0 CHECK (active_users >= 0),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, period)
);

-- Semilla: el catálogo de módulos del ADR 0005 y el plan «semilla» con todos, solo para desarrollo y
-- pruebas. «identidad» y «empresas» son módulos para que toda ruta declare uno (x-module).
INSERT INTO platform.modules (code, kind, description) VALUES
  ('identidad', 'core', 'Usuarios, roles, sesiones y accesos'),
  ('empresas', 'core', 'Datos de la empresa, sedes y cajas'),
  ('catalogo', 'core', 'Productos, servicios, variantes y precios'),
  ('clientes', 'core', 'Clientes y proveedores'),
  ('inventario', 'core', 'Inventario multialmacén'),
  ('ventas', 'core', 'Punto de venta, proformas y cobros'),
  ('compras', 'core', 'Órdenes de compra y recepciones'),
  ('finanzas', 'core', 'Caja, cuentas por cobrar y por pagar'),
  ('facturacion-electronica', 'core', 'Comprobantes electrónicos SUNAT vía Khipu'),
  ('reportes', 'core', 'Reportes y exportes'),
  ('notificaciones', 'core', 'Avisos por correo y recordatorios'),
  ('auditoria', 'core', 'Registro de auditoría de la empresa'),
  ('integraciones', 'core', 'Integraciones con servicios externos'),
  ('rubro.lubricentro', 'rubro', 'Lubricentros: órdenes de servicio y vehículos'),
  ('rubro.libreria', 'rubro', 'Librerías: ediciones, ISBN y consignación'),
  ('rubro.restaurante', 'rubro', 'Restaurantes: salón, comandas, delivery y reservas'),
  ('rubro.veterinaria', 'rubro', 'Veterinarias: mascotas, hospitalización y grooming'),
  ('rubro.odontologia', 'rubro', 'Odontologías: agenda e historia clínica'),
  ('rubro.retail', 'rubro', 'Retail: tiendas y variantes de producto'),
  ('agenda', 'addon', 'Agenda de citas'),
  ('integraciones.whatsapp', 'addon', 'Mensajes por WhatsApp');

INSERT INTO platform.plans (code, name, limits, non_production)
  VALUES ('semilla', 'Semilla (solo desarrollo y pruebas)', '{"users": 1000, "sites": 100, "cpe_per_month": 1000000}', true);
INSERT INTO platform.plan_modules (plan_id, module_code)
  SELECT p.id, m.code FROM platform.plans p CROSS JOIN platform.modules m WHERE p.code = 'semilla';

-- Cambios que invalidan la caché de entitlements de cada réplica (LISTEN entitlements_changed). El
-- payload es el tenant afectado, o vacío cuando el cambio alcanza a todos (catálogo, planes, flags).
-- Corren con el rol que escribe (sin SECURITY DEFINER): NOTIFY no necesita privilegios. tenant_usage no
-- notifica: cambia con cada comprobante y la caché de 30 s acota su retraso.
-- +goose StatementBegin
CREATE FUNCTION platform.notify_tenant_entitlements() RETURNS trigger
  LANGUAGE plpgsql
  SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
  -- TG_ARGV[0] es la columna que lleva el tenant (tenant_id; id en org.tenants).
  PERFORM pg_catalog.pg_notify('entitlements_changed',
    coalesce((CASE WHEN TG_OP = 'DELETE' THEN pg_catalog.to_jsonb(OLD) ELSE pg_catalog.to_jsonb(NEW) END) ->> TG_ARGV[0], ''));
  RETURN NULL;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION platform.notify_all_entitlements() RETURNS trigger
  LANGUAGE plpgsql
  SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
  PERFORM pg_catalog.pg_notify('entitlements_changed', '');
  RETURN NULL;
END $$;
-- +goose StatementEnd

CREATE TRIGGER subscriptions_notify AFTER INSERT OR UPDATE OR DELETE ON platform.subscriptions
  FOR EACH ROW EXECUTE FUNCTION platform.notify_tenant_entitlements('tenant_id');
CREATE TRIGGER tenant_module_overrides_notify AFTER INSERT OR UPDATE OR DELETE ON platform.tenant_module_overrides
  FOR EACH ROW EXECUTE FUNCTION platform.notify_tenant_entitlements('tenant_id');
CREATE TRIGGER tenant_flag_overrides_notify AFTER INSERT OR UPDATE OR DELETE ON platform.tenant_flag_overrides
  FOR EACH ROW EXECUTE FUNCTION platform.notify_tenant_entitlements('tenant_id');
-- Cerrar o reabrir una empresa cambia su readOnly.
CREATE TRIGGER tenants_entitlements_notify AFTER UPDATE OF status, closed_at OR DELETE ON org.tenants
  FOR EACH ROW EXECUTE FUNCTION platform.notify_tenant_entitlements('id');
CREATE TRIGGER modules_notify AFTER INSERT OR UPDATE OR DELETE ON platform.modules
  FOR EACH STATEMENT EXECUTE FUNCTION platform.notify_all_entitlements();
CREATE TRIGGER plans_notify AFTER INSERT OR UPDATE OR DELETE ON platform.plans
  FOR EACH STATEMENT EXECUTE FUNCTION platform.notify_all_entitlements();
CREATE TRIGGER plan_modules_notify AFTER INSERT OR UPDATE OR DELETE ON platform.plan_modules
  FOR EACH STATEMENT EXECUTE FUNCTION platform.notify_all_entitlements();
CREATE TRIGGER feature_flags_notify AFTER INSERT OR UPDATE OR DELETE ON platform.feature_flags
  FOR EACH STATEMENT EXECUTE FUNCTION platform.notify_all_entitlements();

-- GRANT de la matriz del ADR 0001: wanka_app y wanka_worker solo leen; wanka_worker además escribe el uso.
-- wanka_app también lee tenant_usage (aclaración de #171): el límite del plan se compara con el uso en la
-- misma lectura de entitlements. wanka_webhooks y wanka_platform no reciben nada aquí (plataforma, ola 6).
-- Sin GRANT a PUBLIC: el USAGE del esquema no da acceso a ninguna tabla de platform que no se nombre abajo.
GRANT USAGE ON SCHEMA platform TO wanka_app, wanka_worker;
GRANT SELECT ON platform.modules, platform.plans, platform.plan_modules, platform.subscriptions,
  platform.tenant_module_overrides, platform.feature_flags, platform.tenant_flag_overrides, platform.tenant_usage
  TO wanka_app, wanka_worker;
GRANT INSERT, UPDATE ON platform.tenant_usage TO wanka_worker;
