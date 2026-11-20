# Recursos de la API por módulo

Estado: **congelado (2026-11-18)**, historia #160. Un cambio a este documento (ruta, forma, permiso o catálogo) es un PR de contrato aprobado por el architect, con las reglas de [`conventions.md` §0](conventions.md#0-contrato-congelado-cómo-se-cambia). Derivado de las pantallas (`apps/web/src/app/**/page.tsx`) y de sus mocks (`apps/web/src/features/*/mocks/*.ts`), que reflejan la forma que devolverá la API. Cada fila indica la pantalla que lo consume, el mock que fija la forma de la respuesta y el permiso requerido (`modulo.recurso:accion`, acciones `read | create | update | void | export` que corresponden a "Ver, Crear, Editar, Anular, Exportar" de `settings/mocks/settings.ts`).

Convenciones (cursor, errores, idempotencia, dinero como string): [`conventions.md`](conventions.md). Todo `POST` marcado **(I)** exige `Idempotency-Key`. Los contratos definitivos de cada endpoint (request/response con tipos y opcionalidad) se publican en el issue que lo implementa y terminan en el archivo de su módulo en `apps/api/openapi/paths/`.

Ajustes de forma respecto a los mocks, comunes a todos los recursos:
- Importes `number` → string decimal; IDs → UUID string.
- **Todo recurso o petición con dinero lleva `currency` (`PEN` | `USD`)** y, si es USD, `exchangeRate` (string con 3 decimales, como `tipo_cambio` de Khipu). Ventas, pagos, proformas, notas, OC, compras, CxC/CxP y sus pagos, movimientos y arqueos de caja, y consignaciones.
- Relaciones por id + nombre de visualización: los mocks traen solo nombres (`supplier: string`, `warehouse: string`); la API devuelve `supplierId` y `supplierName`.
- Textos ya formateados en los mocks (`lastAccess: "Hoy 10:42"`, `valueLabel`, `sitesNote`, `detail`) no existen en la API: llegan los datos crudos y el frontend formatea.

## identidad (`iam`)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `POST /v1/auth/login` | `/login` | — | público (clase `Public`), cuotas de §7 y bloqueo de la cuenta ([ADR 0004](../adr/0004-in-house-auth.md), nota del 2026-11-19). Responde `200` con `status` `authenticated`, `mfa_required` o `mfa_enrollment_required` (#24, #25) |
| `POST /v1/auth/mfa/verify` | `/login` paso 2FA | — | `challengeToken` en el cuerpo (clase `Public`, `conventions.md` §2) |
| `POST /v1/auth/mfa/totp/enroll`, `.../confirm` | `/perfil` (activar 2FA con la sesión ya iniciada) | — | sesión (clase `NoTenant`); sin `emailCode` (`conventions.md` §2) |
| `POST /v1/auth/mfa/enrollment/totp`, `.../confirm` | `/login`, enrolamiento obligatorio antes de tener sesión | — | `enrollmentToken` en el cuerpo, solo para enrolar y válido hasta el `confirm` correcto (clase `Public`, `conventions.md` §2); `confirm` crea la sesión y exige `{enrollmentToken, code, emailCode}`: el `emailCode` es el código de 6 dígitos que el login manda al correo al responder `mfa_enrollment_required` (decisión del dueño del 2026-11-19, [#25](https://github.com/bacsystem/wanka/issues/25); forma, intentos y topes en `conventions.md` §2) |
| `POST /v1/auth/mfa/enrollment/email-code/resend` | `/login`, «Reenviar código» del enrolamiento obligatorio | — | `enrollmentToken` en el cuerpo (clase `Public`); 3 envíos por token, 60 s entre envíos, y por usuario 10 por hora y 20 en 24 h (`conventions.md` §2) |
| `POST /v1/auth/refresh` | BFF | — | refresh token en el cuerpo (clase `Public`; historia propia, desglose de #24) |
| `POST /v1/auth/logout` | topbar | — | sesión (clase `NoTenant`); revoca la sesión actual, `204` |
| `POST /v1/auth/password/forgot`, `.../reset` | `/login` recuperar | — | público, rate limit (#26). `forgot` responde `202` siempre; el token (`wk_pr_`) vive 15 min ([ADR 0004](../adr/0004-in-house-auth.md)) |
| `POST /v1/auth/password/change` | `/perfil`, cambiar contraseña | — | sesión (clase `NoTenant`), con la contraseña actual; revoca las demás sesiones del usuario (#30) |
| `POST /v1/auth/email-change`, `POST /v1/auth/email-change/confirm` | `/perfil`, cambiar correo | — | `email-change`: sesión (`NoTenant`), con la contraseña actual, `202` siempre; `confirm`: token `wk_ec_` del enlace en el cuerpo (`Public`), 24 h (#30) |
| `POST /v1/auth/invitations/lookup`, `POST /v1/auth/invitations/accept` | pantalla de aceptación de la invitación | — | token `wk_iv_` del enlace en el cuerpo (clase `Public`); 7 días (#28) |
| `GET /v1/me/memberships` | `/seleccionar-empresa` | `auth/mocks/tenants.ts` `TenantOption`, `tenantSites` | sesión sin tenant |
| `POST /v1/auth/session/tenant` | `/seleccionar-empresa`, `TenantSwitcher` | — | membresía activa |
| `GET /v1/me`, `GET /v1/me/entitlements` | shell (sidebar por rubro y módulos) | `Tenant`, `config/navigation.ts` | sesión |
| `PATCH /v1/me` | `/perfil` (nombre y teléfono) | — | sesión (clase `NoTenant`, `conventions.md` §2) (#30) |
| `GET /v1/users`, `PATCH /v1/users/{id}`, `POST /v1/users/invitations`, `POST /v1/users/invitations/{id}/resend`, `DELETE /v1/users/invitations/{id}` | `/configuracion/usuarios`, invite drawer. `{id}` es el id de la **membresía** en la empresa, no el del usuario global. No hay `POST /v1/users`: la única alta es la invitación (corrección del 2026-11-19, #28; compatible, sin consumidores) | `Collaborator`, `securitySummary` | `identidad.users:read` (listar), `:create` (invitar y reenviar), `:update` (cambiar rol, sedes o estado, y anular una invitación). **Nadie otorga lo que no tiene:** asignar un rol exige tener todos sus permisos; el rol `dueno` solo lo asigna o lo quita un dueño, y una membresía de dueño solo la cambia, suspende o borra otro dueño; nadie cambia su propia membresía; siempre queda un dueño activo (`409 conflict`). Rechazo: `403 forbidden` con el `permission` que falta, auditado ([ADR 0004](../adr/0004-in-house-auth.md), nota del 2026-11-19) |
| `GET/POST /v1/roles`, `PATCH /v1/roles/{id}` (`If-Match`), `DELETE /v1/roles/{id}`, `GET /v1/permissions` | matriz de permisos. Los roles base (`system`) no se editan ni se borran; sus permisos salen de su plantilla (#29) | `permissionMatrix`, `permissionModules` | `identidad.roles:read` (listar roles y el catálogo), `:create`, `:update` (editar y borrar un rol propio sin miembros). Un rol propio solo puede tener permisos que su autor tiene (salvo el dueño, que los tiene todos); `403 forbidden` si no |
| `GET /v1/sessions`, `DELETE /v1/sessions/{id}` | sesiones activas | `securitySummary.sessions` | `identidad.sessions:*` |
| `GET /v1/support-access/requests`, `POST /{id}/approve`, `/reject`, `/revoke` | consentimiento de acceso de soporte (#90). La lectura va por `iam.list_support_requests()` y la decisión por `iam.decide_impersonation()`, ambas `SECURITY DEFINER` acotadas a `app.current_tenant()` (ADR 0001) | `IMPERSONATION_GRANTS` | `identidad.support_access:read`, `:approve`. **Nunca** desde una sesión impersonada (403) |

## empresas (`org`)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `GET/PATCH /v1/company` | `/configuracion/sunat` datos de empresa. `ruc` y `legalName` son de solo lectura: `PATCH` no los acepta, y la corrección es un caso de soporte (ADR 0013, §2). El domicilio fiscal y el nombre comercial se sincronizan con Khipu en segundo plano; la respuesta trae `providerSync` (`pending` \| `synced` \| `not_activated`) | `sunatConfig.company` | `empresas.config:*` |
| `PUT /v1/company/logo` | logo del PDF de Khipu: PNG o JPEG reconocido por sus bytes, hasta 204 800 bytes. Wanka no lo guarda: lo reenvía al momento a Khipu. Antes de la activación, `409 invalid_state_transition`; si Khipu falla, `503 dependency_unavailable` con `Retry-After` (ADR 0013, §2) | — | `empresas.config:update` |
| `GET/POST/PATCH /v1/sites` | almacenes y sedes, onboarding. `sunatAnnex` es opcional (4 dígitos, único en la empresa) y no cambia si el local tiene una serie sincronizada con Khipu (`409 invalid_state_transition`). La baja (`PATCH` con `active: false`) es solo de Wanka e inactiva sus cajas y series; con un turno abierto, `409 invalid_state_transition` (ADR 0013, §3) | `tenantSites`, `sunatEstablishments` | `empresas.sites:*` |
| `GET/POST/PATCH /v1/cash-registers` | selección de caja, config | `tenantSites[].cashboxes` | `empresas.sites:update` |
| `GET/POST/PATCH /v1/document-series` | `/configuracion/sunat` series. Sin `DELETE`: una serie se desactiva. Las de CPE se registran solas en Khipu. Reactivar una serie que Khipu tiene con un número mayor o en otro anexo la congela en el acto y responde `409 invalid_state_transition`. La respuesta trae `active` y `providerStatus` (`not_applicable` en las internas \| `pending` \| `synced` \| `conflict`), y `lastNumber` solo se edita mientras está `pending`. Con `providerStatus = synced`, `siteId` ya no cambia; `cashRegisterId` sí, dentro del mismo local (ADR 0013, §3 y §4) | `sunatConfig.series` | `empresas.series:*` |
| `GET/PUT /v1/onboarding` (pasos 1..6) | `/onboarding` | `industryOptions`, `modulesByIndustry` | propietario |
| `GET/PUT /v1/einvoicing/credentials` (permiso `empresas.einvoicing:update`) | conexión con Khipu: estado de la activación (#31) y carga del certificado y las credenciales SOL y API SUNAT, que Wanka reenvía a Khipu con la API key de la empresa. Las series ya no se cargan aquí: se crean en `/v1/document-series` y Wanka las registra en Khipu (corrección del 2026-11-19, ADR 0013). La API key y el `whsec_` **no** se cargan aquí: los carga el operador de Khipu con el comando `wanka einvoicing activate` (ADR 0003, «Alta asistida»; temporal hasta #86) | `sunatConfig.connection`, solo metadatos (entorno BETA/PRODUCCION, plan, activada, lista para emitir; la llave nunca se pide ni se devuelve). El certificado y la clave SOL se guardan **en Khipu**, no en Wanka | `empresas.config:update` |

## clientes (`crm`)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `GET /v1/customers?q=&segment=&status=&cursor=` | `/clientes`, buscador POS, ⌘K | `Customer` | `clientes.customers:read` |
| `GET /v1/customers/summary` | KPIs de `/clientes` | — | idem |
| `POST /v1/customers`, `PATCH /v1/customers/{id}` | alta rápida POS, ficha | `Customer` | `clientes.customers:create/update` |
| `GET /v1/customers/{id}` | `/clientes/[id]` | `CustomerDetail` (kpis, billing, alerts) | read |
| `GET /v1/customers/{id}/ledger`, `/documents`, `/files` | pestañas de la ficha | `CustomerDetail.ledger/documents/files` | read |
| `GET /v1/lookups/ruc/{ruc}`, `/v1/lookups/dni/{dni}` | autocompletar | — | create. RUC vía Khipu `GET /v1/consultas/ruc/{ruc}` con la llave del tenant (60/min y 5 000/día por empresa) y caché de 24 h; timeout 3 s; si falla se ingresa a mano. DNI: misma ruta, timeout y respaldo manual («sin validar»); **la fuente está pendiente de decisión del dueño** (#34, [`conventions.md` §8](conventions.md#8-decisiones-del-dueño)) |

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
| `POST /v1/inventory/counts`, `PUT .../lines`, `POST .../submit`, `POST .../approve` **(I)** | `/inventario/toma-fisica` | `CountLine` | `inventario.counts:create`, `:update`, `:approve` |
| `GET/POST /v1/inventory/consignments` **(I)**, `GET /{id}` | consignación con distribuidoras (issue #112; flag `inventario.consignacion`, librería primero) | `er-02` `CONSIGNMENTS` | `inventario.consignments:read`, `:create` |
| `POST /v1/inventory/consignments/{id}/settlements` **(I)** | liquidación de lo vendido (crea la CxP vía fachada de compras/finanzas) | `CONSIGNMENT_SETTLEMENTS` | `inventario.consignments:update` |
| `GET/POST /v1/inventory/supplier-returns` **(I)** | devoluciones al proveedor con plazo | `SUPPLIER_RETURNS` | `inventario.supplier_returns:read`, `:create` |

## ventas (`sales`)

| Método y ruta | Pantalla / uso | Forma (mock) | Permiso |
|---|---|---|---|
| `POST /v1/sales` **(I)** | `/ventas/nueva` (POS); exige `shiftId`, `currency` | `CartLine`, `PaymentMethod`; responde venta + CPE `pendiente` | `ventas.pos:create`; con descuento `ventas.discounts:create`, y sobre el tope del rol `ventas.discounts:approve` de un supervisor |
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
| `GET /v1/einvoicing/documents/{id}/files/{xml\|cdr\|pdf}` | descargas | `200` con el archivo traído de Khipu por streaming, sin guardarlo (ADR 0003); `503 dependency_unavailable` con `Retry-After` si Khipu falla | read |
| `POST /v1/einvoicing/documents/{id}/credit-notes` **(I)** | `/nota-credito` (catálogo 09) | `SalesDocumentDetail` | `facturacion.credit_notes:create` |
| `POST /v1/einvoicing/documents/{id}/debit-notes` **(I)** | `/nota-debito` (catálogo 10, serie FD01) | idem | `facturacion.debit_notes:create` |
| `POST /v1/einvoicing/documents/{id}/void` **(I)** | diálogo comunicación de baja | — | `facturacion.cpe:void` |
| `POST /v1/einvoicing/documents/{id}/retry` | cola SUNAT, reintento manual | — | `facturacion.cpe:retry` |
| `POST /v1/einvoicing/documents/{id}/send` | enviar al cliente por correo o WhatsApp (issue #81) | — | `facturacion.cpe:send` |
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
| `GET /v1/exports?type=&period=` | exportar CSV (decisión del dueño) y el TXT del PLE que exige SUNAT (sin PDF ni XLSX, ADR 0003) | `200` con el archivo generado al momento por streaming, sin copia guardada. Lectura repetible, sin `Idempotency-Key`. Máximo un mes calendario y 50 000 filas (`422 validation_failed` si se excede). 25 s desde la llegada (ADR 0003) | `*:export` del módulo |

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
| `GET /v1/lubricentro/reminders?status=`, `POST /{id}/call-result` | recordatorios (+5 000 km fijo, aviso 7 días antes) y lista de llamada | `er-06` `MAINTENANCE_REMINDERS` | idem |
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
| `GET /platform/v1/dashboard` | `/admin` | `platformSummary`, `mrrSeries` (agregados) | `plataforma.dashboard:read` |
| `GET /platform/v1/tenants`, `GET /{id}`, `POST` **(I)**, `PATCH /{id}` | `/admin/empresas`, `/[id]` | `PlatformTenant` (los precios de plan se leen de `plans`, no se duplican en el tenant) | `plataforma.tenants:read`, `:create`, `:update` |
| `POST /platform/v1/tenants/{id}/suspend`, `/reactivate` **(I)** | acciones de tenant | — | `plataforma.tenants:suspend` |
| `POST /platform/v1/tenants/{id}/impersonations` **(I)** | soporte: crea la solicitud en estado `solicitado`; solo se canjea si la empresa la aprueba (#90) | — | `plataforma.tenants:impersonate`, requiere ticket |
| `GET/POST/PATCH /platform/v1/plans`, `/subscriptions` | `/admin/planes` | `Plan` | `plataforma.plans:*`, `plataforma.subscriptions:*` |
| `GET /platform/v1/saas-invoices` | `/admin/facturacion` | `SaasInvoice` | `plataforma.billing:read` |
| `GET/PATCH /platform/v1/modules`, `/feature-flags`, `PUT /tenants/{id}/overrides` | `/admin/modulos` | `FeatureFlag` | `plataforma.flags:read`, `:update` |
| `GET /platform/v1/monitoring/providers`, `/incidents`, `/certificates` | `/admin/monitoreo` | `providers`, `latency24h`, `expiringCerts`, `rejectTop` | `plataforma.monitoring:read` |
| `GET/POST/PATCH /platform/v1/tickets` | `/admin/soporte` | `Ticket`, `supportSummary` | `plataforma.tickets:read`, `:update` |
| `GET/POST/PATCH /platform/v1/staff` | `/admin/usuarios` | `StaffUser`, `roleMatrix` | `plataforma.staff:read`, `:create`, `:update` |
| `GET /platform/v1/audit` | `/admin/auditoria` | `AuditEvent` (`ops.ts`) | `plataforma.audit:read`, `:export` |

Nota: el mock de monitoreo muestra varios OSE (`providers`, `OseProvider`). Con Khipu como proveedor único, `monitoring/providers` reporta la salud del adaptador Khipu (latencia, cola, rechazos); la lista multi-OSE del diseño queda como visión futura si se agrega un segundo `InvoicingProvider`.

## Catálogo de permisos (fuente de verdad)

Formato `modulo.recurso:accion`. Este catálogo es la **fuente de verdad**: se carga en `iam.permissions` por migración, y cada ruta lo declara en el OpenAPI con `x-permission` (ADR 0004 y 0009). Acciones: `read`, `read_all` (ver lo de otros usuarios), `create`, `update`, `void`, `export`, `import`, `approve`, `send`, `retry`.

| Módulo | Permisos |
|---|---|
| identidad | `identidad.users:read/create/update`, `identidad.roles:read/create/update`, `identidad.sessions:read/void`, `identidad.support_access:read/approve` (consentimiento de impersonación, #90) |
| empresas | `empresas.config:read/update`, `empresas.sites:read/create/update`, `empresas.series:read/create/update`, `empresas.einvoicing:read/update` |
| clientes | `clientes.customers:read/create/update/export/import` |
| catalogo | `catalogo.items:read/create/update/import/export`, `catalogo.categories:read/create/update`, `catalogo.prices:read/update` |
| inventario | `inventario.warehouses:read/create/update`, `inventario.stock:read/update/export`, `inventario.transfers:read/create/update`, `inventario.counts:read/create/update/approve`, `inventario.consignments:read/create/update`, `inventario.supplier_returns:read/create/approve` |
| ventas | `ventas.pos:read/create`, `ventas.discounts:create/approve`, `ventas.quotes:read/create/update/send`, `ventas.returns:create/approve` |
| facturacion | `facturacion.cpe:read/create/void/retry/send/export`, `facturacion.credit_notes:create`, `facturacion.debit_notes:create`, `facturacion.gre:read/create/update` |
| compras | `compras.suppliers:read/create/update`, `compras.orders:read/create/update`, `compras.documents:read/create/update/export` |
| finanzas | `finanzas.cash:read/read_all/create/update/approve/export`, `finanzas.receivables:read/create/update`, `finanzas.payables:read/create/update` |
| reportes | `reportes.dashboard:read`, `reportes.management:read/export`, `reportes.tax:read/export` |
| agenda | `agenda.appointments:read/create/update` |
| integraciones | `integraciones.config:read/update` |
| auditoria | `auditoria.events:read/export` |
| notificaciones | `notificaciones.reminders:read/update` |
| lubricentro | `lubricentro.vehicles:read/create/update`, `lubricentro.service_orders:read/create/update/void` |
| libreria | `libreria.books:read/create/update/import`, `libreria.school_lists:read/create/update` |
| odontologia, veterinaria | `odontologia.records:read/update`, `veterinaria.records:read/update`, `veterinaria.hospital:read/update`, `veterinaria.grooming:read/update` |
| restaurante, retail | `restaurante.tables:read/update`, `restaurante.orders:read/create/update`, `restaurante.reservations:read/create/update`, `retail.variants:read/update` |

## Correspondencia: código de historia → catálogo

Los issues usan 37 códigos abreviados (relevados con `gh issue list --limit 200 --json number,body`). El PO los actualiza con esta tabla. Cuando un código de historia agrupa varios permisos, el rol plantilla recibe todos.

| Código en la historia | Issues | Permisos del catálogo |
|---|---|---|
| `agenda.editar` | #75 | `agenda.appointments:create`, `:update` |
| `auditoria.ver` | #79, #80 | `auditoria.events:read` |
| `caja.operar` | #66, #67 | `finanzas.cash:create` (abrir), `finanzas.cash:update` (arqueo y cierre) |
| `caja.ver_todos` | #68 | `finanzas.cash:read_all` |
| `catalogo.editar` | #37, #38, #39, #104 | `catalogo.items:create`, `:update`, `catalogo.categories:create`, `:update`, `catalogo.prices:update` (y `libreria.books:update` en #104) |
| `catalogo.importar` | #106 | `catalogo.items:import`, `libreria.books:import` |
| `clientes.editar` | #34 | `clientes.customers:create`, `:update` |
| `clientes.importar` | #36 | `clientes.customers:import` |
| `clinica.editar` | #117, #119 | `odontologia.records:update` o `veterinaria.records:update` según el rubro |
| `compras.editar` | #62, #63, #64 | `compras.suppliers:create/update`, `compras.orders:create/update`, `compras.documents:create/update` |
| `comprobantes.enviar` | #81 | `facturacion.cpe:send` |
| `cpe.baja` | #60 | `facturacion.cpe:void` |
| `cpe.emitir` | #56 | `facturacion.cpe:create` |
| `cpe.nota_credito` | #58 | `facturacion.credit_notes:create` |
| `cpe.nota_debito` | #59 | `facturacion.debit_notes:create` |
| `cpe.reintentar` | #57 | `facturacion.cpe:retry` |
| `cxc.gestionar` | #69 | `finanzas.receivables:read`, `:create`, `:update` |
| `cxp.gestionar` | #70 | `finanzas.payables:read`, `:create`, `:update` |
| `empresa.editar` | #32, #33 | `empresas.config:update`, `empresas.sites:create`, `:update` |
| `gre.emitir` | #61 | `facturacion.gre:create` |
| `integraciones.gestionar` | #77, #78 | `integraciones.config:read`, `:update` |
| `inventario.aprobar` | #46 | `inventario.counts:approve` |
| `inventario.editar` | #41 | `inventario.warehouses:create`, `:update` |
| `inventario.mover` | #43, #44, #65 | `inventario.stock:update`, `inventario.transfers:create`, `:update` |
| `notificaciones.gestionar` | #82 | `notificaciones.reminders:update` |
| `ordenes.crear` | #96 | `lubricentro.service_orders:create` |
| `ordenes.editar` | #97, #98 | `lubricentro.service_orders:update` |
| `proformas.crear` | #53 | `ventas.quotes:create`, `:update`, `:send` |
| `reportes.dashboard` | #71 | `reportes.dashboard:read` |
| `reportes.tributarios` | #73, #74 | `reportes.tax:read`, `:export` |
| `reportes.ver` | #72 | `reportes.management:read` |
| `roles.gestionar` | #29 | `identidad.roles:create`, `:update` |
| `sunat.configurar` | #55 | `empresas.einvoicing:update`, `empresas.series:create`, `:update` |
| `usuarios.gestionar` | #28 | `identidad.users:create`, `:update` |
| `vehiculos.editar` | #94 | `lubricentro.vehicles:create`, `:update` |
| `ventas.crear` | #48, #49 | `ventas.pos:create` |
| `ventas.descuento` | #51 | `ventas.discounts:create`; sobre el tope del rol, `ventas.discounts:approve` del supervisor que autoriza |

La consignación (#112) no tenía código propio: usa `inventario.consignments:*`.

**Altas del congelamiento (2026-11-18).** Ocho permisos nuevos en cuatro recursos, para historias abiertas que piden un permiso que el catálogo no tenía: `ventas.returns:create` y `:approve` (devolución o cambio de un cliente; `approve` autoriza la excepción sin ticket o fuera de plazo, #110 y #122), `inventario.supplier_returns:approve` (devolver al proveedor fuera del plazo pactado, #111), `veterinaria.grooming:read/update` (grooming separado de la historia clínica, para dárselo a un peluquero sin acceso clínico, #118) y `restaurante.reservations:read/create/update` (reservas de mesa, #115). Sus rutas se publican en el issue de cada historia (ADR 0009).

### Historias sin código abreviado → catálogo

Historias de negocio abiertas que no citan un código. Con esta tabla y la anterior, ninguna historia abierta queda sin permiso. «Sin permiso de catálogo» significa que la ruta la protege otra cosa (público con rate limit, la sesión del propio usuario o la membresía), como en las tablas de recursos.

| Issue | Historia | Permisos del catálogo |
|---|---|---|
| #24 | Iniciar sesión | sin permiso de catálogo: público, con rate limit |
| #25 | Verificación en dos pasos | sin permiso de catálogo: la sesión del propio usuario |
| #26 | Recuperar contraseña | sin permiso de catálogo: público, con rate limit |
| #27 | Seleccionar la empresa activa | sin permiso de catálogo: membresía activa en el tenant |
| #30 | Editar mi perfil y contraseña | sin permiso de catálogo: la sesión del propio usuario |
| #31 | Registrar mi empresa | el alta es pública con rate limit y crea el rol dueño; los pasos del onboarding exigen `empresas.config:update`, `empresas.sites:create`, `empresas.series:create` |
| #35 | Ficha del cliente | `clientes.customers:read`; editar contacto, `clientes.customers:update` |
| #40 | Variantes de producto | `catalogo.items:read`, `:update` |
| #42 | Stock y alertas de mínimo | `inventario.stock:read` |
| #45 | Kardex valorizado | `inventario.stock:read`, `:export` |
| #47 | Numeración sin huecos | sin permiso propio: la asigna la venta o el CPE dentro de su transacción (ADR 0008); configurar series, `empresas.series:create`, `:update` |
| #50 | Venta en espera | `ventas.pos:create` |
| #52 | Consultar, reimprimir y reenviar comprobantes | `facturacion.cpe:read`, `ventas.pos:read` (reimpresión), `facturacion.cpe:send` (reenvío) |
| #54 | Convertir proforma en venta | `ventas.quotes:update`, `ventas.pos:create` |
| #76 | Cobrar una cita en el POS | `ventas.pos:create`, `agenda.appointments:update` (marca la cita cobrada) |
| #83 | Notificaciones en la app | sin permiso de catálogo: cada usuario ve las suyas (sesión) |
| #84 | Consola con 2FA obligatorio | plano plataforma, sin permiso de catálogo: login público con 2FA |
| #95 | Buscar vehículo por placa | `lubricentro.vehicles:read` |
| #99 | Cerrar la orden y facturar | `lubricentro.service_orders:update`, `ventas.pos:create` |
| #100 | Próximo cambio por km y fecha | `lubricentro.vehicles:update` (lectura de km), `lubricentro.service_orders:update` |
| #101 | Recordatorios de próximo cambio | `notificaciones.reminders:read`, `:update` |
| #102 | Historial del vehículo | `lubricentro.vehicles:read`, `lubricentro.service_orders:read` |
| #103 | Paquetes de servicio | definirlos, `catalogo.items:create`, `:update`; venderlos, `ventas.pos:create` |
| #105 | Buscar libros | `libreria.books:read` |
| #107 | Listas escolares | `libreria.school_lists:read`, `:create`, `:update` |
| #108 | Cotizar una lista escolar | `libreria.school_lists:read`, `ventas.quotes:create` |
| #109 | Reservar stock de una lista cotizada | `ventas.quotes:create` (la reserva nace con la proforma), `ventas.quotes:update` (liberarla o extenderla) |
| #110 | Devolver un libro del cliente | `ventas.returns:create`; fuera de plazo, `ventas.returns:approve` |
| #111 | Devolver libros al proveedor | `inventario.supplier_returns:create`; fuera de plazo, `inventario.supplier_returns:approve` |
| #113 | Mesas y cuentas del salón | `restaurante.tables:read`, `:update`, `restaurante.orders:create`, `:update` |
| #114 | Comandas a cocina (KDS) | `restaurante.orders:read`, `:update` |
| #115 | Delivery y reservas | `restaurante.orders:create`, `:update` (delivery), `restaurante.reservations:read`, `:create`, `:update` |
| #116 | Mascotas y carné de vacunas | `veterinaria.records:read`, `:update` |
| #118 | Hospitalización y grooming | `veterinaria.hospital:read`, `:update`, `veterinaria.grooming:read`, `:update` |
| #120 | Presupuesto de tratamiento | `odontologia.records:update`; convertirlo en proforma, `ventas.quotes:create` |
| #121 | Matriz talla × color | `retail.variants:read`, `:update` |
| #122 | Cambios y devoluciones | `ventas.returns:create`; sin ticket, `ventas.returns:approve` |

Las historias del kernel (épica #154), las de frontend y las épicas no exponen rutas de negocio propias: aplican los permisos de las rutas que tocan.

## Plantillas de los roles base (identidad, #29)

Estado: **decidido por el dueño (2026-11-19)**, sobre la propuesta del architect y las recomendaciones del coordinador; lo implementa #233 (resolver de permisos y roles base sembrados). Los seis roles base existen en toda empresa desde su alta (#31), con `system = true`: no se borran, no se renombran y sus permisos no se editan, porque salen de su plantilla (`iam.role_templates`, [data-model §6](../data/data-model.md)). Quien necesita otra combinación crea un rol propio. `requires_mfa` se puede subir a `true` en cualquier rol, pero nunca bajar en los que la plantilla marca. Un permiso nuevo del catálogo entra en `dueno` siempre (un test lo exige) y en las demás plantillas solo si el PR que lo agrega lo dice. La migración que siembra las plantillas tiene un test de paridad contra esta tabla.

| Código | Nombre | 2FA | Permisos |
|---|---|---|---|
| `dueno` | Dueño | obligatoria | todo el catálogo, incluido el permiso de costos (alta pendiente, abajo) |
| `administrador` | Administrador | obligatoria | todo el catálogo salvo `identidad.support_access:approve` y `empresas.einvoicing:update` (credenciales SOL y certificado: solo el dueño); incluye el permiso de costos. Lo hace cumplir la regla «nadie otorga lo que no tiene» de la tabla de identidad: un administrador no puede asignarse `dueno` ni crear un rol propio con esos dos permisos |
| `contador` | Contador | obligatoria | lectura de ventas, compras, comprobantes y reportes: `ventas.pos:read`, `ventas.quotes:read`, `compras.suppliers:read`, `compras.orders:read`, `compras.documents:read`, `:export`, `facturacion.cpe:read`, `:export`, `facturacion.gre:read`, `reportes.dashboard:read`, `reportes.management:read`, `:export`, `reportes.tax:read`, `:export`; además `facturacion.credit_notes:create`, `finanzas.cash:read`, `:read_all`, `:export`, `finanzas.receivables:read`, `finanzas.payables:read`, `auditoria.events:read`, `:export`, `clientes.customers:read`, `inventario.stock:read`, `empresas.config:read` y el permiso de costos. Nada de `catalogo.*` |
| `cajero` | Cajero | opcional | cobra y aplica descuentos: `ventas.pos:read`, `:create`, `ventas.discounts:create`, `ventas.quotes:read`, `clientes.customers:read`, `:create`, `catalogo.items:read`, `catalogo.prices:read`, `inventario.stock:read`, `finanzas.cash:read`, `:create`, `:update`, `facturacion.cpe:read`, `:send`, `agenda.appointments:read`, `:update`, `restaurante.tables:read`, `:update`, `restaurante.orders:read`, `:create`, `:update`. Sin devoluciones (`ventas.returns:*`), notas de crédito ni anulaciones (`facturacion.cpe:void`): las hacen el administrador o el dueño |
| `vendedor` | Vendedor | opcional | `ventas.pos:read`, `:create`, `ventas.discounts:create`, `ventas.quotes:read`, `:create`, `:update`, `:send`, `clientes.customers:read`, `:create`, `:update`, `catalogo.items:read`, `catalogo.prices:read`, `inventario.stock:read`, `facturacion.cpe:read`, `:send`, y la caja para cobrar: `finanzas.cash:read`, `:create` (abrir), `:update` (arqueo y cierre) |
| `almacenero` | Almacenero | opcional | `inventario.warehouses:read`, `inventario.stock:read`, `:update` (también recibe las compras: `POST /v1/purchase-orders/{id}/receipts`), `inventario.transfers:read`, `:create`, `:update`, `inventario.counts:read`, `:create`, `:update`, `inventario.consignments:read`, `inventario.supplier_returns:read`, `:create`, `catalogo.items:read`, `compras.orders:read`, `compras.documents:read`, `facturacion.gre:read`, `:create` |

- **2FA** (decisión del dueño): obligatoria para dueño, administrador y contador; opcional para cajero, vendedor y almacenero.
- **Tope de descuento:** el cajero y el vendedor aplican descuentos con `ventas.discounts:create`; el tope será un ajuste por empresa que fija una historia de ventas (hoy es «el tope del rol», #51). Sobre el tope, `ventas.discounts:approve` de un supervisor.
- **Permiso de costos (alta pendiente).** El dueño decidió un permiso para ver costos, catalogo.costs con la acción read, para dueño, administrador y contador. Todavía **no** está en el catálogo de abajo: el test de paridad exige que el catálogo de este documento, la migración y el código Go coincidan, así que el alta va completa en el PR de #233 (fila del catálogo, migración hacia adelante en `iam.permissions`, constante Go y esta tabla con el código entre comillas invertidas). Hasta entonces ninguna ruta lo declara.
- Los permisos de rubro (`lubricentro.*`, `libreria.*`, `veterinaria.*`, `odontologia.*`, `restaurante.reservations:*`, `retail.*`) van solo en `dueno` y `administrador` hasta que el po defina las plantillas por rubro (ADR 0004 las prevé); el módulo apagado los vuelve inertes igual (ADR 0005).

## Catálogo de permisos de plataforma (plano super-admin)

Es un catálogo **separado** del de tenant. Se guarda en `platform.platform_permissions` y se asigna a los roles de staff en `platform.platform_role_permissions`. Lo exige `platform-api` y nunca es válido en el plano tenant, igual que sus tokens (ADR 0004). Las acciones son las mismas del catálogo de tenant, más `suspend` e `impersonate`. La matriz de roles del mock (`platform-admin/mocks/ops.ts`, `roleMatrix`) se traduce a estos códigos.

| Recurso | Permisos | Roles de staff con acceso (según `roleMatrix`) |
|---|---|---|
| dashboard | `plataforma.dashboard:read` | todos |
| tenants | `plataforma.tenants:read`, `:create`, `:update`, `:suspend`, `:impersonate` | read: todos; create/update: SuperAdmin, Finanzas, Ventas; suspend: SuperAdmin, Finanzas; impersonate: SuperAdmin, Soporte |
| plans | `plataforma.plans:read`, `:create`, `:update` | SuperAdmin, Finanzas, Ventas |
| subscriptions | `plataforma.subscriptions:read`, `:update` | SuperAdmin, Finanzas |
| billing (facturación del SaaS) | `plataforma.billing:read`, `:create`, `:export` | SuperAdmin, Finanzas |
| monitoring (OSE, SUNAT, Khipu, certificados) | `plataforma.monitoring:read` | SuperAdmin, Soporte, Ingeniería |
| tickets | `plataforma.tickets:read`, `:create`, `:update` | SuperAdmin, Soporte |
| staff | `plataforma.staff:read`, `:create`, `:update` | SuperAdmin |
| flags (módulos y feature flags) | `plataforma.flags:read`, `:update` | SuperAdmin, Ingeniería |
| audit | `plataforma.audit:read`, `:export` | SuperAdmin, Soporte, Finanzas, Ingeniería |
| config (configuración núcleo) | `plataforma.config:read`, `:update` | SuperAdmin, Ingeniería |

### Correspondencia: historias de plataforma → catálogo

Las historias #85 a #93 ya citan estos códigos concretos (el PO las migró desde el comodín `plataforma.*`). La tabla es la referencia para mantenerlas alineadas.

| Issue | Historia | Permisos del catálogo |
|---|---|---|
| #85 | Listar y ver fichas de empresas | `plataforma.tenants:read` |
| #86 | Crear, suspender y reactivar empresas | `plataforma.tenants:create`, `plataforma.tenants:suspend` (suspender y reactivar) |
| #87 | Gestionar planes y límites | `plataforma.plans:read`, `:create`, `:update` |
| #88 | Gestionar suscripciones y cobros del SaaS | `plataforma.subscriptions:read`, `:update`, `plataforma.billing:read`, `:create` |
| #89 | Monitorear la salud de OSE/SUNAT/Khipu | `plataforma.monitoring:read` |
| #90 | Atender tickets de soporte | `plataforma.tickets:read`, `:update`; para entrar a los datos de una empresa, además `plataforma.tenants:impersonate`, y la empresa aprueba con `identidad.support_access:approve` (plano tenant) |
| #91 | Gestionar usuarios de plataforma | `plataforma.staff:read`, `:create`, `:update` |
| #92 | Administrar feature flags y módulos | `plataforma.flags:read`, `:update` |
| #93 | Auditar acciones de la plataforma | `plataforma.audit:read`, `:export` |
