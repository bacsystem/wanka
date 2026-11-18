# Modelo de datos

Estado: propuesto (2026-11-17). Fuente de partida: los mocks de `apps/web/src/features/*/mocks` y `apps/web/src/types/domain.ts`, que reflejan la forma que devolverá la API. Hoy no existe backend ni base: todo lo de abajo es diseño, no código existente.

Diagramas ER (uno por grupo de módulos):

| Archivo | Esquemas |
|---|---|
| [`er-01-identidad-empresas.mmd`](er-01-identidad-empresas.mmd) | `iam` (usuarios, sesiones, 2FA, roles, permisos, membresías), `org` (tenants, sedes, cajas, series) |
| [`er-02-clientes-catalogo-inventario.mmd`](er-02-clientes-catalogo-inventario.mmd) | `crm`, `catalog`, `inv` (incluye consignación y devoluciones a proveedor) |
| [`er-03-ventas-facturacion.mmd`](er-03-ventas-facturacion.mmd) | `sales`, `einv` |
| [`er-04-compras-finanzas.mmd`](er-04-compras-finanzas.mmd) | `purch`, `fin` |
| [`er-05-plataforma-transversal.mmd`](er-05-plataforma-transversal.mmd) | `platform` (sin RLS de tenant), `audit`, `ops` (idempotencia), cola River |
| [`er-06-rubros.mmd`](er-06-rubros.mmd) | `lube` (lubricentro), `books` (librería) |

Los diagramas muestran las columnas que deciden el diseño; las columnas de auditoría se omiten en la mayoría de entidades para que se lean, pero son obligatorias (ver abajo).

## 1. Reglas comunes a toda tabla de tenant

1. **Un esquema PostgreSQL por bounded context** (`iam`, `org`, `crm`, `catalog`, `inv`, `sales`, `einv`, `purch`, `fin`, `rpt`, `agenda`, `integ`, `audit`, `notify`, `lube`, `books`, `resto`, `vet`, `dental`, `platform`, `ops`). Un módulo solo lee y escribe su esquema; para lo demás llama a la fachada del otro módulo (ver [ADR 0002](../adr/0002-modular-monolith-go.md)).
2. **`tenant_id uuid NOT NULL`** en toda tabla de tenant, con RLS habilitado y forzado ([ADR 0001](../adr/0001-multitenancy-rls.md)):
   ```sql
   ALTER TABLE sales.sales ENABLE ROW LEVEL SECURITY;
   ALTER TABLE sales.sales FORCE ROW LEVEL SECURITY;
   CREATE POLICY tenant_isolation ON sales.sales
     USING (tenant_id = app.current_tenant())
     WITH CHECK (tenant_id = app.current_tenant());
   -- app.current_tenant() = NULLIF(current_setting('app.tenant_id', true), '')::uuid
   ```
   El `NULLIF` es obligatorio. En una conexión del pool que ya tuvo un `set_config(..., true)`, el parámetro queda definido como `''`, y sin `NULLIF` el cast a `uuid` da error. Con `NULLIF` la política no deja ver ni escribir nada: falla cerrada y sin error. Las excepciones a esta política (login, webhooks, auditoría de plataforma, alta de tenant) están en el [ADR 0001](../adr/0001-multitenancy-rls.md) con su matriz de GRANT.
3. **Toda `UNIQUE` o PK natural empieza por `tenant_id`** (`(tenant_id, sku)`, `(tenant_id, code)`, `(tenant_id, ruc)`...). RLS no filtra los índices únicos: una única global revelaría, por un error de duplicado, que otro tenant usa ese valor. Las UK globales a propósito (RUC de la empresa, email de usuario, hashes de tokens, `route_id` de webhooks) son de tablas que no son de tenant y están marcadas en los ER. Una prueba de CI lo verifica.
4. **Claves foráneas compuestas con `tenant_id`.** Cada tabla padre tiene `UNIQUE (tenant_id, id)` y las hijas referencian `(tenant_id, x_id)`. Así es imposible, por construcción, que una venta del tenant A apunte a un cliente del tenant B, aunque un bug fije mal la variable.
5. **Claves primarias `uuid` v7** (ordenadas por tiempo: buena localidad en índices B-tree, generables en Go sin ida a la base). En la API viajan como string; los mocks ya usan `id: string`.
6. **Columnas de auditoría** en toda tabla mutable: `created_at timestamptz NOT NULL DEFAULT now()`, `updated_at timestamptz NOT NULL`, `created_by uuid`, `updated_by uuid`. Las tablas de solo inserción (movimientos, eventos, kardex) solo llevan `created_at`/`created_by`. El detalle de quién cambió qué va a `audit.events` (particionada por mes, solo inserción), escrita por la capa de aplicación en la misma transacción.
7. **Baja lógica** (`deleted_at`) solo en maestros referenciados por documentos fiscales (clientes, ítems, proveedores, almacenes). Los documentos fiscales nunca se borran: se anulan con su flujo.
8. **Índice en cada FK** y en cada filtro/orden de los listados. Compuestos: igualdad primero, rango después; casi siempre empiezan por `tenant_id` (p. ej. `(tenant_id, issue_date DESC, id DESC)` para comprobantes).
9. **Estados como `text` + `CHECK (status IN (...))`**, no `ENUM` de PostgreSQL: agregar un valor a un `CHECK` es una migración simple y reversible; los enums no permiten quitar valores.
10. **Instantes en `timestamptz`** (UTC en la base; la API los devuelve ISO-8601 con zona y el frontend formatea `dd/mm/aaaa` en `America/Lima`). Fechas fiscales sin hora (`issue_date`, `due_on`) en `date`, calculadas en `America/Lima`.

## 2. Dinero, cantidades y redondeo

Decisión en [ADR 0008](../adr/0008-money-and-numbering.md).

| Concepto | Tipo | Motivo |
|---|---|---|
| Importes de documento (total, IGV, base, pagos, saldos) | `numeric(14,2)` | SUNAT exige 2 decimales en totales; hasta 999 mil millones |
| Precio unitario y valor unitario | `numeric(14,4)` | UBL admite hasta 10 decimales en valor unitario; 4 cubre precios por gramo/ml (lubricantes a granel, venta por peso) |
| Costos unitarios y costo promedio (kardex) | `numeric(18,6)` | el promedio ponderado acumula error si se redondea a 2 |
| Cantidades | `numeric(14,4)` | litros, kg, metros; `integer` estrecharía el dominio |
| Porcentajes y tasas (IGV 18 %, detracción) | `numeric(5,2)` | |
| Tipo de cambio | `numeric(6,3)` | Khipu acepta `tipo_cambio` de "hasta 3 enteros y 3 decimales" (`FacturaRequest.tipo_cambio`); se guarda con esa escala en todos lados para que nada se estreche al enviar |
| Moneda | `char(3)` CHECK `IN ('PEN','USD')` | los mocks ya traen `currency: "PEN" \| "USD"`; ambas desde el release 1 (decisión del usuario) |

- **PEN y USD desde el release 1.** Todo registro que guarda dinero tiene su `currency`: ventas, pagos de la venta, proformas, CPE, OC, comprobantes de compra, CxC, CxP y sus pagos, movimientos y conteos de caja, consignaciones. Los documentos en USD llevan `exchange_rate numeric(6,3)`, con un `CHECK` que lo exige si `currency = 'USD'` y lo prohíbe si es `PEN`, porque Khipu responde `422 TIPO_CAMBIO_INVALIDO` si se envía en soles. Un pago en una moneda distinta de la del documento guarda `amount` (moneda pagada), `exchange_rate` y `amount_applied` (moneda de la deuda). Montos de monedas distintas **no se suman**: caja, arqueo, Z, CxC y CxP agregan por moneda. Los reportes tributarios convierten a PEN con el tipo de cambio de cada documento.
- **Origen y redondeo del tipo de cambio (un solo paso).** `ops.exchange_rates (rate_date, currency, buy, sell numeric(6,3), source)` es una tabla global, sin tenant. La llena un job diario desde la fuente oficial (a definir con el PO). **El único redondeo ocurre al ingerir**: half-up a 3 decimales. Desde ahí, documento y envío a Khipu usan el mismo `numeric(6,3)`, sin otro redondeo. Si el job falla se usa el último tipo conocido, marcado `source = 'fallback'`, y un usuario con permiso puede corregirlo en el documento antes de emitir.
- En Go: tipo `money.Amount` sobre `github.com/shopspring/decimal` (o `cockroachdb/apd`, a fijar en ADR 0002); nunca `float64`. En JSON los importes viajan como **string decimal** (`"118.00"`), porque `number` en JavaScript pierde precisión; el frontend usa `parseNumber`/`formatCurrency` de `lib/format.ts`. Esto es un cambio respecto a los mocks (`total: number`): el adaptador del cliente API los convierte (ver [`api/conventions.md`](../api/conventions.md)).
- Redondeo: half-up a 2 decimales por línea y luego suma, en una única función de cálculo (`pricing.Compute`) usada por POS, proformas, notas y OT. **Debe dar lo mismo que Khipu**, que recibe `precio_unitario` con IGV y calcula valor unitario, IGV y totales con tolerancia SUNAT de ±1 (`POST /v1/facturas`). Su algoritmo exacto no está documentado. Por eso, **antes del issue #56, hay una verificación en BETA**: un set de al menos 200 carritos (con descuentos, exonerados, inafectos, USD y cantidades decimales) se envía a `POST /v1/facturas/calcular`, que no numera ni consume cupo, y sus `totales` deben ser idénticos a los de `pricing.Compute`. En producción, `provider_total` (el `totales.total` de Khipu) se compara con `total`; si difieren, se genera una alerta y prevalece el de Khipu como importe legal del CPE. La lógica actual de `features/pos/lib/pos.ts` (precios con IGV incluido, `cartTotals`) es la referencia de comportamiento; la API recalcula siempre y rechaza (`422 totals_mismatch`) si el total enviado difiere.
- `CHECK` de coherencia: `total = subtotal + igv + otros_cargos - descuentos_globales`, saldos `CHECK (balance BETWEEN 0 AND total)`, pagos `CHECK (amount > 0)`.

## 3. Series y correlativos (numeración concurrente)

Tabla `org.document_series` (una fila por `tenant_id + doc_type + series`). Sirve a CPE (`01`, `03`, `07`, `08`, `09`) y a numeraciones internas (`PRO` proformas, `OC` órdenes de compra, `Z` reportes Z, `OT` órdenes de servicio). Hoy los mocks muestran 6 series para un tenant (`settings/mocks/settings.ts`, `sunatConfig.series`) con `last` como último número.

Asignación dentro de la misma transacción que inserta el documento:

```sql
UPDATE org.document_series
   SET last_number = last_number + 1
 WHERE tenant_id = $1 AND id = $2 AND active
RETURNING series, last_number;
```

- El `UPDATE` toma un lock de fila que dura hasta el `COMMIT`. Si la transacción aborta, el número no se consume: **sin huecos**, que es lo que SUNAT espera de un correlativo. Por eso no se usa `SEQUENCE` (no es transaccional: deja huecos en cada rollback).
- Coste: la serie es una fila caliente. Todo lo que no necesite el lock va **antes** del `UPDATE` (validación, cálculo, lectura de precios, descuento de stock). El `UPDATE` de la serie es la penúltima sentencia antes de los `INSERT` del documento y el `COMMIT`. Ninguna llamada de red ocurre dentro de la transacción.
- Partición natural del recurso: SUNAT permite varias series por tipo; la configuración por defecto asigna **una serie por caja** (B001 caja 1, B002 caja 2), así dos cajas no compiten por la misma fila. Para que esto funcione, la caja viaja en cada venta: la sesión queda ligada a `cash_register_id` al entrar (`POST /v1/auth/session/tenant`), la venta exige `shiftId`, y la API verifica que el turno esté abierto, sea de esa caja y de esa sede. La serie se toma de `document_series` por `(cash_register_id, doc_type)`. Ver criterio de carga en [`capacity.md`](../capacity.md).
- `UNIQUE (tenant_id, doc_type, series, number)` en `einv.electronic_documents` es la última barrera contra duplicados.
- Un CPE rechazado conserva su número (no se reutiliza); se emite uno nuevo. Confirmado por Khipu: "el rechazado conserva el suyo" (ADR 0003).
- Khipu acepta el número de Wanka en el campo `correlativo` (facturas, boletas, notas). Por eso cada serie se registra en Khipu con `correlativo_inicial` y es **exclusiva de Wanka**. Si Khipu rechaza con `422` antes de numerar, el número no se consumió allá: el documento se corrige y se reenvía con el mismo número, sin hueco. Las GRE las numera Khipu (su endpoint no admite `correlativo`); `number` queda nulo hasta su respuesta.

## 4. Stock

- `inv.stock_levels` (PK `tenant_id, warehouse_id, variant_id`) guarda `on_hand` y `reserved`; `inv.stock_movements` es el libro de solo inserción del que salen kardex físico y valorizado (formato 13.1, mock `inventory/mocks/kardex.ts`) con `balance_qty` y `balance_avg_cost` calculados al insertar.
- Descuento atómico: `UPDATE ... SET on_hand = on_hand - $q WHERE ... AND (on_hand - reserved >= $q OR $allow_negative)`; 0 filas afectadas = `409 insufficient_stock`.
- **Varias filas en una transacción: siempre en orden de `variant_id`** (`ORDER BY` en el `SELECT ... FOR UPDATE` previo o ordenando las líneas en Go antes de los `UPDATE`) para no producir deadlocks entre dos ventas con los mismos productos en distinto orden.
- Reservas (`inv.stock_reservations`) para órdenes de servicio de lubricentro, proformas aprobadas y ventas en espera; tienen `expires_at` y limpieza periódica.
- Política de stock negativo: `org.tenant_settings.allow_negative_stock` (por defecto `false`); el `CHECK (on_hand >= 0)` no puede ser incondicional, así que se aplica con un trigger que lee la configuración solo cuando la cantidad resultante es negativa. Pregunta abierta al PO: ¿el POS de un minimarket puede vender sin stock registrado?

## 5. Tablas que crecen solas y su limpieza

| Tabla | Retención | Limpieza |
|---|---|---|
| `iam.sessions` | revocadas/expiradas + 30 días | job diario `iam.purge_sessions` por `created_at` |
| `iam.password_resets`, invitaciones | 7 días | job diario |
| `ops.idempotency_keys` | 24 h | job horario por `created_at` (índice) |
| `einv.provider_webhook_events` | 90 días | job diario |
| Tabla de jobs de River | completados 7 días, descartados 30 días | `JobCleaner` de River |
| `audit.events` | 5 años | particiones mensuales; se separan las antiguas |
| Datos de una empresa cerrada | 5 años en solo lectura (decisión del usuario) | `org.tenants.status = cerrada` y `closed_at`; pasados 5 años, el job `org.purge_closed_tenants` borra por tenant, en lotes y con auditoría |
| `inv.stock_reservations` | hasta `expires_at` | job cada 5 min |

El XML, el CDR y el PDF de los CPE no se guardan en Wanka: Khipu los conserva 5 años con el plan Emprende o superior (requisito, ADR 0003) y se piden bajo demanda ([ADR 0003](../adr/0003-einvoicing-khipu-port.md), «Artefactos: Khipu los conserva»). La base guarda solo `provider_ref`, `hash` y `xml_sha256` (este último se fija en la primera descarga; el CDR y el PDF no llevan huella). En R1 no hay bucket: el puerto `BlobStore` queda en el kernel sin adaptador, y el bucket de Railway o S3 entra más adelante, según el crecimiento. Los adjuntos esperan a ese adaptador. Los exportes se generan al momento y no se guardan.

## 6. Migraciones

Herramienta: **goose** (`github.com/pressly/goose/v3`), migraciones SQL en `apps/api/migrations`, embebidas en el binario (`wanka migrate up`). Alternativas consideradas en [ADR 0002](../adr/0002-modular-monolith-go.md): golang-migrate (sin soporte de migraciones Go ni anotación por archivo para salir de la transacción) y Atlas (declarativo, más potente, pero añade un modelo mental y licencia para funciones avanzadas).

- Solo hacia adelante: una migración mergeada no se edita; revertir es una migración compensatoria. No se escriben bloques `Down` para producción (se mantienen solo para entornos locales si el equipo lo quiere).
- Cambios destructivos en 3 despliegues: **expandir** (columna nueva nullable o tabla nueva; la app escribe en ambas) → **migrar** (backfill por lotes con `id > :ultimo LIMIT 5000`, job reanudable) → **contraer** (quitar lo viejo cuando ninguna versión desplegada lo lee).
- Índices sobre tablas con datos: `CREATE INDEX CONCURRENTLY` en archivo propio con `-- +goose NO TRANSACTION`.
- Corren como **pre-deploy command** en Railway con el rol `wanka_migrator` (dueño de los objetos); la app nunca es dueña de tablas (si lo fuera, RLS no se aplicaría sin `FORCE`).
- Prueba en CI que bloquea el merge: consulta a `pg_catalog` que falla si alguna tabla con columna `tenant_id` no tiene `relrowsecurity` y `relforcerowsecurity` y al menos una política; otra que falla si una FK no tiene índice.
- `lock_timeout = '5s'` y `statement_timeout` en la sesión del migrador, para que una migración que espera un lock no congele la producción.

## 7. Extensiones por rubro (nivel arquitectura; el detalle viene del PO)

Cada rubro es un módulo con su propio esquema que **extiende** entidades core por clave (1:1 o 1:N), sin agregar columnas a las tablas core.

**Lubricentro (`lube`)**. Respuestas del usuario ya incorporadas: venta **al contado** (sin crédito: el checkout no crea CxC); cliente persona o empresa (boleta o factura); próximo cambio de aceite = **km actual de la OT + 5 000** y además una fecha, y el recordatorio sale por **WhatsApp y correo** con lo que ocurra primero; diagnóstico de **filtro de aire y de aceite** registrado en la OT (`service_order_inspections`); **solo productos envasados** (cantidades enteras, sin granel).
- `vehicles`: placa normalizada (mayúsculas, sin guion ni espacios) con `UNIQUE (tenant_id, plate)`; un vehículo pertenece a un cliente (`crm.customers`) y puede cambiar de dueño (historial a definir con PO).
- `odometer_readings`: lecturas con fuente; `vehicles.current_km` y `km_per_day_est` se recalculan al insertar una lectura.
- `service_orders` + líneas: numeración interna `OT`, reservas de stock para aceite/filtros, bahía, técnico; al cobrar se crea una venta core (`sales`) en la misma transacción.
- `maintenance_reminders` + `reminder_deliveries`:
  - El intervalo es **fijo: km de la OT + 5 000** (issue #100: no configurable en R2). Es una constante del módulo, no una tabla de reglas.
  - La fecha es **estimada** con `km_per_day_est` del vehículo. Sin historial, el PO define el valor por defecto (pregunta abierta).
  - El aviso sale **7 días antes** de la fecha o cuando el km estimado llega al objetivo, lo que ocurra primero (issue #101). Va por WhatsApp y correo, solo por los canales con consentimiento (`crm.customer_contacts.consent`). Sin consentimiento pasa a `en_lista_llamada`.
  - Se reenvía una sola vez. `reminder_deliveries` (PK `tenant_id, reminder_id, channel, attempt`) garantiza que un cliente no reciba el mismo aviso dos veces.
  - Una OT nueva del mismo vehículo cierra el recordatorio pendiente anterior (`reemplazado`, `superseded_by`). Hay un índice único parcial: un recordatorio activo por vehículo y servicio.
- Catálogo: atributos de lubricante (viscosidad SAE, norma API, presentación) van en `catalog.item_variants.attributes` (jsonb validado por esquema del rubro), no en tablas nuevas, salvo que el PO pida búsquedas por esos campos (entonces columna generada + índice).

**Librería (`books`)**. Respuestas del usuario: **consignación con distribuidoras**, **devoluciones al proveedor con plazo**, listas escolares **cobradas al contado**, **sin libros usados** (no hay atributo de estado del libro) y **sin descuentos por convenio** con colegios.
- `book_details` 1:1 con `catalog.items`: ISBN-13 (con `CHECK` de dígito verificador; el código de barras ISBN se registra también en `catalog.variant_barcodes` para que el escáner del POS lo encuentre), título, edición, año, editorial; `authors` N:M.
- `schools`, `school_lists`, `school_list_items`: listas escolares por colegio, año y grado; cada ítem guarda el texto original de la lista y, si se pudo, el `variant_id` del catálogo. Una lista se convierte en proforma o venta (core `ventas`).
- Variantes (color, presentación de útiles): las del core `catalogo` (decisión de frontera ya comunicada: variantes son core, no del rubro retail).
- **Consignación y devoluciones a proveedor van al core `inventario`**, detrás del flag `inventario.consignacion` (decisión del dueño, issue #112 con etiqueta `modulo:inventario`), y no al rubro: lo central es de quién es el stock y su liquidación. Compras solo aporta el proveedor y el comprobante de la liquidación, a través de su fachada. Modelo en `er-02-clientes-catalogo-inventario.mmd` (esquema `inv`): `consignments` (distribuidora, almacén, `return_by`), `consignment_lines` (recibido, vendido, devuelto, costo acordado), `consignment_settlements` (liquidación periódica de lo vendido, que termina en el comprobante de compra de la distribuidora) y `supplier_returns` (con `due_on` y GRE de devolución si aplica). En inventario, `stock_levels.consigned` indica qué parte de `on_hand` es de terceros; esos movimientos llevan `consigned = true` y **no entran al kardex valorizado** (el inventario no es propio). Un job diario alerta las consignaciones y devoluciones cuyo plazo vence en N días. Qué unidad se vende primero (propia o consignada) es una regla a definir con el PO.

**Rubros existentes en el frontend**: `resto` (mesas, áreas, comandas/KDS, delivery, reservas), `vet` (mascotas, vacunas, hospitalización, grooming), `dental` (odontograma FDI, evoluciones SOAP, plan de tratamiento), `retail` (UX de matriz de variantes y listas de precio sobre el core). Mismo patrón: tablas propias con `tenant_id` + RLS y FK compuesta a entidades core.
