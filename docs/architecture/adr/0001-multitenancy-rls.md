# ADR 0001: Multi-tenancy con base única, `tenant_id` y Row-Level Security

- Estado: aceptado (decisión del usuario); los detalles de implementación son una propuesta; modificado por [0012](0012-maintenance-without-tenant.md) (2026-11-19) en la lista blanca de políticas de la prueba (a) y en la fila de `ops.idempotency_keys` de la matriz de GRANT
- Fecha: 2026-11-17 (revisado tras la revisión del PR #124: caminos que RLS bloqueaba, `NULLIF`, matriz de permisos por rol; 2026-11-18: creación de los roles, épica #154; 2026-11-19: los workers van sin `UserID`, #169; 2026-11-19: excepción acotada al dueño de las `SECURITY DEFINER` para las particiones de `audit.events`, #173)

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

Ningún rol de aplicación tiene `BYPASSRLS` ni es dueño de tablas. Cada servicio se conecta **con su propio rol, que es `LOGIN`**: no hay roles de grupo con usuarios de conexión aparte. El único `NOLOGIN` es `wanka_definer` (corrección del 2026-11-18: el texto anterior decía «`NOLOGIN` salvo por su usuario de conexión», que no coincidía con `data/rls-check/`; ver «Creación de los roles»). Las funciones `SECURITY DEFINER` tienen su propio dueño, `wanka_definer` (`NOLOGIN`, `NOBYPASSRLS`, sin tablas propias), descrito en la sección «Funciones `SECURITY DEFINER`: dueño y patrón».

| Esquema / objeto | `wanka_migrator` | `wanka_app` (api) | `wanka_worker` | `wanka_webhooks` | `wanka_platform` (platform-api) |
|---|---|---|---|---|---|
| Tablas de tenant (`org`*, `crm`, `catalog`, `inv`, `sales`, `einv`, `purch`, `fin`, `agenda`, `integ`, `notify`, `audit`, rubros) | dueño | SELECT/INSERT/UPDATE (DELETE solo donde el módulo lo declara), **con RLS** | igual, con RLS por job | solo INSERT en `einv.provider_webhook_events` (con RLS). **Sin acceso a `integ.provider_credentials`**: no puede leer las API keys `fk_` | **ninguno directo** |
| `org.tenants` | dueño | SELECT con RLS (política extra por membresía) | SELECT con RLS | — | SELECT/UPDATE vía la política `platform_rw` (`TO wanka_platform USING (true)`) |
| `iam.users`, `iam.sessions`, `iam.user_totp` (sin RLS: identidad global; `iam.password_resets` del diseño original es `iam.user_tokens`, con su fila y grants más estrechos en la nota del 2026-11-19: errata, nota del 2026-11-20) | dueño | SELECT/INSERT/UPDATE (en `iam.sessions`, `UPDATE` por columna sin `id`, `user_id`, `impersonator_id`, `refresh_family` ni `created_at`, y un trigger que reserva a `wanka_definer` crear o cambiar una sesión impersonada; aclaración del 2026-11-19, PR #197, [data-model §6](../data/data-model.md)) | DELETE para limpieza, con `SELECT` solo de las columnas que filtra el job (`id`, `created_at`, `revoked_at`, `access_expires_at`, `refresh_expires_at`; nunca hashes): sin ese `SELECT`, el `WHERE` del `DELETE` da `permission denied` (aclaración del 2026-11-18, PR #197) | — | — |
| `iam.mfa_enrollments` (sin RLS: identidad global, una fila por `enrollmentToken` del login; aclaración del 2026-11-19, PR #203, [#25](https://github.com/bacsystem/wanka/issues/25)) | dueño | SELECT/INSERT y `UPDATE` **por columna**, solo `email_code_hmac`, `email_sends`, `email_sent_at`, `totp_secret_enc` y `consumed_at` (nunca `user_id`, `token_hash`, `expires_at` ni `created_at`), sin DELETE; el trigger `iam.guard_mfa_enrollment_writes` fija `created_at` en el `INSERT` y hace irreversible `consumed_at` ([data-model §6](../data/data-model.md)). Las columnas nuevas de `iam.users` (`mfa_failed_attempts`, `mfa_locked_until`, `mfa_lockouts`) van con el `UPDATE` de la fila de arriba | DELETE para limpieza, con `SELECT` solo de `id` y `expires_at` (nunca los hashes ni el secreto) | — | — |
| `iam.memberships`, `iam.membership_sites`, `iam.roles`, `org.sites`, `org.cash_registers` | dueño | con RLS + política `member_read`/`own_memberships` por membresía de `app.user_id` (solo SELECT y **solo sin tenant fijado**) | con RLS | — | — |
| `org.sites`, `org.tenant_settings`, `org.document_series`: columnas de sincronización con Khipu (aclaración del 2026-11-19, [ADR 0013](0013-khipu-company-config-sync.md); ver la nota al final) | dueño | SELECT; INSERT y `UPDATE` **por columna, sin** `provider_synced_version` ni `provider_synced_at`; `provider_conflict_at` solo para poner el congelado, que el trigger `org.guard_series_conflict()` hace irreversible para todo rol (sí `last_number`, que numera `NextNumber`, y `tenant_settings.provider_sync_seq`, que pide la sincronización); sin DELETE en `org.document_series` | SELECT, con RLS por job, y `UPDATE` **solo** de `provider_synced_version` (`org.sites`, `org.tenant_settings`) y de `provider_synced_at`, `provider_conflict_at` (`org.document_series`): no numera, no crea, no borra ni edita nada más de `org` | — | — |
| `iam.role_permissions` | dueño | con RLS (los permisos se resuelven ya con tenant fijado) | con RLS | — | — |
| `iam.permissions` (catálogo global, sin RLS) | dueño | SELECT | SELECT | — | SELECT |
| `platform.*` (sin RLS) | dueño | SELECT solo en las ocho tablas de entitlements: `plans`, `plan_modules`, `modules`, `feature_flags`, `tenant_flag_overrides`, `tenant_module_overrides`, `subscriptions` y `tenant_usage`, más `USAGE` del esquema `platform`, que no da acceso a ninguna otra tabla de `platform` (ninguna tiene GRANT a `wanka_app` ni a `PUBLIC`). Sin RLS, lo que separa a un tenant de otro en esas lecturas es el filtro por `app.current_tenant()` de `internal/platform/entitlements`, el único paquete que las consulta (aclaración del 2026-11-19, #171: faltaba `tenant_usage`, que la lectura de entitlements necesita para comparar el límite del plan con el uso en la misma consulta) | SELECT de las mismas ocho tablas; INSERT/UPDATE solo en `platform.tenant_usage` (el `WHERE` del `UPDATE` y el `ON CONFLICT` necesitan ese SELECT) | — | SELECT/INSERT/UPDATE |
| `ops.webhook_routes` (sin RLS; guarda el `whsec_` cifrado con `WEBHOOK_SECRET_ENC_KEY`) | dueño | INSERT/UPDATE al conectar Khipu | — | solo a través de `ops.resolve_webhook_route` (sin SELECT directo) | SELECT de `route_id`, `tenant_id`, `revoked_at` (GRANT por columna, sin el secreto) |
| `ops.idempotency_keys` (con RLS), `ops.exchange_rates` (global) | dueño | DML / SELECT | DELETE (limpieza) / INSERT (tipo de cambio) | — | SELECT |
| Tablas de River (`ops`) | dueño | INSERT (encolar) | todo | INSERT (encolar) | INSERT (encolar) |
| Funciones `SECURITY DEFINER` (`platform_fn.*`, `iam.*`, `ops.*`) | las crea; **dueño: `wanka_definer`** | EXECUTE de `iam.session_lookup` (búsqueda de sesión antes de conocer el tenant, ADR 0004), `iam.redeem_impersonation`, `iam.decide_impersonation`, `iam.list_support_requests` (sin GRANT sobre `platform.impersonation_grants`) | EXECUTE de `platform_fn.purge_closed_tenant` | EXECUTE de `ops.resolve_webhook_route` | EXECUTE de `platform_fn.provision_tenant`, `suspend_tenant`, `request_impersonation` |

\* `org.tenants` tiene la fila propia del tenant y se trata aparte (siguiente sección).

### Creación de los roles (bootstrap fuera de goose)

Detalle de implementación fijado el 2026-11-18 (épica #154, historia #162).

- **Modelo: un rol `LOGIN` por servicio** (`wanka_migrator`, `wanka_app`, `wanka_worker`, `wanka_webhooks`, `wanka_platform`) y `wanka_definer` `NOLOGIN`. Todos `NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`.
  - Opción descartada por ahora: roles de grupo `NOLOGIN` con los GRANT, más usuarios `LOGIN` que heredan. Su única ventaja es rotar la contraseña sin corte (dos usuarios por servicio, se alterna). Su costo: `current_user` deja de ser el nombre del rol (las pruebas de `rls-check` y cualquier auditoría lo usan), el migrador tendría que hacer `SET ROLE wanka_migrator` en cada sesión para que los objetos no queden a nombre de su usuario de conexión, y hay el doble de roles que mantener.
  - Es reversible sin migrar datos: si hiciera falta rotar sin corte, se crea `wanka_app_a` con `IN ROLE wanka_app` y se pasa `wanka_app` a `NOLOGIN`. Los GRANT y las políticas `TO wanka_app` siguen valiendo para los miembros.
- **Quién los crea: un bootstrap idempotente fuera de goose**, ejecutado por el superusuario. Exige `SUPERUSER`: `CREATEROLE` no basta, porque en PostgreSQL 16 solo un superusuario puede fijar `NOSUPERUSER`, `NOREPLICATION` y `NOBYPASSRLS`, aunque sea para quitarlos. La migración no puede hacerlo: `CREATE ROLE` exige `CREATEROLE`, `ALTER ROLE ... SET` también, y darle `CREATEROLE` a `wanka_migrator` le daría en cada despliegue la capacidad de crear roles con `LOGIN` y administrarlos, que solo se usa al crear un entorno (menor privilegio). El bootstrap crea los roles que faltan, vuelve a fijar sus atributos en cada ejecución, `GRANT wanka_definer TO wanka_migrator`, `GRANT CREATE ON DATABASE` a `wanka_migrator`, `REVOKE CREATE ON SCHEMA public FROM PUBLIC`, el esquema `goose` con dueño `wanka_migrator` y `ALTER ROLE wanka_app SET statement_timeout = '5s'`. Termina comprobándose a sí mismo contra `pg_roles` (incluido que ningún rol `wanka_*` pueda crear en `public`). Referencia ejecutable: [`data/rls-check/01_schema.sql`](../data/rls-check/01_schema.sql), que `run.sh` corre dos veces seguidas.
- **No fija contraseñas.** El admin las pone con `\password <rol>` en `psql`, que envía el hash SCRAM: el texto no queda en el repo, en el script ni en el historial del shell.
- **Orden en un entorno nuevo:** bootstrap → contraseñas → variables en Railway → primer despliegue (el pre-deploy `wanka migrate up` corre como `wanka_migrator`). Si falta un rol, la migración falla en su `GRANT ... TO` y el despliegue no avanza: falla cerrada. Una release que agregue un rol actualiza el bootstrap y se vuelve a correr antes de desplegarla.
- **Railway** ([ADR 0007](0007-deployment-railway.md)): el bootstrap lo corre el admin con el usuario administrador del servicio Postgres, una vez por entorno (`staging`, `production`). Esa credencial no se inyecta en ningún servicio. Cada servicio recibe solo su URL: `DATABASE_URL_MIGRATOR` (pre-deploy), `DATABASE_URL_APP` (api), `DATABASE_URL_WORKER`, `DATABASE_URL_WEBHOOKS` y `DATABASE_URL_PLATFORM`.
- **Rotación** (con un solo usuario por rol): se guarda la variable nueva en Railway sin desplegar, `ALTER ROLE ... PASSWORD` (con `\password`) y se despliega enseguida. Las conexiones ya abiertas del pool siguen vivas; solo fallan las conexiones nuevas durante esos segundos, y el `/readyz` lo muestra. Si eso deja de ser aceptable, se pasa al modelo de grupo descrito arriba.

### Caminos que no tienen un tenant fijado

La revisión los reprodujo en PG16 (cero filas o `new row violates row-level security policy`). Cada uno tiene ahora un mecanismo acotado:

1. **Login y `/seleccionar-empresa`** (el usuario todavía no eligió empresa).
   - `iam.memberships` y `iam.membership_sites` tienen una segunda política, solo de lectura: `CREATE POLICY own_memberships ON iam.memberships FOR SELECT USING (app.current_tenant() IS NULL AND user_id = app.current_user_id() AND status IN ('invitado','activo'))`.
   - `org.tenants` admite `member_read FOR SELECT USING (id = app.current_tenant() OR (app.current_tenant() IS NULL AND id IN (SELECT tenant_id FROM iam.memberships WHERE user_id = app.current_user_id() AND status IN ('invitado','activo'))))`.
   - **Un ex miembro no lee nada de la empresa, ni sin tenant** (aclaración del 2026-11-19, #187, [ADR 0011](0011-membership-loss-keeps-user-session.md)). Una membresía que no está `invitado` ni `activo` (`suspendido`, o borrada) no da lectura de ninguna fila de su empresa: ni de la empresa en `org.tenants`, ni de la propia membresía en `iam.memberships`, ni de rol o sedes. `org.tenants`, `iam.memberships` e `iam.roles` lo exigen con `status IN ('invitado','activo')`, y `org.sites` con `status = 'activo'` (más estricto: una invitada aún no elige sede). La política `own_memberships` de `iam.membership_sites` todavía no tiene su `USING` escrito en este ADR: la historia que la cree le aplicará el mismo filtro de estado. Así «A no aparece en `GET /v1/me/memberships`» lo garantiza la base y no depende de que un handler filtre. Decisión: `own_memberships` también filtra (se descartó dejar que el usuario lea su membresía suspendida) porque la subconsulta de `member_read` pasa por `own_memberships` y una sola política que filtrara dejaría a la otra de rebote. Como esa subconsulta ya recibe el filtro de `own_memberships`, quitarlo de `member_read` no cambia ninguna fila: `rls-check` lo comprueba en `pg_policies.qual` (mutación `member_read_tenants_no_status`), y la lectura de `iam.memberships` con la mutación `own_memberships_no_status`. No cambia nada de `iam.session_lookup`, que lee la membresía con el tenant de la sesión por `tenant_isolation`.
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
   - **Lectura desde el plano tenant.** Las solicitudes viven en `platform.impersonation_grants`, sin RLS, y `wanka_app` **no tiene GRANT sobre `platform.impersonation_grants`**: en `platform` solo lee las ocho tablas de entitlements de la matriz de roles. Un `GRANT SELECT` directo expondría las solicitudes de todos los tenants. `GET /v1/support-access/requests` lee solo a través de `iam.list_support_requests()`:
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
     - **Permisos:** los del alcance del grant, nunca los de un rol del tenant. **En todo alcance** se excluyen `identidad.support_access:*`, `identidad.users:*`, `identidad.roles:*`, `identidad.sessions:*` y `empresas.einvoicing:update` (credenciales de Khipu). Sobre lo que queda, `read_only` tiene solo las acciones `read` y `full` tiene todas las acciones del catálogo.
       **Corrección 2026-11-19, PR #218** (architect): el texto original aplicaba la exclusión solo a `full`, y con `read_only` el staff podía leer accesos de soporte, usuarios, roles y sesiones; eso contradecía el título de este punto y [`api/resources.md`](../api/resources.md) («**Nunca** desde una sesión impersonada»). Solo estrecha permisos (falla cerrada): no cambia la decisión, la corrige ([README §9](../README.md)).
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
- La migración, que corre como `wanka_migrator` (miembro de `wanka_definer`), crea la función y hace `ALTER FUNCTION ... OWNER TO wanka_definer`. PostgreSQL exige que el nuevo dueño tenga `CREATE` en el esquema, así que la migración se lo concede **dentro de una transacción** (`BEGIN; GRANT CREATE ...; ALTER FUNCTION ... OWNER TO wanka_definer; REVOKE CREATE ...; COMMIT;`): si algo falla, el `GRANT` no queda aplicado. Por eso **esta migración nunca se marca `-- +goose NO TRANSACTION`** (ese modo queda reservado a los `CREATE INDEX CONCURRENTLY`, en archivos propios). Se lo concede solo durante ese cambio y lo revoca enseguida.
- Toda función lleva `SET search_path = pg_catalog, pg_temp`, nombres cualificados y `REVOKE ALL ... FROM PUBLIC`.
- Las funciones que tocan tablas con RLS **para un tenant que el llamador no fijó** siguen siempre el mismo patrón: guardan `current_setting('app.tenant_id', true)`, fijan el tenant **que sale de sus propios datos** (la sesión, el grant, el argumento validado) con `set_config(..., true)`, operan a través de `tenant_isolation` y **restauran** el valor del llamador antes de salir. Si fallan, la transacción o el savepoint revierten el `set_config`. No se usa la cláusula `SET app.tenant_id = ''` de `CREATE FUNCTION`, porque un rol no superusuario recibe `permission denied to set parameter` en PostgreSQL 16.
- La prueba de CI (a) agrega una comprobación: toda función con `prosecdef` tiene como dueño a `wanka_definer` y `proconfig` con `search_path`.

**Excepción: DDL de particiones (architect, 2026-11-19, #173).** Crear o enganchar una partición exige ser dueño de la tabla padre, y el único dueño de tablas es `wanka_migrator`. Las funciones de esta lista, y solo ellas, son `SECURITY DEFINER` con dueño **`wanka_migrator`**:

| Función | Quién la ejecuta | Qué hace |
|---|---|---|
| `audit.ensure_partitions(date, integer)` | `wanka_worker` (job diario `audit.ensure_partitions` y arranque del worker) | crea las particiones mensuales de `audit.events` que falten con `CREATE TABLE (LIKE … INCLUDING ALL)` + `ATTACH PARTITION` |

Condiciones, todas verificables en el catálogo o en `data/rls-check/`; una función que no las cumpla no entra en la lista:
- hace solo DDL de particiones sobre una tabla de `wanka_migrator`; nunca DML sobre tablas con RLS ni otro DDL;
- ningún argumento de texto, `bytea`, `json` ni de tipo polimórfico: solo `date`/`integer` con rango validado al entrar, o ninguno. Los nombres que arma salen de esos valores y van con `format('%I')`/`%L`;
- `SET search_path = pg_catalog, pg_temp` como única entrada de `proconfig`, nombres cualificados, `lock_timeout` fijado con `set_config(..., true)` y un lock de aviso que serializa a dos llamadores;
- `REVOKE ALL ... FROM PUBLIC` y `EXECUTE` solo para `wanka_worker`;
- la prueba de catálogo la lista por firma exacta (`regprocedure`) en una lista blanca propia que cita este ADR; una entrada que ya no existe hace fallar la prueba.

Por qué así y no otra cosa:
- *`wanka_definer` como dueño de `audit.events`*: descartado. Es el rol con el que corren todas las demás funciones, y varias reciben `bytea` o `text` del llamador; un error en cualquiera daría el dueño de la bitácora, que puede `ALTER TABLE ... NO FORCE ROW LEVEL SECURITY`, `DETACH` o `DROP`. Además rompe «`wanka_definer` no es dueño de tablas».
- *Que las cree el `migrate` de cada despliegue*: descartado. Goose aplica cada migración una vez, así que haría falta un paso nuevo en `wanka migrate`; la cobertura quedaría atada a la frecuencia de despliegues (un periodo sin deploys más largo que el colchón deja sin partición y hace fallar toda escritura auditada), y el DDL correría igual como `wanka_migrator`, con sesión completa en vez de una función fija.
- *Partición `DEFAULT`*: descartada, medido en PostgreSQL 16.16. Con una `DEFAULT`, cada `ATTACH` toma `AccessExclusiveLock` sobre ella y la recorre; si guarda una fila de ese mes, el `ATTACH` falla (`updated partition constraint for default partition ... would be violated by some row`), la función aborta y tampoco crea los meses siguientes, y arreglarlo exige mover filas a mano. Las filas de la `DEFAULT` no salen con la purga por partición (nadie tiene `DELETE`), y el fallo que debía alertar queda oculto.
- *Un rol propio dueño de `audit`*: menos alcance, pero suma un rol al bootstrap de superusuario en cada entorno y una segunda excepción en las reglas de roles; con una función sin argumentos de texto, la ganancia es pequeña. Se reconsidera si la lista pasa de dos funciones o alguna necesita un argumento de texto.

Qué gana un `wanka_worker` comprometido: crear particiones vacías hasta unos 25 meses hacia adelante y tomar `SHARE UPDATE EXCLUSIVE` sobre `audit.events` mientras mantenga abierta su transacción (no bloquea `INSERT` ni `SELECT`; sí frena un DDL del despliegue, algo que ese rol ya puede hacer con sus propias tablas). No puede elegir nombres, rangos fuera de los validados ni sentencias, y no tiene `CREATE` en el esquema `audit` para plantar una tabla con el nombre de una partición futura.

Si falla: sin la partición del mes, el `INSERT` en `audit.events` falla (`no partition of relation ... found for row`, SQLSTATE `23514`) y con él la transacción del cambio de negocio: falla cerrada, nunca se pierde un evento. El colchón son los 3 meses siguientes que mantienen el job diario y el arranque del worker; los despliegues no influyen. La alerta sobre el job fallido (o sobre menos de 2 meses cubiertos) entra con la historia de observabilidad.

**Purga a 5 años (plataforma, data-model §5).** `DETACH` y `DROP` de una partición también exigen ser dueño y toman `AccessExclusiveLock` sobre `audit.events` (medido), así que es el mismo caso. Si se hace como función, entra en esta lista en su propia historia y con revisión del architect, con estas condiciones además de las de arriba: **sin argumentos**, el corte `interval '5 years'` escrito en el cuerpo (un mes sale solo cuando su límite superior + 5 años ≤ `now()`), las particiones elegidas por su límite leído de `pg_catalog`, no por el nombre, el mismo lock de aviso y `lock_timeout`. Así, quien la ejecute de más solo borra lo que ya venció.

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
| `audit.ensure_partitions(date, integer)` | `wanka_migrator` (excepción «DDL de particiones», #173) | `wanka_worker` sin tenant | ninguna: solo DDL de particiones de `audit.events` | no usa tenant |

**Verificación.** `data/rls-check/` **falla sola**: cada caso compara contra su valor esperado (`test.assert`, `test.expect_error` con el SQLSTATE esperado), y el primer desvío aborta `psql` y deja `run.sh` con salida distinta de 0. `MUTATION=naive_session_lookup` reintroduce el defecto de la versión `LANGUAGE sql` para demostrar que la verificación lo detecta. La carpeta contiene el SQL de estas funciones (versión reducida del esquema, con las mismas políticas, `FORCE RLS` y roles) y un `run.sh` que lo ejecuta en un contenedor desechable de PostgreSQL 16 con los roles reales. La salida está en el PR #124. Cada función tiene además su prueba de integración, que también verifica que no hace nada fuera de su tenant.

### Workers

La tabla de jobs no es de tenant (no tiene RLS). Cada job lleva `tenant_id` en sus argumentos, y el handler abre `InTenantTx(Actor{TenantID: job.TenantID, Kind: system})` antes de tocar datos, **sin `UserID`**, igual que los webhooks y la activación de facturación (ADR 0003). No existe un usuario de sistema: un UUID fijo sería una fila falsa en `iam.users` (global, visible al login y a las membresías) o un `app.user_id` que no apunta a nada, y ninguna política de tenant lo necesita, porque filtran solo por `app.current_tenant()`; las ramas por usuario exigen `app.current_tenant() IS NULL` y no aplican aquí. Con `app.current_user_id()` en `NULL`, las funciones que piden un usuario fallan cerradas. En `audit.events` la fila queda con `actor_type = system` y `actor_user_id` en `NULL`, y en las columnas `created_by`/`updated_by` (anulables, `data/data-model.md` §1) queda `NULL`; el job identifica su origen en `action`/`diff` (p. ej. el id del job), no con un usuario inventado. (Corrección del 2026-11-19, #169: el texto anterior decía `UserID: systemUser`, que nunca se definió; la decisión de correr como `Kind: system` no cambia.) Los jobs que recorren tenants (recordatorios diarios) obtienen la lista desde `platform.tenant_usage` y las suscripciones, y abren una transacción por tenant.

### Migraciones y pruebas

Las migraciones las aplica `wanka_migrator`. Pruebas de CI contra `pg_catalog`:
- (a) toda tabla con columna `tenant_id` tiene `relrowsecurity` y `relforcerowsecurity`, y una política que usa `app.current_tenant()`. Las demás políticas solo pueden ser `FOR SELECT`, su rama por usuario debe contener `app.current_tenant() IS NULL` (se verifica sobre `pg_policies.qual`) y tener un nombre de la lista blanca (`own_memberships` en `iam.memberships` e `iam.membership_sites`; `member_read` en `org.tenants`, `iam.roles`, `org.sites`, `org.cash_registers`; y, como única excepción que no es solo de lectura, `platform_rw` en `org.tenants`, limitada `TO wanka_platform`). Cualquier otra política, o una de esas en otra tabla, hace fallar la prueba. La rama por usuario de `own_memberships` y de `member_read` de `org.tenants` debe contener además `status`, `invitado` y `activo` (#187);
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

## Nota del 2026-11-19 (architect, ADR 0013, revisión del PR #228)

La matriz de «Roles de base de datos y permisos» suma una fila para las columnas de sincronización con Khipu de `org.sites`, `org.tenant_settings` y `org.document_series`. Es más estrecha que la fila genérica de tablas de tenant («igual, con RLS por job»), que le habría dado a `wanka_worker` el `UPDATE` de `last_number`. Los jobs `einv.company_sync` y `einv.series_reconcile`, y el congelado que pide la emisión, corren como `wanka_worker` y solo marcan; `wanka_app` edita y numera, y puede congelar una serie (al rechazar una reactivación), pero no puede marcarla como sincronizada ni levantar un congelado (trigger `org.guard_series_conflict()`). `einv.company_sync_state` (arrendamiento de `einv.company_sync`) es una tabla de tenant con la política estándar: `SELECT`, `INSERT` y `UPDATE` para `wanka_worker` y nada para `wanka_app`. No hay política, rol ni función definer nuevos. Lo prueban `data/rls-check/` (casos `A12` y `K5`) y las mutaciones `grant_worker_update_series`, `grant_app_update_series_sync`, `revoke_worker_series_sync` y `drop_series_conflict_guard`.


## Nota del 2026-11-19 (architect, revisión previa de la épica #3)

La matriz de «Roles de base de datos y permisos» cambia así para la ola 1 de identidad ([data-model §6](../data/data-model.md), «DDL de identidad de la ola 1»); no hay política, rol ni función `SECURITY DEFINER` nuevos:

- `iam.user_tokens` reemplaza a `iam.password_resets` en la fila de identidad global, con grants más estrechos: `wanka_app` tiene `SELECT`, `INSERT` y `UPDATE` **solo** de `consumed_at`; `wanka_worker`, `DELETE` y `SELECT (id, expires_at)`, nunca el hash. Un trigger sin `SECURITY DEFINER` fija `created_at` y hace irreversible `consumed_at`, como `iam.guard_mfa_enrollment_writes()`. Como volver a consumir es un no-op silencioso, el consumo es siempre **condicional**: `UPDATE iam.user_tokens SET consumed_at = now() WHERE token_hash = $1 AND purpose = $2 AND consumed_at IS NULL AND expires_at > now() RETURNING …`, y el flujo sigue solo si devolvió una fila; dos `accept` o `reset` simultáneos con el mismo token dejan pasar uno y el otro recibe `401 session_expired` (revisión del PR #230).
- `iam.role_templates` e `iam.role_template_permissions` son un catálogo global como `iam.permissions`: `SELECT` para `wanka_app` (resolver de permisos y alta por autoservicio) y `wanka_definer` (el alta desde la consola, `platform_fn.provision_tenant`, crea las filas de `iam.roles` de los roles base, no sus permisos: [ADR 0004](0004-in-house-auth.md), nota del 2026-11-19), y ningún DML para los roles de aplicación. `wanka_worker` no recibe nada: ningún job resuelve permisos (revisión del PR #230). Esto precisa el punto 5 de «Caminos que no tienen un tenant fijado»: «los roles plantilla con sus permisos» pasa a ser «los roles plantilla, cuyos permisos salen de la plantilla».
- `wanka_app` recibe `DELETE`, siempre con `tenant_isolation`, en `iam.role_permissions` (editar los permisos de un rol propio), `iam.membership_sites` (cambiar las sedes de una membresía), `iam.roles` (solo roles propios; la FK de `iam.memberships` rechaza borrar uno en uso) e `iam.memberships` (solo en estado `invitado`, para anular una invitación; el mismo trigger rechaza las demás). Borrar una membresía no toca sus sesiones (ADR 0011).
- **Trigger `iam.guard_role_writes()`** (`BEFORE INSERT OR UPDATE OR DELETE` en `iam.roles`, sin `SECURITY DEFINER`, `search_path` fijado; solo un superusuario lo salta), porque `wanka_app` tiene `UPDATE` de fila completa (revisión del PR #230). Rechaza con `42501`: borrar un rol `system`; cambiar `system` o `template_code` de cualquier rol (un rol propio no pasa a base ni al revés); cambiar `name` de un rol `system` (un rol propio sí se renombra, con su `version`; re-revisión del PR #230); bajar `requires_mfa` de un rol `system` por debajo de la de su plantilla; e insertar un rol `system` cuyo `requires_mfa` sea menor que el de su plantilla. Un índice único `(tenant_id, template_code)` impide un segundo rol con la misma plantilla en la empresa, así que nadie crea otro `dueno` aunque inserte con `system = true`. Igual para `iam.memberships`: el trigger rechaza borrar una que no esté en `invitado`.

Los prueba `data/rls-check/` en el PR de la historia que crea las tablas, con un caso por grant y una mutación por cada protección (columnas fuera del `UPDATE` de `iam.user_tokens`, `DELETE` del worker sin `SELECT` del hash, trigger de `iam.memberships`, y una mutación por cada regla del trigger de `iam.roles`: por ejemplo, `UPDATE iam.roles SET template_code = 'dueno', system = true WHERE id = <rol propio>` como `wanka_app` debe dar `42501`, y sin el trigger pasar).

## Nota del 2026-11-20 (architect, #246: los docs frente a las migraciones de #231 y #232)

Alinea este ADR con `migrations/20261120100000_iam_global_identity.sql` (#231) y `migrations/20261120110000_identity_tenant_ddl.sql` (#232). Precisa la decisión y **achica privilegios**: ningún rol recibe un grant nuevo, y dos excepciones quedan más estrechas de lo que permitía el texto anterior ([README §9](../README.md)).

- **Fila de identidad global (errata).** La matriz nombraba `iam.password_resets`, que no existe: es `iam.user_tokens`, con los grants de la nota del 2026-11-19 (`wanka_app`: `SELECT`, `INSERT` y `UPDATE (consumed_at)`; `wanka_worker`: `DELETE` y `SELECT (id, expires_at)`), que son los que aplica #231.
- **`own_memberships` en `iam.membership_sites` ya está escrita.** El punto 1 de «Caminos que no tienen un tenant fijado» decía que su `USING` no estaba escrito. El texto vigente es el de #232, con el mismo filtro de estado que la de `iam.memberships`:
  ```sql
  CREATE POLICY own_memberships ON iam.membership_sites FOR SELECT USING (
    app.current_tenant() IS NULL AND membership_id IN (SELECT id FROM iam.memberships
              WHERE user_id = app.current_user_id() AND status IN ('invitado','activo')));
  ```
- **Las ramas por usuario, solo para `wanka_app`.**
  - **El problema.** PostgreSQL aplica a cualquier rol que lea una tabla todas sus políticas permisivas, y exige `SELECT` sobre las tablas de sus subconsultas aunque la rama no abra nada. Hoy `member_read` de `org.sites`, y la de `org.cash_registers`, que lee `org.sites`, se aplica a todos los roles. Un `DELETE` con `WHERE` también pasa por las políticas de `SELECT`. `wanka_worker` y `wanka_definer` ya tienen `SELECT` sobre `iam.memberships` desde el kernel (`20261118140000_kernel_iam_org_core.sql`); lo que les falta es `SELECT` sobre `iam.membership_sites`. Por eso `data/rls-check/02_migrate.sql` se lo da a los dos, con un comentario que lo justifica.
  - **Qué se fija.** Las seis políticas de la lista blanca con rama por usuario pasan a `TO wanka_app`: `own_memberships` de `iam.memberships` y de `iam.membership_sites`, y `member_read` de `org.tenants`, `iam.roles`, `org.sites` y `org.cash_registers`. `wanka_app` es el único rol que fija `app.user_id` sin tenant, en el login y en `/seleccionar-empresa`. Los demás roles quedan solo con `tenant_isolation`.
  - **Qué cambia en los grants.** `wanka_worker` **no** recibe `SELECT` sobre `iam.membership_sites`. `wanka_definer` tampoco lo tiene hoy en las migraciones reales (`20261118140000_kernel_iam_org_core.sql` solo le da `SELECT` sobre `iam.sessions` e `iam.memberships`; el que aparece en `rls-check/02_migrate.sql` es del esquema reducido): lo recibe la historia de la purga, para su propio `DELETE … WHERE tenant_id = …` sobre esa tabla, no para la política (abajo).
  - **Lo aplica #33**, el primer rol distinto de `wanka_app` que lee `org.sites`. Lo hace con una migración nueva que hace `ALTER POLICY … TO wanka_app` de las seis, sin editar la de #232, y en el mismo PR quita `wanka_worker` del `GRANT SELECT ON iam.membership_sites` de `02_migrate.sql`. Si ese grant siguiera, la mutación de abajo nunca se pondría en rojo.
  - **Cómo se prueba en `rls-check`.** Un caso en el que `wanka_worker`, en `InTenantTx`, lee `org.sites` sin grant sobre `iam.membership_sites`. Y una mutación que devuelve las políticas a `PUBLIC` y debe fallar con `42501`. La prueba (a) de CI acepta y exige ese `TO`.
  - **Opción descartada:** dar `SELECT` sobre `iam.membership_sites` a cada rol que lea sedes. Con RLS no abre filas de otra empresa, pero es un grant sin uso propio que habría que repetir para cada rol futuro.
  - Si la purga de empresa cerrada llegara antes que #33, la migración va en la historia de la purga.
- **La purga de una empresa cerrada y los triggers.**
  - **El problema.** `platform_fn.purge_closed_tenant` (dueño `wanka_definer`) borra todas las filas del tenant. Los triggers de #232 la frenan con `42501`: `iam.guard_role_writes()` rechaza borrar un rol `system`, e `iam.guard_membership_deletes()` rechaza borrar una membresía que no está en `invitado`. Hoy solo eximen al superusuario.
  - **Qué no alcanza.** Eximir todo `DELETE` de `wanka_definer`: la revisión del PR #249 reprodujo que así el definer borra al único dueño de una empresa `activa`. `iam.guard_session_writes()` tampoco sirve de modelo, porque exime a `wanka_definer` de todo.
  - **Qué se fija.** La historia de la purga (`org.purge_closed_tenants`, data-model §5; épica de plataforma #17) reemplaza esas dos funciones con una migración nueva (`CREATE OR REPLACE`). Dejan pasar un `DELETE` de `wanka_definer` **solo** si la empresa de la fila (`OLD.tenant_id`) está en `org.tenants` con `status = 'cerrada'` y `closed_at <= now() - interval '5 years'`, el plazo de conservación del [ADR 0005](0005-industry-modules-feature-flags.md), «Empresa cerrada». El trigger la lee con el `app.tenant_id` que fija la purga, por `tenant_isolation`. En cualquier otro caso, el `DELETE` del definer recibe `42501`, como el de cualquier rol.
  - **Dos barreras.** La función de purga verifica la misma condición antes de borrar y no confía solo en el trigger.
  - **`INSERT` y `UPDATE`.** Sus reglas siguen valiendo también para `wanka_definer`: el alta desde la consola crea los roles base con un `requires_mfa` igual o mayor que el de su plantilla, como la app.
  - **Grants que da la historia de la purga** (en las migraciones reales `wanka_definer` no tiene ninguno de estos; sin ellos el trigger da `42501` al leer `org.tenants` y la purga no borra nada): `SELECT` sobre `org.tenants` (el trigger y la función leen `status` y `closed_at`), y `SELECT, DELETE` sobre `iam.role_permissions`, `iam.membership_sites`, `iam.memberships` e `iam.roles` (el `SELECT` lo exige el `WHERE` de cada `DELETE`), además de los de las demás tablas que borre. Orden de borrado por las FK: `iam.role_permissions` e `iam.membership_sites` antes que `iam.memberships`, y estas antes que `iam.roles`.
  - **`closed_at` inmutable.** La condición de los 5 años se apoya en `closed_at`, y hoy `wanka_platform` la puede escribir (`platform_rw`). Se fija: `closed_at` se pone una sola vez, al pasar a `cerrada`, y después no cambia ni vuelve a `NULL`. Lo impone un trigger `BEFORE UPDATE` de `org.tenants`, sin `SECURITY DEFINER` y con `42501` para todo rol que no sea superusuario. Lo agrega la historia de la purga o la de cierre de empresa, la que llegue primero, con su mutación en `rls-check`.
  - **Pruebas en `rls-check`.** Un caso de purga de una empresa cerrada hace más de 5 años, que pasa. Un caso con una empresa `activa`, y otro con una cerrada hace menos de 5 años, en los que el `DELETE` del definer del único dueño da `42501`. Y una mutación por cada trigger sin la condición.

## Nota del 2026-11-20 (architect, PR #251: grant del aviso de bloqueo del login)

La matriz suma una celda: `wanka_worker` recibe `SELECT (id, email)` sobre `iam.users`, que aplica la migración `20261120130000_identity_login_lockout_notice.sql` de #24. No hay política, rol ni función definer nuevos.

- **Para qué.** El job del outbox que avisa de un bloqueo del login (ADR 0004, nota del 2026-11-19) lleva en sus argumentos solo `user_id` y `locked_until`: la tabla de jobs (`ops.river_job`) no guarda datos personales. Para saber a quién escribir, lee el correo al ejecutarse. `id` lo exige el `WHERE` y `email` es el destinatario.
- **Por qué es el mínimo.**
  - Es un grant por columna: sin `password_hash`, sin los contadores, sin `phone` ni `full_name`, sin `UPDATE`.
  - Leer el correo al ejecutarse evita además mandar el aviso a una dirección vieja si el usuario cambió de correo entre el bloqueo y el envío.
  - Se descartó poner el correo en los argumentos del job, porque dejaría un dato personal en una tabla que conserva los jobs completados 7 días.
  - Se descartó una función `SECURITY DEFINER`, porque agregaría una puerta más para el mismo dato.
- **Si más adelante hace falta otra columna** (por ejemplo, el nombre para el saludo), va como un grant nuevo con su propia nota fechada. No se amplía a la tabla completa.
- **Lo prueba `rls-check`** en el PR #251 (caso `K1f` de `09_worker.sql`): `wanka_worker` lee `email` y un `SELECT` de `password_hash` da `42501`.

## Nota del 2026-11-20 (architect, alta por autoservicio de #31; decisiones del dueño en #4)

Esta nota se corrigió en la revisión del PR #256. La matriz de «Roles de base de datos y permisos» suma tres objetos, amplía la fila de `org.tenants`, crea un esquema y suma una función `SECURITY DEFINER` a la lista. No hay política ni rol nuevos. El contrato está en [`api/conventions.md`](../api/conventions.md) §3, «Alta por autoservicio», y las tablas en [data-model §6](../data/data-model.md). Lo aplica #31, con sus casos y mutaciones de `rls-check` contra las **migraciones reales**, no contra el esquema reducido de `02_migrate.sql`, donde `platform_fn` y los `USAGE` ya existen y ocultarían lo que falta.

| Objeto | `wanka_app` | `wanka_worker` | `wanka_definer` | `wanka_webhooks`, `wanka_platform` |
|---|---|---|---|---|
| `iam.signup_requests` (sin RLS, identidad global, sin `tenant_id`) | `SELECT`, `INSERT (id, kind, email, password_hash, full_name, ruc, legal_name, trade_name, industry, expires_at)` y `UPDATE (consumed_at, tenant_id, password_hash)`; `created_at` lo fija el trigger, que hace irreversibles `consumed_at` y `tenant_id` y solo deja pasar `password_hash` a nulo. Para el `SELECT … FOR UPDATE` de la confirmación alcanza ese `UPDATE` por columna | job `iam.signup_mail`: `SELECT (id, kind, email, created_at)` y `UPDATE (token_hash)`, que el trigger deja fijar una sola vez. Limpieza: `DELETE` y `SELECT (expires_at, consumed_at)`. Nunca `password_hash` ni los demás datos | — | — |
| `ops.signup_idempotency_keys` (sin RLS, clave global, sin `tenant_id`) | los mismos grants que `ops.idempotency_keys`: `SELECT`, `INSERT (key, request_hash)` y `UPDATE (status_code, response, response_headers)`; `created_at` lo pone el servidor, así que una clave no escapa de la limpieza | `DELETE` y `SELECT (created_at)` | — | — |
| `org.tenants` (fila que ya existe) | además del `SELECT`: `INSERT (id, ruc, legal_name, trade_name, industry)` para #31 (`status` y `closed_at` quedan con su valor por defecto, `trial` y nulo, y `tenant_isolation` exige que `id` sea el tenant de la transacción); y `UPDATE (trade_name, updated_at)` para #32. **No** `UPDATE (industry)`: el cambio de rubro lo decide el PO (#32, AC12), porque los módulos no los escribe `wanka_app` | — | `SELECT` (ver la función) | — |
| Esquema `platform_fn` (no existe en las migraciones reales; solo en `rls-check/02_migrate.sql`) | `USAGE` | — | dueño de la función, no del esquema | — |

**`platform_fn.start_self_service_trial(p_tenant uuid)`**
- **Forma.** `SECURITY DEFINER`, dueño `wanka_definer`, `SET search_path = pg_catalog, pg_temp`, nombres cualificados, `EXECUTE` **solo** para `wanka_app` (`REVOKE ALL … FROM PUBLIC`). Se agrega a la lista de funciones definer de este ADR y a la lista blanca de la prueba de catálogo por su firma exacta.
- **Guardas.** Fija y restaura `app.tenant_id` con el patrón de «Funciones `SECURITY DEFINER`». Falla con `42501` si:
  - `p_tenant` no es el `app.current_tenant()` de quien llama;
  - la empresa no está en `trial`;
  - su `created_at` no es de esta transacción (`created_at = now()`);
  - ya tiene una suscripción;
  - **`industry` es nulo**.

  Falla con `55000` (`object_not_in_prerequisite_state`, un error de configuración) si el plan `prueba` no existe o no está activo.
- **Qué inserta.**
  - En `platform.subscriptions`, la suscripción `trial` del plan `prueba`. El plan lo fija la función por código y no es un argumento.
  - `period_start` es hoy en Lima y `period_end` es `period_start + 29`: el último día con escritura, inclusive, así que son 30 días (data-model §6).
  - El override `rubro.<industry>` en `platform.tenant_module_overrides`.
  - Su fila en `audit.events`, con `actor_type = system` y `action = self_service_trial`.
- **Por qué una función.** Un `INSERT` de `wanka_app` en `platform.subscriptions` le dejaría darse cualquier plan. Reusar `platform_fn.provision_tenant` (camino 5), que recibe el plan y crea todo, abriría más de lo necesario. Lo demás del alta lo hace `wanka_app` en `InTenantTx` con el tenant nuevo.
- **Qué crea y qué otorga la migración de #31** (las migraciones reales hoy solo le dan a `wanka_definer` `USAGE` sobre `iam` y `org`, `kernel_iam_org_core.sql`):
  - `CREATE SCHEMA platform_fn`, con dueño `wanka_migrator`, y `GRANT USAGE ON SCHEMA platform_fn TO wanka_app`. `wanka_definer` recibe `CREATE` sobre el esquema solo durante el `ALTER FUNCTION … OWNER`, como en el kernel.
  - Para `wanka_definer`: `USAGE` sobre `app`, `platform` y `audit`; `EXECUTE` de `app.current_tenant()`, si no lo tiene ya por `PUBLIC`; `SELECT` de `org.tenants` y `platform.plans`; `SELECT` e `INSERT` de `platform.subscriptions` y `platform.tenant_module_overrides`; e `INSERT` de `audit.events`, que hoy solo tienen `wanka_app` y `wanka_worker`. Nada más.
- **Mutaciones de `rls-check`.** Una por cada guarda:
  - sin `search_path`;
  - con `EXECUTE` para `PUBLIC`;
  - con un tenant ajeno;
  - con una empresa ya suscrita;
  - con una empresa creada en otra transacción;
  - con `industry` nulo;
  - sin el plan `prueba`;
  - con el plan como argumento.
