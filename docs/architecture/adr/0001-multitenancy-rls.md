# ADR 0001: Multi-tenancy con base única, `tenant_id` y Row-Level Security

- Estado: aceptado (decisión del usuario); los detalles de implementación son una propuesta
- Fecha: 2026-11-17 (revisado tras la revisión del PR #124: caminos que RLS bloqueaba, `NULLIF`, matriz de permisos por rol)

## Contexto

Wanka es multiempresa: un usuario pertenece a N empresas (tenants) con un rol en cada una, y la consola super-admin opera sobre todas. Hoy el frontend elige el tenant con una cookie `tenant_id` sin autorización (`apps/web/src/features/auth/lib/tenant-cookie.ts`, leída en `app/(app)/layout.tsx`). Una fuga de datos entre empresas (comprobantes, clientes, historias clínicas) es el peor incidente posible del producto. Carga supuesta: 2 000 tenants el primer año ([`capacity.md`](../capacity.md)).

## Opciones

1. **Una base por tenant**: aislamiento máximo, pero 2 000 bases, migraciones ×2 000 y un pool por base. Inviable en Railway y caro de operar.
2. **Un esquema por tenant**: las mismas desventajas de migración, más un catálogo de Postgres con miles de esquemas.
3. **Base única con `tenant_id` filtrado solo en la aplicación**: simple, pero un `WHERE` olvidado es una fuga.
4. **Base única con `tenant_id` + RLS** (decisión del usuario): la base rechaza las filas de otro tenant aunque la aplicación se equivoque.

## Decisión

Opción 4, con las reglas siguientes.

### Política estándar

```sql
CREATE FUNCTION app.current_tenant() RETURNS uuid LANGUAGE sql STABLE AS
  $$ SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid $$;
CREATE FUNCTION app.current_user_id() RETURNS uuid LANGUAGE sql STABLE AS
  $$ SELECT NULLIF(current_setting('app.user_id', true), '')::uuid $$;

ALTER TABLE x ENABLE ROW LEVEL SECURITY;
ALTER TABLE x FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON x
  USING (tenant_id = app.current_tenant())
  WITH CHECK (tenant_id = app.current_tenant());
```

`NULLIF(..., '')` es obligatorio. Cuando `set_config(..., true)` termina con la transacción, el parámetro sigue *definido* en esa conexión del pool y vale `''`. Por eso `current_setting('app.tenant_id', true)::uuid` fallaría con `invalid input syntax for type uuid`, en vez de devolver NULL. Con `NULLIF`, una conexión sin tenant ve **cero filas** y no puede insertar: el sistema falla cerrado sin errores.

### Cómo lo fija Go

- Hay un pool `pgxpool` por proceso. Todo acceso a datos de tenant pasa por un único helper:
  ```go
  func (db *DB) InTenantTx(ctx context.Context, tenantID, userID uuid.UUID, fn func(pgx.Tx) error) error
  // BEGIN; SELECT set_config('app.tenant_id', $1, true), set_config('app.user_id', $2, true); fn(tx); COMMIT
  ```
  `set_config(..., true)` equivale a `SET LOCAL`: el valor vale hasta el fin de la transacción. No queda estado entre peticiones, así que es compatible con pooling por transacción (PgBouncer en modo `transaction`, si se agrega).
- El `tenant_id` sale de la sesión autenticada (ADR 0004). Nunca se toma del cuerpo, de la URL ni de una cabecera.
- Las lecturas también van en transacción (`BEGIN READ ONLY`).
- Un linter de arquitectura en CI verifica que ningún paquete fuera de `internal/platform/db` use el pool directamente.
- Las FK son compuestas con `tenant_id`, y **toda restricción `UNIQUE` o PK natural de una tabla de tenant empieza por `tenant_id`**. RLS no filtra los índices únicos: una `UNIQUE (code)` global dejaría saber, por un error de duplicado, que otro tenant tiene ese código. La única excepción son las PK `uuid` que genera el servidor, porque el cliente no las elige y no se puede sondearlas. Las UK globales a propósito están marcadas en los ER (RUC de la empresa, email del usuario, hashes de tokens, `route_id`) y no son de tenant.

### Roles de base de datos y permisos (GRANT)

Ningún rol de aplicación tiene `BYPASSRLS` ni es dueño de tablas. Todos son `NOLOGIN` salvo por su usuario de conexión.

| Esquema / objeto | `wanka_migrator` | `wanka_app` (api) | `wanka_worker` | `wanka_webhooks` | `wanka_platform` (platform-api) |
|---|---|---|---|---|---|
| Tablas de tenant (`org`*, `crm`, `catalog`, `inv`, `sales`, `einv`, `purch`, `fin`, `agenda`, `integ`, `notify`, `audit`, rubros) | dueño | SELECT/INSERT/UPDATE (DELETE solo donde el módulo lo declara), **con RLS** | igual, con RLS por job | INSERT en `einv.provider_webhook_events`, SELECT en `integ.provider_credentials`, con RLS | **ninguno directo** |
| `org.tenants` | dueño | SELECT con RLS (política extra por membresía) | SELECT con RLS | — | SELECT/UPDATE vía política `TO wanka_platform USING (true)` |
| `iam.users`, `iam.sessions`, `iam.user_totp`, `iam.password_resets` (sin RLS: identidad global) | dueño | SELECT/INSERT/UPDATE | DELETE para limpieza | — | — |
| `iam.memberships`, `iam.membership_sites`, `iam.roles`, `iam.role_permissions` | dueño | con RLS + política por `app.user_id` (solo SELECT) | con RLS | — | — |
| `iam.permissions` (catálogo global, sin RLS) | dueño | SELECT | SELECT | — | SELECT |
| `platform.*` (sin RLS) | dueño | SELECT solo en `plans`, `plan_modules`, `modules`, `feature_flags`, `tenant_flag_overrides`, `tenant_module_overrides`, `subscriptions` (entitlements) | INSERT/UPDATE solo en `platform.tenant_usage`; SELECT de entitlements | — | SELECT/INSERT/UPDATE |
| `ops.webhook_routes` (sin RLS) | dueño | INSERT/UPDATE al conectar Khipu | — | SELECT | SELECT |
| `ops.idempotency_keys` (con RLS), `ops.exchange_rates` (global) | dueño | DML / SELECT | DELETE (limpieza) / INSERT (tipo de cambio) | — | SELECT |
| Tablas de River (`ops`) | dueño | INSERT (encolar) | todo | INSERT (encolar) | INSERT (encolar) |
| Funciones `SECURITY DEFINER` (`platform_fn.*`, `iam.*`) | dueño | EXECUTE de `iam.redeem_impersonation` | EXECUTE de `platform_fn.purge_closed_tenant` | EXECUTE de `ops.resolve_webhook_route` | EXECUTE de `platform_fn.provision_tenant`, `suspend_tenant`, `create_impersonation_grant` |

\* `org.tenants` tiene la fila propia del tenant y se trata aparte (siguiente sección).

### Caminos que no tienen un tenant fijado

La revisión los reprodujo en PG16 (cero filas o `new row violates row-level security policy`). Cada uno tiene ahora un mecanismo acotado:

1. **Login y `/seleccionar-empresa`** (el usuario todavía no eligió empresa).
   - `iam.memberships` y `iam.membership_sites` tienen una segunda política, solo de lectura: `CREATE POLICY own_memberships ON iam.memberships FOR SELECT USING (user_id = app.current_user_id())`.
   - `org.tenants` admite `FOR SELECT USING (id = app.current_tenant() OR id IN (SELECT tenant_id FROM iam.memberships WHERE user_id = app.current_user_id()))`.
   - La API abre la transacción con solo `app.user_id` (sin `app.tenant_id`) y llama a `iam.list_my_memberships()`. Es una vista `security_invoker` que une membresías, empresa, rol, sedes y cajas, y solo ve lo del propio usuario.
   - `POST /v1/auth/session/tenant` valida la membresía por la misma política antes de fijar el tenant.
2. **Webhook de Khipu** (`/webhooks/khipu/{routeId}`).
   - La ruta se resuelve con `ops.webhook_routes`, una tabla **sin RLS** que solo contiene `route_id` (128 bits aleatorios), `tenant_id`, `provider` y `revoked_at`. El servicio `webhooks` la lee mediante la función `ops.resolve_webhook_route(route_id)`, que devuelve el `tenant_id` o NULL.
   - Después abre `InTenantTx(tenant)`, lee el `whsec_` en `integ.provider_credentials` (con RLS), verifica el HMAC e inserta el evento.
   - Conocer un `route_id` no da acceso a datos; solo permite intentar una firma que fallará sin el secreto.
3. **Auditoría de acciones de plataforma.** `audit.events.tenant_id` es `NOT NULL` y está protegido con RLS.
   - Las acciones del staff van a `platform.audit_events`, sin RLS y legible solo por `wanka_platform`.
   - Cuando una acción del staff afecta a un tenant (alta, suspensión, impersonación), la función `SECURITY DEFINER` que la ejecuta escribe **además** una fila en `audit.events` con el `tenant_id` destino y `actor_type = platform_staff`. Así la empresa ve quién entró.
4. **Uso por tenant para la consola.** `wanka_worker` tiene GRANT `INSERT, UPDATE` solo en `platform.tenant_usage`. El job que cuenta los CPE de cada tenant escribe ahí en una transacción aparte, sin datos de negocio.
5. **Alta de tenant desde la consola.** `wanka_platform` no tiene DML en `iam` ni en `org`.
   - Todo el alta ocurre en `platform_fn.provision_tenant(ruc, razon_social, rubro, plan, owner_email)`: `SECURITY DEFINER`, `SET search_path = pg_catalog, platform_fn`, nombres cualificados, validación de argumentos y una sola transacción.
   - La función fija `app.tenant_id` al tenant nuevo y crea: `org.tenants`, la sede, el almacén, la caja, las series, los roles plantilla con sus permisos, `iam.users` (si el email no existe), la membresía del propietario en estado `invitado` y las dos filas de auditoría.
   - Devuelve el `tenant_id` y un token de invitación.
6. **Impersonación de soporte.**
   - `platform_fn.create_impersonation_grant(ticket_id, tenant_id, scope)` crea el grant (60 min como máximo) con un token de canje de un uso.
   - El plano tenant lo canjea con `iam.redeem_impersonation(token)`, una función `SECURITY DEFINER` que valida el hash, la vigencia y la revocación, y crea una sesión de tenant con `impersonator_id`.
   - Desde ahí todo pasa por RLS normal.
7. **Purga de empresas cerradas** (5 años): la hace `platform_fn.purge_closed_tenant(tenant_id)`, que el worker ejecuta por lotes y con auditoría en plataforma.

Cada función `SECURITY DEFINER` tiene su prueba de integración, que también verifica que no hace nada fuera de su tenant.

### Workers

La tabla de jobs no es de tenant (no tiene RLS). Cada job lleva `tenant_id` en sus argumentos, y el handler abre `InTenantTx(job.TenantID, systemUser)` antes de tocar datos. Los jobs que recorren tenants (recordatorios diarios) obtienen la lista desde `platform.tenant_usage` y las suscripciones, y abren una transacción por tenant.

### Migraciones y pruebas

Las migraciones las aplica `wanka_migrator`. Pruebas de CI contra `pg_catalog`:
- (a) toda tabla con columna `tenant_id` tiene `relrowsecurity` y `relforcerowsecurity`, y una política que usa `app.current_tenant()`;
- (b) ningún índice único de esas tablas omite `tenant_id`, salvo una lista blanca explícita;
- (c) ninguna política usa `current_setting` sin `NULLIF`.

Prueba de integración: crear dos tenants y comprobar que cada repositorio no ve filas del otro, ni siquiera tras devolver la conexión al pool y reutilizarla sin `set_config`. Las siete rutas de la sección anterior también se prueban con los roles reales.

## Consecuencias

- Buenas:
  - una consulta sin filtro no fuga datos;
  - las excepciones son pocas, tienen nombre y se pueden auditar;
  - hay una sola base que migrar y respaldar.
- Malas:
  - cada petición paga un `BEGIN` + `set_config` (unos 0,1 ms, a medir);
  - las consultas analíticas cross-tenant solo se hacen con tablas agregadas;
  - los índices deben empezar por `tenant_id`.
- Riesgo: una función `SECURITY DEFINER` mal escrita es una puerta trasera. Mitigación:
  - hay pocas (las siete listadas) y las revisa el arquitecto;
  - tienen `search_path` fijo;
  - cada una tiene pruebas dedicadas.
- Qué haría cambiar la decisión:
  - Un tenant con un volumen que degrade a los demás. Se movería a su propia base reutilizando el mismo esquema.
  - Un requisito contractual de aislamiento físico.
