# ADR 0003: Facturación electrónica vía Khipu detrás del puerto `InvoicingProvider`

- Estado: propuesto. Contrastado con la documentación pública de Khipu, consultada el 2026-11-17. Lo que la documentación no dice queda marcado **SUPUESTO** o como **verificación en BETA**.
- Fecha: 2026-11-17 (revisado tras la revisión del PR #124: carrera entre la respuesta y el webhook, estados y códigos completos, orden por serie, redondeo)

> **Nombres.** En este ADR, "Khipu" es la **API del proveedor de facturación electrónica**. Su portal web (repo `bacsystem/khipu`, `portal/`) está construido sobre el mismo sistema de diseño que adopta el frontend de Wanka: el paquete `khipu-design-system` (ver `c4/04-components-web.mmd`). Son la misma familia de productos, pero en estos documentos "Khipu" se refiere a la API y "khipu-design-system" al paquete de UI.

Fuentes (citadas abajo como [ref], [guía], [errores], [entornos], [planes]):
- [ref] Referencia de la API, OpenAPI 3.0.1 publicado en `https://api-khipu.bacsystemsolutions.com/openapi.json`. Se citan las secciones *Autenticación*, *Sobre de respuesta*, *Estados del comprobante* y *Plazos SUNAT* de `info.description`, y cada operación por su ruta. Página: <https://khipu.bacsystemsolutions.com/developers>
- [guía] <https://khipu.bacsystemsolutions.com/developers/guia>
- [errores] <https://khipu.bacsystemsolutions.com/developers/errores>
- [entornos] <https://khipu.bacsystemsolutions.com/developers/entornos>
- [planes] Página pública de planes, <https://khipu.bacsystemsolutions.com/#precios>, consultada el 2026-11-17. Se citan solo los plazos de conservación, no los precios.
- [código] Código de Khipu (repo público bacsystem/khipu @53bd4bc): seed de planes `V37__planes_y_suscripciones.sql`, `ConsultarComprobanteService`, `FacturaController` (`GET /v1/facturas/descarga`), `ApiKeyFilter` y la política de privacidad del portal. Se citan solo plazos y comportamiento, no precios.

## Contexto

Wanka emite CPE ante SUNAT: factura (01), boleta (03), nota de crédito (07), nota de débito (08), baja de comprobantes y guía de remisión remitente (09). El frontend ya modela los estados (`SunatStatus` en `apps/web/src/types/domain.ts`) y muestra una "cola SUNAT" en el dashboard.

El usuario decidió que **el POS no espera la aceptación**: el ticket se imprime con el CPE "pendiente", y la emisión se encola, se reintenta y genera una alerta si el backlog envejece.

## Lo que Khipu ofrece (hechos)

| Tema | Hecho | Fuente |
|---|---|---|
| Autenticación | `X-Api-Key: fk_…`. **Una llave pertenece a una única empresa (RUC)** | [ref] *Autenticación* |
| Operaciones solo del portal | El alta de empresa, de API key y de webhook (`POST /v1/empresas`, `POST /v1/empresa/api-keys`, `POST /v1/webhooks`) exige sesión del portal. Con una API key responden `403` (`REQUIERE_SESION`) | [ref]; [guía] *Webhooks* |
| URL | `https://api-khipu.bacsystemsolutions.com`, la misma en BETA y PRODUCCION. El entorno es una propiedad de la empresa | [guía]; [entornos] |
| Firma | **Khipu firma el XML** con el `.p12/.pfx` de la empresa (`POST /v1/empresa/certificado`) | [guía] *Firma y certificados* |
| Factura y boleta | `POST /v1/facturas` valida, numera, firma y envía en la misma llamada. `precio_unitario` incluye IGV y es mayor que 0; Khipu calcula el IGV y los totales con tolerancia ±1 | [ref] `POST /v1/facturas` |
| Cálculo sin emitir | `POST /v1/facturas/calcular`: mismo cuerpo y mismas reglas, sin numerar ni consumir cupo | [ref] |
| Numeración propia | `correlativo` opcional (int64 > 0) en facturas y notas. Si ya existe: `409 DUPLICADO`. Series con `correlativo_inicial`. Un número rechazado no se reasigna. **"Las emisiones sobre una misma serie se procesan en orden"** | [ref]; [entornos] |
| Tipo de cambio | `tipo_cambio` de hasta 3 enteros y 3 decimales, solo fuera de PEN (en PEN: `422 TIPO_CAMBIO_INVALIDO`) | [ref] `FacturaRequest.tipo_cambio` |
| Notas | `POST /v1/notas` sobre un comprobante aceptado. La serie lleva la letra del documento de origen (`FC`/`FD` o `BC`/`BD`). Sobre boleta no se admiten NC 04, 05, 08 ni 13 | [ref] |
| Baja | `POST /v1/facturas/{id}/baja`: Khipu arma la RA o el RC. Solo comprobantes aceptados, hasta el día 7 en hora de Lima. **Sin idempotencia**; una baja en curso bloquea otra. Estados de la baja: `GENERADA → ENVIADA → ACEPTADA / RECHAZADA`, más `ERROR_ENVIO` | [ref] `BajaResponse.estado` |
| Resumen diario | Lo gestiona Khipu. Una boleta que pasó el envío individual se informa en un resumen (`baja.condicion = ALTA`) | [ref] |
| GRE | `POST /v1/guias-remision` con `Idempotency-Key`. Khipu numera. Estados `FIRMADO`, `ERROR_ENVIO`, `ENVIANDO`, `ENVIADO_PENDIENTE_TICKET`, `ACEPTADO`, `ACEPTADO_CON_OBS`, `RECHAZADO`. **Experimental** y sin envío en BETA | [ref] `GuiaRemisionResponse.estado`; [guía] |
| Respuesta | `201` con `estado_documento`, `hash`, `fecha_limite_envio`, `cdr`, `totales` y `enlaces` (PDF con QR desde `FIRMADO`) | [ref] `ComprobanteResponse` |
| Conservación de artefactos | XML y CDR: Gratis 1 año; Emprende, Negocio y Pro 5 años (el seed de planes dice que «heredan la de Emprende»). El plazo solo se guarda y se muestra: **no hay purga automática** de XML, CDR ni PDF. El PDF se genera la primera vez que se pide y se guarda junto al XML; si cambia el diseño, se regenera con otra clave | [planes]; [código] |
| Salida en bloque | `GET /v1/facturas/descarga`: ZIP por streaming con XML, PDF y CDR, filtrado por fecha, estado o serie, con un tope de 500 por petición (`422 DESCARGA_EXCEDE_EL_TOPE`). Dar de baja la cuenta no borra nada; la suspensión bloquea toda llamada con la API key (`403 CUENTA_SUSPENDIDA`); el plan vencido solo bloquea la emisión (`402 PLAN_VENCIDO`) | [código] |
| Estados del comprobante | Enum de `GET /v1/facturas?estado`: `RECIBIDO`, `INVALIDO`, `FIRMADO`, `ERROR_ENVIO`, `PENDIENTE_AGRUPACION`, `ENVIADO`, `ACEPTADO`, `ACEPTADO_CON_OBS`, `RECHAZADO`, `ANULADO`, `FUERA_DE_PLAZO`, `DESCARTADO` | [ref] `GET /v1/facturas` |
| Reintentos hacia SUNAT | Los hace Khipu: hasta 20 en ~74 h. Forzado con `POST /v1/facturas/{id}/enviar` (`409 ENVIO_EN_CURSO` si hay un envío en vuelo) | [entornos]; [ref] |
| Idempotencia | `Idempotency-Key` en facturas, notas y guías. Vigencia 24 h. Mismo cuerpo: `200`. Otro cuerpo: `422 IDEMPOTENCIA_INVALIDA` | [errores] |
| Búsqueda por número | `GET /v1/facturas?serie=&numero=` | [ref] |
| Webhooks | Solo estados finales: `comprobante.aceptado`, `.aceptado_con_obs`, `.rechazado`, `.error_envio`, `.fuera_de_plazo`, `.anulado`, `.descartado`. Firma `X-Khipu-Signature: t=..,v1=..` (HMAC-SHA256 de `t.cuerpo`). Deduplicación por `X-Khipu-Delivery`. **Sin orden garantizado.** 8 intentos en ~22 h; pausa tras 10 fallos | [guía] *Webhooks* |
| Límites | Sin tope por segundo. **Cuota mensual del plan sumada entre todas las empresas de la cuenta** (`429 LIMITE_PLAN`, `402 PLAN_VENCIDO`). **BETA: 1 000 documentos por RUC al mes, también sumados entre las empresas en BETA de la cuenta** (`429 LIMITE_PRUEBAS`). Consulta RUC: 60/min y 5 000/día por empresa | [entornos] |
| Errores | Sobre `{estado, datos, codigo, mensaje, errores}`; `trace_id` en los 500; un 502 puede no ser JSON | [ref]; [errores] |

## Supuestos y verificaciones en BETA (antes del issue #56)

1. **Redondeo.** El algoritmo de totales de Khipu no está documentado. **Verificación en BETA:** el set de carritos de `data/data-model.md` §2, enviado a `POST /v1/facturas/calcular`, debe dar totales idénticos a `pricing.Compute`.
2. **Orden y huecos por serie.** No se sabe si Khipu acepta un `correlativo` N+1 cuando N todavía no llegó (por ejemplo, N bloqueado por un `422`). **Verificación en BETA:** enviar N+1 antes que N, y N después. El diseño de abajo funciona en ambos casos.
3. **`PENDIENTE_AGRUPACION`.** Está en el enum, pero sin descripción. Supuesto: la boleta espera su resumen diario. Se trata como "en curso".
4. Timeouts y latencia típica; si se envía `Retry-After`.
5. El modelo de cuentas: una cuenta para todos los tenants (comparten la cuota mensual y la de BETA) o una por tenant. **Pregunta al usuario.**
6. Cómo automatizar el alta en Khipu, que hoy solo se hace desde el portal. **Pregunta al usuario.**
7. Disponibilidad de la GRE en el release 1. **Pregunta al usuario.**
8. Si los webhooks de GRE y de bajas usan los mismos eventos `comprobante.*`.
9. Conservación por plan. **Resuelta** ([código]): Emprende, Negocio y Pro conservan 5 años, y no hay purga automática. **Queda abierto**, como pregunta a Khipu, el compromiso contractual de no introducir una purga mientras Wanka use el servicio.
10. Conservación del PDF. **Resuelta** ([código]): Khipu lo genera bajo demanda y lo guarda junto al XML. Sus bytes cambian si cambia el diseño.
11. Exportación en bloque. **Resuelta** ([código]): ZIP por rango de fechas, con un tope de 500 por petición. La baja de la cuenta no borra nada; la suspensión corta las descargas.

## Opciones

1. **Llamada síncrona en la venta**: una caída de Khipu o de SUNAT detiene la caja, y además contradice la decisión del usuario.
2. **Asíncrona con outbox y máquina de estados**, detrás de un puerto.
3. **Firmar y enviar desde Wanka**: descartada, porque Khipu ya firma y reintenta.

## Decisión

Opción 2.

### Puerto

```go
type InvoicingProvider interface {
    Emit(ctx context.Context, tenant TenantCreds, doc Document, idemKey string) (EmitResult, error)                // POST /v1/facturas, /v1/notas
    EmitDispatchGuide(ctx context.Context, tenant TenantCreds, g DispatchGuide, idemKey string) (EmitResult, error) // POST /v1/guias-remision
    RequestVoid(ctx context.Context, tenant TenantCreds, ref ProviderRef, reason string) (VoidResult, error)        // POST /v1/facturas/{id}/baja
    ListVoids(ctx context.Context, tenant TenantCreds, ref ProviderRef) ([]VoidResult, error)                      // GET /v1/facturas/{id}/bajas
    GetVoid(ctx context.Context, tenant TenantCreds, voidRef string) (VoidResult, error)                           // GET /v1/bajas/{id}
    GetDocument(ctx context.Context, tenant TenantCreds, ref ProviderRef) (StatusResult, error)                    // GET /v1/facturas/{id}, /v1/guias-remision/{id}
    FindByNumber(ctx context.Context, tenant TenantCreds, series string, number int64) (StatusResult, bool, error)  // GET /v1/facturas?serie=&numero=
    FetchArtifact(ctx context.Context, tenant TenantCreds, ref ProviderRef, kind ArtifactKind) (io.ReadCloser, error)
    VerifyWebhook(secret []byte, header http.Header, rawBody []byte, now time.Time) (WebhookEvent, error)
}
```

### Credenciales por tenant

La API key `fk_…` se guarda cifrada con AES-GCM (`INTEGRATIONS_ENC_KEY`) en `integ.provider_credentials`, con RLS. Solo la leen `api` y `worker`. El `whsec_` va en `ops.webhook_routes`, cifrado con una clave aparte, `WEBHOOK_SECRET_ENC_KEY`. El servicio público `webhooks` solo accede a él mediante `ops.resolve_webhook_route()`: no puede leer las API keys ni recibe `INTEGRATIONS_ENC_KEY` (ADR 0001). El certificado y la clave SOL se cargan en Khipu.

### Numeración y orden por serie

- Wanka asigna el número en la transacción de la venta (ADR 0008) y lo envía en `correlativo`. Las series se registran en Khipu con `correlativo_inicial` y son exclusivas de Wanka. Las GRE las numera Khipu.
- **Un emisor por serie, en orden.** Khipu procesa en orden lo que recibe de una serie, pero si Wanka enviara en paralelo desde 20 workers, el orden de llegada no sería el de los números.
  - **Un arrendamiento por serie, en una fila, sin transacción abierta y con fencing.** La tabla `einv.series_emitters (tenant_id, series_id, holder, lease_until, wakeup_at)` se toma, confirmando enseguida, con un upsert. Así una serie nueva, que todavía no tiene fila, también se emite:
    ```sql
    INSERT INTO einv.series_emitters AS e (tenant_id, series_id, holder, lease_until)
    VALUES ($t, $s, $job, now() + interval '60 seconds')
    ON CONFLICT (tenant_id, series_id) DO UPDATE
       SET holder = EXCLUDED.holder, lease_until = EXCLUDED.lease_until
     WHERE e.lease_until < now() OR e.holder = EXCLUDED.holder
    RETURNING holder;   -- 0 filas = otro drenador lo tiene
    ```
    Además, la fila se crea en la misma transacción que crea la serie (`empresas`, al registrar la serie y al aprovisionar), así que el upsert es solo la red de seguridad. Prueba: crear una serie nueva, vender y verificar que el CPE se emite. **Toda renovación es `UPDATE ... SET lease_until = now() + 60 s WHERE series_id = $s AND holder = $job AND lease_until > now()`**: si afecta 0 filas, el arrendamiento se perdió (venció o lo tomó otro) y el drenaje **se aborta** sin emitir nada más. El arrendamiento se renueva **antes** de cada llamada a Khipu, así que cubre la llamada (60 s > 30 s de timeout). Nunca se usa `pg_*_xact_lock`, que obligaría a retener una transacción y su conexión durante las llamadas HTTP.
  - **Por documento, tres pasos y ninguna red dentro de una transacción.**
    1. Una transacción corta renueva el arrendamiento (con fencing) y lee la **cabeza** de la serie: el pendiente no bloqueado de menor número. Luego hace `COMMIT`.
    2. Llamada HTTP a Khipu, fuera de toda transacción (hasta 30 s).
    3. Otra transacción corta graba los datos del proveedor y la transición. Luego hace `COMMIT`. Grabar los datos del proveedor es idempotente y correcto aunque el arrendamiento se haya perdido entre medio. Lo que el fencing impide es **seguir** emitiendo.

    El XML, el CDR y el PDF no se descargan, ni en el bucle ni después: los conserva Khipu (ver «Artefactos: Khipu los conserva»).
  - **Backoff sin bucle caliente y sin adelantar números.** Si la cabeza tiene `next_attempt_at > now()` (un error transitorio anterior), el drenador **se detiene**: no emite N+1, porque eso rompería el orden. Suelta el arrendamiento, fija `wakeup_at = next_attempt_at` en `series_emitters` y devuelve `JobSnooze(next_attempt_at - now())`. Con Khipu caído, cada serie hace un solo intento por escalón de backoff: no hay amplificación. La política de hueco solo aplica a documentos **bloqueados** (`blocked_reason`), no a los que están en backoff.
  - **Sin avisos perdidos, sin unicidad en River.** River exige que `UniqueOpts.ByState` incluya `running` (`riverqueue/river`, `insert_opts.go`: "The following states must be included in ByState if set: … Running …"; `validate()` devuelve `UniqueOpts.ByState must contain all required states`). Con unicidad, una venta que llega durante un drenaje, o el reencolado del propio drenador, se deduplicaría contra el job en `running`, y el aviso se perdería. Por eso **`cpe.emit` no usa unicidad**: la exclusión la da el arrendamiento.
    - La venta encola `cpe.emit {series_id}` en su transacción, siempre.
    - Un job que **no** consigue el arrendamiento termina enseguida (`nil`): el drenador vigente lo cubre por la regla siguiente.
    - Un job que la consigue y encuentra la cabeza en backoff mira `wakeup_at`. Si ya hay otro job dormido hasta ese momento (`wakeup_at <= next_attempt_at`), termina. Si no, duerme él. Así no se acumulan jobs dormidos.
    - **Regla del drenador:** al no encontrar cabeza, suelta el arrendamiento (`COMMIT`) y **vuelve a leer sin arrendamiento**. Si hay pendientes, intenta retomarlo y sigue drenando; si otro lo tomó, termina. Solo sale cuando una relectura hecha *después* de soltarlo no encuentra nada.
    - Por qué no se pierde ningún aviso. Para cada venta nueva hay dos casos. (a) Su job intentó el arrendamiento antes de que se soltara. Entonces su documento ya estaba confirmado, porque el job corre después del `COMMIT` de la venta, y la relectura posterior a soltar lo ve. (b) Lo intentó después. Entonces lo encontró libre y lo tomó.
    - Si el drenador cae, el arrendamiento vence a los 60 s y lo toma el siguiente job, o la conciliación.
    - Costo: con muchas ventas en una serie durante un drenaje hay jobs duplicados, pero terminan en una sola sentencia.
  - Distintas series se emiten en paralelo (concurrencia de la cola `cpe`, ADR 0006), y una serie por caja reparte la carga.
  - **Pruebas:**
    - carga con 20 series y un doble de Khipu a 3 s, vigilando la espera y las conexiones en uso de `pgxpool` (ninguna retenida durante el HTTP);
    - el mismo doble respondiendo **503 durante 1 minuto**: se cuentan las llamadas por serie (una por escalón de backoff) y se verifica que llegan en orden de número;
    - un test que **venza el arrendamiento a mitad del drenaje** y compruebe que el drenador viejo aborta en la siguiente renovación y que nunca hay dos `holder` emitiendo;
    - un N+1 encolado justo cuando el drenaje termina, **contra River real**, que debe salir en menos de 5 s.
- **Política de hueco.** Un documento bloqueado por `422` se aparta (`blocked_reason = validation`) y el drenaje sigue con N+1.
  - Si la verificación en BETA del punto 2 muestra que Khipu **rechaza** un N+1 cuando N falta, la política cambia a "detener la serie": la serie queda en pausa, con alerta inmediata y una acción "corregir" en la UI.
  - En ese caso las demás cajas siguen con sus series, y la caja afectada puede pasar temporalmente a una serie de contingencia configurada.

### Artefactos: Khipu los conserva

Decisión del dueño (2026-11-17): Khipu almacena y sirve el PDF, el XML y el CDR de los CPE durante 5 años, con el plan Emprende o superior. Wanka no los guarda, y en R1 no hay bucket.

- **Requisito de plan.** Según [planes] y [código], Gratis conserva 1 año y Emprende, Negocio y Pro, 5 años. Las cuentas de Khipu que use Wanka tienen que estar en Emprende o en un plan superior. Cómo se cumple depende del modelo de cuentas (pregunta 5):
  - con una cuenta compartida, esa cuenta está en Emprende o superior;
  - con una cuenta por tenant, el alta (#31) y la conexión con Khipu (#55) exigen ese plan y lo registran. Guardar una copia propia para los tenants en Gratis necesitaría el adaptador de bucket del `BlobStore`, que no existe en R1, así que no se ofrece.

  Hoy Khipu no borra artefactos de ningún plan: no hay purga automática, y su política de privacidad dice que los conserva mientras la cuenta exista y la normativa lo exija ([código]). Que siga así es un compromiso contractual, no técnico: ver la pregunta 9.
- **Qué guarda Wanka.** Por documento guarda `provider_ref`, `hash` (el resumen del XML firmado que devuelve Khipu) y `xml_sha256`.
  - El `xml_sha256` sirve porque Khipu escribe el XML firmado una sola vez ([código]).
  - Se fija en la primera descarga con `UPDATE einv.electronic_documents SET xml_sha256 = $1 WHERE tenant_id = $t AND id = $id AND xml_sha256 IS NULL RETURNING xml_sha256`.
    - Si la sentencia afecta 0 filas, se lee el valor guardado.
    - El archivo se sirve solo si coincide con ese valor. Si difiere, no se sirve y se genera una alerta crítica.
    - Así, dos primeras descargas concurrentes con contenidos distintos no pasan desapercibidas: la segunda choca con la huella de la primera.
  - Un documento que nunca se descargó no tiene `xml_sha256`; lo firmado sigue respaldado por `hash`.
  - El CDR y el PDF se sirven sin huella. El PDF lo genera Khipu y sus bytes cambian si cambia el diseño, así que Wanka nunca le calcula un hash.
- **Descarga bajo demanda.** Reimprimir, reenviar y descargar (#52) piden el artefacto a Khipu por `provider_ref` (el endpoint exacto se confirma en [ref]). El `api` hace de proxy por **streaming**, sin guardar el archivo ni cargarlo entero en memoria.
  - **Fuera de `http.TimeoutHandler`.** La ruta `GET /v1/einvoicing/documents/{id}/files/{xml|cdr|pdf}` no pasa por ese handler, que guardaría la respuesta entera en memoria, y tiene su propio presupuesto, declarado en `api/conventions.md`: 3 s de conexión a Khipu y 8 s hasta los encabezados de la respuesta. Si no llegan a tiempo, la API responde `503 dependency_unavailable` (RFC 9457) con `Retry-After`. Una vez empezado el streaming, la transferencia completa tiene un tope de 25 s, por debajo de los 27 s del BFF y los 30 s del borde.
  - **Sin reintentos en el adaptador** (ADR 0006: reintentos en una sola capa). La petición interactiva falla rápido con el `503`. Para enviar el comprobante por correo o WhatsApp (#81), el job de notificaciones pide el archivo a Khipu al enviarlo, y los reintentos son los de River.
  - **Concurrencia.** Hasta 2 descargas simultáneas por tenant, el mismo cupo de «exportes y PDF» de `api/conventions.md`, y un tope global de 16 por réplica de `api`. Las descargas usan un límite propio, separado de la cola de emisión. Al llenarse, la API responde `429 rate_limited` (por tenant) o `503 overloaded` (global), con `Retry-After`. El 16 sale de la ley de Little con unos 2 descargas/s por réplica y hasta 8 s de espera; se ajusta con la prueba de carga de #52.
  - El ticket del POS no depende de Khipu: se imprime con los datos de Wanka.
- **PDF.** Lo genera Khipu bajo demanda la primera vez que se pide (desde `FIRMADO`) y lo guarda junto al XML. Si cambia el diseño, lo regenera con otra clave ([código]). Wanka no genera el PDF.
- **Salida al terminar el servicio.** `GET /v1/facturas/descarga` devuelve un ZIP por streaming con XML, PDF y CDR, filtrado por rango de fechas, con un tope de 500 comprobantes por petición (por eso se descarga por tramos). Dar de baja la cuenta no borra nada ni corta el acceso; **suspenderla sí corta todas las llamadas, también las descargas** (`403 CUENTA_SUSPENDIDA`), mientras que un plan vencido solo bloquea la emisión ([código]). La herramienta de exportación por tramos no entra en R1.
- **Exportes.** Libros electrónicos, reportes y exportes se generan al momento, por streaming y sin guardarse, con el mismo cupo y presupuesto que las descargas. El rango se valida antes de generar (`422 validation_failed` si excede el máximo).
- **`BlobStore`.** El puerto vive en el kernel, sin adaptador de bucket en R1 (solo un doble en memoria para las pruebas). El adaptador compatible con S3 (bucket de Railway o S3) entra más adelante, según el crecimiento, sin tocar los módulos.

### Estados: correspondencia completa con Khipu

| Wanka (`status`, `status_rank`) | Estados de Khipu (`provider_status`) |
|---|---|
| `pendiente` (1) | todavía no aceptado por Khipu: en cola, reintentando el salto Wanka→Khipu, o bloqueado (`blocked_reason`) |
| `enviado` (2) | `RECIBIDO`, `FIRMADO`, `ENVIADO`, `ERROR_ENVIO`, `PENDIENTE_AGRUPACION`, `ENVIANDO`, `ENVIADO_PENDIENTE_TICKET` |
| `aceptado` (3) | `ACEPTADO` |
| `observado` (3) | `ACEPTADO_CON_OBS` |
| `rechazado` (3) | `RECHAZADO`, `INVALIDO`, `FUERA_DE_PLAZO`, `DESCARTADO` (los tres últimos generan alerta a soporte) |
| `anulado` (4) | `ANULADO` |

Las transiciones solo suben de rango: `UPDATE ... SET status = $s, status_rank = $r WHERE id = $id AND status_rank < $r`. La excepción es `aceptado → observado`, que no ocurre porque ambos son finales. Un evento tardío o repetido no hace nada.

### Códigos de error de Khipu → comportamiento de Wanka

| Clase | Códigos ([errores]) | Qué hace Wanka |
|---|---|---|
| Transitorio | red, timeout, `500 INTERNO`, `502`/`503` (incluso sin JSON), `503 SUNAT_NO_DISPONIBLE`, `409 ENVIO_EN_CURSO` | reintento del salto Wanka→Khipu con backoff y jitter, como máximo 10 en ~20 h (dentro de las 24 h de la clave) |
| Ya existe | `409 DUPLICADO`; o reintento pasadas las 24 h | `FindByNumber` y adopción si coinciden total y receptor; si no, alerta crítica |
| Corregir | `422` de validación y de reglas (`VALIDACION`, `RECEPTOR_INVALIDO`, `FORMA_PAGO_INVALIDA`, `TIPO_CAMBIO_INVALIDO`, `REDONDEO_INVALIDO`, `ITEM_INVALIDO`, `NOTA_INVALIDA`, `DOCUMENTO_RELACIONADO_INVALIDO`, …) | `pendiente` + `blocked_reason = validation`; se reenvía con el mismo número tras corregir |
| Configuración del tenant | `422 SERIE_NO_CONFIGURADA`, `SERIE_INVALIDA`, `CERTIFICADO_NO_CARGADO/VENCIDO/INVALIDO`, `CREDENCIALES_SOL_NO_CARGADAS`, `CREDENCIALES_SOL_FALTAN`, `CREDENCIALES_SOL_RECHAZADAS`, `CREDENCIALES_SUNAT_NO_CARGADAS`, `CREDENCIALES_GRE_RECHAZADAS`, `CREDENCIALES_API_SUNAT_*`, `FIRMA_FALLIDA`, `XSD_INVALIDO` | `blocked_reason = config`; pausa de la emisión del tenant y banner en `/configuracion/sunat` |
| Credencial | `401 NO_AUTORIZADO` | `blocked_reason = auth`; pausa del tenant |
| Operación no permitida | `403 REQUIERE_SESION`, `EMPRESA_AJENA`, `ROL_INSUFICIENTE`, `SOPORTE_SOLO_LECTURA` | es un error de programación o de configuración (se llamó con API key una operación del portal, o la llave es de otra empresa). Sin reintento; alerta a ingeniería |
| Cuota | `429 LIMITE_PLAN`, `429 LIMITE_PRUEBAS` (BETA, compartida entre las empresas de la cuenta), `402 PLAN_VENCIDO` | `blocked_reason = quota`; pausa de la cuenta (o del tenant, si las cuentas son por tenant) y alerta crítica a plataforma. Monitoreo de la cuota al 80 % con `GET /v1/cuenta` (solo portal: la mide plataforma) |
| SUNAT ya lo tiene | `409 SUNAT_YA_LO_TIENE`: el documento queda `ERROR_ENVIO`, fuera de los reintentos automáticos | `enviado` + `blocked_reason = sunat_ya_lo_tiene`; **no se reemite**; tarea para soporte (verificar el CDR en SOL) y `POST /v1/facturas/{id}/cdr/recuperar` manual |
| Estado | `409 ESTADO_NO_ENVIABLE`, `FUERA_DE_PLAZO`, `NO_ACEPTADO`, `422 BAJA_INVALIDA` | sin reintento; se aplica el estado real con `GetDocument` y se informa al usuario |
| No encontrado | `404 NO_ENCONTRADO`, `404 SIN_CDR` (CDR aún no disponible) | `SIN_CDR`: se reintenta la descarga más tarde. `NO_ENCONTRADO`: alerta |
| No disponible en BETA | `422 NO_DISPONIBLE_EN_BETA` (CDR, GRE y consultas que BETA no ofrece) | sin reintento ni alerta en BETA: se registra y se omite el paso (el CDR queda vacío); en PRODUCCION no debería ocurrir y genera alerta |

### Respuesta, webhook y conciliación sin carreras

1. **Datos del proveedor primero, transición después.** Al recibir `201`/`200`, el worker graba en una sentencia idempotente `provider_ref`, `hash`, `send_deadline`, `provider_total` y los enlaces, **sin depender del estado**: `UPDATE ... SET provider_ref = $ref, ... WHERE id = $id AND (provider_ref IS NULL OR provider_ref = $ref)`. Después aplica la transición por rango. Si el webhook llegó antes y ya dejó el documento en `aceptado`, la respuesta igual completa los datos que faltaban.
2. **El webhook resuelve el documento por serie y número**, no por `provider_ref`: la carga trae `datos.serie` y `datos.numero`, y esos valores los asignó Wanka, así que existen desde la venta. El handler del servicio `webhooks` verifica la firma, deduplica, guarda el evento y encola `cpe.sync(tenant, serie, numero)`.
3. **`cpe.sync` consulta el estado actual** (`GetDocument` si ya hay `provider_ref`; si no, `FindByNumber`) y aplica primero los datos y luego la transición. El orden de los eventos no importa.
4. **`cpe.reconcile`** (líder único, cada minuto) recorre **todos los estados no finales**:
   - `pendiente` sin job vivo, o con `next_attempt_at` vencido hace más de 5 min: lo reencola;
   - `enviado` sin novedad en más de 5 min: `GetDocument`, con backoff por documento;
   - bajas `solicitada`, `generada`, `enviada` o `error_envio`: `GetVoid`;
   - documentos con `blocked_reason`: genera una alerta según su antigüedad y su `send_deadline`.
5. Timeouts: conexión 3 s, total 30 s (incluye la ida a SUNAT; **supuesto**, se mide en BETA).

### Bajas

Wanka crea `einv.voidings` (con un índice único parcial: una baja en curso por documento) y el job `cpe.void`.
- Antes de cada intento consulta `ListVoids`, porque la baja no tiene idempotencia; si ya hay una baja en curso, la adopta.
- La API valida antes: el documento está aceptado u observado, y no pasaron más de 7 días.
- **Una boleta ya entregada al cliente no se anula: corresponde una nota de crédito.** La UI lo pregunta.

### GRE

Mismo patrón, con `Idempotency-Key`. El traslado no depende de la aceptación de la GRE para registrarse (ver `sequences/04-transfer-gre.mmd`).

### Ticket del POS

El ticket muestra la serie y el número que asignó Wanka, con el CPE "pendiente". Opción para el PO: esperar hasta 3 s el `hash` (disponible desde `FIRMADO`) para imprimir el ticket completo.

### Plan B por fallo

| Fallo | Sistema | Usuario |
|---|---|---|
| Khipu lento o caído | cola `cpe` con concurrencia fija y backoff; alerta a los 15 min | ticket "pendiente"; panel "cola SUNAT"; píldora SUNAT en advertencia |
| SUNAT caída (`ERROR_ENVIO`) | Khipu reintenta; Wanka concilia | CPE "enviado" |
| Validación (`422`) | `blocked_reason`; la serie sigue o se detiene según la verificación 2 | "corregir y reenviar" |
| Cuota (`LIMITE_PLAN`, `LIMITE_PRUEBAS`) | pausa y alerta crítica | aviso en el dashboard |
| Credenciales o configuración | pausa del tenant | banner en `/configuracion/sunat` |
| Webhooks pausados | la conciliación cubre el hueco; alerta para reactivarlos | nada |
| Khipu lento o caído al descargar un artefacto | `503 dependency_unavailable` con `Retry-After`, sin reintentos; alerta si sube la tasa de error | «documento no disponible por ahora»; el ticket del POS se reimprime igual |
| Cuenta de Khipu suspendida (`403 CUENTA_SUSPENDIDA`) | alerta crítica: se cortan la emisión y las descargas | banner en `/configuracion/sunat`; los documentos ya emitidos no se pueden descargar hasta reactivar la cuenta |

## Consecuencias

- Buenas: la caja no depende ni de Khipu ni de SUNAT; los reintentos son seguros; las carreras entre la respuesta, el webhook y la conciliación se resuelven con escrituras idempotentes y transiciones por rango; el orden por serie está garantizado del lado de Wanka.
- Malas:
  - el ticket "pendiente";
  - las series son exclusivas de Wanka;
  - el alta en Khipu es manual mientras no exista una API de integrador;
  - la cuota es compartida si se usa una sola cuenta;
  - un emisor por serie limita el throughput de una serie a 1 / latencia de Khipu;
  - el XML, el CDR y el PDF dependen de Khipu: que no introduzca una purga (pregunta 9), que esté disponible al descargarlos y que la cuenta no esté suspendida, porque la suspensión corta también las descargas.
- Qué haría cambiar la decisión:
  - una API de integrador de Khipu: se automatiza el alta sin cambiar el puerto;
  - si Khipu acepta correlativos fuera de orden, se puede relajar el emisor único por serie;
  - si Khipu no asume el compromiso de no purgar, o si una suspensión de cuenta resulta un riesgo real: se activa el adaptador de bucket del `BlobStore` y un job que copie los artefactos con `GET /v1/facturas/descarga`.
- **Criterios de aceptación con el sistema externo** (issues de facturación, empezando por #56):
  - emisión de factura y boleta, `422`, nota de crédito, baja y webhook firmado en **BETA de Khipu**;
  - las verificaciones en BETA 1 (redondeo) y 2 (orden y huecos por serie);
  - el CDR y la GRE no existen en BETA ([entornos]), así que su prueba es un criterio aparte.
