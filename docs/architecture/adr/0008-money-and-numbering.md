# ADR 0008: Dinero en `numeric` con escala explícita y correlativos sin huecos

- Estado: propuesto
- Fecha: 2026-11-17

## Contexto

Los mocks manejan importes como `number` de JavaScript (`total: number` en `SalesDocument`, precios con IGV incluido en `CatalogItem.price`) y el POS calcula totales en `features/pos/lib/pos.ts` (`cartTotals`). Las series viven en la configuración SUNAT (`settings/mocks/settings.ts`, `sunatConfig.series` con `last`). SUNAT exige correlativos por serie y totales a 2 decimales; el kardex valorizado (13.1) necesita costo promedio sin error acumulado.

## Opciones (dinero)

1. `float8` / `float64`: errores de redondeo en sumas; inaceptable para tributos.
2. Enteros en céntimos (`bigint`): exacto, pero no cubre precios unitarios a 4 decimales ni costos a 6 sin cambiar de escala por campo.
3. **`numeric` con escala explícita por tipo de dato** + decimal en Go + string en JSON.

## Opciones (numeración)

1. `SEQUENCE` de Postgres por serie: sin contención, pero deja huecos en cada rollback y crear miles de secuencias (una por tenant y serie) es incómodo.
2. **Contador en fila (`document_series.last_number`) incrementado en la misma transacción**: sin huecos; la fila es un punto de contención.
3. Asignar el número de forma diferida (al emitir): sin contención en la venta, pero el ticket se imprimiría sin número.

## Decisión

Dinero: opción 3 (escalas en [`data/data-model.md`](../data/data-model.md) §2: importes `numeric(14,2)`, precios `numeric(14,4)`, costos `numeric(18,6)`, cantidades `numeric(14,4)`); `shopspring/decimal` en Go; string decimal en JSON; redondeo half-up por línea en una única función de cálculo compartida; la API recalcula y rechaza totales que no cuadren.

Numeración: opción 2, con el lock **intencional** sobre la fila de la serie. Por qué y cuánto cuesta:
- El lock dura desde el `UPDATE` de la serie hasta el `COMMIT`. Se ubica al final de la transacción (todo lo demás antes), sin llamadas de red dentro, para que dure ~1-3 ms (a medir).
- Con una serie por caja (configuración por defecto) dos cajas no compiten. La caja viaja en cada venta: la sesión queda ligada a `cash_register_id` (`POST /v1/auth/session/tenant`), `POST /v1/sales` exige `shiftId`, y la API toma la serie de `document_series` por `(cash_register_id, doc_type)` tras verificar que el turno esté abierto y sea de esa caja. Una caja hace a lo sumo una venta cada pocos segundos: la contención real se da solo si varias cajas comparten serie.
- Con 3 ms de lock, una serie soporta ~300 ventas/s en serie; el supuesto por tenant es 0,5/s. Criterio de carga en [`capacity.md`](../capacity.md).
- Sin huecos: si la transacción aborta, el incremento se deshace. Los números de CPE rechazados no se reutilizan (confirmado en la documentación de Khipu). Si Khipu rechaza con `422` antes de numerar, el documento se reenvía con el mismo número (ADR 0003).
- El número viaja a Khipu en `correlativo`; las series de Wanka se registran en Khipu con `correlativo_inicial` y son exclusivas. Khipu procesa en orden las emisiones de una misma serie, así que tener una serie por caja también reparte esa cola.
- Las numeraciones internas (proformas, OC, Z, OT) usan el mismo mecanismo.

## Consecuencias

- Buenas: exactitud tributaria; correlativos sin huecos ni duplicados (respaldado por `UNIQUE`); un solo mecanismo para todas las series.
- Malas: el frontend debe tratar importes como string (cambio respecto a los mocks; `lib/format.ts` y `parseNumber` ya existen); serie compartida por muchas cajas serializa; `numeric` es más lento que enteros (irrelevante a este volumen).
- Qué haría cambiar la decisión: espera de lock en la serie p95 > 20 ms medida en producción ⇒ partir (más series) o, si la serie compartida es requisito del cliente, asignación diferida (opción 3) con número provisional.

## Nota del 2026-11-19 (architect, épica #4)

Correcciones que no cambian la decisión (detalle en el [ADR 0013](0013-khipu-company-config-sync.md)):

- **GRE (tipo 09): excepción documentada a «un solo mecanismo para todas las series».** La numera Khipu (`EmitirGuiaRemisionService.java:64`; su endpoint no admite `correlativo`), como ya decían el ADR 0003 y [data-model §3](../data/data-model.md). `empresas.NextNumber` nunca numera el 09, y la ausencia de huecos en el 09 es responsabilidad de Khipu. Todas las demás series, de CPE e internas, siguen este ADR.
- **Huecos:** Khipu acepta saltos (`GREATEST(ultimo_numero, n)`; solo rechaza el repetido). La ausencia de huecos en las series que numera Wanka depende **solo** de este mecanismo; Khipu no la vigila.
- **«Exclusivas»:** Khipu no marca dueño en la serie. La exclusividad la sostiene Wanka, y una serie de CPE solo numera si está sincronizada con Khipu y sin conflicto: el `UPDATE` de la serie suma esas condiciones, sobre la misma fila y con el mismo lock (ADR 0013, §3 y §4).
