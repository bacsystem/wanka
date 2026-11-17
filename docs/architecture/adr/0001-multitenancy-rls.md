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
  type Actor struct {
      TenantID, UserID uuid.UUID
      Kind             SessionKind // user | impersonated | system (jobs)
      ImpersonatorID   *uuid.UUID  // solo si Kind == impersonated
  }
  func (db *DB) InTenantTx(ctx context.Context, a Actor, fn func(pgx.Tx) error) error
  // BEGIN;
  // SELECT set_config('app.tenant_id', $1, true), set_config('app.user_id', $2, true),
  //        set_config('app.session_kind', $3, true), set_config('app.impersonator_id', coalesce($4, ''), true);
  // fn(tx); COMMIT
  ```
  `InTenantTx` **siempre** fija `app.session_kind`, y no compila sin un `Actor` con `Kind`: no hay un camino que deje la variable vacía. Las funciones `SECURITY DEFINER` que dependen del tipo de sesión **fallan cerradas**: si `app.session_kind` está vacío o no es el esperado, rechazan.
  `set_config(..., true)` equivale a `SET LOCAL`: el valor vale hasta el fin de la transacción. No queda estado entre peticiones, así que es compatible con pooling por transacción (PgBouncer en modo `transaction`, si se agrega).
- El `tenant_id` sale de la sesión autenticada (ADR 0004). Nunca se toma del cuerpo, de la URL ni de una cabecera.
- Las lecturas también van en transacción (`BEGIN READ ONLY`).
- Un linter de arquitectura en CI verifica que ningún paquete fuera de `internal/platform/db` use el pool directamente.
- Las FK son compuestas con `tenant_id`, y **toda restricción `UNIQUE` o PK natural de una tabla de tenant empieza por `tenant_id`**. RLS no filtra los índices únicos: una `UNIQUE (code)` global dejaría saber, por un error de duplicado, que otro tenant tiene ese código. La única excepción son las PK `uuid` que genera el servidor, porque el cliente no las elige y no se puede sondearlas. Las UK globales a propósito están marcadas en los ER (RUC de la empresa, email del usuario, hashes de tokens, `route_id`) y no son de tenant.

### Roles de base de datos y permisos (GRANT)

Ningún rol de aplicación tiene `BYPASSRLS` ni es dueño de tablas. Todos son `NOLOGIN` salvo por su usuario de conexión. Las funciones `SECURITY DEFINER` tienen su propio dueño, `wanka_definer` (`NOLOGIN`, `NOBYPASSRLS`, sin tablas propias), descrito en la sección «Funciones `SECURITY DEFINER`: dueño y patrón».

| Esquema / objeto | `wanka_migrator` | `wanka_app` (api) | `wanka_worker` | `wanka_webhooks` | `wanka_platform` (platform-api) |
|---|---|---|---|---|---|
| Tablas de tenant (`org`*, `crm`, `catalog`, `inv`, `sales`, `einv`, `purch`, `fin`, `agenda`, `integ`, `notify`, `audit`, rubros) | dueño | SELECT/INSERT/UPDATE (DELETE solo donde el módulo lo declara), **con RLS** | igual, con RLS por job | solo INSERT en `einv.provider_webhook_events` (con RLS). **Sin acceso a `integ.provider_credentials`**: no puede leer las API keys `fk_` | **ninguno directo** |
| `org.tenants` | dueño | SELECT con RLS (política extra por membresía) | SELECT con RLS | — | SELECT/UPDATE vía la política `platform_rw` (`TO wanka_platform USING (true)`) |
| `iam.users`, `iam.sessions`, `iam.user_totp`, `iam.password_resets` (sin RLS: identidad global) | dueño | SELECT/INSERT/UPDATE | DELETE para limpieza | — | — |
| `iam.memberships`, `iam.membership_sites`, `iam.roles`, `org.sites`, `org.cash_registers` | dueño | con RLS + política `member_read`/`own_memberships` por membresía de `app.user_id` (solo SELECT y **solo sin tenant fijado**) | con RLS | — | — |
| `iam.role_permissions` | dueño | con RLS (los permisos se resuelven ya con tenant fijado) | con RLS | — | — |
| `iam.permissions` (catálogo global, sin RLS) | dueño | SELECT | SELECT | — | SELECT |
| `platform.*` (sin RLS) | dueño | SELECT solo en `plans`, `plan_modules`, `modules`, `feature_flags`, `tenant_flag_overrides`, `tenant_module_overrides`, `subscriptions` (entitlements) | INSERT/UPDATE solo en `platform.tenant_usage`; SELECT de entitlements | — | SELECT/INSERT/UPDATE |
| `ops.webhook_routes` (sin RLS; guarda el `whsec_` cifrado con `WEBHOOK_SECRET_ENC_KEY`) | dueño | INSERT/UPDATE al conectar Khipu | — | solo a través de `ops.resolve_webhook_route` (sin SELECT directo) | SELECT de `route_id`, `tenant_id`, `revoked_at` (GRANT por columna, sin el secreto) |
| `ops.idempotency_keys` (con RLS), `ops.exchange_rates` (global) | dueño | DML / SELECT | DELETE (limpieza) / INSERT (tipo de cambio) | — | SELECT |
| Tablas de River (`ops`) | dueño | INSERT (encolar) | todo | INSERT (encolar) | INSERT (encolar) |
| Funciones `SECURITY DEFINER` (`platform_fn.*`, `iam.*`, `ops.*`) | las crea; **dueño: `wanka_definer`** | EXECUTE de `iam.session_lookup` (búsqueda de sesión antes de conocer el tenant, ADR 0004), `iam.redeem_impersonation`, `iam.decide_impersonation`, `iam.list_support_requests` (sin GRANT sobre el esquema `platform`) | EXECUTE de `platform_fn.purge_closed_tenant` | EXECUTE de `ops.resolve_webhook_route` | EXECUTE de `platform_fn.provision_tenant`, `suspend_tenant`, `request_impersonation` |

\* `org.tenants` tiene la fila propia del tenant y se trata aparte (siguiente sección).

### Caminos que no tienen un tenant fijado

La revisión los reprodujo en PG16 (cero filas o `new row violates row-level security policy`). Cada uno tiene ahora un mecanismo acotado:

1. **Login y `/seleccionar-empresa`** (el usuario todavía no eligió empresa).
   - `iam.memberships` y `iam.membership_sites` tienen una segunda política, solo de lectura: `CREATE POLICY own_memberships ON iam.memberships FOR SELECT USING (app.current_tenant() IS NULL AND user_id = app.current_user_id())`.
   - `org.tenants` admite `member_read FOR SELECT USING (id = app.current_tenant() OR (app.current_tenant() IS NULL AND id IN (SELECT tenant_id FROM iam.memberships WHERE user_id = app.current_user_id())))`.
   - **Las ramas por usuario solo valen sin tenant fijado** (`app.current_tenant() IS NULL`). Postgres combina con OR las políticas permisivas: sin esa condición, dentro de `InTenantTx(A)` un usuario que también es miembro de B vería sedes, cajas, roles, membresías y la empresa de B, y un listado o reporte de A sin `WHERE tenant_id` los mezclaría (lo reprodujo la revisión en PG16).
   - **Política `member_read`** (solo `FOR SELECT`) en las tres tablas que la vista necesita y que antes solo tenían `tenant_isolation`. Con ella, la vista devuelve rol, `requires_mfa`, sedes y cajas, y la 2FA obligatoria por rol se puede aplicar:
     ```sql
     -- rol de cada membresia propia
     CREATE POLICY member_read ON iam.roles FOR SELECT USING (
       tenant_id = app.current_tenant()
       OR (app.current_tenant() IS NULL AND id IN (SELECT role_id FROM iam.memberships
                 WHERE user_id = app.current_user_id() AND status IN ('invitado','activo'))));
     -- sedes en el alcance de cada membresia propia
     CREATE POLICY member_read ON org.sites FOR SELECT USING (
       tenant_id = app.current_tenant()
       OR (app.current_tenant() IS NULL AND EXISTS (SELECT 1 FROM iam.memberships m
                  WHERE m.user_id = app.current_user_id() AND m.status = 'activo' AND m.tenant_id = sites.tenant_id
                    AND (m.all_sites OR EXISTS (SELECT 1 FROM iam.membership_sites ms
                                                WHERE ms.membership_id = m.id AND ms.site_id = sites.id)))));
     -- cajas de esas sedes
     CREATE POLICY member_read ON org.cash_registers FOR SELECT USING (
       tenant_id = app.current_tenant()
       OR (app.current_tenant() IS NULL AND site_id IN (SELECT id FROM org.sites)));  -- org.sites ya filtrado por su propia politica
     ```
     Solo exponen lo que el usuario puede elegir: su rol (no los demás roles de la empresa), y sus sedes y cajas. No exponen permisos ni datos de negocio.
   - La API abre la transacción con solo `app.user_id` (sin `app.tenant_id`) y llama a `iam.list_my_memberships()`, una vista `security_invoker` que une membresía, empresa, rol (`name`, `requires_mfa`), sedes y cajas. **Prueba de integración:** (1) con solo `app.user_id` fijado, la vista devuelve rol, `requires_mfa`, sedes y cajas no nulos para las membresías propias, y cero filas de otros usuarios; (2) con un usuario miembro de A y de B y `app.tenant_id = A`, `org.sites`, `org.cash_registers`, `iam.roles`, `iam.memberships` y `org.tenants` devuelven **cero filas de B**.
   - `POST /v1/auth/session/tenant` valida la membresía por la misma política antes de fijar el tenant.
2. **Webhook de Khipu** (`/webhooks/khipu/{routeId}`).
   - La ruta se resuelve con `ops.webhook_routes`, una tabla **sin RLS** con `route_id` (128 bits aleatorios), `tenant_id`, `provider`, `revoked_at` y `webhook_secret_enc` (el `whsec_` cifrado con **`WEBHOOK_SECRET_ENC_KEY`**, una clave distinta de `INTEGRATIONS_ENC_KEY`).
   - El servicio `webhooks` solo puede llamar a `ops.resolve_webhook_route(route_id)` (`SECURITY DEFINER`). La función devuelve `(tenant_id, webhook_secret_enc)` de **esa** ruta, o nada.
   - Con eso verifica el HMAC, abre `InTenantTx(Actor{TenantID: tenant, Kind: system})` e inserta el evento.
   - **Ya no tiene acceso a `integ.provider_credentials`** ni recibe `INTEGRATIONS_ENC_KEY`: aunque se comprometa el único servicio Go expuesto, no se pueden descifrar las API keys `fk_` de nadie. Prueba: con el rol `wanka_webhooks`, `SELECT api_key_enc FROM integ.provider_credentials` debe fallar con `permission denied`.
   - Riesgo residual acotado: el rol puede fijar cualquier `app.tenant_id` e insertar un evento falso en `einv.provider_webhook_events`. Es inocuo, porque `cpe.sync` nunca confía en el contenido del evento: vuelve a consultar el estado en Khipu con la llave del tenant.
   - Conocer un `route_id` no da acceso a datos; solo permite intentar una firma que fallará sin el secreto.
3. **Auditoría de acciones de plataforma.** `audit.events.tenant_id` es `NOT NULL` y está protegido con RLS.
   - Las acciones del staff van a `platform.audit_events`, sin RLS y legible solo por `wanka_platform`.
   - Cuando una acción del staff afecta a un tenant (alta, suspensión, impersonación), la función `SECURITY DEFINER` que la ejecuta escribe **además** una fila en `audit.events` con el `tenant_id` destino y `actor_type = platform_staff`. Así la empresa ve quién entró.
4. **Uso por tenant para la consola.** `wanka_worker` tiene GRANT `INSERT, UPDATE` solo en `platform.tenant_usage`. El job que cuenta los CPE de cada tenant escribe ahí en una transacción aparte, sin datos de negocio.
5. **Alta de tenant desde la consola.** `wanka_platform` no tiene DML en `iam` ni en `org`.
   - Todo el alta ocurre en `platform_fn.provision_tenant(ruc, razon_social, rubro, plan, owner_email)`: `SECURITY DEFINER` con dueño `wanka_definer`, `SET search_path = pg_catalog, pg_temp`, nombres cualificados, validación de argumentos y una sola transacción. Usa el patrón de «Funciones `SECURITY DEFINER`»: fija y restaura `app.tenant_id`.
   - La función fija `app.tenant_id` al tenant nuevo y crea: `org.tenants`, la sede, el almacén, la caja, las series, los roles plantilla con sus permisos, `iam.users` (si el email no existe), la membresía del propietario en estado `invitado` y las dos filas de auditoría.
   - Devuelve el `tenant_id` y un token de invitación.
6. **Impersonación de soporte, con consentimiento de la empresa** (issue #90: «solo con consentimiento de la empresa y queda auditado»).
   - `platform_fn.request_impersonation(ticket_id, tenant_id, scope, reason)` crea el grant en estado **`solicitado`**. Solo se ejecuta con el permiso `plataforma.tenants:impersonate`, y notifica a los usuarios de la empresa con permiso `identidad.support_access:approve` (por defecto, propietario y administrador).
   - La empresa aprueba o rechaza en el plano tenant (`POST /v1/support-access/requests/{id}/approve|reject`). La API verifica el permiso y llama a `iam.decide_impersonation(grant_id, decision)`, una función `SECURITY DEFINER` que solo actúa si `grant.tenant_id = app.current_tenant()`. Guarda `approved_by`, `approved_at` y `consent_expires_at`: el consentimiento dura como máximo 24 h y el acceso, 60 min desde el canje.
   - `iam.redeem_impersonation(token)` **rechaza** todo grant que no esté `aprobado`, que tenga el consentimiento vencido o que haya sido revocado. Si es válido, crea la sesión de tenant con `impersonator_id`. La empresa puede revocar en cualquier momento, y la revocación corta la sesión.
   - Auditoría en `platform.audit_events` y en `audit.events` del tenant: solicitud, decisión, canje, acciones y cierre.
   - **Lectura desde el plano tenant.** Las solicitudes viven en `platform.impersonation_grants`, sin RLS, y `wanka_app` **no tiene GRANT sobre el esquema `platform`**. Un `GRANT SELECT` directo expondría las solicitudes de todos los tenants. `GET /v1/support-access/requests` lee solo a través de `iam.list_support_requests()`:
     ```sql
     CREATE FUNCTION iam.list_support_requests()
       RETURNS TABLE (id uuid, ticket_id uuid, scope text, reason text, status text,
                      requested_at timestamptz, approved_by uuid, approved_at timestamptz,
                      consent_expires_at timestamptz, redeemed_at timestamptz, revoked_at timestamptz)
       LANGUAGE sql STABLE SECURITY DEFINER
       SET search_path = pg_catalog, pg_temp
     AS $$
       SELECT g.id, g.ticket_id, g.scope, g.reason, g.status, g.created_at, g.approved_by,
              g.approved_at, g.consent_expires_at, g.redeemed_at, g.revoked_at
         FROM platform.impersonation_grants g
        WHERE app.current_tenant() IS NOT NULL
          AND g.tenant_id = app.current_tenant()
        ORDER BY g.created_at DESC
        LIMIT 200
     $$;
     ALTER FUNCTION iam.list_support_requests() OWNER TO wanka_definer;
     REVOKE ALL ON FUNCTION iam.list_support_requests() FROM PUBLIC;
     GRANT EXECUTE ON FUNCTION iam.list_support_requests() TO wanka_app;
     ```
     No recibe argumentos: el tenant sale de `app.current_tenant()`, que solo fija `InTenantTx`. Sin tenant fijado devuelve 0 filas. No devuelve el `exchange_token_hash` ni los datos del staff, salvo el ticket. `iam.decide_impersonation` aplica la misma condición.
   - **Una sesión impersonada no puede autorizar accesos de soporte ni gestionar identidad.** La sesión creada por `iam.redeem_impersonation` lleva `impersonator_id`.
     - **Permisos:** los del alcance del grant, nunca los de un rol del tenant. `read_only` tiene solo las acciones `read`. `full` tiene todas las acciones del catálogo **menos** `identidad.support_access:*`, `identidad.users:*`, `identidad.roles:*`, `identidad.sessions:*` y `empresas.einvoicing:update` (credenciales de Khipu).
     - **Doble barrera, que falla cerrada:**
       1. El middleware arma el conjunto de permisos con esa lista de exclusión.
       2. `iam.decide_impersonation` solo actúa si `NULLIF(current_setting('app.session_kind', true), '') = 'user'` **y** `app.impersonator_id` está vacío. Si `app.session_kind` no está fijado, vale `impersonated` o vale `system`, lanza `insufficient_privilege`: **ausencia de la variable = rechazo**, no aprobación. La revisión reprodujo el caso inverso (sin la variable, la versión anterior aprobaba).
     - En la base: `iam.sessions` tiene `CHECK ((mfa_method = 'platform') = (impersonator_id IS NOT NULL))`. Una sesión con 2FA de plataforma es siempre impersonada, y viceversa, así que no puede existir una sesión de staff "normal" en el plano tenant.
     - Así, el staff no puede aprobar su propia solicitud ni otra.
   - Pruebas:
     - canjear un grant `solicitado` o vencido debe fallar;
     - con dos tenants, `iam.list_support_requests()` con `app.tenant_id = A` devuelve solo las solicitudes de A, y 0 filas sin tenant;
     - con el rol `wanka_app`, `SELECT FROM platform.impersonation_grants` da `permission denied`;
     - canjear un grant `full` e intentar aprobar otra solicitud o crear un usuario responde `403 forbidden`;
     - llamar a `iam.decide_impersonation` en una transacción **sin** `app.session_kind`, o con `impersonated` o `system`, falla con `insufficient_privilege`; solo aprueba con `user`;
     - insertar una sesión con `mfa_method = 'platform'` y sin `impersonator_id` viola el `CHECK`.
   - **Pregunta abierta al PO (no decidida):** si se admite una excepción para incidentes P1 sin consentimiento previo (por ejemplo con doble firma de staff y aviso inmediato a la empresa). Mientras no se decida, no hay excepción.
7. **Purga de empresas cerradas** (5 años): la hace `platform_fn.purge_closed_tenant(tenant_id)`, que el worker ejecuta por lotes y con auditoría en plataforma.

### Funciones `SECURITY DEFINER`: dueño y patrón (verificado en PostgreSQL 16)

**El problema.** Una función `SECURITY DEFINER` corre con los privilegios de su dueño, pero **RLS se le sigue aplicando**: ningún rol tiene `BYPASSRLS`, y las tablas de tenant tienen `FORCE ROW LEVEL SECURITY`, que alcanza incluso al dueño de la tabla. Por eso, si la función lee o escribe una tabla con RLS sin un tenant fijado, ve 0 filas o recibe `new row violates row-level security policy`. La revisión lo reprodujo con `iam.session_lookup` en su versión `LANGUAGE sql`. Que el dueño sea superusuario lo "arreglaría", pero eso es justamente lo que este ADR prohíbe.

**Decisión.**
- Todas las funciones `SECURITY DEFINER` tienen un dueño único y explícito: **`wanka_definer`**. Es `NOLOGIN` y `NOBYPASSRLS`, no es dueño de ninguna tabla, y solo tiene los `GRANT` de tabla que sus funciones necesitan.
- La migración, que corre como `wanka_migrator` (miembro de `wanka_definer`), crea la función y hace `ALTER FUNCTION ... OWNER TO wanka_definer`. PostgreSQL exige que el nuevo dueño tenga `CREATE` en el esquema, así que la migración se lo concede solo durante ese cambio y lo revoca enseguida.
- Toda función lleva `SET search_path = pg_catalog, pg_temp`, nombres cualificados y `REVOKE ALL ... FROM PUBLIC`.
- Las funciones que tocan tablas con RLS **para un tenant que el llamador no fijó** siguen siempre el mismo patrón: guardan `current_setting('app.tenant_id', true)`, fijan el tenant **que sale de sus propios datos** (la sesión, el grant, el argumento validado) con `set_config(..., true)`, operan a través de `tenant_isolation` y **restauran** el valor del llamador antes de salir. Si fallan, la transacción o el savepoint revierten el `set_config`. No se usa la cláusula `SET app.tenant_id = ''` de `CREATE FUNCTION`, porque un rol no superusuario recibe `permission denied to set parameter` en PostgreSQL 16.
- La prueba de CI (a) agrega una comprobación: toda función con `prosecdef` tiene como dueño a `wanka_definer` y `proconfig` con `search_path`.

**Auditoría de todas las funciones del diseño**:

| Función | Dueño | Quién la ejecuta | Tablas con RLS que toca | Cómo obtiene el tenant |
|---|---|---|---|---|
| `iam.session_lookup(bytea)` | `wanka_definer` | `wanka_app` (antes de `InTenantTx`) | lee `iam.memberships` | `set_config` con el `tenant_id` de la sesión encontrada; restaura |
| `iam.list_my_memberships` | *no es función: vista `security_invoker`* | `wanka_app` con solo `app.user_id` | `iam.memberships`, `iam.roles`, `org.sites`, `org.tenants` | políticas `own_memberships`/`member_read` del llamador (sin tenant) |
| `iam.list_support_requests()` | `wanka_definer` | `wanka_app` dentro de `InTenantTx` | ninguna (`platform.impersonation_grants` no tiene RLS) | filtra por `app.current_tenant()` del llamador; sin tenant, 0 filas |
| `iam.decide_impersonation(uuid, boolean)` | `wanka_definer` | `wanka_app` dentro de `InTenantTx`, `session_kind = user` | escribe `audit.events` | el del llamador; falla cerrada sin `session_kind = user` |
| `iam.redeem_impersonation(bytea, uuid, bytea)` | `wanka_definer` | `wanka_app` sin tenant | escribe `audit.events` (y `iam.sessions`, sin RLS) | `set_config` con el `tenant_id` del grant aprobado; restaura |
| `ops.resolve_webhook_route(text)` | `wanka_definer` | `wanka_webhooks` | ninguna (`ops.webhook_routes` no tiene RLS) | devuelve el tenant de la ruta |
| `platform_fn.provision_tenant(...)` | `wanka_definer` | `wanka_platform` | escribe `org.tenants`, `org.sites`, `iam.roles`, `iam.memberships`, `audit.events` | `set_config` con el tenant nuevo; restaura |
| `platform_fn.request_impersonation(...)` | `wanka_definer` | `wanka_platform` | escribe `audit.events` | `set_config` con el tenant del argumento; restaura |
| `platform_fn.suspend_tenant(uuid)` | `wanka_definer` | `wanka_platform` | actualiza `org.tenants` (la política `platform_rw` es `TO wanka_platform` y no alcanza al dueño de la función) | `set_config` con el tenant del argumento; restaura |
| `platform_fn.purge_closed_tenant(uuid)` | `wanka_definer` | `wanka_worker` | borra en todas las tablas del tenant | `set_config` con el tenant del argumento; restaura |

**Verificación.** `data/rls-check/` contiene el SQL de estas funciones (versión reducida del esquema, con las mismas políticas, `FORCE RLS` y roles) y un `run.sh` que lo ejecuta en un contenedor desechable de PostgreSQL 16 con los roles reales. La salida está en el PR #124. Cada función tiene además su prueba de integración, que también verifica que no hace nada fuera de su tenant.

### Workers

La tabla de jobs no es de tenant (no tiene RLS). Cada job lleva `tenant_id` en sus argumentos, y el handler abre `InTenantTx(Actor{TenantID: job.TenantID, UserID: systemUser, Kind: system})` antes de tocar datos. Los jobs que recorren tenants (recordatorios diarios) obtienen la lista desde `platform.tenant_usage` y las suscripciones, y abren una transacción por tenant.

### Migraciones y pruebas

Las migraciones las aplica `wanka_migrator`. Pruebas de CI contra `pg_catalog`:
- (a) toda tabla con columna `tenant_id` tiene `relrowsecurity` y `relforcerowsecurity`, y una política que usa `app.current_tenant()`. Las demás políticas solo pueden ser `FOR SELECT`, su rama por usuario debe contener `app.current_tenant() IS NULL` (se verifica sobre `pg_policies.qual`) y tener un nombre de la lista blanca (`own_memberships` en `iam.memberships` e `iam.membership_sites`; `member_read` en `org.tenants`, `iam.roles`, `org.sites`, `org.cash_registers`; y, como única excepción que no es solo de lectura, `platform_rw` en `org.tenants`, limitada `TO wanka_platform`). Cualquier otra política, o una de esas en otra tabla, hace fallar la prueba;
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
