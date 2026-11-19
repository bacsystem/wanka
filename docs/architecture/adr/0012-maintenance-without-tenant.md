# ADR 0012: Mantenimiento sin tenant en tablas con RLS (limpieza de `ops.idempotency_keys`)

- Estado: propuesto (architect, 2026-11-19, historia #177). Pasa a aceptado cuando el dueño apruebe el PR de #177.
- Fecha: 2026-11-19
- Modifica: [ADR 0001](0001-multitenancy-rls.md), solo en lo que lista «Qué cambia del ADR 0001». El resto del ADR 0001 sigue vigente. Su texto no se reescribe ([README §9](../README.md)): ante una diferencia, manda este ADR.

## Contexto

`ops.idempotency_keys` es una tabla de tenant con `FORCE ROW LEVEL SECURITY` (migración `apps/api/migrations/20261119170000_kernel_idempotency_keys.sql`). Su retención es de 24 h ([data-model §5](../data/data-model.md)) y la borra un job horario del worker (`apps/api/internal/platform/jobs/purge.go`, `purgeIdempotencyKeys`), que corre por `db.InNoTenantTx`, sin tenant. Sin tenant fijado, `tenant_isolation` no deja ver ninguna fila: el job borraría 0 filas para siempre y la tabla crecería sin límite.

El worker tampoco puede recorrer los tenants: `org.tenants` tiene RLS (`member_read` exige `app.user_id`, `platform_rw` es `TO wanka_platform`), y `platform.tenant_usage`, que el ADR 0001 («Workers») indica para recorrer tenants, no existe todavía en el kernel.

El ADR 0001, prueba (a), solo admite políticas extra `FOR SELECT` de una lista blanca cerrada, con `platform_rw` como única excepción de escritura. La historia #164 implementa esa lista en `apps/api/internal/platform/db/catalog_test.go` (`policyAllow`).

Hay más casos de este tipo en camino: la limpieza de `einv.provider_webhook_events` (90 días), la de `inv.stock_reservations` y el mantenimiento de las particiones de `audit.events` (#173). Por eso este ADR fija un criterio general, no solo el caso de #177.

## Opciones

| Opción | Costo | Riesgo | Reversible | Esfuerzo |
|---|---|---|---|---|
| (a) Dos políticas `TO wanka_worker` (`FOR SELECT` y `FOR DELETE`), acotadas por antigüedad, y enmendar la lista blanca | dos entradas en la lista blanca y una prueba | el worker ve `created_at` de filas vencidas de todos los tenants (solo esa columna, por GRANT) y puede borrarlas. Es inocuo: la retención es un mínimo | sí: `DROP POLICY` en una migración nueva | bajo (ya implementada) |
| (b) Función `SECURITY DEFINER` de `wanka_definer` que purgue | una función más que auditar | **no resuelve el problema**: FORCE RLS también alcanza a `wanka_definer`, que sin tenant ve 0 filas y tampoco puede listar tenants. Necesitaría la misma política, pero `TO wanka_definer`, el rol que es dueño de todas las funciones privilegiadas: es más privilegio en un rol más sensible | sí | medio |
| (c) Purgar por tenant, leyendo la lista por una vía permitida | 2 000 transacciones por hora para 2 000 tenants ([`capacity.md`](../capacity.md)), casi todas sin nada que borrar | depende de `platform.*`, que no está en esta rama (bloquea #177). Un tenant que falte en la lista (cerrado, en provisión o mal sincronizado) deja claves para siempre | sí | medio, y bloqueado |
| (d) Particionar por hora o por día y hacer `DROP` de particiones | particiones que crear por adelantado, y un job con DDL | **rompe la idempotencia**: en PostgreSQL la PK de una tabla particionada debe incluir la clave de partición, así que `(tenant_id, key)` pasaría a ser única **por partición**. La misma clave enviada a las 23:59 y reintentada a las 00:01 caería en otra partición y el efecto se ejecutaría dos veces. Además `DROP`/`DETACH` exige ser dueño de la tabla (`wanka_migrator`), así que también necesitaría un definer | difícil (migración de datos) | alto |
| No hacer nada | ninguno | la tabla crece sin límite (≈ una fila por `POST` con clave) | — | — |

## Decisión

**Opción (a), con una condición más: la rama cross-tenant solo vale sin tenant fijado.**

```sql
CREATE POLICY purge_expired_read ON ops.idempotency_keys FOR SELECT TO wanka_worker
  USING (app.current_tenant() IS NULL AND created_at < now() - interval '24 hours');
CREATE POLICY purge_expired ON ops.idempotency_keys FOR DELETE TO wanka_worker
  USING (app.current_tenant() IS NULL AND created_at < now() - interval '24 hours');
```

- La política de lectura es necesaria porque el `WHERE` de un `DELETE` pasa también por las políticas `FOR SELECT`.
- `app.current_tenant() IS NULL`: es la misma regla que el ADR 0001 aplica a las ramas por usuario. Dentro de `InTenantTx(A)` un job de A no alcanza las claves de B, aunque escriba un `DELETE` sin filtro de tenant. Verificado en PostgreSQL 16 con `rls-check`: con el tenant A fijado, `DELETE ... WHERE created_at < now() - interval '24 hours'` borra 1 fila (la de A) y no 2.
- GRANT de `wanka_worker`: `SELECT (created_at)` y `DELETE`. Nunca `INSERT` ni `UPDATE`, ni `SELECT` de `key`, `request_hash`, `response` o `response_headers`.
- Así, un worker defectuoso **sin tenant** no puede borrar claves vigentes. Con tenant fijado, sí podría borrar las de ese tenant: es la confianza que el ADR 0001 ya da al worker sobre los datos de un tenant («igual, con RLS por job»).

### Criterio general: mantenimiento sin tenant

Vale para todo job de mantenimiento que deba tocar filas de varios tenants sin fijar ninguno. Se elige en este orden:

1. **Por tenant, si se puede.** Si el trabajo tiene sentido de negocio por empresa (recordatorios, uso) y existe una vía permitida para listar los tenants (`platform.tenant_usage`, suscripciones), se abre una `InTenantTx` por tenant, como dice el ADR 0001 («Workers»). No hace falta ninguna política.
2. **Borrado por antigüedad o estado de filas técnicas: política de mantenimiento.** Esto vale para claves, eventos recibidos, reservas vencidas y filas parecidas: datos que caducan, no datos de negocio que alguien consulte. Se usa una política con estas condiciones, todas obligatorias:
   - un nombre `purge_*` y `TO wanka_worker`, el único rol de mantenimiento; nunca `PUBLIC`;
   - solo `FOR SELECT` y `FOR DELETE`. Nunca `INSERT`, `UPDATE` ni `ALL`;
   - `USING` empieza con `app.current_tenant() IS NULL AND`, y el resto solo compara columnas de tiempo o de estado con constantes (`created_at < now() - interval 'N'`, `expires_at < now()`). No lee otras variables `app.*`, no hace subconsultas y no llama funciones del esquema;
   - el GRANT de `SELECT` es por columna y solo cubre las columnas del predicado;
   - la lista blanca de la prueba (a) fija el texto exacto de `pg_policies.qual`, para que ampliar el predicado haga fallar CI. `rls-check` debe tener tres casos: sin tenant no borra filas vigentes; con un tenant fijado no alcanza a otro; y una mutación `USING (true)` o sin `IS NULL` debe detectarse;
   - el job borra en lotes acotados, una transacción por lote, apoyado en un índice del predicado.
3. **DDL o varias tablas: no se resuelve con políticas.** Crear, separar o borrar particiones exige ser dueño de la tabla, y ninguna política da DDL. Ese caso va por una migración o por una función `SECURITY DEFINER` del ADR 0001, y lo decide su propia historia (las particiones de `audit.events`, #173). Lo que sí aplica de este criterio: nunca `BYPASSRLS`, y si la función lee filas sin tenant, la política que lo permita cumple el punto 2.

No se usa una función `SECURITY DEFINER` para un `DELETE` por antigüedad: no evita la política (FORCE RLS alcanza al definer) y concentraría el privilegio en `wanka_definer`.

### Qué cambia del ADR 0001

- **Prueba (a), lista blanca.** Las políticas extra son las del ADR 0001 más las de mantenimiento que cumplen el punto 2 del criterio. Las de hoy: `purge_expired_read` (`FOR SELECT`) y `purge_expired` (`FOR DELETE`) en `ops.idempotency_keys`, `TO wanka_worker`. La frase «las demás políticas solo pueden ser `FOR SELECT`» pasa a decir: «solo `FOR SELECT`, salvo `platform_rw` y las `purge_*` de mantenimiento (`FOR DELETE`)».
- **Matriz de GRANT, fila de `ops.idempotency_keys`** (la de `ops.exchange_rates` no cambia):

| | `wanka_app` | `wanka_worker` | `wanka_webhooks` | `wanka_platform` |
|---|---|---|---|---|
| `ops.idempotency_keys` (con RLS) | `SELECT`; `INSERT (tenant_id, key, request_hash)`; `UPDATE (status_code, response, response_headers)`; **sin `DELETE`**. `principal_id` y `created_at` los fija el servidor (`DEFAULT`), conventions §3 | `SELECT (created_at)` y `DELETE`, a través de `purge_expired_read` y `purge_expired` | — | **ninguno**: sin tenant fijado no vería ninguna fila, y la consola no necesita las claves |

## Consecuencias

- Buenas:
  - la limpieza es una sentencia por lote, sin depender de `platform.*`, y no bloquea #177;
  - la excepción tiene nombre, rol, comando y predicado fijos, y CI la vigila;
  - el criterio deja resuelta la siguiente tabla que caduca, sin otro ADR.
- Malas:
  - la lista blanca deja de ser «solo lectura más `platform_rw`»: hay una segunda clase de excepción de escritura, que hay que revisar en cada PR que agregue una;
  - sin tenant, el worker puede contar cuántas filas vencidas hay y ver sus `created_at`, de todos los tenants (no ve de qué tenant son);
  - con tenant fijado, el worker puede borrar claves vigentes de ese tenant. Es un error de código, no de aislamiento.
- Qué haría cambiar la decisión:
  - Si el `DELETE` por lotes no alcanza el ritmo de llegada, la decisión se revisa. Hoy no está medido: la referencia es 100 ventas/s en el pico ([`capacity.md`](../capacity.md) §1), y la prueba de carga de #177 debe medir la duración del lote de 1 000 filas. Particionar sigue sin servir por el problema de la PK. La alternativa sería purgar más seguido.
  - Si llega un rol de mantenimiento distinto de `wanka_worker`, las políticas se mueven a ese rol.
