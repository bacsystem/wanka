# Convenciones de la API

Estado: propuesto (2026-11-17). Aplica a `api` (plano tenant, `/v1`) y `platform-api` (plano plataforma, `/platform/v1`). La fuente de verdad del contrato será `apps/api/openapi/openapi.yaml` (spec-first, [ADR 0009](../adr/0009-api-contract-openapi.md)); este documento fija las reglas que ese archivo debe cumplir. Esqueleto inicial: [`openapi.yaml`](openapi.yaml).

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

| HTTP | `code` | Significado |
|---|---|---|
| 400 | `malformed_request` | JSON inválido, tipo incorrecto |
| 401 | `unauthenticated`, `session_expired` | sin sesión válida; el BFF intenta refresh una vez |
| 402 | `subscription_inactive` | suscripción suspendida o morosa |
| 403 | `forbidden` (falta permiso), `module_not_enabled`, `plan_limit_reached`, `tenant_access_denied`, `site_out_of_scope`, `tenant_read_only` (empresa cerrada: solo `GET` y exportes, ADR 0005), `mfa_required` (único código de 2FA: la membresía elegida exige 2FA y la sesión entró con `mfa_method = none`; el login nunca lo devuelve como error, porque allí la 2FA es un paso con `status: mfa_required` o `mfa_enrollment_required` en un 200) | |
| 404 | `not_found` | también cuando el recurso existe en otro tenant (no se revela) |
| 409 | `conflict`, `insufficient_stock`, `shift_not_open`, `invalid_state_transition`, `request_in_progress` (solo por `lock_timeout` de idempotencia), `duplicate` | |
| 412 | `version_mismatch` | `If-Match` no coincide |
| 422 | `validation_failed`, `totals_mismatch`, `idempotency_key_reused`, `void_window_expired`, `exchange_rate_required`, `discount_exceeds_role_limit` | regla de negocio |
| 429 | `rate_limited` | con `Retry-After` |
| 503 | `overloaded`, `dependency_unavailable` | backpressure; con `Retry-After` |
| 500 | `internal` | sin detalles internos; solo `requestId` |

Un fallo de Khipu **nunca** se propaga como 500 en la petición del usuario: la emisión es asíncrona y el estado del CPE refleja el problema. **Excepción:** la descarga de XML, CDR y PDF depende de Khipu de forma síncrona (ADR 0003, «Artefactos: Khipu los conserva»). Si falla, responde `503 dependency_unavailable` con `Retry-After`, nunca `500`.

## 5. Paginación, filtros y orden

- Listados por **cursor**: `GET /v1/sales/documents?limit=50&cursor=<opaco>`. El cursor codifica la última clave de orden (`issueDate`, `id`); la consulta es `WHERE (issue_date, id) < (:d, :id) ORDER BY issue_date DESC, id DESC LIMIT :n+1`. Sin `OFFSET`.
- `limit` por defecto 25, máximo 100 (exportes: endpoint asíncrono aparte).
- Respuesta:
  ```json
  { "data": [ ... ], "page": { "nextCursor": "eyJ...", "hasMore": true } }
  ```
- `PaginationBar` del frontend muestra números de página: con cursor se ofrece "anterior / siguiente" y el total aproximado solo donde se pide explícitamente (`?includeTotal=true`, con tope de costo; usa `count(*)` limitado o estimación). Cambio de UX a acordar con el PO.
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
- Endpoints con cuota más estricta: `auth/login` (5/min por IP+email), `auth/password/forgot` (3/h por email), descargas de artefactos (2 concurrentes por tenant y 16 por réplica de `api`) y exportes, incluido el PDF del Z (2 concurrentes por tenant y 4 por réplica), en cupos separados (ADR 0003), búsquedas `q=` (10/s por sesión).
- Cuerpo máximo 1 MB. Los adjuntos irán por URL prefirmada directa al bucket cuando exista su adaptador (`BlobStore`); en R1 no hay bucket (data-model §5).
- Timeouts (el de quien llama siempre mayor que el del llamado): Postgres `statement_timeout` 5 s para el rol de la app → handler de la API 10 s (`http.TimeoutHandler`; lectura de cabeceras 5 s) → BFF 12 s hacia la API → proxy/borde 30 s. Excepción: las descargas de artefactos y los exportes van por streaming **fuera** de `http.TimeoutHandler`, con un presupuesto propio contado **desde que llega la petición**. Descargas: 3 s para conectar con Khipu y 8 s para sus encabezados (si no, `503 dependency_unavailable` RFC 9457). Exportes: cursor de servidor con `SET LOCAL statement_timeout = '20s'` por lote (`FETCH`) en una transacción de solo lectura; el exporte completo lo acota el `context` de 25 s. En ambos casos, 25 s en total → BFF 27 s para esas rutas → borde 30 s (ADR 0003, «Artefactos: Khipu los conserva»). `IdleTimeout` del servidor Go (120 s) mayor que el keep-alive del proxy de Railway (supuesto 60 s; medir).
