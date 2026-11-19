# Convenciones de la API

Estado: **congelado (2026-11-18)**, historia #160. Aplica a `api` (plano tenant, `/v1`) y `platform-api` (plano plataforma, `/platform/v1`). La fuente de verdad del contrato será `apps/api/openapi/openapi.yaml` (spec-first, [ADR 0009](../adr/0009-api-contract-openapi.md)); este documento fija las reglas que ese archivo debe cumplir. Esqueleto inicial: [`openapi.yaml`](openapi.yaml).

## 0. Contrato congelado: cómo se cambia

Este documento y [`resources.md`](resources.md) están congelados desde el 2026-11-18. Los módulos de las olas 1 a 6 implementan contra ellos; ninguno los cambia por su cuenta.

- **Todo cambio** a estas reglas, al catálogo de errores (§4), a la paginación (§5), a los tipos en el cable (§6), a las cabeceras (§3) o al catálogo de permisos de `resources.md` es un **PR de contrato**: un PR a `docs/architecture/api/` aprobado por el architect. El PR dice si el cambio es compatible y qué issues afecta.
- **Cambios compatibles** (no cambian la versión): campo opcional nuevo en una respuesta, endpoint nuevo, permiso nuevo en el catálogo, `code` nuevo en §4 (los clientes tratan un `code` desconocido por su `status` y su `title`), valor nuevo en un enum abierto.
- **Cambios incompatibles** (exigen versión nueva): quitar o renombrar un campo, una ruta o un `code`; cambiar el HTTP de un `code`; volver obligatorio un campo de request; cambiar el tipo o el formato de un campo; cambiar la paginación o la semántica de una cabecera. Se publica `/v2` del recurso afectado y `/v1` sigue respondiendo con `Deprecation` y `Sunset` (fecha al menos **90 días** después), según §1.
- **Excepción registrada (2026-11-18, [ADR 0011](../adr/0011-membership-loss-keeps-user-session.md)):** este cambio **no es compatible** según la lista anterior, y se aplica en `/v1` sin `/v2`. Tiene dos partes. La membresía perdida deja de ser `401 session_expired` y pasa a `403 tenant_access_denied`: la misma situación cambia de HTTP y de acción del BFF. Y una sesión que la búsqueda no encuentra deja de ser `401 unauthenticated` (token inexistente) y pasa a `401 session_expired`. Es aceptable porque no hay consumidores: ni la API ni el BFF existen todavía, y ningún código de `apps/` emite ni lee estos códigos. Desde el primer cliente real, un cambio así exige `/v2`.
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
| `X-Request-Id` | req/resp | correlación; el BFF lo genera. La API conserva el que llega si tiene de 1 a 128 caracteres de `[A-Za-z0-9_-]`; si falta o no cumple, lo **reemplaza** por uno propio (nunca responde `400` por esta cabecera). El valor final vuelve en la respuesta, en cada log y en el `requestId` del error |
| `ETag` | resp | versión del recurso |
| `Retry-After` | resp | en todo `429` y `503`, y en `409 request_in_progress`; siempre en segundos enteros ≥ 1 (nunca fecha HTTP) |
| `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` | resp | cuota del limitador de la aplicación (§7), con los campos separados del borrador IETF `draft-ietf-httpapi-ratelimit-headers` hasta su versión -06 (no los campos `RateLimit` y `RateLimit-Policy` de las versiones posteriores). Enteros: capacidad del bucket, fichas que quedan (redondeo hacia abajo) y segundos hasta que el bucket vuelve a estar lleno (redondeo hacia arriba). Van en toda respuesta de una ruta que pasó por el limitador, incluida la `429`; si aplican dos buckets (sesión y tenant), se informa el de menos fichas. Las respuestas rechazadas antes del limitador (`401`, `403` de sesión) no las llevan, ni tampoco el `503 overloaded` por tope de claves del limitador (no hay un bucket que informar), ni el `503 overloaded` por plazo vencido del handler ni el `500 internal` por `panic`: estos dos los genera el kernel y descartan las cabeceras del handler (§4, «Plazo del handler»). Los nombres de campo no distinguen mayúsculas (RFC 9110 §5.1): el cliente debe compararlos sin distinguirlas, porque pueden salir como `RateLimit-Limit` o `Ratelimit-Limit` |

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

**Lista cerrada de `code`.** Un handler o middleware solo devuelve códigos de esta tabla; agregar uno es un PR de contrato (§0). La columna «Lo emite» dice quién lo produce: «kernel» son los middlewares y la base HTTP comunes a todas las rutas, en el orden de la [secuencia 08](../sequences/08-entitlement-enforcement.mmd) (sesión → empresa cerrada → suscripción → módulo → límite → 2FA → permiso; gana el primero que falla). El paso «sesión» incluye la membresía: si está borrada o no activa, la sesión se desliga de esa empresa sin revocarse, y la ruta de tenant responde `403 tenant_access_denied` ahí, antes de los entitlements, con los datos que ya devolvió `iam.session_lookup` ([ADR 0011](../adr/0011-membership-loss-keeps-user-session.md)).

| HTTP | `code` | Lo emite | Significado |
|---|---|---|---|
| 400 | `malformed_request` | kernel | JSON inválido, tipo incorrecto en un campo o parámetro, o `Content-Type` distinto de `application/json` en una ruta que recibe JSON (no hay `415`) |
| 401 | `unauthenticated` | kernel | en los dos planos, lo que se rechaza **sin consultar la base**: sin `Authorization`, formato inválido o prefijo de otro plano (`wka_` en `/v1` o `wk_` en `/platform/v1`). El BFF intenta un refresh una vez |
| 401 | `session_expired` | kernel | la búsqueda de la sesión del plano no devuelve fila: `iam.session_lookup` en `/v1`, o la de `platform.platform_sessions` en `/platform/v1` (por ejemplo, un `wka_at_` bien formado que no existe). La búsqueda **no distingue** inexistente, expirada y revocada: las tres filtran igual. Incluye la impersonada cuyo grant se revocó o venció, porque esa sesión queda revocada. El BFF intenta un refresh una vez y, si falla, lleva a `/login`. La membresía perdida **no** da este código: ver `tenant_access_denied` |
| 401 | `invalid_credentials` | identidad | solo en `POST /v1/auth/login` y `/platform/v1/auth/login`: correo o contraseña incorrectos, con el mismo mensaje y tiempo si el correo no existe ([secuencia 01](../sequences/01-login-2fa-tenant.mmd)). El BFF no hace refresh |
| 402 | `subscription_inactive` | kernel | suscripción `morosa`, `suspendida` o `cancelada`: **bloqueo total** del tenant salvo la exportación de datos (ver abajo, decisión del dueño del 2026-11-18) |
| 403 | `forbidden` | kernel | falta el permiso del catálogo que declara la ruta (`x-permission`); el problema lleva `permission` |
| 403 | `module_not_enabled` | kernel | el módulo de la ruta (`x-module`) no está habilitado para el tenant; el problema lleva `module` |
| 403 | `plan_limit_reached` | kernel | la ruta consume un límite del plan y `used ≥ limit`; el problema lleva `limit` y `used` |
| 403 | `tenant_read_only` | kernel | empresa **cerrada** (`org.tenants.status = cerrada`, ADR 0005): solo pasan `GET`, incluidos los exportes. No se usa para la morosidad |
| 403 | `mfa_required` | kernel | único código de 2FA: la membresía elegida exige 2FA y la sesión entró con `mfa_method = none`. El login nunca lo devuelve como error: allí la 2FA es un paso con `status: mfa_required` o `mfa_enrollment_required` en un 200 |
| 403 | `tenant_access_denied` | identidad y kernel | el usuario no tiene membresía activa en la empresa: en `POST /v1/auth/session/tenant` (identidad), o en una ruta de tenant cuando la membresía de la sesión se borró o dejó de estar activa, o la sesión no tiene empresa (kernel, paso de sesión, antes de los entitlements: la sesión queda sin tenant y **no** se revoca, [ADR 0011](../adr/0011-membership-loss-keeps-user-session.md)). El BFF no hace refresh y lleva a `/seleccionar-empresa`; si no queda ninguna membresía activa, cierra la sesión y lleva a `/login` |
| 403 | `site_out_of_scope` | módulo | la operación pide una sede distinta de la de la sesión y el rol no tiene alcance sobre ella |
| 404 | `not_found` | kernel y módulo | ruta o recurso inexistente; también cuando el recurso existe en otro tenant (no se revela). Una ruta es método + camino: un método no declarado (incluido `OPTIONS`, porque no hay CORS, §2) también es `404`, sin `405` |
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
| 503 | `overloaded` | kernel | backpressure: cola o pool llenos (pool de base sin conexión libre en 200 ms, cupo global de descargas o exportes, tope de claves del limitador), o el handler superó su plazo de 10 s (§7, ver abajo); con `Retry-After` (`1` en el kernel) |
| 503 | `dependency_unavailable` | kernel y módulo | una dependencia no responde: la base al buscar la sesión (`iam.session_lookup`) o al leer los entitlements, la base cuando cancela una sentencia por `statement_timeout` en cualquier ruta (SQLSTATE `57014`, `db.ErrStatementTimeout`; ver abajo), o Khipu en las descargas síncronas. Con `Retry-After` (`1` en el kernel y en el `statement_timeout`). Un fallo de la base al buscar la sesión nunca es `401` ni `500`; un pool sin conexión libre es `503 overloaded` |
| 500 | `internal` | kernel | error no previsto; sin detalles internos, solo `requestId` |

### Peticiones que no llegan a la API: excepción a «todo error es RFC 9457»

`net/http` rechaza antes de llamar a la cadena de la API lo que no logra interpretar como una petición HTTP: un camino con escape inválido (`/%zz`), una línea de petición que no es HTTP, un salto de línea suelto en una cabecera o cabeceras que superan su límite. Esas peticiones **no llegan a la API**: las responde el servidor de Go con `400` (o `431`) en `text/plain`, sin `X-Request-Id`, sin `code` y sin cuerpo RFC 9457, y no pasan por el log de acceso. Es una excepción conocida a la regla de que toda respuesta de error es `application/problem+json`; no se reemplaza el analizador de `net/http`. El BFF solo envía peticiones bien formadas, así que en la práctica solo las ve quien llame a la API directamente.

### Suscripción inactiva: bloqueo total salvo exportar (decisión del dueño, 2026-11-18)

Con la suscripción en `morosa`, `suspendida` o `cancelada` ([`er-05`](../data/er-05-plataforma-transversal.mmd), `platform.subscriptions.status`) el tenant **no puede operar**: toda ruta del plano tenant responde `402 subscription_inactive`, sea `GET` o no. No hay modo de solo lectura durante la mora. Siguen disponibles:

- `GET /v1/exports` (exportación de los datos del tenant, con su permiso `*:export`, sus topes y su cupo, §7);
- lo que no depende de un tenant o sirve para salir del bloqueo: `/v1/auth/*`, `GET /v1/me`, `GET /v1/me/memberships` y `GET /v1/me/entitlements` (la UI lo lee para mostrar el motivo del bloqueo y el enlace al exporte).

`tenant_read_only` queda **solo** para la empresa cerrada (ADR 0005), que conserva 5 años de lectura. A una empresa cerrada no se le evalúa la suscripción: su lectura es una obligación de conservación y no depende de que pague, así que su único bloqueo es `tenant_read_only`. El detalle y su aplicación a los jobs están en el [ADR 0005](../adr/0005-industry-modules-feature-flags.md), «Suscripción inactiva».

Un fallo de Khipu **nunca** se propaga como 500 en la petición del usuario: la emisión es asíncrona y el estado del CPE refleja el problema. **Excepción:** la descarga de XML, CDR y PDF depende de Khipu de forma síncrona (ADR 0003, «Artefactos: Khipu los conserva»). Si falla, responde `503 dependency_unavailable` con `Retry-After`, nunca `500`.

### `statement_timeout`: `503 dependency_unavailable` en toda ruta (architect, 2026-11-18)

Cuando la base cancela una sentencia por `statement_timeout` (los 5 s del rol `wanka_app`, o el `SET LOCAL` de 20 s por lote de un exporte, §7), `InTenantTx` devuelve `db.ErrStatementTimeout` y `httpx.WriteDBError` responde `503 dependency_unavailable` con `Retry-After: 1`, en el kernel y en los módulos por igual. Es el mismo caso que el [ADR 0004](../adr/0004-in-house-auth.md) ya fija para `iam.session_lookup`, extendido a toda sentencia.

- **Mismo cuerpo que cualquier `dependency_unavailable`**: `title` y `detail` genéricos («a dependency is not responding»). La respuesta no dice que fue la base, ni qué sentencia, ni cuánto tardó, ni distingue el kernel del módulo. El mensaje de PostgreSQL (traducido según `lc_messages`) nunca llega al cliente.
- **Lo que distingue el caso va al log, no al cable.** Una línea `warn` por respuesta, con `requestId`, el patrón de la ruta (no la URL con ids), la fase (`session_lookup`, `entitlements`, `handler` o `export`), el módulo si lo hay, `sqlstate=57014` y el tiempo transcurrido. Nunca el texto SQL, sus parámetros, el mensaje de PostgreSQL, tokens ni hashes. Un contador por fase y ruta alimenta dos alertas: en el kernel señala saturación de la base; repetido en la misma ruta de módulo señala una consulta cara o un bug, y se abre un issue.
- **Seguro de reintentar.** Una sentencia cancelada aborta la transacción e `InTenantTx` hace `ROLLBACK`: no se confirma nada, tampoco la `Idempotency-Key` (§3, solo se guardan `2xx`). Pero **el BFF no reintenta un `503` por su cuenta**: sus únicos reintentos automáticos son el de un error de red en un `POST` con clave (§3) y el refresh ante un `401` (ADR 0004). `Retry-After` es para quien decide reintentar, el usuario. Así una consulta que siempre tarda no se multiplica contra la base.
- **No es `ErrStatementTimeout`**: la cancelación por el `context` de la petición (mismo SQLSTATE, pero con el `ctx` terminado), que no escribe respuesta si el cliente se fue; y el `lock_timeout` de idempotencia (`55P03`), que sigue siendo `409 request_in_progress`. En un exporte, si el `FETCH` vence después de empezar el streaming, el archivo queda truncado (ADR 0003): ya no hay problema RFC 9457 que enviar.
- **Opciones descartadas.** `500 internal` («error no previsto») convierte la saturación de la base en un error sin `Retry-After` y contradice el precedente del ADR 0004, donde el mismo timeout en la búsqueda de sesión es `503`. Un `code` nuevo (`query_timeout`) sería compatible, pero revela un detalle interno y promete una distinción que la API no puede hacer: el mismo timeout lo causa una consulta cara o una base saturada por otros, y la acción del cliente es la misma. `504` es el código de un proxy que no recibió a tiempo la respuesta de su servidor: es lo que verá el usuario si vence el plazo del borde (§7), y usarlo en la API mezclaría «la base tardó» con «la API no respondió»; además exigiría un `code` nuevo. Separar el kernel del módulo en el cable revela la estructura interna sin cambiar lo que hace el cliente.
- **Qué lo haría cambiar.** Si una pantalla necesita decirle al usuario «acota el rango» (reportes, búsquedas), la herramienta es validar el rango antes de consultar (`422 validation_failed`, como el conteo acotado del exporte), no un código de timeout.
- **Compatible según §0**: no se agrega, quita ni renombra ningún `code`, y ningún `code` cambia de HTTP. La situación no tenía respuesta documentada (caía en el genérico `500 internal`), y ningún código de `apps/` la emite todavía: `WriteDBError` aún no traduce `ErrStatementTimeout`. Se aplica en #165 (`WriteDBError`, su línea de log y su contador, con la fase `handler` por defecto) y en #174 (las fases `session_lookup` y `entitlements`).

### Plazo del handler: `503 overloaded` con cuerpo RFC 9457 (architect, 2026-11-18)

Cuando un handler supera su plazo de 10 s (§7), la API responde **`503 overloaded` con `Retry-After: 1`** y el cuerpo RFC 9457 de cualquier `overloaded`. El plazo lo aplica un middleware del kernel (`httpx`) con la semántica de `http.TimeoutHandler`, no `http.TimeoutHandler` mismo: ese responde su propio `503` con un cuerpo HTML fijo, sin `Content-Type: application/problem+json`, sin `code`, sin `requestId` y sin `Retry-After`.

- **Qué hace el middleware.** Pasa al handler un `context` con el plazo; la respuesta del handler se guarda en un buffer y solo se envía si termina a tiempo. Al vencer, cancela el `context` (pgx cancela la sentencia en curso e `InTenantTx` hace `ROLLBACK` y suelta la conexión), **descarta** lo que el handler ya hubiera escrito en el buffer, incluidas sus cabeceras, y envía el problema. Lo que el handler escriba después recibe `http.ErrHandlerTimeout` y no llega al cliente. No hay respuesta mezclada: el cliente recibe la del handler completa o el `503`, nunca las dos. Un `panic` del handler sigue hasta el middleware de recuperación (`500 internal`).
- **Rutas de streaming fuera del middleware.** Las descargas de artefactos y los exportes no pasan por él (necesitan enviar antes de terminar y no deben guardar el archivo en memoria) y tienen su presupuesto de 25 s desde la llegada (§7). Si vence antes del primer byte, responden un problema RFC 9457; si vence a mitad del streaming, la conexión se corta (ADR 0003).
- **Por qué `overloaded`.** El plazo es un presupuesto de la propia API y la causa habitual es la saturación: una base lenta en varias sentencias que no llegan cada una a su `statement_timeout`, CPU o un lock. La acción del cliente es la misma que con cualquier backpressure: reintentar más tarde.
- **Opciones descartadas.** `dependency_unavailable` afirmaría que una dependencia no respondió, cuando la causa puede ser la CPU o un lock propio (el `statement_timeout` sí es un veredicto de la base, por eso tiene ese código). `500 internal` quita `Retry-After` y no es un error imprevisto. `504` es el código del proxy (§7) y exigiría un `code` nuevo. Un `code` nuevo (`handler_timeout`) revela un detalle interno sin cambiar la acción del cliente. El `503` en texto de `http.TimeoutHandler` rompe la regla de que todo error es RFC 9457.
- **Lo que distingue el caso va al log.** Una línea `warn` con `requestId`, el patrón de la ruta, el plazo y el tiempo transcurrido, y un contador por ruta: es la señal para separarlo de un pool lleno.
- **Qué lo haría cambiar.** Si la medición muestra que los plazos vencidos se deben sobre todo a una dependencia concreta, esa llamada debe tener su propio timeout más corto que el del handler y responder su código; no se cambia el del plazo.
- **Compatible según §0**: ningún `code` nuevo ni cambio de HTTP, y ningún código de `apps/` aplicaba todavía el plazo. Se aplica en #165.

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

- Rate limit por sesión y por tenant en la aplicación (token bucket; en memoria mientras haya una réplica, en Postgres/Redis al escalar, ver [ADR 0007](../adr/0007-deployment-railway.md)), y por IP en el borde. Se aplica **después** del paso de sesión (la clave sale de la sesión) y antes de los entitlements. Una petición rechazada no consume fichas de ningún bucket. Al exceder, `429 rate_limited` con `Retry-After` igual a los segundos hasta la próxima ficha del bucket que rechazó (redondeo hacia arriba, mínimo 1). Cabeceras de cuota en §3.
- Endpoints con cuota más estricta: `auth/login` (5/min por IP+email), `auth/password/forgot` (3/h por email), descargas de artefactos (2 concurrentes por tenant y 16 por réplica de `api`) y exportes CSV (2 concurrentes por tenant y 4 por réplica), en cupos separados (ADR 0003), búsquedas `q=` (10/s por sesión).
- Cuerpo máximo 1 MB. Los adjuntos irán por URL prefirmada directa al bucket cuando exista su adaptador (`BlobStore`); en R1 no hay bucket (data-model §5).
- Timeouts (el de quien llama siempre mayor que el del llamado): Postgres `statement_timeout` 5 s para el rol de la app (si vence, `503 dependency_unavailable` con `Retry-After: 1`, §4) → handler de la API 10 s (middleware de plazo del kernel con la semántica de `http.TimeoutHandler`; si vence, `503 overloaded` RFC 9457 con `Retry-After: 1`, §4; lectura de cabeceras 5 s) → BFF 12 s hacia la API → proxy/borde 30 s. Excepción: las descargas de artefactos y los exportes van por streaming **fuera** de ese middleware, con un presupuesto propio contado **desde que llega la petición**. Descargas: 3 s para conectar con Khipu y 8 s para sus encabezados (si no, `503 dependency_unavailable` RFC 9457). Exportes: cursor de servidor con `SET LOCAL statement_timeout = '20s'` por lote (`FETCH`) en una transacción de solo lectura; el exporte completo lo acota el `context` de 25 s. En ambos casos, 25 s en total → BFF 27 s para esas rutas → borde 30 s (ADR 0003, «Artefactos: Khipu los conserva»). `IdleTimeout` del servidor Go (120 s) mayor que el keep-alive del proxy de Railway (supuesto 60 s; medir).

## 8. Decisiones del dueño

Decididas y ya reflejadas en este documento:

- **Paginación por cursor** (2026-11-18), §5. Seguimiento de frontend: #178.
- **Suscripción inactiva = bloqueo total salvo exportar** (2026-11-18), §4. Aplicación: #175.
- **Membresía perdida ⇒ `/seleccionar-empresa`, no `/login`** (2026-11-18), §4 (`tenant_access_denied`) y [ADR 0011](../adr/0011-membership-loss-keeps-user-session.md). Aplicación: #174 (API) y #27 (BFF y `/seleccionar-empresa`). No hay `code` nuevo, pero el cambio **no es compatible** según §0: la membresía perdida pasa de `401` a `403`, y «sin fila» pasa de `unauthenticated` a `session_expired`. Se acepta en `/v1` como excepción registrada en §0, porque no hay consumidores.
- Ya tomadas antes: PEN y USD desde R1, dinero como string decimal, errores RFC 9457, tenant desde la sesión (sin cabecera).

**Pendientes de decisión.** No bloquean el congelamiento: el contrato ya fija la forma del dato, y la decisión solo elige de dónde sale.

| Tema | Qué falta decidir | Historias afectadas | Qué ya está fijo |
|---|---|---|---|
| Fuente oficial del tipo de cambio | de dónde lo descarga el job diario (SBS, SUNAT u otra) y a qué hora | #170 (la tabla `ops.exchange_rates` y `fx.Rate` no dependen de la fuente) y el job de ingesta, que #170 deja fuera de alcance y aún no tiene issue | `exchangeRate` string con 3 decimales (§6), `source` `official`/`manual`/`fallback`, un solo redondeo al ingerir y `422 exchange_rate_required` si no hay ninguno ([data-model §2](../data/data-model.md)) |
| Fuente del DNI | qué servicio responde `GET /v1/lookups/dni/{dni}` (vía Khipu u otro proveedor) | #34 (clientes) y el alta rápida del POS (#48) | la ruta, su permiso, el timeout de 3 s y que, si la consulta falla, el cliente se ingresa a mano marcado «sin validar» ([`resources.md`](resources.md), clientes) |

