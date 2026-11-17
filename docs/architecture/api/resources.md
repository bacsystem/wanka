# Recursos de la API por módulo

Estado: borrador (2026-11-17). Derivado de las pantallas (`apps/web/src/app/**/page.tsx`) y de sus mocks (`apps/web/src/features/*/mocks/*.ts`), que reflejan la forma que devolverá la API. Cada fila indica la pantalla que lo consume, el mock que fija la forma de la respuesta y el permiso requerido (`modulo.recurso:accion`, acciones `read | create | update | void | export` que corresponden a "Ver, Crear, Editar, Anular, Exportar" de `settings/mocks/settings.ts`).

Convenciones (cursor, errores, idempotencia, dinero como string): [`conventions.md`](conventions.md). Todo `POST` marcado **(I)** exige `Idempotency-Key`. Los contratos definitivos de cada endpoint (request/response con tipos y opcionalidad) se publican en el issue que lo implementa y terminan en `openapi.yaml`.

Ajustes de forma respecto a los mocks, comunes a todos los recursos:
- Importes `number` → string decimal; IDs → UUID string.
- Relaciones por id + nombre de visualización: los mocks traen solo nombres (`supplier: string`, `warehouse: string`); la API devuelve `supplierId` y `supplierName`.
- Textos ya formateados en los mocks (`lastAccess: "Hoy 10:42"`, `valueLabel`, `sitesNote`, `detail`) no existen en la API: llegan los datos crudos y el frontend formatea.

## identidad (`iam`)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `POST /v1/auth/login` | `/login` | — | público, rate limit |
| `POST /v1/auth/mfa/verify` | `/login` paso 2FA | — | challenge token |
| `POST /v1/auth/mfa/totp/enroll`, `.../confirm` | `/perfil` | — | sesión |
| `POST /v1/auth/refresh` | BFF | — | refresh token |
| `POST /v1/auth/logout` | topbar | — | sesión |
| `POST /v1/auth/password/forgot`, `.../reset` | `/login` recuperar | — | público, rate limit |
| `GET /v1/me/memberships` | `/seleccionar-empresa` | `auth/mocks/tenants.ts` `TenantOption`, `tenantSites` | sesión sin tenant |
| `POST /v1/auth/session/tenant` | `/seleccionar-empresa`, `TenantSwitcher` | — | membresía activa |
| `GET /v1/me`, `GET /v1/me/entitlements` | shell (sidebar por rubro y módulos) | `Tenant`, `config/navigation.ts` | sesión |
| `GET/POST /v1/users`, `PATCH /v1/users/{id}`, `POST /v1/users/invitations` | `/configuracion/usuarios`, invite drawer | `Collaborator`, `securitySummary` | `identidad.users:*` |
| `GET/POST/PATCH /v1/roles`, `GET /v1/permissions` | matriz de permisos | `permissionMatrix`, `permissionModules` | `identidad.roles:*` |
| `GET /v1/sessions`, `DELETE /v1/sessions/{id}` | sesiones activas | `securitySummary.sessions` | `identidad.sessions:*` |

## empresas (`org`)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `GET/PATCH /v1/company` | `/configuracion/sunat` datos de empresa | `sunatConfig.company` | `empresas.config:*` |
| `GET/POST/PATCH /v1/sites` | almacenes y sedes, onboarding | `tenantSites`, `sunatEstablishments` | `empresas.sites:*` |
| `GET/POST/PATCH /v1/cash-registers` | selección de caja, config | `tenantSites[].cashboxes` | `empresas.sites:update` |
| `GET/POST/PATCH /v1/document-series` | `/configuracion/sunat` series | `sunatConfig.series` | `empresas.series:*` |
| `GET/PUT /v1/onboarding` (pasos 1..6) | `/onboarding` | `industryOptions`, `modulesByIndustry` | propietario |
| `GET/PUT /v1/einvoicing/credentials` | conexión con Khipu: el propietario pega la API key `fk_…` de su empresa en Khipu; Wanka la cifra, la verifica con `GET /v1/empresa` de Khipu y registra las series con `correlativo_inicial` | `sunatConfig.connection`, solo metadatos (entorno BETA/PRODUCCION, verificado, prefijo de la llave; la llave nunca se devuelve). El certificado y la clave SOL se cargan **en Khipu**, no en Wanka | `empresas.config:update` |

## clientes (`crm`)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `GET /v1/customers?q=&segment=&status=&cursor=` | `/clientes`, buscador POS, ⌘K | `Customer` | `clientes.customers:read` |
| `GET /v1/customers/summary` | KPIs de `/clientes` | — | idem |
| `POST /v1/customers`, `PATCH /v1/customers/{id}` | alta rápida POS, ficha | `Customer` | `clientes.customers:create/update` |
| `GET /v1/customers/{id}` | `/clientes/[id]` | `CustomerDetail` (kpis, billing, alerts) | read |
| `GET /v1/customers/{id}/ledger`, `/documents`, `/files` | pestañas de la ficha | `CustomerDetail.ledger/documents/files` | read |
| `GET /v1/lookups/ruc/{ruc}`, `/v1/lookups/dni/{dni}` | autocompletar | — | create. RUC vía Khipu `GET /v1/consultas/ruc/{ruc}` con la llave del tenant (60/min y 5 000/día por empresa) y caché de 24 h; timeout 3 s; si falla se ingresa a mano. Fuente del DNI a definir |

## catalogo (`catalog`)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `GET /v1/catalog/items?nature=&category=&q=` | `/catalogo/[[...tab]]` | `CatalogEntry`, `catalogSummary` | `catalogo.items:read` |
| `GET /v1/catalog/pos-items?site=` | POS (precio + stock de la sede) | `CatalogItem` (`pos/mocks/catalog.ts`) | `ventas.pos:create` |
| `GET /v1/catalog/lookup?code=` | escáner POS (SKU, EAN, ISBN) | `CatalogItem` | idem |
| `POST/PATCH /v1/catalog/items` | alta/edición | `CatalogEntry` | create/update |
| `GET/POST/PATCH /v1/catalog/categories` | categorías | `CatalogCategory` | `catalogo.categories:*` |
| `GET /v1/catalog/items/{id}/variants`, `PUT` (matriz) | `/catalogo/variantes/[sku]` | `Variant`, `productMock` | `catalogo.items:update` |
| `GET/POST/PATCH /v1/catalog/price-lists` | listas de precio | `priceLists` | `catalogo.prices:*` |

## inventario (`inv`)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `GET /v1/inventory/warehouses`, `POST/PATCH` | `/inventario/almacenes` | `WarehouseCard`, `warehousesSummary` | `inventario.warehouses:*` |
| `GET /v1/inventory/stock-levels?warehouse=&state=&category=` | `/inventario/stock`, alertas dashboard | `StockItem`, `StockAlert` | `inventario.stock:read` |
| `GET /v1/inventory/movements?kind=&from=&to=` | `/inventario/movimientos` | `InventoryMovement`, `movementsSummary` | read |
| `POST /v1/inventory/adjustments` **(I)** | merma / ajuste | `InventoryMovement` | `inventario.stock:update` |
| `GET /v1/inventory/kardex?variant=&warehouse=&period=` | `/inventario/kardex` (13.1) | `KardexRow`, `kardexItem` | read / export |
| `POST /v1/inventory/transfers` **(I)**, `GET /v1/inventory/transfers` | traslados, `transfersInTransit` | — | `inventario.transfers:create` |
| `POST /v1/inventory/transfers/{id}/receipts` **(I)** | `/inventario/recepcion/[guia]` | `ReceptionLine`, `receptionMock` | `inventario.transfers:update` |
| `POST /v1/inventory/counts`, `PUT .../lines`, `POST .../close` **(I)** | `/inventario/toma-fisica` | `CountLine` | `inventario.counts:*` |

## ventas (`sales`)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `POST /v1/sales` **(I)** | `/ventas/nueva` (POS) | `CartLine`, `PaymentMethod`; responde venta + CPE `pendiente` | `ventas.pos:create` |
| `GET /v1/sales/{id}` | reimpresión | — | `ventas.pos:read` |
| `GET /v1/quotes`, `/summary` | `/ventas/proformas` | `Quote`, `quotesSummary` | `ventas.quotes:read` |
| `POST /v1/quotes` **(I)**, `PATCH /v1/quotes/{id}` (If-Match) | `/ventas/proformas/nueva`, `/[id]` | `quoteDetailMock`, `QuoteLine` | create/update |
| `POST /v1/quotes/{id}/send`, `/convert` **(I)** | enviar, convertir a venta | — | update / `ventas.pos:create` |
| `GET /public/quotes/{token}`, `POST .../approve` | enlace de aprobación del cliente | — | token de un uso, rate limit |

## facturacion-electronica (`einv`)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `GET /v1/einvoicing/documents?type=&status=&from=&to=&q=` | `/ventas/comprobantes`, dashboard | `SalesDocument`, `documentsSummary` | `facturacion.cpe:read` |
| `GET /v1/einvoicing/documents/{id}` | `/ventas/comprobantes/[id]` | `SalesDocumentDetail` (lines, cdr, timeline) | read |
| `GET /v1/einvoicing/documents/{id}/files/{xml\|cdr\|pdf}` | descargas | 302 a URL prefirmada (5 min) | read |
| `POST /v1/einvoicing/documents/{id}/credit-notes` **(I)** | `/nota-credito` (catálogo 09) | `SalesDocumentDetail` | `facturacion.cpe:void` |
| `POST /v1/einvoicing/documents/{id}/debit-notes` **(I)** | `/nota-debito` (catálogo 10, serie FD01) | idem | `facturacion.cpe:create` |
| `POST /v1/einvoicing/documents/{id}/void` **(I)** | diálogo comunicación de baja | — | `facturacion.cpe:void` |
| `POST /v1/einvoicing/documents/{id}/retry` | cola SUNAT, reintento manual | — | `facturacion.cpe:update` |
| `GET /v1/einvoicing/queue` | dashboard "cola SUNAT" | `SunatQueueItem` | read |
| `GET /v1/dispatch-guides`, `/{id}`, `POST` **(I)** | `/ventas/guias`, `/[id]` | `DispatchGuide`, `guideDetailMock` | `facturacion.gre:*` |
| `POST /v1/dispatch-guides/{id}/events` | seguimiento e incidencia | `guideDetailMock.timeline` | `facturacion.gre:update` |
| `POST /webhooks/khipu/{routeId}` | Khipu (un endpoint por tenant) | `X-Khipu-Signature` HMAC-SHA256, dedupe por `X-Khipu-Delivery` (ADR 0003) | — |

## compras (`purch`)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `GET /v1/purchases/documents`, `/summary`, `POST` **(I)** | `/compras` (RCE/SIRE) | `Purchase`, `purchasesSummary` | `compras.documents:*` |
| `POST /v1/purchases/sire/sync` | conciliación SIRE (job asíncrono, 202) | `SireStatus` | `compras.documents:update` |
| `GET/POST/PATCH /v1/suppliers`, `GET /{id}` | `/compras/proveedores`, `/[id]` | `Supplier` | `compras.suppliers:*` |
| `GET/POST /v1/purchase-orders` **(I)**, `PATCH /{id}` | `/compras/ordenes`, `/nueva` | `PurchaseOrder` | `compras.orders:*` |
| `POST /v1/purchase-orders/{id}/receipts` **(I)** | recepción contra OC (entrada de stock) | — | `inventario.stock:update` |
| `GET/POST /v1/consignments` **(I)**, `GET /{id}` | consignación con distribuidoras (librería primero; flag `compras.consignacion`) | `er-04` `CONSIGNMENTS` | `compras.consignments:*` |
| `POST /v1/consignments/{id}/settlements` **(I)** | liquidación de lo vendido | — | `compras.consignments:update` |
| `GET/POST /v1/supplier-returns` **(I)** | devoluciones al proveedor con plazo | `SUPPLIER_RETURNS` | `compras.returns:*` |

## finanzas (`fin`)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `POST /v1/cash/shifts` **(I)** (apertura) | selección de caja | `tenantSites[].cashboxes` | `finanzas.cash:create` |
| `GET /v1/cash/shifts/current?register=` | `/finanzas/caja` | `shift` (`cash/mocks/cash.ts`) | `finanzas.cash:read` |
| `GET /v1/cash/shifts/{id}/movements`, `POST` **(I)** (egreso/retiro) | movimientos de caja | `CashMovement` | read / create |
| `POST .../count-start`, `PUT .../counts`, `POST .../close` **(I)** | arqueo y Z | `shift.channels`, `denominations` | `finanzas.cash:update`, `:approve` |
| `GET /v1/cash/z-reports`, `/{id}` | `/finanzas/caja/historial` | `Shift` (`shifts.ts`) | read / export |
| `GET /v1/receivables`, `/summary`, `POST /{id}/payments` **(I)** | `/finanzas/cuentas-por-cobrar` | `Receivable`, `receivablesSummary` | `finanzas.receivables:*` |
| `GET /v1/payables`, `/summary`, `POST /{id}/payments` **(I)** | `/finanzas/cuentas-por-pagar` | `Payable`, `payablesSummary` | `finanzas.payables:*` |

## reportes (`rpt`, solo lectura)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `GET /v1/dashboard` | `/` (por rubro) | `DashboardData` | `reportes.dashboard:read` |
| `GET /v1/reports/management?from=&to=&site=` | `/reportes/gestion` | `management.ts` | `reportes.management:read` |
| `GET /v1/reports/sales-register?period=` | `/reportes` RVIE | `SalesRegisterRow`, `reportsSummary` | `reportes.tax:read` |
| `GET /v1/reports/pdt621?period=` | `/reportes/liquidacion` | `pdt621`, `kardexConsolidated` | `reportes.tax:read` |
| `POST /v1/exports` **(I)**, `GET /v1/exports/{id}` | exportar Excel/PDF/TXT SUNAT | 202 + URL prefirmada al terminar | `*:export` del módulo |

## agenda, integraciones, auditoria, notificaciones

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `GET /v1/agenda/resources` (sillones, consultorios, bahías) | `/agenda` | `Chair` | `agenda.appointments:read` |
| `GET /v1/agenda/appointments?date=&resource=`, `POST` **(I)**, `PATCH` | `/agenda`, reprogramar, lista de espera | `Appointment`, `agendaSummary` | `agenda.appointments:*` |
| `GET /v1/integrations`, `PUT /{id}` | `/configuracion/integraciones` | `Integration` | `integraciones.config:*` |
| `GET /v1/audit/events?module=&user=&from=` | `/configuracion/auditoria` | `AuditEvent` (`shifts.ts`) | `auditoria.events:read` |
| `GET /v1/notifications`, `POST /{id}/read` | campana del topbar | — | sesión |

## Rubros

| Método y ruta | Pantalla / uso | Forma | Módulo |
|---|---|---|---|
| `GET/POST/PATCH /v1/lubricentro/vehicles`, `GET ?plate=` | recepción por placa (pantalla por diseñar) | ver `data/er-06-rubros.mmd` | `rubro.lubricentro` |
| `POST /v1/lubricentro/vehicles/{id}/odometer` | lectura de km | — | idem |
| `GET/POST /v1/lubricentro/service-orders` **(I)**, `PATCH /{id}` (If-Match), `POST /{id}/checkout` **(I)** | OT → venta | — | idem |
| `GET/PUT /v1/lubricentro/service-rules`, `GET /v1/lubricentro/reminders` | recordatorios por km | — | idem |
| `GET/PUT /v1/libreria/books/{itemId}`, `GET /v1/libreria/books?isbn=&author=` | ficha de libro | `book_details` | `rubro.libreria` |
| `GET/POST/PATCH /v1/libreria/school-lists`, `POST /{id}/to-quote` **(I)** | listas escolares | — | idem |
| `GET /v1/restaurant/tables`, `PATCH /{id}`, `GET/POST /v1/restaurant/orders`, `GET /v1/restaurant/kds` | `/salon`, `/salon/cocina` | `DiningTable`, `OrderItem`, `KdsTicket` | `rubro.restaurante` |
| `GET/POST /v1/restaurant/deliveries`, `/riders`, `/reservations` | `/salon/delivery`, `/salon/reservas` | `DeliveryOrder`, `Rider`, `Reservation` | idem |
| `GET/POST/PATCH /v1/vet/pets`, `/{id}/vaccines`, `/{id}/evolutions` | `/mascotas`, `/mascotas/[id]` | `Pet`, `Vaccine`, `VetEvolution` | `rubro.veterinaria` |
| `GET /v1/vet/kennels`, `POST /v1/vet/hospitalizations/{id}/vitals`, `/medications` | `/hospitalizacion` | `Kennel`, `Vital`, `Medication` | idem |
| `GET/POST /v1/vet/grooming-jobs` | `/grooming` | `GroomingJob` | idem |
| `GET/PUT /v1/dental/patients/{customerId}/odontogram`, `GET/POST .../evolutions`, `.../treatment-plan` | `/clientes/[id]/historia` | `odontogram.ts` (`ToothState`, `Evolution`) | `rubro.odontologia` |

## Plano plataforma (`platform-api`, `/platform/v1`)

| Método y ruta | Pantalla | Forma (mock) | Rol de staff |
|---|---|---|---|
| `POST /platform/v1/auth/login`, `/mfa/verify`, `/refresh`, `/logout` | `/admin/login` (2FA obligatorio; SSO en ADR futuro) | — | — |
| `GET /platform/v1/dashboard` | `/admin` | `platformSummary`, `mrrSeries` (agregados) | superadmin, finanzas |
| `GET /platform/v1/tenants`, `GET /{id}`, `POST` **(I)**, `PATCH /{id}` | `/admin/empresas`, `/[id]` | `PlatformTenant` (los precios de plan se leen de `plans`, no se duplican en el tenant) | `tenants:*` |
| `POST /platform/v1/tenants/{id}/suspend`, `/reactivate` **(I)** | acciones de tenant | — | `tenants:suspend` |
| `POST /platform/v1/tenants/{id}/impersonations` **(I)** | soporte | — | `tenants:impersonate`, requiere ticket |
| `GET/POST/PATCH /platform/v1/plans`, `/subscriptions` | `/admin/planes` | `Plan` | finanzas, ventas |
| `GET /platform/v1/saas-invoices` | `/admin/facturacion` | `SaasInvoice` | finanzas |
| `GET/PATCH /platform/v1/modules`, `/feature-flags`, `PUT /tenants/{id}/overrides` | `/admin/modulos` | `FeatureFlag` | superadmin, ingeniería |
| `GET /platform/v1/monitoring/providers`, `/incidents`, `/certificates` | `/admin/monitoreo` | `providers`, `latency24h`, `expiringCerts`, `rejectTop` | soporte, ingeniería |
| `GET/POST/PATCH /platform/v1/tickets` | `/admin/soporte` | `Ticket`, `supportSummary` | soporte |
| `GET/POST/PATCH /platform/v1/staff` | `/admin/usuarios` | `StaffUser`, `roleMatrix` | superadmin |
| `GET /platform/v1/audit` | `/admin/auditoria` | `AuditEvent` (`ops.ts`) | superadmin |

Nota: el mock de monitoreo muestra varios OSE (`providers`, `OseProvider`). Con Khipu como proveedor único, `monitoring/providers` reporta la salud del adaptador Khipu (latencia, cola, rechazos); la lista multi-OSE del diseño queda como visión futura si se agrega un segundo `InvoicingProvider`.
