# ADR 0003: Facturación electrónica vía Khipu detrás del puerto `InvoicingProvider`

- Estado: propuesto. Contrastado con la documentación pública de Khipu (consultada el 2026-11-17); lo que la documentación no dice queda marcado **SUPUESTO**.
- Fecha: 2026-11-17

> Nombre: "Khipu" aquí es el **proveedor de facturación electrónica**. No tiene relación con el sistema de diseño "khipu" que adopta el frontend.

Fuentes (citadas abajo como [ref], [guía], [errores], [entornos]):
- [ref] Referencia de la API, OpenAPI 3.0.1 publicado en `https://api-khipu.bacsystemsolutions.com/openapi.json` (secciones *Autenticación*, *Sobre de respuesta*, *Estados del comprobante*, *Plazos SUNAT* de `info.description`, y las operaciones citadas por ruta). Página: <https://khipu.bacsystemsolutions.com/developers>
- [guía] <https://khipu.bacsystemsolutions.com/developers/guia> (flujo de integración, tipos de documento, firma, síncrono/asíncrono, idempotencia, webhooks)
- [errores] <https://khipu.bacsystemsolutions.com/developers/errores> (códigos, estados, plazos, guía de reintentos)
- [entornos] <https://khipu.bacsystemsolutions.com/developers/entornos> (BETA/producción, límites, cuotas, reintentos)

## Contexto

Wanka emite CPE ante SUNAT: factura (01), boleta (03), nota de crédito (07), nota de débito (08), baja de comprobantes y guía de remisión remitente (09). El frontend ya modela los estados (`SunatStatus` en `apps/web/src/types/domain.ts`: `aceptado | pendiente | rechazado`; guías con `en_transito | entregado` en `features/dispatch-guides/mocks/guides.ts`) y muestra una "cola SUNAT" en el dashboard (`SunatQueueItem`).

Decisión del usuario: **el POS no espera la aceptación**. El ticket se imprime con el CPE "pendiente" y la emisión se encola y reintenta, con alerta si el backlog envejece.

## Lo que Khipu ofrece (hechos)

| Tema | Hecho | Fuente |
|---|---|---|
| Autenticación | Cabecera `X-Api-Key: fk_…`. **Una llave pertenece a una única empresa (RUC)**, así que el RUC no va en las llamadas | [ref] *Autenticación* |
| Alta de empresa, API keys, webhooks | Solo desde el portal, con sesión JWT (`POST /v1/empresas`, `POST /v1/empresa/api-keys`, `POST /v1/webhooks`: "Requiere sesión del portal"). No se pueden crear con una API key | [ref] operaciones citadas; [guía] *Webhooks* |
| URL | `https://api-khipu.bacsystemsolutions.com`, la misma para BETA y PRODUCCION; el entorno es una propiedad de la empresa que cambia soporte | [guía] paso 6; [entornos] |
| Firma | **Khipu firma el XML** (XML-DSig) con el `.p12/.pfx` de la empresa (`POST /v1/empresa/certificado`) y valida contra los XSD de SUNAT | [guía] *Firma y certificados*; [ref] `/v1/empresa/certificado` |
| Factura y boleta | `POST /v1/facturas` (serie `F###` o `B###`): valida, numera, firma y envía en la misma llamada (salvo `enviar_automatico: false`). `precio_unitario` con IGV incluido; Khipu calcula IGV y totales (tolerancia ±1) | [ref] `POST /v1/facturas` |
| Numeración propia | Campo opcional `correlativo` (int64 > 0) en facturas y notas; si ya existe, `409 DUPLICADO`. Khipu recomienda omitirlo. Series con `correlativo_inicial` (`POST /v1/series`). Un número rechazado nunca se reasigna | [ref] `FacturaRequest.correlativo`, `POST /v1/series`; [errores] *RECHAZADO* |
| Notas | `POST /v1/notas` (07/08) sobre un comprobante **aceptado**; serie con la letra del origen (`FC##`/`FD##` sobre factura, `BC##`/`BD##` sobre boleta). Sobre boleta no se admiten NC 04, 05, 08 ni 13 | [ref] `POST /v1/notas` |
| Baja | `POST /v1/facturas/{id}/baja` con `motivo`. Khipu arma la comunicación de baja (RA) para facturas o el resumen diario (RC) para boletas y sus notas. Solo `ACEPTADO`/`ACEPTADO_CON_OBS` y hasta el día 7 desde la emisión (hora de Lima). Una baja en curso bloquea otra. Consulta: `GET /v1/bajas/{id}` (reconsulta el ticket), `GET /v1/facturas/{id}/bajas` | [ref] `POST /v1/facturas/{id}/baja`, `GET /v1/bajas/{id}` |
| Resumen diario de boletas | Lo gestiona Khipu: la boleta va sola hasta el 5.º día; el 6.º y el 7.º en resumen diario; después, en resumen fuera de plazo | [ref] *Plazos SUNAT*; [guía] *Tipos de documento* |
| GRE | `POST /v1/guias-remision` (09 remitente, 31 transportista). Khipu numera (no hay `correlativo`). Asíncrona con ticket: `ENVIADO_PENDIENTE_TICKET` y Khipu consulta durante ~24 h. **Marcada experimental, "envío apagado en el servidor"**; BETA no envía GRE; requiere credenciales API SUNAT (`PUT /v1/empresa/credenciales-api-sunat`) | [guía] *Tipos de documento*, *Procesamiento*; [entornos] |
| Respuesta síncrona | `201` con `estado_documento`, `hash`, `fecha_limite_envio`, `cdr` (si SUNAT ya respondió) y `enlaces` a `xml`, `pdf` (con QR y hash, desde `FIRMADO`) y `cdr` | [ref] `ComprobanteResponse`; [guía] *Contenido de la respuesta* |
| Estados | `RECIBIDO → FIRMADO → ENVIADO → ACEPTADO / ACEPTADO_CON_OBS / RECHAZADO`; además `ERROR_ENVIO`, `INVALIDO`, `ANULADO`, `FUERA_DE_PLAZO`, `DESCARTADO`; GRE: `ENVIANDO`, `ENVIADO_PENDIENTE_TICKET` | [ref] *Estados del comprobante*; [errores] *Estados* |
| Reintentos hacia SUNAT | **Los hace Khipu**: `ERROR_ENVIO` se reintenta solo, hasta 20 veces (~74 h), con espera creciente; se puede forzar con `POST /v1/facturas/{id}/enviar` (`409 ENVIO_EN_CURSO` si ya hay un envío en vuelo) | [entornos] *Tiempos de espera y reintentos*; [ref] `/enviar` |
| Idempotencia | `Idempotency-Key` (UUID o 8-100 caracteres) en `POST /v1/facturas`, `/v1/notas` y `/v1/guias-remision`; vigencia 24 h por empresa y endpoint; mismo cuerpo → `200` con el comprobante ya emitido; otro cuerpo → `422 IDEMPOTENCIA_INVALIDA`; si la llamada falló, la clave queda libre. **No aplica a la baja** | [errores] *Idempotency Guidance*; [ref] parámetros de cada operación |
| Consulta por número | `GET /v1/facturas?serie=&numero=` | [ref] `GET /v1/facturas` |
| Webhooks | Hasta 5 por empresa; eventos solo de estados finales: `comprobante.aceptado`, `.aceptado_con_obs`, `.rechazado`, `.error_envio`, `.fuera_de_plazo`, `.anulado`, `.descartado`. Cuerpo `{evento, creado_en, datos}`. Cabeceras `X-Khipu-Event`, `X-Khipu-Delivery` (estable entre reintentos) y `X-Khipu-Signature: t=<unix>,v1=<hex>` = HMAC-SHA256 de `t + "." + cuerpo crudo` con el secreto `whsec_…`. Tolerancia sugerida 300 s. Responder 2xx en < 10 s. Al menos una vez; **sin orden garantizado** (consultar `GET /v1/facturas/{id}` para el estado actual). 8 intentos en ~22 h; tras 10 fallos seguidos el endpoint se pausa | [guía] *Webhooks* |
| Límites | Sin tope por segundo en emisión; **las emisiones de una misma serie se procesan en orden**. Cuota mensual del plan **sumando todas las empresas de la cuenta** (`429 LIMITE_PLAN`; `402 PLAN_VENCIDO` tras la gracia). Consulta RUC: 60/min y 5 000/día por empresa. BETA: 1 000 documentos por RUC al mes | [entornos] |
| Otros límites | 999 ítems por comprobante; descripción de ítem de 1 a 500 caracteres; boleta sin documento del comprador hasta S/ 700.00 (en soles); boleta solo al contado (crédito va en factura) | [entornos]; [ref] `POST /v1/facturas` |
| Plazos | Factura y sus notas: 3 días calendario; boleta: 5 días sola, 7 en resumen; baja: 7 días | [ref] *Plazos SUNAT*; [errores] *Deadlines* |
| Errores | Sobre `{estado, datos, codigo, mensaje, errores}`; `codigo` estable (no parsear `mensaje`); `trace_id` en los 500; `422` antes de consumir numeración; puede llegar un 502 no JSON desde un proxy | [ref] *Sobre de respuesta*; [errores]; [guía] *Errores* |
| BETA | Homologación sin validez tributaria; sin CDR (`422 NO_DISPONIBLE_EN_BETA`); sin GRE | [entornos] |

## Supuestos (la documentación no lo dice)

1. Timeouts recomendados del lado cliente y latencia típica de `POST /v1/facturas` (incluye la ida a SUNAT en la misma llamada). Se mide en BETA.
2. Si la API devuelve `Retry-After` en 429/503 (no documentado).
3. Restricciones de `correlativo` más allá de int64 > 0: por ejemplo, si un hueco en la serie bloquea la cola de esa serie, dado que "se procesan en orden".
4. El modelo de cuentas para Wanka: una cuenta Khipu con N empresas (comparten cuota mensual) o una cuenta por tenant. **Pregunta abierta al usuario.**
5. Cómo se automatiza el alta de un tenant en Khipu (empresa, API key, webhook), que hoy solo se hace desde el portal. **Pregunta abierta.**
6. Disponibilidad de GRE en producción para el release 1. **Pregunta abierta.**
7. La carga del webhook de una baja (`comprobante.anulado`) y si trae el `id` de la baja.

## Opciones

1. **Llamada síncrona en la petición de venta**: el cajero espera a Khipu, que a su vez espera a SUNAT en la misma llamada. Una caída detiene la caja, y la llamada de red alarga los locks si se hace dentro de la transacción. Contradice la decisión del usuario.
2. **Asíncrona con outbox y máquina de estados** detrás de un puerto: la venta confirma con el CPE `pendiente` y el número asignado por Wanka; un worker emite.
3. **Firmar y enviar a SUNAT desde Wanka**: descartada; Khipu ya firma y reintenta.

## Decisión

Opción 2.

**Puerto** (en `internal/facturacion/app`), con nombres de Wanka y no de Khipu:
```go
type InvoicingProvider interface {
    Emit(ctx context.Context, tenant TenantCreds, doc Document, idemKey string) (EmitResult, error)        // 01, 03 (POST /v1/facturas), 07, 08 (POST /v1/notas)
    EmitDispatchGuide(ctx context.Context, tenant TenantCreds, g DispatchGuide, idemKey string) (EmitResult, error) // 09 (POST /v1/guias-remision)
    RequestVoid(ctx context.Context, tenant TenantCreds, ref ProviderRef, reason string) (VoidResult, error)        // POST /v1/facturas/{id}/baja
    GetDocument(ctx context.Context, tenant TenantCreds, ref ProviderRef) (StatusResult, error)                     // GET /v1/facturas/{id}, /v1/guias-remision/{id}
    FindByNumber(ctx context.Context, tenant TenantCreds, series string, number int64) (StatusResult, bool, error)  // GET /v1/facturas?serie=&numero=
    GetVoid(ctx context.Context, tenant TenantCreds, voidRef string) (VoidResult, error)                           // GET /v1/bajas/{id}
    FetchArtifact(ctx context.Context, tenant TenantCreds, ref ProviderRef, kind ArtifactKind) (io.ReadCloser, error) // xml, pdf, cdr
    VerifyWebhook(secret []byte, header http.Header, rawBody []byte, now time.Time) (WebhookEvent, error)
}
```
Errores tipados que traduce el adaptador desde `codigo`: `ErrTransient` (red, timeout, 500 `INTERNO`, 502/503 incluso sin JSON, `409 ENVIO_EN_CURSO`), `ErrNeedsCorrection` (422 de validación: no se consumió número en Khipu), `ErrAlreadyExists` (`409 DUPLICADO`), `ErrQuota` (`429 LIMITE_PLAN`, `402 PLAN_VENCIDO`), `ErrAuth` (401), `ErrNotAllowed` (409 `ESTADO_NO_ENVIABLE`, `FUERA_DE_PLAZO`, 422 `BAJA_INVALIDA`). Habrá un adaptador `fake` para pruebas locales.

**Credenciales por tenant.** La API key `fk_…` es de la empresa, así que Wanka guarda **una por tenant**, cifrada con AES-GCM (clave maestra `INTEGRATIONS_ENC_KEY`), en `integ.provider_credentials (tenant_id, provider, api_key_enc, webhook_secret_enc, webhook_route_id, environment)`. No hay una API key global en variables de entorno. El certificado y la clave SOL los sube el tenant **a Khipu**; Wanka no los guarda (de ahí `sunatConfig.certificate` del mock: Wanka solo muestra metadatos).

**Numeración.** Para facturas, boletas y notas, Wanka asigna el número en la transacción de la venta (ADR 0008) y lo envía en `correlativo`, porque el ticket se imprime antes de llamar a Khipu. Cada serie se registra en Khipu con `correlativo_inicial` (al aprovisionar o al crear la serie) y es **exclusiva de Wanka**: si otro sistema emite en ella, aparece `409 DUPLICADO`. Las GRE las numera Khipu, porque el endpoint no admite `correlativo` y la guía no se imprime en el POS. Su número se guarda al responder.

**Máquina de estados** ([`sequences/00-cpe-state-machine.mmd`](../sequences/00-cpe-state-machine.mmd)). El estado propio se deriva del de Khipu, y el estado crudo se guarda en `provider_status`:

| Wanka | Khipu |
|---|---|
| `pendiente` | todavía no aceptado por Khipu (job en cola o reintentando el salto Wanka→Khipu), o `422` a la espera de corrección |
| `enviado` | `RECIBIDO`, `FIRMADO`, `ENVIADO`, `ERROR_ENVIO` (Khipu reintenta), `ENVIANDO`, `ENVIADO_PENDIENTE_TICKET` |
| `aceptado` | `ACEPTADO` |
| `observado` | `ACEPTADO_CON_OBS` |
| `rechazado` | `RECHAZADO`, `INVALIDO`, `FUERA_DE_PLAZO`, `DESCARTADO` (los tres últimos alertan a soporte) |
| `anulado` | `ANULADO` |

Cada transición es `UPDATE ... WHERE id = $id AND status = $anterior` más una fila en `einv.edoc_events`. Una transición que no aplica no hace nada, así un webhook duplicado o tardío es inocuo. NC y ND son documentos propios que referencian al origen.

**Una capa de reintento por salto.** Wanka reintenta **solo el salto Wanka→Khipu**. Khipu reintenta Khipu→SUNAT. Wanka nunca llama a `/enviar` de forma automática: solo la acción manual "reintentar" del usuario o de soporte la usa.
- Job `cpe.emit` en la cola `cpe` (River, ADR 0006), con concurrencia fija (bulkhead). La API key va por petición. `Idempotency-Key = electronic_documents.id`.
- Timeouts: conexión 3 s, total 30 s (incluye la ida a SUNAT; **supuesto**, se ajusta con la medición en BETA).
- `ErrTransient` se reintenta con backoff exponencial y jitter, máximo 10 intentos en unas 20 h, por debajo de la vigencia de 24 h de la clave de idempotencia y de `fecha_limite_envio`. Pasado ese plazo, o ante `ErrAlreadyExists`, el adaptador consulta `FindByNumber` y adopta el documento existente si coincide (mismo total y receptor). Si no coincide, alerta crítica.
- `ErrNeedsCorrection`: el CPE queda `pendiente` con `blocked_reason = validation` y el mensaje de Khipu. El usuario corrige (por ejemplo, el RUC del cliente) y se reenvía **con el mismo número**, que Khipu no llegó a consumir. Para que esto sea raro, la API de Wanka valida antes del commit las mismas reglas: factura con RUC, boleta sin documento hasta S/ 700.00 en soles, boleta al contado, catálogos 03, 06, 07 y 51, 999 ítems como máximo.
- `ErrQuota` pausa la emisión del tenant o de toda la cuenta y alerta a plataforma. No se reintenta en bucle.
- `ErrAuth` pausa los jobs del tenant y muestra un banner en `/configuracion/sunat`.

**Webhooks.** Ruta `/webhooks/khipu/{webhook_route_id}`, un id aleatorio por tenant, porque la carga no trae el RUC y cada empresa tiene su propio secreto. El handler:
1. verifica HMAC-SHA256 sobre los bytes crudos, en tiempo constante y con tolerancia de 300 s;
2. deduplica por `X-Khipu-Delivery` en `einv.provider_webhook_events`;
3. encola `cpe.sync` y responde 2xx en milisegundos.

El job **consulta el estado actual** con `GetDocument` en vez de confiar en el orden de los eventos.

**Conciliación.** El job periódico `cpe.reconcile` (líder único, cada minuto) toma los documentos `enviado` sin novedad en más de 5 min y llama a `GetDocument` con backoff por documento. Cubre los estados intermedios (que no se publican), los webhooks pausados tras 10 fallos y los perdidos. Alerta cuando un CPE se acerca a `fecha_limite_envio`.

**Bajas.** Wanka crea `einv.voidings (document_id, reason, provider_void_id, status)` y el job `cpe.void`. Como la baja **no tiene idempotencia**, antes de cada reintento el job consulta `GET /v1/facturas/{id}/bajas` y, si ya existe una en curso, la adopta. El estado final llega por `GetVoid` o por el webhook `comprobante.anulado`. Reglas que la API de Wanka valida antes: origen `aceptado` u `observado` y 7 días como máximo desde la emisión (hora de Lima). Además, una **boleta ya entregada al cliente no se anula: va nota de crédito**. Khipu no puede validarlo, así que la UI lo pregunta.

**GRE.** Mismo patrón, con `Idempotency-Key`. El estado `ENVIADO_PENDIENTE_TICKET` lo sigue consultando Khipu. Mientras Khipu la marque experimental, el traslado de inventario no depende de la aceptación de la GRE para registrarse (ver `sequences/04-transfer-gre.mmd`).

**Ticket del POS.** El ticket muestra serie y número (los asigna Wanka) con el estado "pendiente". Opción a validar con el PO: como el `201` de Khipu ya trae `hash` y PDF con QR desde `FIRMADO` (no hace falta esperar a SUNAT), el POS puede esperar hasta 3 s a que el worker obtenga el hash e imprimir el ticket completo. Si no llega, imprime "pendiente" y ofrece reimprimir desde el comprobante. Esto no contradice "no esperar la aceptación".

**Plan B por fallo**

| Fallo | Qué hace el sistema | Qué ve el usuario |
|---|---|---|
| Khipu lento | la cola `cpe` crece con concurrencia fija | ticket "pendiente"; panel "cola SUNAT" |
| Khipu caído | reintentos con backoff; alerta a soporte a los 15 min | igual; píldora SUNAT del topbar en advertencia |
| SUNAT caída (`ERROR_ENVIO`) | Khipu reintenta; Wanka concilia | CPE "enviado" |
| Validación (`422`) | `pendiente` + `blocked_reason` | aviso "corregir y reenviar" en el comprobante |
| Cuota (`429 LIMITE_PLAN`) | pausa la emisión y alerta crítica a plataforma | aviso en el dashboard |
| Credenciales (`401`) | pausa los jobs del tenant | banner en `/configuracion/sunat` |
| Webhooks pausados | la conciliación cubre; alerta para reactivarlos | nada |

## Consecuencias

- Buenas: la caja no depende de Khipu ni de SUNAT; reintentos seguros gracias a la idempotencia de Khipu y a `FindByNumber`; un solo responsable de reintento por salto; cambiar o sumar un proveedor es escribir otro adaptador.
- Malas: el cliente puede llevarse un ticket "pendiente"; numerar en Wanka exige series exclusivas y corrección con el mismo número; el alta en Khipu hoy es manual (portal); si la cuota es por cuenta, un tenant puede agotar la de todos.
- Qué haría cambiar la decisión: si Khipu ofreciera una API de integrador para el alta, se automatiza el aprovisionamiento sin cambiar el puerto. Si la latencia medida de `enviar_automatico: false` (solo numerar y firmar) fuera consistentemente < 500 ms, se podría dejar que Khipu numere y obtener el hash en línea con un tiempo de espera corto.
- **Prueba contra el sistema externo** es criterio de aceptación de los issues de facturación: emisión de factura y boleta, rechazo, `422`, nota de crédito, baja y webhook firmado en **BETA de Khipu**, no solo contra el `fake`. El CDR y la GRE no se pueden probar en BETA ([entornos]), así que su prueba es un criterio aparte en producción controlada o queda bloqueada hasta que exista un entorno.
