# ADR 0001: Multi-tenancy con base única, `tenant_id` y Row-Level Security

- Estado: aceptado (decisión del usuario; detalles de implementación propuestos)
- Fecha: 2026-11-17

## Contexto

Wanka es multiempresa: un usuario pertenece a N empresas (tenants) con un rol en cada una, y la consola super-admin opera sobre todas. Hoy el frontend elige el tenant con una cookie `tenant_id` sin autorización (`apps/web/src/features/auth/lib/tenant-cookie.ts`, leída en `app/(app)/layout.tsx`). Una fuga de datos entre empresas (comprobantes, clientes, historias clínicas) es el peor incidente posible del producto. Carga supuesta: 2 000 tenants en el año 1 ([`capacity.md`](../capacity.md)).

## Opciones

1. **Base por tenant**: aislamiento máximo; 2 000 bases, migraciones ×2 000, pools por base. Inviable en Railway y caro de operar.
2. **Esquema por tenant**: mismas desventajas de migración y de catálogo de Postgres con miles de esquemas.
3. **Base única con `tenant_id` filtrado solo en la aplicación**: simple, pero un `WHERE` olvidado es una fuga.
4. **Base única con `tenant_id` + RLS** (decisión del usuario): la base rechaza filas de otro tenant aunque la aplicación se equivoque.

## Decisión

Opción 4, con estas reglas:

**Roles de base de datos**
| Rol | Atributos | Uso |
|---|---|---|
| `wanka_migrator` | dueño de esquemas y tablas | solo migraciones (pre-deploy) |
| `wanka_app` | `NOBYPASSRLS`, sin ser dueño, DML en esquemas de tenant | `api` |
| `wanka_worker` | `NOBYPASSRLS`; además DML en la tabla de jobs de River y `ops` | `worker` |
| `wanka_platform` | `NOBYPASSRLS`; DML en `platform`, `org.tenants`; `EXECUTE` en funciones `SECURITY DEFINER` acotadas | `platform-api` |

Ningún rol de aplicación tiene `BYPASSRLS` ni es dueño de tablas de tenant. Todas las tablas de tenant tienen `ENABLE` y `FORCE ROW LEVEL SECURITY` y la política `tenant_id = current_setting('app.tenant_id', true)::uuid` en `USING` y `WITH CHECK`.

**Cómo lo fija Go**
- Pool `pgxpool` por proceso. Todo acceso a datos de tenant pasa por un único helper:
  ```go
  func (db *DB) InTenantTx(ctx context.Context, tenantID uuid.UUID, fn func(pgx.Tx) error) error
  // BEGIN; SELECT set_config('app.tenant_id', $1, true); SELECT set_config('app.user_id', $2, true); fn(tx); COMMIT
  ```
  `set_config(..., true)` equivale a `SET LOCAL`: vive solo hasta el fin de la transacción. Al devolverse la conexión al pool no queda estado; por eso es compatible con pooling por transacción (PgBouncer en modo `transaction`, si algún día se agrega) y no hay fugas entre peticiones que reutilizan la conexión.
- El `tenant_id` sale de la sesión autenticada (ver ADR 0004), nunca del cuerpo, de la URL ni de una cabecera.
- Lecturas también van en transacción (`BEGIN READ ONLY`): un `SELECT` fuera de `InTenantTx` no ve nada (falla cerrado) y se detecta en pruebas.
- Linter de arquitectura en CI: ningún paquete fuera de `internal/platform/db` usa el pool directamente.
- Las claves foráneas incluyen `tenant_id` (FK compuestas), segunda barrera estructural.

**Workers**: la tabla de jobs no es de tenant (sin RLS). El job lleva `tenant_id` en sus argumentos y el handler abre `InTenantTx(job.TenantID)` antes de tocar datos. Jobs que recorren tenants (recordatorios diarios) iteran la lista de tenants con el módulo activo y abren una transacción por tenant.

**Consola super-admin (cross-tenant)**
- `platform-api` usa `wanka_platform`, que no puede leer tablas de tenant directamente (RLS sin variable = cero filas).
- Lo agregado que la consola necesita (uso de CPE por mes, usuarios activos, estado de certificados, salud de la cola) lo **escribe el worker** en tablas `platform.tenant_usage`/vistas materializadas por evento o job periódico; la consola lee eso, no datos de negocio.
- Operaciones puntuales cross-tenant (aprovisionar un tenant, suspenderlo) son funciones `SECURITY DEFINER` en un esquema `platform_fn`, con `search_path` fijo, que fijan `app.tenant_id` al tenant recibido y hacen solo lo suyo; cada una escribe en `audit.events`.
- Soporte que necesita ver datos de un tenant usa **impersonación**: `platform-api` emite un grant (`platform.impersonation_grants`: ticket obligatorio, alcance `read_only` o `full`, máx. 60 min) y el plano tenant crea una sesión de tenant marcada `impersonator_id`; todas las acciones quedan auditadas con ambos actores y el tenant puede ver el registro. Así el staff accede por la misma puerta y con las mismas políticas RLS que un usuario.

**Migraciones**: las crea `wanka_migrator`; prueba en CI contra `pg_catalog` que falla si una tabla con columna `tenant_id` no tiene RLS habilitado, forzado y con política; prueba de integración que crea dos tenants y verifica que cada repositorio no ve filas del otro.

## Consecuencias

- Buenas: una consulta sin filtro no fuga datos; las políticas son auditables en un solo lugar; una sola base que migrar y respaldar.
- Malas: cada petición paga un `BEGIN` + `set_config` (~0,1 ms, a medir); las consultas analíticas cross-tenant no son posibles desde la app (se resuelven con tablas agregadas); el planificador añade el predicado de la política: los índices deben empezar por `tenant_id` para que no cueste.
- Riesgo: una función `SECURITY DEFINER` mal escrita es una puerta trasera. Mitigación: pocas, revisadas por el arquitecto, `search_path` fijo, pruebas dedicadas.
- Qué haría cambiar la decisión: un tenant con volumen que degrade a los demás (se movería a su propia base reutilizando el mismo esquema, porque el código no asume base única más allá del pool), o un requisito contractual de aislamiento físico.
