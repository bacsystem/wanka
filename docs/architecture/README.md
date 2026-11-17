# Arquitectura del backend de Wanka

Estado: propuesto (2026-11-17). Este documento solo describe diseño; todavía no existe código de backend. Hoy el repositorio tiene únicamente `apps/web`: Next.js con datos mock en `apps/web/src/features/*/mocks`, que reflejan la forma que tendrá la API.

Decisiones fijadas por el usuario, que este diseño no reabre:
- Go + PostgreSQL.
- Base única con `tenant_id` y Row-Level Security.
- Facturación electrónica vía **Khipu**.
- Railway con Docker al inicio y luego una nube mayor.
- Autenticación propia.
- Todos los módulos del producto, con prioridad para los rubros **lubricentro** y **librería**.

> **Khipu y khipu-design-system.** *Khipu* es la **API del proveedor de facturación electrónica** (ADR 0003). *khipu-design-system* es el **sistema de diseño** que adopta el frontend de Wanka (PR #2, fijado en `github:bacsystem/khipu-design-system#v0.5.23`). Son la misma familia de productos: el portal web de Khipu (`bacsystem/khipu`, `portal/`) está construido sobre ese mismo paquete. En estos documentos, "Khipu" a secas es la API de facturación, y "khipu-design-system" es el paquete de UI.

Los diagramas son archivos Mermaid (`.mmd`). GitHub los renderiza al abrirlos y también se pueden previsualizar con la extensión de Mermaid. Todos se validaron con `@mermaid-js/mermaid-cli`; ver [Validación](#validación).

## 1. Visión en una página

- **Monolito modular hexagonal en Go** ([ADR 0002](adr/0002-modular-monolith-go.md)): un binario con cuatro roles: `serve --plane=tenant`, `serve --plane=platform`, `serve --plane=webhooks` y `worker`. Cada bounded context es un paquete con `domain/app/adapters` y una fachada pública. La venta POS es **una transacción local** que cubre ventas, inventario, numeración, caja y CPE pendiente más el job de emisión.
- **Aislamiento por RLS** ([ADR 0001](adr/0001-multitenancy-rls.md)): cada transacción fija `app.tenant_id` con `set_config(..., true)`, que es compatible con pooling. Los roles de base no tienen `BYPASSRLS` y las FK compuestas incluyen `tenant_id`. La consola super-admin lee agregados y accede a datos de un tenant solo mediante impersonación auditada.
- **Dos planos de autenticación** ([ADR 0004](adr/0004-in-house-auth.md)): tokens opacos con sesión en Postgres, refresh rotativo con detección de reuso, TOTP obligatorio para propietario, administrador, contador y staff, y RBAC `modulo.recurso:accion` por membresía (usuario × empresa × rol × sedes). Un token de un plano nunca sirve en el otro.
- **Facturación asíncrona** ([ADR 0003](adr/0003-einvoicing-khipu-port.md)): puerto `InvoicingProvider` y adaptador Khipu, contrastados con su documentación. Wanka numera y envía `correlativo`; Khipu firma, envía a SUNAT y reintenta ese tramo. Wanka reintenta solo el salto Wanka→Khipu, con la `Idempotency-Key` de Khipu. Hay webhooks firmados con HMAC y una conciliación periódica.
- **Outbox en Postgres con River** ([ADR 0006](adr/0006-async-jobs-outbox.md)): encolado en la misma transacción, colas con concurrencia propia, y tareas periódicas que corren una sola vez aunque haya varias réplicas.
- **Módulos de rubro enchufables** ([ADR 0005](adr/0005-industry-modules-feature-flags.md)): plan, overrides y feature flags por tenant, aplicados en cada petición.
- **Dinero `numeric` con escala explícita y correlativos sin huecos** ([ADR 0008](adr/0008-money-and-numbering.md)).
- **Contrato spec-first OpenAPI 3.1** ([ADR 0009](adr/0009-api-contract-openapi.md)).
- **Railway con Docker, 12-factor** ([ADR 0007](adr/0007-deployment-railway.md)).

## 2. Bounded contexts (alineados a la taxonomía del PO)

| Etiqueta del PO | Módulo Go | Esquema | Notas de frontera |
|---|---|---|---|
| identidad | `internal/identidad` | `iam` | auth, usuarios, roles, permisos, sesiones |
| empresas | `internal/empresas` | `org` | tenant, sedes, cajas, onboarding, configuración, **series**. La asignación de correlativos es un servicio de este módulo que usan facturación, ventas, compras, finanzas y lubricentro |
| clientes | `internal/clientes` | `crm` | |
| catalogo | `internal/catalogo` | `catalog` | **las variantes son core** (talla/color, EAN, listas de precio), no del rubro retail: las usan retail y librería |
| inventario | `internal/inventario` | `inv` | almacenes, stock, reservas, movimientos, kardex, toma física, traslados; **consignación con distribuidoras y devoluciones a proveedor** (flag `inventario.consignacion`, issue #112) |
| ventas | `internal/ventas` | `sales` | POS, proformas |
| facturacion-electronica | `internal/facturacion` | `einv` | CPE, NC/ND, bajas, GRE, **adaptador Khipu** |
| compras | `internal/compras` | `purch` | proveedores, OC, registro de compras SIRE; aporta proveedor y comprobante a la liquidación de consignación vía fachada |
| finanzas | `internal/finanzas` | `fin` | caja/turnos/Z, CxC, CxP |
| reportes | `internal/reportes` | `rpt` (lectura) | gestión, tributarios, PDT 621, exportes asíncronos |
| agenda | `internal/agenda` | `agenda` | recursos (sillón, consultorio, bahía), citas |
| integraciones | `internal/integraciones` | `integ` | conectores de terceros (pasarelas, WhatsApp, exportes, webhooks salientes) y bóveda de credenciales cifradas por tenant. El adaptador de Khipu vive en facturación |
| auditoria | `internal/auditoria` | `audit` | eventos de solo inserción |
| notificaciones | `internal/notificaciones` | `notify` | correo y WhatsApp, plantillas, campana |
| plataforma | `internal/plataforma` | `platform` | tenants, planes, suscripciones, facturación del SaaS, flags, monitoreo, soporte, impersonación |
| rubros | `internal/rubros/{lubricentro,libreria,restaurante,veterinaria,odontologia,retail}` | `lube`, `books`, `resto`, `vet`, `dental` | extienden entidades core por FK; el core no los conoce |

Fronteras ya alineadas con el PO: las variantes son core `catalogo`, y la consignación es core `inventario` (decisión del dueño, issue #112 con etiqueta `modulo:inventario`).

## 3. Diagramas C4

| Nivel | Archivo | Qué muestra |
|---|---|---|
| 1 Contexto | [`c4/01-context.mmd`](c4/01-context.mmd) | usuarios de empresa, staff de plataforma, cliente final; Khipu, SUNAT, mensajería, pasarelas y storage |
| 2 Contenedores | [`c4/02-containers.mmd`](c4/02-containers.mmd) | `web` (Next.js BFF), `api`, `platform-api`, `worker` (misma imagen Go), Postgres y object storage, con el rol de base de cada uno |
| 3 Componentes API | [`c4/03-components-api.mmd`](c4/03-components-api.mmd) | middleware en orden (auth → tenant → entitlements → RBAC → idempotencia), módulos core y de rubro, kernel compartido, puerto y adaptador de Khipu |
| 3 Componentes web | [`c4/04-components-web.mmd`](c4/04-components-web.mmd) | `proxy.ts`, Server Components y Server Actions, BFF con cookies httpOnly por plano y cliente generado desde OpenAPI |

**Por qué un monolito modular y no microservicios.** La operación central, cobrar en el POS, tiene que ser atómica a través de cinco módulos. Con servicios separados haría falta una saga para algo que en una base es un `COMMIT`. El equipo es chico y no hay ninguna necesidad medida de escalar un módulo por separado. Las fronteras se verifican en CI, así que extraer un módulo más adelante sigue siendo posible. Detalle y librerías en el [ADR 0002](adr/0002-modular-monolith-go.md).

**Frontend.** La UI viene del paquete `khipu-design-system`, fijado por tag: shell, navegación, componentes, tokens, apariencia y `Providers`, con iconos Phosphor (la migración de los restantes de Lucide es el issue #123). El patrón del BFF se alinea con el portal de Khipu: refresco en `proxy.ts` y en el route handler proxy con single-flight, y clientes `lib/api/*.ts` escritos a mano (ADR 0004 y ADR 0009). El navegador nunca llama a la API. Las páginas (Server Components) y las mutaciones (Server Actions o route handlers) llaman a la API desde el servidor con el token guardado en cookies httpOnly. Este flujo reemplaza la cookie `tenant_id` actual (`apps/web/src/features/auth/lib/tenant-cookie.ts`), que hoy elige la empresa sin autorización. En Next 16 el middleware se llama `proxy.ts`; antes de implementarlo hay que revisar la documentación local en `node_modules/next/dist/docs/` (lo indica `apps/web/AGENTS.md`).

## 4. Datos

Modelo y reglas en [`data/data-model.md`](data/data-model.md): `tenant_id` y RLS en todas las tablas de tenant, FK compuestas, columnas de auditoría, dinero y cantidades `numeric` con escala explícita, numeración concurrente sin huecos, stock con locks ordenados, limpieza de las tablas que crecen solas, migraciones con goose en esquema expandir → migrar → contraer, y extensiones de lubricentro y librería.

| ER | Archivo |
|---|---|
| identidad + empresas | [`data/er-01-identidad-empresas.mmd`](data/er-01-identidad-empresas.mmd) |
| clientes + catálogo + inventario (con consignación) | [`data/er-02-clientes-catalogo-inventario.mmd`](data/er-02-clientes-catalogo-inventario.mmd) |
| ventas + facturación electrónica | [`data/er-03-ventas-facturacion.mmd`](data/er-03-ventas-facturacion.mmd) |
| compras + finanzas (PEN y USD, arqueo por método y moneda) | [`data/er-04-compras-finanzas.mmd`](data/er-04-compras-finanzas.mmd) |
| plataforma + transversales | [`data/er-05-plataforma-transversal.mmd`](data/er-05-plataforma-transversal.mmd) |
| rubros (lubricentro, librería) | [`data/er-06-rubros.mmd`](data/er-06-rubros.mmd) |

## 5. Secuencias

| Flujo | Archivo | Puntos clave |
|---|---|---|
| Estados de un CPE | [`sequences/00-cpe-state-machine.mmd`](sequences/00-cpe-state-machine.mmd) | correspondencia de los estados de Khipu con los de Wanka; transiciones condicionadas |
| Login + 2FA + empresa | [`sequences/01-login-2fa-tenant.mmd`](sequences/01-login-2fa-tenant.mmd) | sesión sin tenant → TOTP → `session/tenant` emite tokens ligados a empresa y sede |
| Venta POS → stock → CPE | [`sequences/02-pos-sale-cpe.mmd`](sequences/02-pos-sale-cpe.mmd) | una transacción sin red; ticket "pendiente"; worker con idempotencia, `FindByNumber`, `422` con el mismo número; webhook HMAC |
| Nota de crédito y baja | [`sequences/03-credit-note-void.mmd`](sequences/03-credit-note-void.mmd) | NC con serie `FC`/`BC` según el origen; baja de hasta 7 días sin idempotencia en Khipu (se consulta antes de reintentar) |
| Traslado con GRE | [`sequences/04-transfer-gre.mmd`](sequences/04-transfer-gre.mmd) | stock en tránsito; Khipu numera la GRE; recepción parcial |
| Cierre de caja (arqueo, Z) | [`sequences/05-cash-close.mmd`](sequences/05-cash-close.mmd) | bloqueo de ventas en arqueo, aprobación del supervisor, Z inmutable con hash |
| Lubricentro: OT → venta | [`sequences/06-lubricentro-service-order.mmd`](sequences/06-lubricentro-service-order.mmd) | placa, km, diagnóstico de filtros, cobro al contado, recordatorio a +5 000 km o fecha por WhatsApp y correo |
| Alta de tenant desde la consola | [`sequences/07-tenant-provisioning.mmd`](sequences/07-tenant-provisioning.mmd) | función `SECURITY DEFINER` acotada; conexión con Khipu hoy manual |
| Plan, flags y permisos por petición | [`sequences/08-entitlement-enforcement.mmd`](sequences/08-entitlement-enforcement.mmd) | 401 → 402 → 403 módulo/límite/permiso, con caché invalidada por `NOTIFY` |

## 6. Despliegue

- [`deployment/railway.mmd`](deployment/railway.mmd): servicios, red privada, Postgres, bucket, pre-deploy de migraciones y nombres de variables de entorno. Railway expone el puerto completo de cada servicio, así que `api` no tiene dominio público y los webhooks entran por un servicio `webhooks` aparte que solo monta esa ruta. Las API keys de Khipu son por tenant y van **cifradas en la base**, no en variables.
- [`deployment/cloud-migration.md`](deployment/cloud-migration.md): health checks, apagado ordenado, backups, equivalencias en AWS/GCP/Azure y disparadores para migrar.

## 7. API

- [`api/conventions.md`](api/conventions.md): REST `/v1`, `camelCase`, tenant tomado de la sesión (sin cabecera), `Idempotency-Key`, errores RFC 9457 con `code`, paginación por cursor, dinero como string, rate limits y cadena de timeouts.
- [`api/resources.md`](api/resources.md): recursos por módulo derivados de cada pantalla y su mock, con permiso y módulo.
- [`api/openapi.yaml`](api/openapi.yaml): esqueleto OpenAPI 3.1 (`POST /v1/sales`, `GET /v1/einvoicing/documents`, esquemas comunes).

## 8. Capacidad y caminos calientes

[`capacity.md`](capacity.md): la carga por tenant está aceptada (50 usuarios concurrentes, 30 ventas por minuto, p95 < 800 ms); la de toda la plataforma es un supuesto (2 000 tenants, 30 ventas/s esperadas y 100 de diseño, p95 de la venta < 300 ms). Incluye el tamaño de pools según la ley de Little y un criterio de carga por camino caliente: venta POS, serie compartida, emisión CPE, búsqueda POS, login, listados y webhooks.

## 9. ADR

| # | Decisión | Estado |
|---|---|---|
| [0001](adr/0001-multitenancy-rls.md) | Multi-tenancy con base única, `tenant_id` y RLS; acceso cross-tenant de la consola | aceptado |
| [0002](adr/0002-modular-monolith-go.md) | Monolito modular hexagonal en Go; librerías base | propuesto |
| [0003](adr/0003-einvoicing-khipu-port.md) | Facturación vía Khipu tras `InvoicingProvider` (con hechos de la documentación) | propuesto |
| [0004](adr/0004-in-house-auth.md) | Autenticación y RBAC propios; plano super-admin separado | aceptado |
| [0005](adr/0005-industry-modules-feature-flags.md) | Módulos de rubro, planes y feature flags; empresa cerrada en solo lectura | propuesto |
| [0006](adr/0006-async-jobs-outbox.md) | Jobs y outbox en Postgres con River | propuesto |
| [0007](adr/0007-deployment-railway.md) | Railway con Docker, portable | aceptado |
| [0008](adr/0008-money-and-numbering.md) | Dinero `numeric` y correlativos sin huecos | propuesto |
| [0009](adr/0009-api-contract-openapi.md) | Contrato spec-first OpenAPI 3.1 | propuesto |
| [0010](adr/0010-frontend-khipu-migration.md) | Migración total del frontend al design system khipu | propuesto |

**Proceso de los ADR, desde que este PR se mergea:**
- Un ADR nuevo toma el siguiente número libre (`adr/NNNN-titulo-en-kebab.md`), con contexto, opciones, decisión, consecuencias (también las malas), estado y fecha. Se agrega a esta tabla en el mismo PR.
- Un ADR en estado *aceptado* no se reescribe. Mientras está *propuesto*, se corrige en su PR de revisión.
- Para revertirlo o reemplazarlo se escribe un ADR nuevo que lo cita, y el anterior pasa a *reemplazado por NNNN*. Lo único que se le edita al anterior es la línea de estado.
- Las correcciones que no cambian la decisión (erratas, enlaces, números de línea) se pueden hacer en el mismo ADR.

Las dependencias nuevas que proponen los ADR (River, sqlc, goose, oapi-codegen, shopspring/decimal, pquerna/otp, OpenTelemetry y el SDK de S3) **requieren aprobación del usuario** antes de agregarse.

## 10. Supuestos y preguntas abiertas

Verificaciones en BETA de Khipu, criterios previos al issue #56 (no son preguntas: se cierran con una prueba). **Ya figuran como criterios de aceptación en el #56**, así que el gate vive en el issue:
- **Redondeo.** Los totales de `pricing.Compute` deben ser idénticos a los de `POST /v1/facturas/calcular` en al menos 200 carritos (`data/data-model.md` §2, ADR 0003).
- **Orden y huecos por serie.** Comprobar si Khipu acepta el correlativo N+1 antes que N. De eso depende la política de hueco (ADR 0003 y 0006).

Khipu. La documentación de Khipu no responde estos puntos; detalle en el ADR 0003:
1. ¿Una cuenta de Khipu para todos los tenants, o una por tenant? Con una sola cuenta, la cuota mensual se comparte entre todos.
2. ¿Habrá una API de integrador para dar de alta empresa, API key y webhook? Hoy ese alta solo se hace desde el portal.
3. ¿Se puede usar la GRE en el release 1? Está marcada como experimental y no existe en BETA.
4. Timeouts y latencia típica de las llamadas; si se envía `Retry-After`; qué significa `PENDIENTE_AGRUPACION`; si los webhooks de GRE y de bajas usan los eventos `comprobante.*`. La cuota de BETA (`LIMITE_PRUEBAS`) también se comparte entre las empresas de una cuenta.

Producto (PO):
5. Esperar hasta 3 s a que llegue el hash para imprimir el ticket completo, o imprimir siempre "pendiente".
6. Venta con stock negativo (`allow_negative_stock`).
7. En consignación, qué unidad se vende primero: la propia o la consignada.
8. Fecha estimada del próximo cambio de aceite cuando el vehículo no tiene historial de km (el intervalo es fijo: +5 000 km, #100; el aviso sale 7 días antes, #101).
9. Fuente del tipo de cambio y del DNI.
10. 2FA por SMS (no propuesto).
   - Impersonación de soporte en incidentes P1: ¿se admite alguna excepción al consentimiento previo de la empresa (#90), por ejemplo con doble firma de staff y aviso inmediato? No está decidida; mientras tanto no hay excepción (ADR 0001).
11. Paginación numerada frente a cursor (cambio de UX en `PaginationBar`).
12. Si el despacho físico requiere la GRE aceptada.

Infraestructura (tech-lead):
13. Bucket de Railway o S3 externo.
14. Plan de Postgres de Railway: máximo de conexiones, PITR y extensiones.
15. Backend de observabilidad.
16. Licencia del OpenAPI.

Supuestos de carga: los de toda la plataforma en `capacity.md`, sujetos a la prueba de carga.

## 11. Detalle de las reglas para agentes (resumidas en `CLAUDE.md`)

`CLAUDE.md` solo lleva las reglas; el detalle vive aquí y en los ADR.

- **Binario:** una imagen con `serve --plane=tenant|platform|webhooks`, `worker` y `migrate`. Librerías propuestas, pendientes de aprobación del dueño: River, sqlc, goose, oapi-codegen, shopspring/decimal, pquerna/otp, OpenTelemetry y el SDK de S3 de AWS (ADR 0002).
- **Multi-tenancy (ADR 0001):**
  - `app.current_tenant()` es `NULLIF(current_setting('app.tenant_id', true), '')::uuid`;
  - `InTenantTx(ctx, Actor{TenantID, UserID, Kind, ImpersonatorID})` fija siempre `app.tenant_id`, `app.user_id` y `app.session_kind`, y el tenant sale de la sesión, nunca del cuerpo, la URL ni una cabecera;
  - los roles de aplicación no son dueños de tablas;
  - las funciones `SECURITY DEFINER` tienen `REVOKE ALL ... FROM PUBLIC`, y cuando necesitan un tenant que el llamador no fijó, lo fijan desde sus propios datos y restauran el valor del llamador;
  - las únicas políticas extra permitidas son las de solo lectura con nombre, más `platform_rw` en `org.tenants`, limitada a `wanka_platform`.
- **RLS check:** `MUTATION=naive_session_lookup sh data/rls-check/run.sh` muestra una corrida que falla.
- **API:** REST `/v1`, `camelCase`, errores RFC 9457 con `code`, paginación por cursor, `Idempotency-Key` obligatoria en los `POST` que crean, y dinero como string decimal (`api/conventions.md`). El catálogo de permisos tiene el plano tenant y el `plataforma.*`. Los mocks del frontend se reemplazan por funciones `lib/api/*` tipadas con `openapi.yaml`.
- **Dinero (ADR 0008):** importes `(14,2)`, precios unitarios `(14,4)`, costos `(18,6)` y cantidades `(14,4)`. `exchange_rate numeric(6,3)` es el formato de Khipu. Los correlativos salen de un contador por serie, sin llamadas de red dentro de la transacción.
- **Facturación (ADR 0003):** asíncrona con outbox, un emisor en orden por serie, y webhooks más conciliación. Las verificaciones en BETA son el redondeo contra `POST /v1/facturas/calcular` y el correlativo N+1 antes que N.
- **Despliegue (ADR 0007):** servicios `web`, `api`, `platform-api`, `worker` y `webhooks`. `/v1` solo se alcanza por la red privada, con configuración 12-factor.
- **Diagramas:** se validan con `npx -y @mermaid-js/mermaid-cli -i file.mmd -o /tmp/x.svg`.

## Validación

Cada `.mmd` se renderizó con `npx -y @mermaid-js/mermaid-cli -i <archivo> -o <scratch>/<archivo>.svg`, con la salida fuera del repositorio. Un archivo inválido hace fallar el comando; se comprobó con un diagrama roto a propósito. El esqueleto OpenAPI se validó con `npx @redocly/cli lint`: es válido, y solo queda un aviso porque falta el campo `license`.
