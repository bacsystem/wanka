# ADR 0006: Trabajo asíncrono con outbox transaccional en PostgreSQL (River)

- Estado: propuesto (River quedó aprobado por el dueño el 2026-11-17, ADR 0002)
- Fecha: 2026-11-17

## Contexto

Necesitan ejecución diferida y garantizada: emisión y conciliación de CPE (ADR 0003), procesamiento de webhooks, notificaciones (invitaciones, recordatorios por km del lubricentro, citas), PDF de reportes Z y exportes, sincronización SIRE, agregados para la consola (`platform.tenant_usage`) y limpiezas (sesiones, idempotencia, reservas). El encolado debe ser **atómico con el cambio de datos** (si la venta se confirma, su emisión queda encolada; si aborta, no). Con varias réplicas, los periódicos no deben correr N veces.

## Opciones

1. **Broker externo (RabbitMQ, SQS, Redis Streams)**: otro servicio que operar en Railway, y el problema de doble escritura (base + broker) obliga igualmente a un outbox.
2. **Outbox propio en Postgres + worker con `FOR UPDATE SKIP LOCKED`**: sin dependencias, pero hay que escribir reintentos, backoff, jobs únicos, periódicos con elección de líder, limpieza y UI de fallidos.
3. **River** (cola de jobs para Go sobre Postgres): `InsertTx` en la misma transacción, `SKIP LOCKED`, reintentos con backoff exponencial y jitter, máximo de intentos, jobs únicos por clave, colas con concurrencia propia, periódicos con líder único, limpieza y estado `discarded` visible.

## Decisión

Opción 3; si la dependencia no se aprueba, opción 2 con la misma semántica (la interfaz `jobs.Enqueuer` del kernel oculta cuál se usa).

- Colas con concurrencia explícita por réplica de worker (bulkhead): `cpe` 20, `webhooks` 10, `notifications` 10, `reports` 2, `maintenance` 2. Ajustables por variable de entorno.
- **Conexiones frente a concurrencia.** Ningún job mantiene una conexión durante una llamada de red: toma una conexión del pool solo en sus transacciones cortas (unos 5-20 ms por documento), y la llamada HTTP corre sin conexión. Por eso 44 jobs concurrentes caben en un pool de 10. Ley de Little: 100 CPE/s × 2 transacciones × 10 ms ≈ 2 conexiones en uso, más 1 para el `LISTEN` y el fetch de River. El tamaño del pool del worker (`DB_POOL_MAX_CONNS=10`) se valida con la prueba de carga de ADR 0003 (espera de `pgxpool` p99 < 50 ms).
- **Orden por serie en la cola `cpe`.** River no garantiza FIFO por clave. Khipu, en cambio, procesa en orden lo que recibe de cada serie, y Wanka numera. Para que el orden de llegada sea el de los números, `cpe.emit` **no usa unicidad de River**: River exige que `UniqueOpts.ByState` incluya `running` (`insert_opts.go`), y con eso las ventas que llegan durante un drenaje se deduplicarían contra el drenador. La exclusión la da un **arrendamiento en fila con fencing** (`einv.series_emitters`, 60 s, renovado antes de cada llamada con `WHERE holder = $job`, y se aborta si afecta 0 filas). El drenaje va de menor a mayor número, con `COMMIT` por documento y sin transacción abierta durante la llamada a Khipu. Un job que no obtiene el arrendamiento termina enseguida. Si la cabeza está en backoff, el drenador se detiene y hace `JobSnooze` hasta `next_attempt_at`, uno solo por serie gracias a `wakeup_at`. Después de soltar el arrendamiento, el drenador vuelve a leer y sigue si encuentra pendientes (ADR 0003, «Numeración y orden por serie»). Los artefactos se suben a S3 en el job aparte `cpe.fetch_artifacts`. Las 20 tareas concurrentes trabajan en **series distintas**: dentro de una serie hay un único emisor. La política ante un hueco (seguir con N+1 o detener la serie) queda sujeta a la verificación 2 en BETA, antes del #56.
- Entrega **al menos una vez**: todo handler es idempotente (transiciones condicionadas por estado, claves únicas, `idem_key` hacia terceros).
- Reintentos solo en errores transitorios, en esta única capa (el adaptador HTTP no reintenta por su cuenta). Errores definitivos ⇒ `JobCancel` y estado de negocio explícito (p. ej. CPE `rechazado`).
- Jobs fallidos definitivamente (`discarded`) se ven en la consola de plataforma (monitoreo) y generan alerta; un job que falla siempre no bloquea a los demás (no hay orden FIFO estricto por tenant, salvo el drenaje en orden de cada serie de CPE, descrito arriba, y los jobs únicos por clave, como un solo `cpe.void` en curso por documento).
- Periódicos (`cpe.reconcile` cada minuto, recordatorios diarios 08:00 `America/Lima`, limpiezas) con el líder de River: corren una vez aunque haya N réplicas.
- Todo job lleva `tenant_id` en sus argumentos y abre `InTenantTx` (ADR 0001).
- La tabla de jobs vive en el esquema `ops` (no de tenant) con retención: completados 7 días, descartados 30 días.

## Consecuencias

- Buenas: atomicidad sin doble escritura; un solo backing service (Postgres) en Railway; observabilidad por SQL.
- Malas: la cola compite con el tráfico OLTP por la misma base (a 100 jobs/s es poco; a medir en la prueba de carga); acoplamiento a una librería (mitigado por la interfaz del kernel).
- Qué haría cambiar la decisión: volumen de jobs que degrade la base (medido: p. ej. > 1 000 jobs/s sostenidos o autovacuum incapaz de seguir la tabla de jobs) o necesidad de fan-out a sistemas externos: entonces outbox + broker dedicado.
