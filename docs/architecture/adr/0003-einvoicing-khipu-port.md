# ADR 0003: Facturación electrónica vía Khipu detrás del puerto `InvoicingProvider`

- Estado: propuesto. Contrastado con la documentación pública de Khipu (consultada el 2026-11-17) y con su código público (bacsystem/khipu @53bd4bc, [código]). Lo que ninguna de las dos fuentes responde queda marcado **SUPUESTO** o como **verificación en BETA**.
- Fecha: 2026-11-17 (revisado tras la revisión del PR #124: carrera entre la respuesta y el webhook, estados y códigos completos, orden por serie, redondeo)

> **Nombres.** En este ADR, "Khipu" es la **API del proveedor de facturación electrónica**. Su portal web (repo `bacsystem/khipu`, `portal/`) está construido sobre el mismo sistema de diseño que adopta el frontend de Wanka: el paquete `khipu-design-system` (ver `c4/04-components-web.mmd`). Son la misma familia de productos, pero en estos documentos "Khipu" se refiere a la API y "khipu-design-system" al paquete de UI.

Fuentes (citadas abajo como [ref], [guía], [errores], [entornos], [planes]):
- [ref] Referencia de la API, OpenAPI 3.0.1 publicado en `https://api-khipu.bacsystemsolutions.com/openapi.json`. Se citan las secciones *Autenticación*, *Sobre de respuesta*, *Estados del comprobante* y *Plazos SUNAT* de `info.description`, y cada operación por su ruta. Página: <https://khipu.bacsystemsolutions.com/developers>
- [guía] <https://khipu.bacsystemsolutions.com/developers/guia>
- [errores] <https://khipu.bacsystemsolutions.com/developers/errores>
- [entornos] <https://khipu.bacsystemsolutions.com/developers/entornos>
- [planes] Página pública de planes, <https://khipu.bacsystemsolutions.com/#precios>, consultada el 2026-11-17. Se citan solo los plazos de conservación, no los precios.
- [código] Código de Khipu (repo público bacsystem/khipu @53bd4bc). Se citan solo plazos y comportamiento, no precios. Archivos que respaldan cada tema:
  - redondeo: `ItemCalculado.java`, `Totales.java`, `FacturaRequest.java`;
  - numeración y N+1: `EmitirComprobanteService.java`, `JdbcSerieRepository.java`;
  - alta asistida y sesión: `AdminAltaAsistidaController.java`, `EmpresaController.java`, `EmpresasController.java`, `WebhooksController.java`;
  - webhooks y su log: `EventoWebhook.java`, `JdbcComprobanteRepository.java`, `EntregarWebhooksService.java`;
  - GRE: `GuiaRemisionController.java` y `application.yml` (envío apagado);
  - timeouts hacia SUNAT y ausencia de `Retry-After`: `application.yml`, `EmitirComprobanteService.java`;
  - cuotas y límites: `TopeDelPlanService.java`, `Limites.java`, seed `V37__planes_y_suscripciones.sql`;
  - bajas: `BajaController.java`, `DarDeBajaService.java`;
  - artefactos: `ConsultarComprobanteService.java`, `FacturaController.java` (`GET /v1/facturas/descarga`), `ApiKeyFilter.java`, `S3DocumentStorage.java`.
- [docs] Documentos del mismo repo de Khipu (@53bd4bc): la política de privacidad del portal (`portal/src/content/legal/privacidad.ts`). Es un texto legal, no comportamiento del código.

## Contexto

Wanka emite CPE ante SUNAT: factura (01), boleta (03), nota de crédito (07), nota de débito (08), baja de comprobantes y guía de remisión remitente (09). El frontend ya modela los estados (`SunatStatus` en `apps/web/src/types/domain.ts`) y muestra una "cola SUNAT" en el dashboard.

El usuario decidió que **el POS no espera la aceptación**: el ticket se imprime con el CPE "pendiente", y la emisión se encola, se reintenta y genera una alerta si el backlog envejece.

## Lo que Khipu ofrece (hechos)

| Tema | Hecho | Fuente |
|---|---|---|
| Autenticación | `X-Api-Key: fk_…`. **Una llave pertenece a una única empresa (RUC)** | [ref] *Autenticación* |
| Operaciones solo del portal | El alta de empresa, de API key y de webhook (`POST /v1/empresas`, `POST /v1/empresa/api-keys`, `POST /v1/webhooks`) exige sesión del portal. Con una API key responden `403` (`REQUIERE_SESION`). Con la API key sí se cargan el certificado, las credenciales SOL y API SUNAT y los datos fiscales, y se crean series. No hay API de integrador para terceros; el operador de Khipu tiene un **alta asistida**, `POST /v1/admin/cuentas` (`X-Platform-Key`, con `Idempotency-Key` de 1 h), que crea cuenta, empresa, serie y API key | [ref]; [guía] *Webhooks*; [código] |
| URL | `https://api-khipu.bacsystemsolutions.com`, la misma en BETA y PRODUCCION. El entorno es una propiedad de la empresa | [guía]; [entornos] |
| Firma | **Khipu firma el XML** con el `.p12/.pfx` de la empresa (`POST /v1/empresa/certificado`) | [guía] *Firma y certificados* |
| Factura y boleta | `POST /v1/facturas` valida, numera, firma y envía en la misma llamada. `precio_unitario` incluye IGV y es mayor que 0; Khipu calcula el IGV y los totales con tolerancia ±1 | [ref] `POST /v1/facturas` |
| Cálculo sin emitir | `POST /v1/facturas/calcular`: mismo cuerpo y mismas reglas, sin numerar ni consumir cupo | [ref] |
| Numeración propia | `correlativo` opcional (int64 > 0) en facturas y notas. Si ya existe: `409 DUPLICADO`. Series con `correlativo_inicial`. **Acepta N+1 antes que N, y N después**: solo rechaza duplicados y sube la serie con `GREATEST(ultimo_numero, n)`. Un `422` no consume el número. Un comprobante `RECHAZADO` por SUNAT conserva su número: reenviarlo da `409 DUPLICADO`. **"Las emisiones sobre una misma serie se procesan en orden"** | [ref]; [entornos]; [código] |
| Tipo de cambio | `tipo_cambio` de hasta 3 enteros y 3 decimales, solo fuera de PEN (en PEN: `422 TIPO_CAMBIO_INVALIDO`) | [ref] `FacturaRequest.tipo_cambio` |
| Notas | `POST /v1/notas` sobre un comprobante aceptado. La serie lleva la letra del documento de origen (`FC`/`FD` o `BC`/`BD`). Sobre boleta no se admiten NC 04, 05, 08 ni 13 | [ref] |
| Baja | `POST /v1/facturas/{id}/baja`: Khipu arma la RA o el RC. Solo comprobantes aceptados, hasta el día 7 en hora de Lima. **Sin idempotencia**; una baja en curso bloquea otra. Estados de la baja: `GENERADA → ENVIADA → ACEPTADA / RECHAZADA`, más `ERROR_ENVIO` | [ref] `BajaResponse.estado` |
| Resumen diario | Lo gestiona Khipu. Una boleta que pasó el envío individual se informa en un resumen (`baja.condicion = ALTA`) | [ref] |
| GRE | `POST /v1/guias-remision` con `Idempotency-Key`. Khipu numera. Estados `FIRMADO`, `ERROR_ENVIO`, `ENVIANDO`, `ENVIADO_PENDIENTE_TICKET`, `ACEPTADO`, `ACEPTADO_CON_OBS`, `RECHAZADO`. **Experimental**: el envío viene apagado en el servidor (`422 GRE_NO_DISPONIBLE`), no hay envío en BETA y no emite webhooks | [ref] `GuiaRemisionResponse.estado`; [guía]; [código] |
| Respuesta | `201` con `estado_documento`, `hash`, `fecha_limite_envio`, `cdr`, `totales` y `enlaces` (PDF con QR desde `FIRMADO`) | [ref] `ComprobanteResponse` |
| Conservación de artefactos | XML y CDR: Gratis 1 año; Emprende, Negocio y Pro 5 años (el seed de planes dice que «heredan la de Emprende»). El plazo solo se guarda y se muestra: **no hay purga automática** de XML, CDR ni PDF. El PDF se genera la primera vez que se pide y se guarda junto al XML; si cambia el diseño, se regenera con otra clave | [planes]; [código] |
| Salida en bloque | `GET /v1/facturas/descarga`: ZIP por streaming con XML, PDF y CDR, filtrado por fecha, estado o serie, con un tope de 500 por petición (`422 DESCARGA_EXCEDE_EL_TOPE`). Dar de baja la cuenta no borra nada; la suspensión bloquea toda llamada con la API key (`403 CUENTA_SUSPENDIDA`); el plan vencido solo bloquea la emisión (`402 PLAN_VENCIDO`) | [código] |
| Estados del comprobante | Enum de `GET /v1/facturas?estado`: `RECIBIDO`, `INVALIDO`, `FIRMADO`, `ERROR_ENVIO`, `PENDIENTE_AGRUPACION`, `ENVIADO`, `ACEPTADO`, `ACEPTADO_CON_OBS`, `RECHAZADO`, `ANULADO`, `FUERA_DE_PLAZO`, `DESCARTADO` | [ref] `GET /v1/facturas` |
| Reintentos hacia SUNAT | Los hace Khipu: hasta 20 en ~74 h (dato de la documentación, no confirmado en el código). Forzado con `POST /v1/facturas/{id}/enviar` (`409 ENVIO_EN_CURSO` si hay un envío en vuelo). Con `enviar_automatico`, `POST /v1/facturas` llama a SUNAT dentro de la misma petición, con 15 s de timeout. Khipu **no envía `Retry-After`** | [entornos]; [ref]; [código] |
| Idempotencia | `Idempotency-Key` en facturas, notas y guías. Vigencia 24 h. Mismo cuerpo: `200`. Otro cuerpo: `422 IDEMPOTENCIA_INVALIDA` | [errores] |
| Búsqueda por número | `GET /v1/facturas?serie=&numero=` | [ref] |
| Webhooks | Solo cambios de estado de un comprobante: `comprobante.aceptado`, `.aceptado_con_obs`, `.rechazado`, `.error_envio`, `.fuera_de_plazo`, `.anulado`, `.descartado`. **La GRE no emite webhooks, y una baja rechazada o en error tampoco** (la aceptada llega como `comprobante.anulado`). Firma `X-Khipu-Signature: t=..,v1=..` (HMAC-SHA256 de `t.cuerpo`). Deduplicación por `X-Khipu-Delivery`. **Sin orden garantizado.** Reintentos con esperas crecientes (1 min, 5 min, 15 min, 1 h…); pausa tras 10 fallos seguidos; el log de entregas dura **30 días**; hasta 5 endpoints por empresa | [guía] *Webhooks*; [código] |
| Límites | **Sin limitador de tasa** en la emisión. Cuota mensual del plan sumada entre todas las empresas de la cuenta (`429 LIMITE_PLAN`, `402 PLAN_VENCIDO`). BETA: 1 000 documentos por RUC al mes, sumados entre las empresas en BETA de la cuenta (`429 LIMITE_PRUEBAS`). Consulta RUC: 60/min y 5 000/día por empresa | [entornos]; [código] |
| Errores | Sobre `{estado, datos, codigo, mensaje, errores}`; `trace_id` en los 500; un 502 puede no ser JSON | [ref]; [errores] |

## Supuestos y verificaciones en BETA (antes del issue #56)

Respondidas con el código de Khipu ([código], repo público bacsystem/khipu @53bd4bc) y con decisiones del dueño (2026-11-17):

1. **Redondeo. Resuelta** ([código]). Khipu calcula todo desde `precio_unitario` (con IGV en las gravadas). Por línea: valor unitario = precio / (1 + tasa) a 10 decimales HALF_UP; base = valor × cantidad a 2 decimales HALF_UP; IGV a 2 decimales HALF_UP. En el modo «precio exacto», el IGV de la línea es lo que falta para llegar a precio × cantidad; si la suma de los desvíos frente a base × tasa pasa de 0,50, **todo el comprobante** vuelve al modo base × tasa. `pricing.Compute` replica ese algoritmo, incluido el cambio de modo por comprobante. Los 200 carritos contra `POST /v1/facturas/calcular` quedan como **prueba de regresión**, no como descubrimiento.
2. **Orden y huecos por serie. Resuelta** ([código]): Khipu acepta N+1 antes que N, y N después. La política de hueco es la de «seguir con N+1» (ver «Numeración y orden por serie»). En BETA queda una **confirmación ligera**: enviar N+1 antes que N una vez.
3. **`PENDIENTE_AGRUPACION`. Resuelta** ([código]): está en el enum, pero ningún servicio lo asigna. Tratarlo como «en curso» es correcto e inofensivo.
4. **Timeouts y `Retry-After`. Resuelta** ([código]): Khipu llama a SUNAT dentro de la petición con 15 s de timeout; los 30 s de Wanka cubren eso y la firma. Khipu no envía `Retry-After`: el backoff es siempre el de Wanka. La latencia típica no está medida: se mide en la prueba de carga de #56.
5. **Modelo de cuentas. Resuelta** (decisión del dueño): una cuenta de Khipu por empresa, en Emprende o superior, creada con el alta asistida (ver «Alta asistida»).
6. **Automatizar el alta. Resuelta** (decisión del dueño): no hay API de integrador; el alta la hace el operador de Khipu con el alta asistida, en un máximo de 24 horas. La API de integrador queda para más adelante.
7. **GRE en R1. Resuelta** (decisión del dueño, [código]): la GRE (#61) pasa a R2. Es experimental, su envío viene apagado y no existe en BETA.
8. **Webhooks de GRE y de bajas. Resuelta** ([código]): la GRE no emite webhooks, así que su estado sale de la conciliación. Una baja rechazada o en error no emite evento: se resuelve sondeando `GET /v1/bajas/{id}` (ver «Bajas»).
9. Conservación por plan. **Resuelta** ([código]): Emprende, Negocio y Pro conservan 5 años, y no hay purga automática. **Queda abierto**, como pregunta a Khipu, el compromiso contractual de no introducir una purga mientras Wanka use el servicio, y de configurar el volumen o el bucket que usa para que conserve los archivos al menos ese plazo, porque la conservación física depende de esa configuración del operador ([código], `S3DocumentStorage`).
10. Conservación del PDF. **Resuelta** ([código]): Khipu lo genera bajo demanda y lo guarda junto al XML. Sus bytes cambian si cambia el diseño.
11. Exportación en bloque. **Resuelta** ([código]): ZIP por rango de fechas, con un tope de 500 por petición. La baja de la cuenta no borra nada; la suspensión corta las descargas.
12. Si el alta asistida puede registrar también el webhook, para no depender de la sesión del portal en el paso 4. **Pregunta a Khipu.**

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

### Alta asistida (modelo de cuentas)

Decisión del dueño (2026-11-17): las empresas las crea el **operador de Khipu** con el alta asistida (`POST /v1/admin/cuentas`): **una cuenta de Khipu por empresa (tenant), en el plan Emprende o superior** (en PRODUCCION; un tenant de prueba en BETA admite cualquier plan). El operador activa la empresa en un máximo de **24 horas**. La API de integrador queda para más adelante. Pasos del alta de un tenant (#31 y #55):

| Paso | Quién | Cómo |
|---|---|---|
| 1. Pedir el alta con RUC, razón social y entorno: PRODUCCION para la empresa real, BETA solo para un tenant de prueba aparte (ver «Un entorno por tenant») | soporte de Wanka | manual, al operador de Khipu, indicando el plan Emprende o superior. El tenant queda en «Pendiente de activación de facturación» (#31) |
| 2. Crear la cuenta (Emprende o superior), la empresa, su serie inicial y la API key `fk_` | operador de Khipu | `POST /v1/admin/cuentas`, manual desde el lado de Khipu, en un máximo de 24 horas |
| 3. Obtener la URL del webhook | operador de Khipu | comando `wanka einvoicing route --tenant <id>`: genera un `routeId` aleatorio de 128 bits y muestra la URL `/webhooks/khipu/{routeId}`. **No escribe nada en la base**: la fila de `ops.webhook_routes` la crea `activate`, completa y con su secreto |
| 4. Registrar el webhook en Khipu con esa URL y obtener el `whsec_` | operador de Khipu | manual: `/v1/webhooks` exige una sesión del portal de la cuenta, no la API key (ver la pregunta 12) |
| 5. Activar la facturación en Wanka | operador de Khipu | comando `wanka einvoicing activate` (ver abajo): carga la API key, el `whsec_` y el plan, fija el entorno y, en PRODUCCION, rechaza un plan inferior a Emprende |
| 6. Cargar el certificado, las credenciales SOL y API SUNAT, los datos fiscales y una serie por caja con `correlativo_inicial` | dueño de la empresa, con `empresas.einvoicing:update` | `/configuracion/sunat` (#55). Wanka los envía a Khipu con la API key; el dueño nunca ve la llave ni el `whsec_` |

- **Comando de activación (temporal hasta la consola de R4, #86).** Decisión del dueño (2026-11-17, opción A): el operador de Khipu registra la activación también en Wanka con un comando interno y auditado del binario, `wanka einvoicing activate --tenant <id> --environment BETA|PRODUCCION --plan gratis|emprende|negocio|pro --company-ref <id> --route-id <routeId> --operator <correo> [--rotate]`.
  - **Entrada de secretos.** La API key y el `whsec_` se leen de la entrada estándar **sin eco**, con `golang.org/x/term` (aprobada por el dueño el 2026-11-17; registro en `README.md` §9). Nunca se leen de argumentos, variables ni historial, y no se escriben en logs.
  - **Validaciones.**
    - **Plan.** `--plan` admite `gratis`, para que el rechazo pueda ocurrir de verdad. Con PRODUCCION y `gratis`, el comando rechaza con el motivo «Plan de facturación insuficiente» (#55): solo escribe en `audit.events` la acción `einvoicing:activate_rejected` con el motivo, y la empresa sigue como estaba. En BETA (tenant de prueba) se admite cualquier plan. El plan es una declaración del operador: con la API key no se puede leer el plan de la cuenta.
    - **Llave.** Verifica la API key con `GET /v1/empresa` de Khipu (3 s de conexión, 10 s en total). La empresa que devuelve tiene que coincidir con `--company-ref`.
    - **Ruta.** `--route-id` tiene que tener el formato del paso 3 (128 bits) y no existir para otro tenant. Si el operador lo copió mal, los webhooks no llegan y la alerta de `webhook_verified_at` lo hace visible.
  - **Transiciones de `activate`.** Primero se valida todo lo que no depende del estado (formato del `--route-id`, llave contra `GET /v1/empresa` y empresa igual a `--company-ref`), **fuera de toda transacción**, porque es una llamada de red. Después, en una transacción corta, se toma `pg_advisory_xact_lock` con una clave derivada del tenant y de `einvoicing:activate`. Así dos `activate` simultáneos sobre el mismo tenant se serializan, y el segundo vuelve a leer el estado ya cambiado. El lock no necesita GRANT: `wanka_app` solo tiene SELECT en `org.tenants` y no podría bloquear esa fila. **Todo rechazo** deja una fila `einvoicing:activate_rejected` en `audit.events`, con el motivo, en una transacción propia, y nada más.

    Las filas se evalúan en orden y gana la primera que coincide.

    | Estado de origen | Banderas | Efecto | `audit.events` |
    |---|---|---|---|
    | cualquiera | llave inválida, empresa distinta de `--company-ref` o `--route-id` mal formado o de otro tenant | rechazo con el motivo | `einvoicing:activate_rejected` |
    | cualquiera | PRODUCCION con `--plan gratis` | rechazo «Plan de facturación insuficiente» | `einvoicing:activate_rejected` |
    | pendiente (sin `activated_at`) | BETA con cualquier plan, o PRODUCCION con Emprende o superior | crea `provider_credentials` con ese `environment`, que ya no cambia, y la ruta; fija `activated_at` | `einvoicing:activate` |
    | activo | `--environment` distinto del guardado, con o sin `--rotate` | rechazo «El entorno de facturación no se puede cambiar» (ver «Un entorno por tenant») | `einvoicing:activate_rejected` |
    | activo | `--company-ref` distinto de `provider_company_ref`, con o sin `--rotate` | rechazo «La empresa de Khipu no se puede cambiar» | `einvoicing:activate_rejected` |
    | activo, mismo entorno y empresa | sin `--rotate`, mismo plan | rechazo «La facturación ya está activa» | `einvoicing:activate_rejected` |
    | activo, mismo entorno y empresa | sin `--rotate`, otro plan válido para el entorno (por ejemplo, de Gratis a Emprende en BETA o de Emprende a Negocio en PRODUCCION) | cambia solo `provider_plan`; no lee secretos ni toca la ruta | `einvoicing:plan_change` |
    | activo, mismo entorno y empresa | `--rotate` | rota la llave y el `whsec_`; revoca la ruta anterior y crea la nueva; `webhook_verified_at` vuelve a nulo; conserva `emission_ready_at` | `einvoicing:rotate` |

  - **Un entorno por tenant.** Decisión del dueño (2026-11-18): cada tenant vive en un único entorno de Khipu durante toda su vida. Las pruebas en BETA se hacen en un tenant de prueba aparte, y la empresa real se activa directamente en PRODUCCION. No hay paso de BETA a PRODUCCION ni el inverso.
    - **Por qué serie y número no son ambiguos.** Un tenant tiene un solo entorno y una sola empresa de Khipu (`provider_company_ref`) para siempre: la tabla de arriba rechaza cambiar cualquiera de los dos, y `--rotate` exige la misma empresa. Así bastan las UK sin entorno: `(tenant_id, doc_type, series)` en `org.document_series` y `(tenant_id, doc_type, series, number)` en `einv.electronic_documents` (data-model §3). Un número dentro de un tenant corresponde a un único comprobante en una única empresa de Khipu, y los CPE de BETA viven en su propio tenant de prueba, así que nunca entran en el RVIE, el PLE ni los reportes de la empresa real.
    - **Qué la haría cambiar.** Si hiciera falta probar en BETA con los datos de la empresa real y después pasar a PRODUCCION, habría que separar los documentos por entorno (columna en la serie y en el documento, y en las dos UK) y filtrar los reportes: un ADR nuevo.
  - **Dónde corre y con qué permisos (ADR 0001, sin `BYPASSRLS`).** Corre en el servicio `api`, el único que tiene `INTEGRATIONS_ENC_KEY` y `WEBHOOK_SECRET_ENC_KEY`, con el rol `wanka_app` (`DATABASE_URL_APP`) y dentro de `InTenantTx(Actor{TenantID: <id>, Kind: system})`. `wanka_app` ya tiene, con la política estándar `tenant_isolation`, `INSERT/UPDATE` en `integ.provider_credentials`, `INSERT/UPDATE` en `ops.webhook_routes` e `INSERT` en `audit.events`. Por eso **no hace falta una política, un rol, un GRANT ni una función `SECURITY DEFINER` nuevos**, y `data/rls-check/` no cambia.
    - Se usa `Kind: system` y no un tipo de sesión nuevo para no tocar el `Actor` del ADR 0001, que está aceptado. Las funciones definer que dependen del tipo de sesión ya rechazan `system` (por ejemplo, `iam.decide_impersonation`).
    - Se descartó una función `platform_fn.*` llamada por `wanka_platform`: obligaría a dar `INTEGRATIONS_ENC_KEY` a `platform-api` y a sumar una función definer, con su prueba en `rls-check`, para un mecanismo temporal.
  - **Qué escribe**, en una sola transacción:
    - en `integ.provider_credentials`: `api_key_enc`, `environment`, `provider_plan`, `provider_company_ref`, `verified_at`, `activated_at` y `activated_by` (el correo del operador);
    - la fila de `ops.webhook_routes`, completa (`route_id`, `tenant_id`, `provider` y el `whsec_` cifrado), que se crea solo aquí; así nunca existe una ruta sin secreto y `ops.resolve_webhook_route` no puede devolver una (`webhook_secret_enc` sigue `NOT NULL`, como en `data/rls-check/`);
    - una fila en `audit.events` con `actor_type = platform_staff`, `action = einvoicing:activate` (o `einvoicing:rotate`, `einvoicing:plan_change` o `einvoicing:activate_rejected`) y el operador declarado en `--operator` como referencia. El `Actor` de la transacción es `Kind: system` solo para fijar las variables de sesión; el comando escribe `actor_type = platform_staff` de forma explícita en la fila, no lo deriva del `Actor`. Va en `audit.events` del tenant y no en `platform.audit_events` por la excepción aprobada que se describe más abajo.

    Con `activated_at`, #31 pasa a «Activación completada: configura certificado y SOL» y se habilita #55.
  - **Quién puede correrlo.** Solo quien tiene acceso al shell del servicio `api` en Railway. Ese control es más débil que el de una consola con 2FA y permisos, y es la razón por la que el comando es temporal: con #86, la activación pasa a la consola de plataforma, con `platform.audit_events`, y el comando se retira.
  - **Excepción aprobada por el dueño (2026-11-17).** Hasta #86, la auditoría del comando va en `audit.events` del tenant, con el `--operator` declarado y **sin autenticar**. El único control es el acceso al shell del servicio `api` en Railway. Es una excepción a la regla de ADR 0001, según la cual las acciones del staff van a `platform.audit_events`. Condiciones de retiro: cuando llegue la consola de #86, se retira el comando y se hace un backfill de sus filas de `audit.events` (acciones `einvoicing:*`) a `platform.audit_events`.
- **Estados de la facturación del tenant** en `integ.provider_credentials`:
  - sin fila o sin `activated_at`: «Pendiente de activación de facturación». La emisión está bloqueada y no se consume ningún correlativo (#31);
  - `activated_at`: «Activación completada: configura certificado y SOL»;
  - `emission_ready_at`: el dueño cargó el certificado, SOL y las series, y la prueba de conexión dio «Conexión correcta» (#55). Recién entonces se puede emitir;
  - `webhook_verified_at`: primer evento firmado que llega por la ruta. Lo escribe el `worker` en `cpe.sync`, dentro de `InTenantTx`, porque el servicio `webhooks` (rol `wanka_webhooks`) no tiene acceso a `integ.provider_credentials` (ADR 0001); no hace falta un GRANT nuevo. No bloquea la emisión, porque la conciliación cubre la falta de webhooks, pero mientras sea nulo hay una alerta para soporte.

  Los pasos 1, 2 y 4 ocurren fuera de Wanka. Si pasan más de 24 horas desde el pedido sin `activated_at`, el dueño sigue viendo el estado pendiente con un medio de contacto y soporte recibe una alerta (#31).
- La API key la ve una sola vez quien la crea, y la carga en Wanka el mismo operador con el comando del paso 5, sin pasar por terceros ni por correo. Wanka nunca la escribe en variables ni en logs.
- Khipu no limita el ritmo de emisión (solo hay cuota mensual por cuenta): Wanka limita el suyo (concurrencia fija de la cola `cpe` y un emisor por serie). Como cada tenant tiene su cuenta, la cuota mensual y la de BETA son por tenant, no compartidas.

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
- **Política de hueco.** Un documento bloqueado por `422` se aparta (`blocked_reason = validation`) y el drenaje sigue con N+1. Khipu lo acepta ([código], punto 2), y el `422` no consume el número allá, así que el documento corregido sale después con el mismo número.
- **Número rechazado por SUNAT.** Queda consumido: reenviarlo con el mismo número da `409 DUPLICADO` ([código]). Se emite un comprobante nuevo con otro número, y el rechazado queda en la serie como hueco documentado (`rechazado`).

### Artefactos: Khipu los conserva

Decisión del dueño (2026-11-17): Khipu almacena y sirve el PDF, el XML y el CDR de los CPE durante 5 años, con el plan Emprende o superior. Wanka no los guarda, y en R1 no hay bucket.

- **Requisito de plan.** Según [planes] y [código], Gratis conserva 1 año y Emprende, Negocio y Pro, 5 años. Cada tenant tiene su propia cuenta de Khipu, creada con el alta asistida en el plan Emprende o superior (ver «Alta asistida»), así que sus artefactos se conservan 5 años:
  - no se usa una cuenta compartida entre tenants;
  - guardar una copia propia necesitaría el adaptador de bucket del `BlobStore`, que no existe en R1, así que no se ofrece.

  Hoy Khipu no borra artefactos de ningún plan: no hay purga automática ([código]). Su política de privacidad dice que los conserva mientras la cuenta exista y la normativa lo exija ([docs]). La conservación física depende del volumen o del bucket que configure el operador de Khipu: «versionado y retención se configuran en el bucket» ([código], `S3DocumentStorage`). Que siga así es un compromiso contractual, no técnico: ver la pregunta 9.
- **Qué guarda Wanka.** Por documento guarda `provider_ref`, `hash` (el resumen del XML firmado que devuelve Khipu) y `xml_sha256`.
  - El `xml_sha256` sirve porque Khipu escribe el XML firmado una sola vez ([código]).
  - Se fija en la primera descarga con `UPDATE einv.electronic_documents SET xml_sha256 = $1 WHERE tenant_id = $t AND id = $id AND xml_sha256 IS NULL RETURNING xml_sha256`.
    - Si la sentencia afecta 0 filas, se lee el valor guardado.
    - El archivo se sirve solo si coincide con ese valor. Si difiere, no se sirve y se genera una alerta crítica.
    - Así, dos primeras descargas concurrentes con contenidos distintos no pasan desapercibidas: la segunda choca con la huella de la primera.
  - Un documento que nunca se descargó no tiene `xml_sha256`; lo firmado sigue respaldado por `hash`.
  - El CDR y el PDF se sirven sin huella. El PDF lo genera Khipu y sus bytes cambian si cambia el diseño, así que Wanka nunca le calcula un hash.
- **Descarga bajo demanda.** Reimprimir, reenviar y descargar (#52) piden el artefacto a Khipu por `provider_ref` (el endpoint exacto se confirma en [ref]). El `api` hace de proxy por **streaming**, sin guardar el archivo ni cargarlo entero en memoria.
  - **Fuera de `http.TimeoutHandler`.** La ruta `GET /v1/einvoicing/documents/{id}/files/{xml|cdr|pdf}` no pasa por ese handler, que guardaría la respuesta entera en memoria, y tiene su propio presupuesto, declarado en `api/conventions.md`. Todos los plazos se cuentan **desde que llega la petición**: 3 s para conectar con Khipu y 8 s para recibir sus encabezados. Si no llegan, la API responde `503 dependency_unavailable` (RFC 9457) con `Retry-After`. La descarga completa tiene un tope de **25 s**, también contados desde la llegada. Así el peor caso queda por debajo de los 27 s del BFF y los 30 s del borde. Si el tope se alcanza a mitad del streaming, la conexión se corta sin el fragmento final, y el BFF lo trata como error, no como archivo completo.
  - **Sin reintentos en el adaptador** (ADR 0006: reintentos en una sola capa). La petición interactiva falla rápido con el `503`. Para enviar el comprobante por correo o WhatsApp (#81), el job de notificaciones pide el archivo a Khipu al enviarlo, y los reintentos son los de River.
  - **Concurrencia.** Hasta 2 descargas simultáneas por tenant (el cupo de descargas de `api/conventions.md`) y un tope global de 16 por réplica de `api`, con un límite propio, separado de la cola de emisión. Al llenarse, la API responde `429 rate_limited` (por tenant) o `503 overloaded` (global), con `Retry-After`. El cupo se ocupa durante toda la descarga, así que en la ley de Little (L = λ·W) W es la duración completa, no solo la espera de los encabezados. Con un p95 supuesto de 4 s, 16 cupos sostienen unas 4 descargas/s por réplica; en el peor caso (25 s, un cliente lento), unas 0,64/s, y el resto recibe `503 overloaded`. El valor se ajusta con el p95 medido en la prueba de carga de #52, que incluye clientes lentos y mide el porcentaje de `503 overloaded`.
  - El ticket del POS no depende de Khipu: se imprime con los datos de Wanka.
- **PDF.** Lo genera Khipu bajo demanda la primera vez que se pide (desde `FIRMADO`) y lo guarda junto al XML. Si cambia el diseño, lo regenera con otra clave ([código]). Wanka no genera el PDF.
- **Salida al terminar el servicio.** `GET /v1/facturas/descarga` devuelve un ZIP por streaming con XML, PDF y CDR, filtrado por rango de fechas, con un tope de 500 comprobantes por petición (por eso se descarga por tramos). Dar de baja la cuenta no borra nada ni corta el acceso; **suspenderla sí corta todas las llamadas, también las descargas** (`403 CUENTA_SUSPENDIDA`), mientras que un plan vencido solo bloquea la emisión ([código]). La herramienta de exportación por tramos no entra en R1.
- **Exportes.** Libros electrónicos, reportes y exportes se generan en el `api` al momento, desde Postgres, por streaming y sin guardarse. Decisión del dueño (2026-11-17): **solo CSV**, sin PDF ni XLSX, y sin librería nueva. Lectura del architect, no decisión del dueño: los libros electrónicos del PLE (#73) se entregan en el TXT plano que exige SUNAT, que tampoco necesita librería. El reporte Z no genera PDF: se ve en pantalla y se imprime desde el navegador con estilos de impresión (`sequences/05-cash-close.mmd`).
  - **Ruta.** `GET /v1/exports?type=&period=` es una lectura sin efectos: se puede repetir sin `Idempotency-Key` y no guarda copia.
  - **Rango máximo.** Un periodo tributario (un mes calendario) y 50 000 filas, lo que se alcance primero. Se valida antes de generar, en la misma transacción `REPEATABLE READ READ ONLY` que el exporte, para que el conteo y el archivo vean la misma foto de los datos:
    - el periodo, con los parámetros de la petición;
    - las filas, con `SELECT count(*) FROM (<consulta del exporte> LIMIT 50001) q`, un conteo acotado que nunca recorre más de 50 001 filas. Si devuelve 50 001, se responde `422 validation_failed` con el campo `period` en `errors`, antes de enviar ningún byte.
  - **Tiempo.** Corre fuera de `http.TimeoutHandler`. Las filas se leen con un cursor de servidor (`DECLARE … CURSOR`, luego `FETCH 1000` por lote), sin cargar el resultado en memoria. Hay dos límites:
    - **Por lote:** `SET LOCAL statement_timeout = '20s'`. Cada `FETCH` es una sentencia, y el primero puede tener que ordenar o agregar todo el periodo; los 5 s del rol de la app no alcanzan para ese caso.
    - **Exporte completo:** el `context` de la petición, con 25 s desde que llega. Al vencer, cancela la consulta en curso (pgx) y corta el streaming.
  - **Concurrencia.** 2 exportes por tenant y 4 por réplica de `api`, separados del cupo de descargas, para que una consulta de hasta 20 s no ocupe más de 4 de las 20 conexiones del pool ni la CPU que necesita el POS. Al llenarse, `429 rate_limited` o `503 overloaded` con `Retry-After`.
  - **Si falla a mitad.** Si la consulta falla después de empezar el streaming, el archivo queda truncado: la conexión se corta sin el fragmento final y el BFF lo informa como error.
  - **Medición.** La prueba de carga de reportes (#73) mide que un exporte del rango máximo, con datos de capacidad, termine dentro de los 25 s.
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
| Cuota | `429 LIMITE_PLAN`, `429 LIMITE_PRUEBAS` (BETA; cada tenant tiene su cuenta, así que la cuota es suya), `402 PLAN_VENCIDO` | `blocked_reason = quota`; pausa del tenant y alerta crítica a plataforma. Monitoreo de la cuota al 80 % con `GET /v1/cuenta` (solo portal: la mide plataforma) |
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
   - bajas en estado no final (`solicitada`, `generada`, `enviada` o `error_envio`) **sin un `cpe.void` vivo** (ni en curso ni con snooze) y sin `stalled_at`: las reencola. Son los mismos estados que atiende el job: `solicitada` la pide a Khipu, y las otras tres las sondea. El job retoma la misma cadencia desde `requested_at`; la conciliación no consulta a Khipu por su cuenta;
   - documentos con `blocked_reason`: genera una alerta según su antigüedad y su `send_deadline`.
5. Timeouts: conexión 3 s, total 30 s (incluye la ida a SUNAT; **supuesto**, se mide en BETA).

### Bajas

Wanka crea `einv.voidings` (con un índice único parcial: una baja en curso por documento) y el job `cpe.void`.
- Antes de cada intento consulta `ListVoids`, porque la baja no tiene idempotencia ([código]: sin clave; una baja en curso bloquea otra); si ya hay una baja en curso, la adopta.
- **Resultado de la baja.** Una baja aceptada llega por el webhook `comprobante.anulado`. Una rechazada o en error **no emite webhook** ([código]), así que el propio job `cpe.void` sondea `GET /v1/bajas/{id}` con una **única cadencia**: a los 1, 5, 15 y 60 min de pedirla, y luego cada hora hasta 24 h, mientras la baja esté en un estado no final (`generada`, `enviada` o `error_envio`; en Khipu, `GENERADA`, `ENVIADA` o `ERROR_ENVIO`), los mismos que reencola la conciliación. Esta es la cadencia de referencia; la conciliación y `sequences/03-credit-note-void.mmd` la siguen.
  - **Mecanismo.** Entre consultas, el job devuelve `JobSnooze` de River hasta la siguiente marca, calculada desde `einv.voidings.requested_at`. No usa los reintentos por error: ADR 0006 los reserva para errores transitorios, y unas 27 consultas pasarían el `MaxAttempts` por defecto de River (25). Que el snooze no consuma intentos se verifica en la versión de River que se fije.
  - **Al tope.** Pasadas 24 h sin estado final, el job deja la baja con `stalled_at`, emite la alerta «La baja no se ha resuelto» (#60) para el tenant y para soporte, y termina. La UI muestra esa alerta en vez de quedarse en «En baja».
  - Si mientras tanto llega `comprobante.anulado`, `cpe.sync` cierra la baja y el job, al despertar, ve el estado final y termina.
- La API valida antes: el documento está aceptado u observado, y no pasaron más de 7 días.
- **Una boleta ya entregada al cliente no se anula: corresponde una nota de crédito.** La UI lo pregunta.

### GRE

Pasa a R2 (decisión del dueño, 2026-11-17): es experimental en Khipu, su envío viene apagado y no existe en BETA ([código]). Mismo patrón, con `Idempotency-Key`. Como no emite webhooks, su estado sale solo de la conciliación (`/{id}/ticket`). El traslado no depende de la aceptación de la GRE para registrarse (ver `sequences/04-transfer-gre.mmd`).

### Ticket del POS

El ticket muestra la serie y el número que asignó Wanka, con el CPE "pendiente". Opción para el PO: esperar hasta 3 s el `hash` (disponible desde `FIRMADO`) para imprimir el ticket completo.

### Plan B por fallo

| Fallo | Sistema | Usuario |
|---|---|---|
| Khipu lento o caído | cola `cpe` con concurrencia fija y backoff; alerta a los 15 min | ticket "pendiente"; panel "cola SUNAT"; píldora SUNAT en advertencia |
| SUNAT caída (`ERROR_ENVIO`) | Khipu reintenta; Wanka concilia | CPE "enviado" |
| Validación (`422`) | `blocked_reason`; la serie sigue con N+1 (Khipu lo acepta) | "corregir y reenviar" |
| Rechazo de SUNAT | el número queda consumido; se emite uno nuevo | "emitir de nuevo" con otro número |
| Baja rechazada o en error (sin webhook) | sondeo de `GET /v1/bajas/{id}` con backoff y conciliación | la baja muestra «rechazada» con el motivo de SUNAT |
| Cuota (`LIMITE_PLAN`, `LIMITE_PRUEBAS`) | pausa y alerta crítica | aviso en el dashboard |
| Credenciales o configuración | pausa del tenant | banner en `/configuracion/sunat` |
| Webhooks pausados (tras 10 fallos seguidos) | la conciliación cubre el hueco; alerta para reactivarlos. El log de entregas de Khipu dura 30 días: un hueco más largo solo se recupera por conciliación | nada |
| Khipu lento o caído al descargar un artefacto | `503 dependency_unavailable` con `Retry-After`, sin reintentos; alerta si sube la tasa de error | «documento no disponible por ahora»; el ticket del POS se reimprime igual |
| Cuenta de Khipu suspendida (`403 CUENTA_SUSPENDIDA`) | alerta crítica: se cortan la emisión y las descargas | banner en `/configuracion/sunat`; los documentos ya emitidos no se pueden descargar hasta reactivar la cuenta |

## Consecuencias

- Buenas: la caja no depende ni de Khipu ni de SUNAT; los reintentos son seguros; las carreras entre la respuesta, el webhook y la conciliación se resuelven con escrituras idempotentes y transiciones por rango; el orden por serie está garantizado del lado de Wanka.
- Malas:
  - el ticket "pendiente";
  - las series son exclusivas de Wanka;
  - el alta en Khipu es manual y depende del operador de Khipu (alta asistida) mientras no exista una API de integrador;
  - la baja no tiene idempotencia: se consulta antes de cada intento;
  - un número rechazado por SUNAT queda consumido;
  - Khipu no limita el ritmo de emisión: el límite lo pone Wanka;
  - el log de webhooks de Khipu dura 30 días;
  - un emisor por serie limita el throughput de una serie a 1 / latencia de Khipu;
  - el XML, el CDR y el PDF dependen de Khipu: que no introduzca una purga (pregunta 9), que esté disponible al descargarlos y que la cuenta no esté suspendida, porque la suspensión corta también las descargas.
- Qué haría cambiar la decisión:
  - una API de integrador de Khipu: se automatiza el alta sin cambiar el puerto;
  - Khipu acepta correlativos fuera de orden ([código]), así que se podría relajar el emisor único por serie si el throughput de una serie lo pide; no se cambia sin una medición;
  - si Khipu no asume el compromiso de no purgar, o si una suspensión de cuenta resulta un riesgo real: se activa el adaptador de bucket del `BlobStore` y un job que copie los artefactos con `GET /v1/facturas/descarga`.
- **Criterios de aceptación con el sistema externo** (issues de facturación, empezando por #56):
  - emisión de factura y boleta, `422`, nota de crédito, baja y webhook firmado en **BETA de Khipu**;
  - la regresión de redondeo (200 carritos contra `/calcular`, punto 1) y la confirmación ligera de N+1 antes que N (punto 2);
  - una baja rechazada sin webhook, resuelta sondeando `GET /v1/bajas/{id}`;
  - el CDR no existe en BETA ([entornos]), así que su prueba es un criterio aparte; la GRE va en R2.

## Nota del 2026-11-19 (architect, épica #4)

Correcciones tras contrastar la épica #4 con el código de Khipu. El detalle y su porqué están en el [ADR 0013](0013-khipu-company-config-sync.md); aquí solo lo que cambia de este ADR:

- **Paso 1 del alta asistida:** el correo que recibe la invitación al portal de Khipu es un buzón de soporte de Wanka, uno por tenant, nunca el del cliente. La serie inicial es la boleta (`03`) del local principal (anexo `0000`) que creó el onboarding. Antes del alta, el operador busca el RUC con `GET /v1/admin/empresas?q=`. Si el RUC ya tiene empresa en Khipu (`409 DUPLICADO`, `AltaAsistidaService.java:71`), el tenant sigue pendiente y la única salida es vincular esa empresa (ADR 0013, §1).
- **Paso 6:** el dueño carga el certificado y las credenciales SOL y API SUNAT. Los datos fiscales, los anexos y las series **no** los carga a mano: Wanka los registra con el job `einv.company_sync` a partir de lo que el dueño configura en #32 y #33 (ADR 0013, §3).
- **Validación de `activate`:** además de `--company-ref`, compara `ruc`, `razonSocial` y `entorno` de `GET /v1/empresa` con el tenant y con `--environment`. Si difieren, rechaza con `einvoicing:activate_rejected` («El RUC de Khipu no coincide», «La razón social de Khipu no coincide», «El entorno de Khipu no coincide»). Al activar, encola `einv.company_sync` y `einv.series_reconcile`.
- **`409 DUPLICADO` al emitir que no se puede adoptar** (otro total o receptor): además de la alerta crítica, la serie queda congelada (`provider_conflict_at`) y el documento con `blocked_reason = series_conflict` (ADR 0013, §4).
- **«Las series son exclusivas de Wanka»** (Consecuencias): Khipu no lo impone. Wanka lo sostiene por construcción (el cliente no entra al portal) y lo vigila con la conciliación diaria de `GET /v1/series` (ADR 0013, §4).
- **Preguntas a Khipu nuevas:**
  - 13. Cómo genera el operador una API key para una empresa que ya existe en otra cuenta, sin que pase por terceros (el caso del RUC duplicado).
  - 14. Cómo se corrige la razón social de una empresa: hoy no hay endpoint, ni para el operador.
