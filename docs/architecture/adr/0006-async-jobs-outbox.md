# ADR 0006: Trabajo asíncrono con outbox transaccional en PostgreSQL (River)

- Estado: propuesto (River es una dependencia nueva: requiere aprobación del usuario vía tech-lead)
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
- Entrega **al menos una vez**: todo handler es idempotente (transiciones condicionadas por estado, claves únicas, `idem_key` hacia terceros).
- Reintentos solo en errores transitorios, en esta única capa (el adaptador HTTP no reintenta por su cuenta). Errores definitivos ⇒ `JobCancel` y estado de negocio explícito (p. ej. CPE `rechazado`).
- Jobs fallidos definitivamente (`discarded`) se ven en la consola de plataforma (monitoreo) y generan alerta; un job que falla siempre no bloquea a los demás (no hay orden FIFO estricto por tenant, salvo jobs únicos por clave, p. ej. un solo `cpe.void` en curso por documento).
- Periódicos (`cpe.reconcile` cada minuto, recordatorios diarios 08:00 `America/Lima`, limpiezas) con el líder de River: corren una vez aunque haya N réplicas.
- Todo job lleva `tenant_id` en sus argumentos y abre `InTenantTx` (ADR 0001).
- La tabla de jobs vive en el esquema `ops` (no de tenant) con retención: completados 7 días, descartados 30 días.

## Consecuencias

- Buenas: atomicidad sin doble escritura; un solo backing service (Postgres) en Railway; observabilidad por SQL.
- Malas: la cola compite con el tráfico OLTP por la misma base (a 100 jobs/s es poco; a medir en la prueba de carga); acoplamiento a una librería (mitigado por la interfaz del kernel).
- Qué haría cambiar la decisión: volumen de jobs que degrade la base (medido: p. ej. > 1 000 jobs/s sostenidos o autovacuum incapaz de seguir la tabla de jobs) o necesidad de fan-out a sistemas externos: entonces outbox + broker dedicado.
