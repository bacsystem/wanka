# Convenciones de la API

Estado: **congelado (2026-11-18)**, historia #160. Aplica a `api` (plano tenant, `/v1`) y `platform-api` (plano plataforma, `/platform/v1`). La fuente de verdad del contrato será `apps/api/openapi/openapi.yaml` (spec-first, [ADR 0009](../adr/0009-api-contract-openapi.md)); este documento fija las reglas que ese archivo debe cumplir. Esqueleto inicial: [`openapi.yaml`](openapi.yaml).

## 0. Contrato congelado: cómo se cambia

Este documento y [`resources.md`](resources.md) están congelados desde el 2026-11-18. Los módulos de las olas 1 a 6 implementan contra ellos; ninguno los cambia por su cuenta.

- **Todo cambio** a estas reglas, al catálogo de errores (§4), a la paginación (§5), a los tipos en el cable (§6), a las cabeceras (§3) o al catálogo de permisos de `resources.md` es un **PR de contrato**: un PR a `docs/architecture/api/` aprobado por el architect. El PR dice si el cambio es compatible y qué issues afecta.
- **Cambios compatibles** (no cambian la versión): campo opcional nuevo en una respuesta, endpoint nuevo, permiso nuevo en el catálogo, `code` nuevo en §4 (los clientes tratan un `code` desconocido por su `status` y su `title`), valor nuevo en un enum abierto.
- **Cambios incompatibles** (exigen versión nueva): quitar o renombrar un campo, una ruta o un `code`; cambiar el HTTP de un `code`; volver obligatorio un campo de request; cambiar el tipo o el formato de un campo; cambiar la paginación o la semántica de una cabecera. Se publica `/v2` del recurso afectado y `/v1` sigue respondiendo con `Deprecation` y `Sunset` (fecha al menos **90 días** después), según §1.
- Las decisiones del dueño que siguen abiertas (§8) no bloquean el congelamiento: el contrato ya fija la forma, y la decisión solo cambia el origen del dato.

## 1. Estilo y versionado

- REST sobre HTTPS, JSON UTF-8, campos en `camelCase`, igual que los mocks, para que los tipos de `types/domain.ts` y de cada `features/*/mocks` se reemplacen por los generados desde OpenAPI sin capa de transformación.
- Recursos en plural, en inglés, agrupados por módulo: `/v1/sales`, `/v1/einvoicing/documents`, `/v1/inventory/stock-levels`. Las rutas del frontend (en español) no se reflejan en la API.
- Acciones que no son CRUD: subrecurso verbo con `POST` (`/v1/einvoicing/documents/{id}/void`, `/v1/cash/shifts/{id}/close`).
- Versión mayor en la ruta (`/v1`). Cambios compatibles (campo opcional nuevo, endpoint nuevo, valor nuevo en un enum documentado como abierto) no cambian la versión. Un cambio incompatible crea `/v2` del recurso afectado y `/v1` se mantiene con cabecera `Deprecation` y `Sunset` al menos 90 días.
- Los clientes deben ignorar campos desconocidos y tratar los enums de estado como abiertos (mostrar valor genérico si llega uno nuevo).

## 2. Autenticación, planos y resolución de tenant

| | Plano tenant | Plano plataforma |
|---|---|---|
| Servicio | `api` | `platform-api` |
| Prefijo de ruta | `/v1` | `/platform/v1` |
| Token | `Authorization: Bearer wk_at_<random>` | `Authorization: Bearer wka_at_<random>` |
| Cookie en el navegador (las pone el BFF) | `wk_at`, `wk_rt`, `wk_at_exp` (`Path=/`) | `wka_at`, `wka_rt`, `wka_at_exp` (`Path=/`; los nombres separan los planos, porque los route handlers `/api/admin/*` están fuera de `/admin`) |
| Tabla de sesiones | `iam.sessions` | `platform.platform_sessions` |
| Rol de base de datos | `wanka_app` (sujeto a RLS) | `wanka_platform` |

- Tokens opacos de 256 bits; en la base se guarda solo `sha256(token)`. Un token de un plano **no existe** en la tabla del otro, y además el prefijo se rechaza en el middleware antes de consultar: un token de plataforma nunca sirve en el plano tenant ni al revés.
- **Tenant y sede salen de la sesión**, no de una cabecera ni de la URL. La sesión queda ligada a `tenant_id` y `site_id` al llamar `POST /v1/auth/session/tenant`; cambiar de empresa emite tokens nuevos. Esto reemplaza la cookie `tenant_id` actual del frontend (`features/auth/lib/tenant-cookie.ts`), que hoy elige el tenant sin autorización. No hay `X-Tenant-Id`: evita que un cliente pida datos de otro tenant cambiando una cabecera.
- Endpoints sin tenant (antes de elegir empresa): `/v1/auth/*`, `GET /v1/me`, `GET /v1/me/memberships`.
- El navegador nunca habla con la API: solo el BFF de Next.js, servidor a servidor. CORS deshabilitado en la API.
- Webhooks entrantes (`/webhooks/khipu/{routeId}`, uno por tenant) no usan tokens de usuario. Se verifica la firma `X-Khipu-Signature` (HMAC-SHA256 de `t.cuerpo` con el secreto `whsec_` del tenant) con una tolerancia de 300 s, y se deduplica por `X-Khipu-Delivery` (ADR 0003).

## 3. Cabeceras

| Cabecera | Dirección | Uso |
|---|---|---|
| `Authorization` | req | `Bearer <token>` |
| `Idempotency-Key` | req | **obligatoria** en `POST` que crean documentos o mueven dinero/stock (ventas, notas, bajas, pagos, traslados, cierres de caja, checkout de OT, alta de tenant); opcional en el resto de `POST`. UUID generado por el cliente al iniciar la operación (no al hacer clic) |
| `If-Match` | req | versión (`"v12"`) para `PATCH`/`PUT` de recursos editables concurrentemente (proformas, OT, configuración); `412` si no coincide |
| `X-Request-Id` | req/resp | correlación; el BFF lo genera si falta y aparece en cada log y en el error |
| `ETag` | resp | versión del recurso |
| `Retry-After` | resp | en `429` y `503` |
| `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` | resp | cuota por cliente |

### Idempotencia

- Tabla `ops.idempotency_keys (tenant_id, key)` con hash del cuerpo y la respuesta, retención 24 h.
- Misma clave + mismo cuerpo → se devuelve la respuesta guardada (mismo código y cuerpo) con `Idempotent-Replayed: true`.
- Misma clave + cuerpo distinto → `422 idempotency_key_reused`.
- La clave se inserta **primero**, en la misma transacción que el efecto (`INSERT ... ON CONFLICT DO NOTHING`). Si la transacción aborta, la clave no queda registrada y el reintento se ejecuta.
- **Solo se guardan respuestas `2xx`** (aclaración del 2026-11-18, épica #154). Toda respuesta `4xx` o `5xx` de una ruta con clave implica `ROLLBACK` de su transacción, y con él se va la clave: el reintento con la misma clave se ejecuta de nuevo y puede responder distinto (por ejemplo, `409 shift_not_open` y, tras abrir el turno, `201`). Es intencional: el cliente reusa la clave en todos los reintentos de la misma operación (se genera al iniciarla), así que guardar un `4xx` dejaría la operación bloqueada en ese error durante 24 h. No hay doble efecto, porque un `4xx` no confirmó nada. Los códigos que produce la propia idempotencia (`422 idempotency_key_reused`, `409 request_in_progress`) tampoco se guardan.
- Una clave con más de 24 h que el job horario aún no borró **se sigue tratando como completada** (se repite su respuesta): la retención es un mínimo, no un máximo.
- Misma clave mientras la primera sigue en curso: el `INSERT` de la segunda **espera** el lock del índice único de la primera, porque la fila no confirmada ya ocupa la clave. Si la primera confirma, la segunda ve el conflicto y devuelve la respuesta guardada. Si aborta, la segunda continúa como nueva. La espera está acotada por `lock_timeout = 3s` en esa sentencia: si se vence, la API responde `409 request_in_progress` con `Retry-After: 1`. Es el único caso en que aparece ese código.
- Todas las respuestas de la API usan los mismos códigos: un cuerpo distinto con la misma clave es siempre `422 idempotency_key_reused`, nunca 409.
- El BFF no reintenta `POST` sin `Idempotency-Key`; con clave, como máximo 1 reintento ante error de red.

## 4. Formato de error

RFC 9457 (`application/problem+json`) con un `code` estable que el frontend traduce:

```json
{
  "type": "https://docs.wanka.pe/errors/insufficient_stock",
  "title": "Stock insuficiente",
  "status": 409,
  "code": "insufficient_stock",
  "detail": "Stock disponible 2 NIU en Almacén central",
  "requestId": "01JD3W6Y8M...",
  "errors": [
    { "field": "lines[1].quantity", "code": "insufficient_stock", "meta": { "variantId": "...", "available": "2" } }
  ]
}
```

(El dominio del `type` es ilustrativo; se fija al publicar la documentación.)

**Lista cerrada de `code`.** Un handler o middleware solo devuelve códigos de esta tabla; agregar uno es un PR de contrato (§0). La columna «Lo emite» dice quién lo produce: «kernel» son los middlewares y la base HTTP comunes a todas las rutas, en el orden de la [secuencia 08](../sequences/08-entitlement-enforcement.mmd) (sesión → empresa cerrada → suscripción → módulo → límite → 2FA → permiso; gana el primero que falla). El paso «sesión» incluye la membresía: una membresía borrada o no activa responde `401 session_expired` ahí, antes de los entitlements, con los datos que ya devolvió `iam.session_lookup`.

| HTTP | `code` | Lo emite | Significado |
|---|---|---|---|
| 400 | `malformed_request` | kernel | JSON inválido o tipo incorrecto en un campo o parámetro |
| 401 | `unauthenticated` | kernel | sin token, token inexistente o de otro plano (prefijo `wka_` en `/v1` o `wk_` en `/platform/v1`). El BFF intenta un refresh una vez |
| 401 | `session_expired` | kernel | sesión expirada o revocada, o membresía que dejó de estar activa. El BFF intenta un refresh una vez y, si falla, lleva a `/login` |
| 401 | `invalid_credentials` | identidad | solo en `POST /v1/auth/login` y `/platform/v1/auth/login`: correo o contraseña incorrectos, con el mismo mensaje y tiempo si el correo no existe ([secuencia 01](../sequences/01-login-2fa-tenant.mmd)). El BFF no hace refresh |
| 402 | `subscription_inactive` | kernel | suscripción `morosa`, `suspendida` o `cancelada`: **bloqueo total** del tenant salvo la exportación de datos (ver abajo, decisión del dueño del 2026-11-18) |
| 403 | `forbidden` | kernel | falta el permiso del catálogo que declara la ruta (`x-permission`); el problema lleva `permission` |
| 403 | `module_not_enabled` | kernel | el módulo de la ruta (`x-module`) no está habilitado para el tenant; el problema lleva `module` |
| 403 | `plan_limit_reached` | kernel | la ruta consume un límite del plan y `used ≥ limit`; el problema lleva `limit` y `used` |
| 403 | `tenant_read_only` | kernel | empresa **cerrada** (`org.tenants.status = cerrada`, ADR 0005): solo pasan `GET`, incluidos los exportes. No se usa para la morosidad |
| 403 | `mfa_required` | kernel | único código de 2FA: la membresía elegida exige 2FA y la sesión entró con `mfa_method = none`. El login nunca lo devuelve como error: allí la 2FA es un paso con `status: mfa_required` o `mfa_enrollment_required` en un 200 |
| 403 | `tenant_access_denied` | identidad | `POST /v1/auth/session/tenant` con un tenant donde el usuario no tiene membresía activa |
| 403 | `site_out_of_scope` | módulo | la operación pide una sede distinta de la de la sesión y el rol no tiene alcance sobre ella |
| 404 | `not_found` | kernel y módulo | ruta o recurso inexistente; también cuando el recurso existe en otro tenant (no se revela) |
| 409 | `conflict` | módulo | conflicto genérico de estado que no tiene un código propio |
| 409 | `duplicate` | módulo | viola una unicidad de negocio (documento de cliente, SKU, serie) |
| 409 | `insufficient_stock` | inventario, ventas | stock disponible menor que el pedido; `errors[].meta` lleva `variantId` y `available` |
| 409 | `shift_not_open` | finanzas, ventas | la caja de la sesión no tiene turno abierto |
| 409 | `invalid_state_transition` | módulo | la acción no vale en el estado actual del recurso (anular un CPE rechazado, cerrar un turno cerrado) |
| 409 | `request_in_progress` | kernel | solo por el `lock_timeout` de idempotencia (§3); con `Retry-After: 1` |
| 412 | `version_mismatch` | kernel | `If-Match` no coincide con la versión actual |
| 413 | `payload_too_large` | kernel | cuerpo mayor que 1 MB (§7) |
| 422 | `validation_failed` | kernel y módulo | regla de validación; el detalle va en `errors[]` por campo |
| 422 | `idempotency_key_reused` | kernel | misma `Idempotency-Key` con otro cuerpo (§3) |
| 422 | `totals_mismatch` | ventas, facturación | los totales enviados no coinciden con `pricing.Compute` |
| 422 | `void_window_expired` | facturación | venció el plazo de SUNAT para la comunicación de baja |
| 422 | `exchange_rate_required` | ventas, compras, finanzas | moneda USD sin tipo de cambio, o sin tipo de cambio disponible para la fecha |
| 422 | `discount_exceeds_role_limit` | ventas | el descuento supera el tope del rol y no viene la aprobación de un supervisor |
| 429 | `rate_limited` | kernel | cuota por cliente o tenant superada (§7); con `Retry-After` |
| 503 | `overloaded` | kernel | backpressure: cola o pool llenos; con `Retry-After` |
| 503 | `dependency_unavailable` | kernel y módulo | una dependencia no responde: la base al buscar la sesión (`iam.session_lookup`) o al leer los entitlements, o Khipu en las descargas síncronas. Con `Retry-After` (`1` en el kernel). Un fallo de la base al buscar la sesión nunca es `401` ni `500`; un pool sin conexión libre es `503 overloaded` |
| 500 | `internal` | kernel | error no previsto; sin detalles internos, solo `requestId` |

### Suscripción inactiva: bloqueo total salvo exportar (decisión del dueño, 2026-11-18)

Con la suscripción en `morosa`, `suspendida` o `cancelada` ([`er-05`](../data/er-05-plataforma-transversal.mmd), `platform.subscriptions.status`) el tenant **no puede operar**: toda ruta del plano tenant responde `402 subscription_inactive`, sea `GET` o no. No hay modo de solo lectura durante la mora. Siguen disponibles:

- `GET /v1/exports` (exportación de los datos del tenant, con su permiso `*:export`, sus topes y su cupo, §7);
- lo que no depende de un tenant o sirve para salir del bloqueo: `/v1/auth/*`, `GET /v1/me`, `GET /v1/me/memberships` y `GET /v1/me/entitlements` (la UI lo lee para mostrar el motivo del bloqueo y el enlace al exporte).

`tenant_read_only` queda **solo** para la empresa cerrada (ADR 0005), que conserva 5 años de lectura. A una empresa cerrada no se le evalúa la suscripción: su lectura es una obligación de conservación y no depende de que pague, así que su único bloqueo es `tenant_read_only`. El detalle y su aplicación a los jobs están en el [ADR 0005](../adr/0005-industry-modules-feature-flags.md), «Suscripción inactiva».

Un fallo de Khipu **nunca** se propaga como 500 en la petición del usuario: la emisión es asíncrona y el estado del CPE refleja el problema. **Excepción:** la descarga de XML, CDR y PDF depende de Khipu de forma síncrona (ADR 0003, «Artefactos: Khipu los conserva»). Si falla, responde `503 dependency_unavailable` con `Retry-After`, nunca `500`.

## 5. Paginación, filtros y orden

- Listados por **cursor**: `GET /v1/sales/documents?limit=50&cursor=<opaco>`. El cursor codifica la última clave de orden (`issueDate`, `id`); la consulta es `WHERE (issue_date, id) < (:d, :id) ORDER BY issue_date DESC, id DESC LIMIT :n+1`. Sin `OFFSET`.
- `limit` por defecto 25, máximo 100 (los exportes van aparte: `GET /v1/exports` por streaming, §7). El cursor es opaco y solo vale con los mismos filtros y orden con que se emitió; uno alterado o ajeno responde `400 malformed_request`.
- Respuesta:
  ```json
  { "data": [ ... ], "page": { "nextCursor": "eyJ...", "hasMore": true } }
  ```
- **Decisión del dueño (2026-11-18): paginación por cursor en todos los listados**, sin paginación numerada. El frontend navega con **anterior / siguiente**: «siguiente» usa `nextCursor`; «anterior» usa el cursor previo de una pila que guarda la pantalla (la API no devuelve `prevCursor`). Al cambiar un filtro o el orden, se vuelve al inicio.
- El total («de N») solo aparece donde el endpoint lo declara y el cliente lo pide con `?includeTotal=true`. Responde `page.total` (entero) y `page.totalIsEstimate` (booleano): `count(*)` con tope o una estimación cuando la tabla es grande. Sin ese parámetro, `page` no trae total.
- Adaptar `PaginationBar` (hoy con números de página) a este contrato es el issue de frontend [#178](https://github.com/bacsystem/wanka/issues/178).
- Los KPI de cabecera de cada listado (los `*Summary` de los mocks, p. ej. `documentsSummary`, `stockSummary`) son un endpoint separado `GET .../summary` con los mismos filtros, para que el listado no pague la agregación.
- Filtros como query params con nombres del recurso (`status=aceptado&status=observado`, `issuedFrom=2026-09-01`, `q=` para búsqueda de texto). Orden: `sort=-issueDate`, solo sobre campos indexados declarados por endpoint.

## 6. Tipos en el cable

| Dato | Representación |
|---|---|
| IDs | string (UUID v7) |
| Dinero | string decimal con escala fija: `"1180.00"`; precios unitarios hasta 4 decimales |
| Cantidades | string decimal: `"2.5000"` o normalizado `"2.5"` (a fijar en OpenAPI) |
| Instantes | ISO-8601 con zona: `"2026-09-12T11:42:00-05:00"` |
| Fechas fiscales | `"2026-09-12"` |
| Moneda | `"PEN"` / `"USD"`. **Obligatoria en todo request y recurso con dinero** (PEN y USD desde R1) |
| Tipo de cambio | `exchangeRate`: string con 3 decimales (`"3.756"`), obligatorio si la moneda es USD y prohibido si es PEN (igual que `tipo_cambio` de Khipu, ADR 0003) |
| Importes de request | `MoneyPositive` (> 0) para importes y pagos, `MoneyNonNegative` (>= 0) para descuentos, `Money` con signo solo para diferencias calculadas por el servidor; coinciden con los `CHECK` de la base |
| Medios de pago | `efectivo`, `tarjeta`, `yape`, `plin`, `transferencia`, `credito` (solo factura). El `yape_plin` de `types/domain.ts` es una agrupación de UI que se separa al integrar |
| Número de CPE | `series` + `number` (entero) por separado; `display` opcional `"F001-00000842"`. `number` y `display` son `null` en una GRE hasta que Khipu la numera |
| Estados | string en minúsculas, valores de los mocks (`aceptado`, `pendiente`, `rechazado`, ...) |

## 7. Límites y protección

- Rate limit por sesión y por tenant en la aplicación (token bucket; en memoria mientras haya una réplica, en Postgres/Redis al escalar, ver [ADR 0007](../adr/0007-deployment-railway.md)), y por IP en el borde.
- Endpoints con cuota más estricta: `auth/login` (5/min por IP+email), `auth/password/forgot` (3/h por email), descargas de artefactos (2 concurrentes por tenant y 16 por réplica de `api`) y exportes CSV (2 concurrentes por tenant y 4 por réplica), en cupos separados (ADR 0003), búsquedas `q=` (10/s por sesión).
- Cuerpo máximo 1 MB. Los adjuntos irán por URL prefirmada directa al bucket cuando exista su adaptador (`BlobStore`); en R1 no hay bucket (data-model §5).
- Timeouts (el de quien llama siempre mayor que el del llamado): Postgres `statement_timeout` 5 s para el rol de la app → handler de la API 10 s (`http.TimeoutHandler`; lectura de cabeceras 5 s) → BFF 12 s hacia la API → proxy/borde 30 s. Excepción: las descargas de artefactos y los exportes van por streaming **fuera** de `http.TimeoutHandler`, con un presupuesto propio contado **desde que llega la petición**. Descargas: 3 s para conectar con Khipu y 8 s para sus encabezados (si no, `503 dependency_unavailable` RFC 9457). Exportes: cursor de servidor con `SET LOCAL statement_timeout = '20s'` por lote (`FETCH`) en una transacción de solo lectura; el exporte completo lo acota el `context` de 25 s. En ambos casos, 25 s en total → BFF 27 s para esas rutas → borde 30 s (ADR 0003, «Artefactos: Khipu los conserva»). `IdleTimeout` del servidor Go (120 s) mayor que el keep-alive del proxy de Railway (supuesto 60 s; medir).

## 8. Decisiones del dueño

Decididas y ya reflejadas en este documento:

- **Paginación por cursor** (2026-11-18), §5. Seguimiento de frontend: #178.
- **Suscripción inactiva = bloqueo total salvo exportar** (2026-11-18), §4. Aplicación: #175.
- Ya tomadas antes: PEN y USD desde R1, dinero como string decimal, errores RFC 9457, tenant desde la sesión (sin cabecera).

**Pendientes de decisión.** No bloquean el congelamiento: el contrato ya fija la forma del dato, y la decisión solo elige de dónde sale.

| Tema | Qué falta decidir | Historias afectadas | Qué ya está fijo |
|---|---|---|---|
| Fuente oficial del tipo de cambio | de dónde lo descarga el job diario (SBS, SUNAT u otra) y a qué hora | #170 (la tabla `ops.exchange_rates` y `fx.Rate` no dependen de la fuente) y el job de ingesta, que #170 deja fuera de alcance y aún no tiene issue | `exchangeRate` string con 3 decimales (§6), `source` `official`/`manual`/`fallback`, un solo redondeo al ingerir y `422 exchange_rate_required` si no hay ninguno ([data-model §2](../data/data-model.md)) |
| Fuente del DNI | qué servicio responde `GET /v1/lookups/dni/{dni}` (vía Khipu u otro proveedor) | #34 (clientes) y el alta rápida del POS (#48) | la ruta, su permiso, el timeout de 3 s y que, si la consulta falla, el cliente se ingresa a mano marcado «sin validar» ([`resources.md`](resources.md), clientes) |

