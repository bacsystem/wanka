# Capacidad, concurrencia y caminos calientes

Estado: propuesto (2026-11-17). La carga por tenant (50 usuarios concurrentes, 30 ventas por minuto, p95 < 800 ms) está **aceptada por el usuario**; las cifras de toda la plataforma son **supuestos del arquitecto**. Ninguna es una medición. Las decisiones que dependen de ellas quedan sujetas a la prueba de carga descrita al final.

## 1. Carga objetivo

| Supuesto | Valor | Fuente |
|---|---|---|
| Tenants activos (año 1) | 2 000 | supuesto del arquitecto |
| Usuarios concurrentes por tenant (pico) | 50 | aceptado por el usuario |
| Ventas por tenant en el pico | 30 por minuto (0,5/s) | aceptado por el usuario |
| Fracción de tenants en pico a la vez | 10 % (200 tenants) | supuesto: horarios de almuerzo y salida escolar se solapan, no todos |
| Ventas POS en el pico, toda la plataforma | 200 × 0,5 = **~100/s de diseño**; 30/s esperado | 30/s es el supuesto inicial de año 1, 100/s el margen para el que se valida |
| Peticiones API totales en el pico | ~10 lecturas por venta → **~300 req/s esperado, 1 000 req/s de diseño** | estimado |
| Datos al año 1 | ~2 000 tenants × 30 000 CPE/año ≈ 60 M filas en `electronic_documents`; 4-5 veces eso en líneas y movimientos | estimado |
| p95 de la petición `POST /v1/sales` (commit, sin emisión) | **< 300 ms** | objetivo del arquitecto |
| p95 de cualquier petición de usuario | < 800 ms | aceptado por el usuario |
| Emisión CPE extremo a extremo (venta → aceptado), Khipu sano | p95 < 60 s; alerta si un CPE pendiente supera 15 min | objetivo |

### De los números al tamaño (ley de Little)

- API: 300 req/s × 50 ms medios ≈ **15 peticiones en vuelo**; a 1 000 req/s ≈ 50. Una réplica de Go con 2 vCPU atiende eso si el cuello no es la base.
- Conexiones a Postgres: cada petición usa la conexión solo durante su transacción (~10-30 ms). 300 req/s × 20 ms ≈ **6 conexiones activas**; a 1 000 req/s ≈ 20. Pool explícito por réplica: `api` 20, `worker` 10, `webhooks` 5, `platform-api` 5 → 40 en total, más 1 del pre-deploy (ADR 0007). Los jobs no retienen conexiones durante las llamadas HTTP (ADR 0006), dentro de "unas pocas por núcleo" de una base de 8 vCPU. El total se reparte entre réplicas: con 2 réplicas de `api`, 10 cada una.
- Worker CPE: Khipu no tiene tope por segundo en emisión, pero cada llamada incluye la ida a SUNAT, y una misma serie se procesa en orden (ADR 0003). 100 emisiones/s con 1-3 s por llamada (supuesto, a medir en BETA) dan entre 100 y 300 llamadas en vuelo. La concurrencia de la cola `cpe` es explícita (inicial 20 por réplica) y se sube midiendo. El límite real lo pone la **cuota mensual del plan de Khipu**, que se suma entre las empresas de la cuenta; se monitorea con alerta al 80 %.

## 2. Caminos calientes y su criterio de carga

| Camino | Recurso compartido | Riesgo | Mitigación de diseño | Criterio de carga (debe pasar antes del release) |
|---|---|---|---|---|
| `POST /v1/sales` | filas de `stock_levels`, fila de `document_series`, pool de conexiones | contención por lock, deadlocks | transacción corta sin red; locks de stock en orden de `variant_id`; serie al final; una serie por caja | 50 usuarios virtuales por tenant en 20 tenants (una cuenta por VU), rampa a 100 ventas/s: p95 < 300 ms, errores < 1 %, 0 deadlocks, 0 números duplicados o con huecos |
| Numeración de una serie | `org.document_series` (una fila) | serializa todas las ventas de esa serie | lock al final de la tx; serie por caja | 20 ventas concurrentes sobre **la misma** serie: espera de lock p95 < 20 ms, correlativos 1..N sin huecos ni repetidos tras 10 000 ventas con 5 % de abortos forzados |
| Emisión CPE (worker) | cola River, Khipu | backlog, reintentos que amplifican caída | concurrencia limitada por cola, backoff exponencial con jitter, una sola capa de reintento (River), breaker si se mide necesario | 100 CPE/s durante 10 min contra un doble de Khipu con 1 s de latencia: backlog estable; con el doble caído 5 min: ningún 500 al usuario, backlog drena en < 10 min al volver |
| Búsqueda POS (`/v1/catalog/lookup`, `pos-items`) | índices de catálogo | latencia percibida del cajero | índices `(tenant_id, sku)`, `(tenant_id, code)` de barcodes, trigram para `q` | 200 búsquedas/s: p95 < 100 ms |
| Login | CPU por argon2id | CPU saturada por fuerza bruta | rate limit por IP+email; semáforo de hashing = nº de núcleos; 429 rápido | 50 logins/s: p95 < 800 ms, el resto de endpoints no degrada > 10 % |
| Listados de comprobantes | `electronic_documents` (60 M filas) | `OFFSET`, conteos | cursor + índice `(tenant_id, issue_date DESC, id DESC)`; summary aparte | p95 < 300 ms en un tenant con 500 000 CPE |
| Webhooks Khipu | `/webhooks/khipu/{routeId}` | ráfagas | solo verificar, guardar y encolar (< 20 ms) | 300 webhooks/s sin errores |

Las cuentas: **una cuenta por usuario virtual** (si no, se mide el lock de una sola sesión), generador de carga en una máquina distinta del servidor, y corrida de control en la misma tanda. Herramienta propuesta: k6 (scripts en el repo; decisión del tech-lead).

### Sobrecarga medida de `InTenantTx`

Medida en #163 con `BenchmarkAutocommitQuery`, `BenchmarkTxBaseline` y `BenchmarkInTenantTx` (`apps/api/internal/platform/db/tenant_bench_test.go`), contra PostgreSQL 16 real en Docker (macOS, Apple M4 Max, loopback; cada ida y vuelta de red cuesta ~0,2 ms en este entorno), una consulta `SELECT 1` por transacción, un solo cliente, mediana de 8 corridas de 2 000 iteraciones:

| Caso | Tiempo por operación | Idas y vueltas |
|---|---|---|
| Consulta suelta (autocommit) | ~0,24 ms | 1 |
| `BEGIN` + consulta + `COMMIT`, sin variables | ~0,66 ms | 3 |
| `InTenantTx` (`BEGIN` + `set_config` ×4 + consulta + `COMMIT`) | ~0,87 ms | 4 |

El `set_config` suma **~0,2 ms** sobre una transacción normal, es decir, una ida y vuelta: en el servidor el `SELECT set_config(...)` de las cuatro variables tarda 0,044 ms (`EXPLAIN ANALYZE`). El coste depende de la latencia de red entre la API y la base, no del cómputo: en el mismo entorno de Railway será menor. Es la referencia para el presupuesto de ~10-30 ms por transacción de arriba; hay que repetirla en la prueba de carga. Si hiciera falta recortarlo, `BEGIN` y `set_config` pueden ir en una sola ida y vuelta con un lote de pgx; hoy no se justifica.

## 3. Degradación y backpressure

- Pool de conexiones lleno más de 200 ms → la API responde `503 overloaded` con `Retry-After: 1` en vez de encolar indefinidamente (`pgxpool` con `context` de 200 ms para adquirir conexión).
- Cola CPE con edad del más antiguo > 15 min → alerta; > `send_deadline - 24 h` → alerta crítica al tenant y a soporte.
- Khipu caído → las ventas siguen (CPE `pendiente`), el ticket se imprime con estado pendiente (decisión del usuario), el dashboard muestra la cola.
- Descargas de artefactos: se traen de Khipu al momento, por streaming y sin guardarse, con 2 simultáneas por tenant y 16 por réplica de `api`. Exportes (CSV por decisión del dueño, más el TXT del PLE que exige SUNAT; sin PDF ni XLSX): se generan al momento, con 2 por tenant y 4 por réplica, como máximo un mes y 50 000 filas, con `statement_timeout` de 20 s por lote del cursor y el exporte completo acotado por el `context` de 25 s. Todo con un tope de 25 s desde la llegada (ADR 0003, «Artefactos: Khipu los conserva»). No hay 202 ni notificación.

## 4. Observabilidad mínima

- RED por endpoint (tasa, errores, p95/p99) y saturación: conexiones en uso/espera de `pgxpool`, profundidad y edad de cada cola River, goroutines, CPU, memoria.
- `X-Request-Id` en cada log (JSON, `log/slog`) y propagado a Khipu como cabecera si lo admite.
- Métricas de negocio con alerta: CPE pendientes por antigüedad, tasa de rechazo, latencia de Khipu, sesiones fallidas por minuto.
- Exportación OTLP con el SDK de OpenTelemetry (aprobado, ADR 0002). Falta elegir el backend de observabilidad que recibe las trazas y métricas, un servicio externo que requiere aprobación del dueño (README §10, pregunta 15).
