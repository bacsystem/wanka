-- +goose Up
-- Identidad (story #233, epic #3): el permiso de costos y las seis plantillas de los roles base. Corre como
-- wanka_migrator, dueño de las tablas del catálogo (ningún rol de aplicación escribe en ellas).
-- Fuente de verdad: docs/architecture/api/resources.md («Catálogo de permisos» y «Plantillas de los roles base»);
-- internal/identidad tiene un test de paridad que falla si esta migración y esa tabla difieren en un permiso, un nombre o
-- un valor de 2FA. Un permiso nuevo del catálogo entra en `dueno` con la migración que lo agrega (un test lo exige).
-- Idempotente: se puede volver a correr sin duplicar ni fallar.

-- ===== Permiso de costos (alta pendiente del contrato, resources.md) =====
INSERT INTO iam.permissions (code, description) VALUES
  ('catalogo.costs:read', 'Ver costos')
ON CONFLICT (code) DO NOTHING;

-- ===== Plantillas =====
INSERT INTO iam.role_templates (code, name, requires_mfa) VALUES
  ('dueno', 'Dueño', true),
  ('administrador', 'Administrador', true),
  ('contador', 'Contador', true),
  ('cajero', 'Cajero', false),
  ('vendedor', 'Vendedor', false),
  ('almacenero', 'Almacenero', false)
ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, requires_mfa = EXCLUDED.requires_mfa;

-- ===== Permisos de cada plantilla =====
-- `dueno` y `administrador` otorgan todo el catálogo (`*`); `administrador` sin las dos excepciones.
INSERT INTO iam.role_template_permissions (template_code, permission_code)
SELECT 'dueno', code FROM iam.permissions
ON CONFLICT DO NOTHING;

INSERT INTO iam.role_template_permissions (template_code, permission_code)
SELECT 'administrador', code FROM iam.permissions
 WHERE code NOT IN ('identidad.support_access:approve', 'empresas.einvoicing:update')
ON CONFLICT DO NOTHING;

INSERT INTO iam.role_template_permissions (template_code, permission_code) VALUES
  ('contador', 'ventas.pos:read'),
  ('contador', 'ventas.quotes:read'),
  ('contador', 'compras.suppliers:read'),
  ('contador', 'compras.orders:read'),
  ('contador', 'compras.documents:read'),
  ('contador', 'compras.documents:export'),
  ('contador', 'facturacion.cpe:read'),
  ('contador', 'facturacion.cpe:export'),
  ('contador', 'facturacion.gre:read'),
  ('contador', 'facturacion.credit_notes:create'),
  ('contador', 'reportes.dashboard:read'),
  ('contador', 'reportes.management:read'),
  ('contador', 'reportes.management:export'),
  ('contador', 'reportes.tax:read'),
  ('contador', 'reportes.tax:export'),
  ('contador', 'finanzas.cash:read'),
  ('contador', 'finanzas.cash:read_all'),
  ('contador', 'finanzas.cash:export'),
  ('contador', 'finanzas.receivables:read'),
  ('contador', 'finanzas.payables:read'),
  ('contador', 'auditoria.events:read'),
  ('contador', 'auditoria.events:export'),
  ('contador', 'clientes.customers:read'),
  ('contador', 'inventario.stock:read'),
  ('contador', 'empresas.config:read'),
  ('contador', 'catalogo.costs:read')
ON CONFLICT DO NOTHING;

INSERT INTO iam.role_template_permissions (template_code, permission_code) VALUES
  ('cajero', 'ventas.pos:read'),
  ('cajero', 'ventas.pos:create'),
  ('cajero', 'ventas.discounts:create'),
  ('cajero', 'ventas.quotes:read'),
  ('cajero', 'clientes.customers:read'),
  ('cajero', 'clientes.customers:create'),
  ('cajero', 'catalogo.items:read'),
  ('cajero', 'catalogo.prices:read'),
  ('cajero', 'inventario.stock:read'),
  ('cajero', 'finanzas.cash:read'),
  ('cajero', 'finanzas.cash:create'),
  ('cajero', 'finanzas.cash:update'),
  ('cajero', 'facturacion.cpe:read'),
  ('cajero', 'facturacion.cpe:send'),
  ('cajero', 'agenda.appointments:read'),
  ('cajero', 'agenda.appointments:update'),
  ('cajero', 'restaurante.tables:read'),
  ('cajero', 'restaurante.tables:update'),
  ('cajero', 'restaurante.orders:read'),
  ('cajero', 'restaurante.orders:create'),
  ('cajero', 'restaurante.orders:update')
ON CONFLICT DO NOTHING;

INSERT INTO iam.role_template_permissions (template_code, permission_code) VALUES
  ('vendedor', 'ventas.pos:read'),
  ('vendedor', 'ventas.pos:create'),
  ('vendedor', 'ventas.discounts:create'),
  ('vendedor', 'ventas.quotes:read'),
  ('vendedor', 'ventas.quotes:create'),
  ('vendedor', 'ventas.quotes:update'),
  ('vendedor', 'ventas.quotes:send'),
  ('vendedor', 'clientes.customers:read'),
  ('vendedor', 'clientes.customers:create'),
  ('vendedor', 'clientes.customers:update'),
  ('vendedor', 'catalogo.items:read'),
  ('vendedor', 'catalogo.prices:read'),
  ('vendedor', 'inventario.stock:read'),
  ('vendedor', 'facturacion.cpe:read'),
  ('vendedor', 'facturacion.cpe:send'),
  ('vendedor', 'finanzas.cash:read'),
  ('vendedor', 'finanzas.cash:create'),
  ('vendedor', 'finanzas.cash:update')
ON CONFLICT DO NOTHING;

INSERT INTO iam.role_template_permissions (template_code, permission_code) VALUES
  ('almacenero', 'inventario.warehouses:read'),
  ('almacenero', 'inventario.stock:read'),
  ('almacenero', 'inventario.stock:update'),
  ('almacenero', 'inventario.transfers:read'),
  ('almacenero', 'inventario.transfers:create'),
  ('almacenero', 'inventario.transfers:update'),
  ('almacenero', 'inventario.counts:read'),
  ('almacenero', 'inventario.counts:create'),
  ('almacenero', 'inventario.counts:update'),
  ('almacenero', 'inventario.consignments:read'),
  ('almacenero', 'inventario.supplier_returns:read'),
  ('almacenero', 'inventario.supplier_returns:create'),
  ('almacenero', 'catalogo.items:read'),
  ('almacenero', 'compras.orders:read'),
  ('almacenero', 'compras.documents:read'),
  ('almacenero', 'facturacion.gre:read'),
  ('almacenero', 'facturacion.gre:create')
ON CONFLICT DO NOTHING;
