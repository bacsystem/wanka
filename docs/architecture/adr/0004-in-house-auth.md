# ADR 0004: Autenticación y autorización propias (sin Keycloak/Auth0)

- Estado: aceptado "hacerlo en casa" (decisión del usuario); diseño propuesto
- Fecha: 2026-11-17

## Contexto

Las pantallas ya existen como flujos mock: login + 2FA + recuperación (`app/(auth)/login`), selección de empresa y sede (`/seleccionar-empresa`, mock `auth/mocks/tenants.ts` con `role`, `sites`, `cashboxes`), usuarios y roles con matriz de permisos por módulo y acción (`settings/mocks/settings.ts`: `permissionModules`, `permissionActions` Ver/Crear/Editar/Anular/Exportar, colaboradores con `twoFa: authenticator | sms | pendiente`), y una consola super-admin con login propio "SSO + 2FA" (`app/(admin-auth)/admin/login`). Un usuario pertenece a N empresas con un rol por empresa y alcance por sede.

## Opciones

1. **JWT de acceso sin estado + refresh**: verificación sin base, pero la revocación inmediata exige lista negra compartida; gestión de claves y rotación.
2. **Tokens opacos con sesión en Postgres** (acceso corto + refresh rotativo): una lectura por PK por petición (~0,2 ms), revocación inmediata, sin claves que rotar.
3. **Sesión con cookie directa a la API**: obligaría a exponer la API al navegador (CORS, CSRF en la API).

## Decisión

Opción 2, con el BFF de Next.js como único cliente.

**Credenciales**
- Contraseñas con **argon2id** (parámetros iniciales m=64 MiB, t=3, p=2, a medir: objetivo ~150 ms por hash en la réplica), formato PHC para re-hash transparente si cambian los parámetros. Mínimo 10 caracteres, comprobación contra lista de contraseñas filtradas (offline).
- Hashing con semáforo de concurrencia = nº de núcleos (backpressure: `503` si la cola de hashing se llena).
- Bloqueo progresivo: 5 fallos → 15 min; mismo mensaje y tiempo para email inexistente.

**Tokens y sesiones**
- Acceso `wk_at_` (15 min) y refresh `wk_rt_` (rotativo; 30 días de inactividad, 90 días absolutos; en cajas compartidas la empresa puede acortarlo). Opacos, 256 bits, solo `sha256` en `iam.sessions`.
- Rotación del refresh en cada uso; reuso de un refresh ya rotado ⇒ se revoca toda la familia (`refresh_family`) y se avisa al usuario, **salvo dentro de la gracia de 10 s**: si llega el refresh inmediatamente anterior (`prev_refresh_hash`) antes de 10 s desde `rotated_at`, la API **no revoca y devuelve solo un access nuevo**. Nunca devuelve un refresh: la base solo guarda `sha256` de los tokens, así que no tiene en claro el refresh vigente, y emitir otro bifurcaría la familia. El BFF, al recibir solo `access_token`, actualiza `wk_at` y `wk_at_exp` y **no toca `wk_rt`**. Las dos pestañas, o las dos réplicas de `web`, comparten el jar de cookies del navegador, que ya recibió el refresh ganador. Prueba de integración: dos refresh concurrentes del mismo token; al minuto la familia sigue válida y existe un solo refresh vigente.
- **Dónde se refresca en el BFF (Next 16).** Los Server Components **no pueden escribir cookies**: si refrescaran durante el render, el token nuevo se perdería y el siguiente render reusaría el viejo, lo que revocaría la familia. Por eso hay solo dos lugares que refrescan, igual que el portal de Khipu (`bacsystem/khipu`, `portal/`: `middleware.ts` + `app/api/proxy/[...path]/route.ts` con `refrescarUnaVez`):
  1. `proxy.ts` (antes `middleware.ts`) refresca **de forma proactiva antes del render** cuando `wk_at_exp` (cookie no secreta con el vencimiento del acceso, porque el token es opaco) indica menos de 60 s. Escribe las cookies en la respuesta y en la petición que sigue al render.
  2. El route handler proxy (`app/api/proxy/[...path]`) y los route handlers de mutación reintentan **una vez** ante un 401, refrescando con **single-flight**: un candado por sesión a nivel de módulo, para que dos peticiones concurrentes compartan una sola rotación.

  La gracia de 10 s cubre lo que el single-flight no ve, como dos instancias de `web` o dos pestañas. Los Server Components solo leen la cookie, y un 401 a mitad del render redirige a `/login?next=`.
- Revocación: cerrar sesión, "cerrar otras sesiones", cambio de contraseña, suspensión de usuario o de membresía ⇒ `revoked_at`, efecto inmediato.
- `authz_version` en la membresía: cambiar el rol invalida la caché de permisos de esa membresía.
- Cookies (las pone el BFF, solo en `proxy.ts` y route handlers): `wk_at`, `wk_rt`, `wk_at_exp`, con `httpOnly`, `Secure` y `SameSite=Lax`. `wk_rt` lleva `Path=/`, porque `proxy.ts` lo necesita en cualquier ruta para refrescar antes del render. El riesgo de enviarlo en cada petición se acota con `httpOnly`, la rotación y la detección de reuso. CSRF: SameSite + comprobación de `Origin` en mutaciones del BFF.

**2FA**
- TOTP (RFC 6238, 30 s, ventana ±1, rechazo de código reutilizado), secreto cifrado con AES-GCM (`TOTP_ENC_KEY`), 10 códigos de recuperación de un uso (hash).
- **Obligatorio** (decisión del usuario) para propietario, administrador y contador (`iam.roles.requires_mfa = true` en esas plantillas) y para todo el staff de plataforma. Para los demás roles es opcional, y la empresa puede volverlo obligatorio marcando `requires_mfa` en sus roles. **Se decide en un solo punto: después de validar la contraseña y antes de emitir tokens** (`sequences/01-login-2fa-tenant.mmd`). La API lee las membresías propias con `iam.list_my_memberships()`. Si el usuario tiene TOTP, se le pide el código. Si no lo tiene y algún rol lo exige, se le pide enrolarse con un token que solo sirve para eso. Si no lo tiene y ningún rol lo exige, entra sin 2FA. `POST /v1/auth/session/tenant` solo hace de guarda: responde `403 mfa_required` cuando la membresía se creó después del login. La sesión guarda `mfa_satisfied` (verdadero si pasó la 2FA **o si no se le exigía**) y `mfa_method` (`totp`, `recovery_code`, `none`). El middleware de cada petición exige `mfa_satisfied = true`, así que el cajero sin 2FA opcional entra; la guarda de `session/tenant` compara `requires_mfa` del rol elegido con `mfa_method`. Prueba e2e: un cajero sin TOTP inicia sesión y `GET /v1/me` responde 200.
- **`requires_mfa` se comprueba en cada petición, no solo al elegir empresa.** **De dónde sale la versión vigente:** la búsqueda de la sesión ocurre **antes** de conocer el tenant y el usuario, así que corre como `wanka_app` **sin** `app.tenant_id` ni `app.user_id`. Un `JOIN` directo contra `iam.memberships`, que tiene RLS, devolvería NULL: lo reprodujo la revisión. Por eso la búsqueda es una función acotada:
  ```sql
  -- Dueño: wanka_definer (NOLOGIN, NOBYPASSRLS, no es dueño de tablas). Ver ADR 0001, "Funciones SECURITY DEFINER".
  CREATE FUNCTION iam.session_lookup(p_access_hash bytea)
    RETURNS TABLE (session_id uuid, user_id uuid, tenant_id uuid, site_id uuid, cash_register_id uuid, membership_id uuid,
                   session_authz_version bigint, mfa_satisfied boolean, mfa_method text, impersonator_id uuid,
                   current_authz_version bigint, membership_status text)
    LANGUAGE plpgsql STABLE SECURITY DEFINER
    SET search_path = pg_catalog, pg_temp
  AS $$
  DECLARE s iam.sessions%ROWTYPE; v bigint; st text; prev text := current_setting('app.tenant_id', true);
  BEGIN
    SELECT * INTO s FROM iam.sessions x
     WHERE x.access_hash = p_access_hash AND x.revoked_at IS NULL AND x.access_expires_at > now()
       AND x.mfa_satisfied;   -- una sesion a medio login (sin el codigo TOTP) no autentica peticiones
    IF NOT FOUND THEN RETURN; END IF;
    IF s.membership_id IS NOT NULL THEN
      PERFORM set_config('app.tenant_id', s.tenant_id::text, true);   -- el tenant de ESA sesion
      SELECT m.authz_version, m.status INTO v, st FROM iam.memberships m WHERE m.id = s.membership_id;  -- via tenant_isolation
      PERFORM set_config('app.tenant_id', coalesce(prev, ''), true);  -- restaura el valor del llamador
    END IF;
    RETURN QUERY SELECT s.id, s.user_id, s.tenant_id, s.site_id, s.cash_register_id, s.membership_id, s.authz_version,
                        s.mfa_satisfied, s.mfa_method, s.impersonator_id, v, st;
  END $$;
  ALTER FUNCTION iam.session_lookup(bytea) OWNER TO wanka_definer;
  REVOKE ALL ON FUNCTION iam.session_lookup(bytea) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION iam.session_lookup(bytea) TO wanka_app;
  ```
  Devuelve también `site_id` y `cash_register_id`, porque la venta toma la serie de la caja de la sesión (`sequences/02-pos-sale-cpe.mmd`), y `mfa_satisfied`. Filtra las sesiones con `mfa_satisfied = false`, que es lo que asume `sequences/08-entitlement-enforcement.mmd`.
  **Por qué así:** el dueño de la función no tiene `BYPASSRLS` e `iam.memberships` tiene `FORCE ROW LEVEL SECURITY`. Una versión `LANGUAGE sql` con `JOIN` directo ve la membresía con RLS y sin tenant, así que devuelve `NULL` y revocaría todas las sesiones en su primera petición (lo reprodujo la revisión). La función lee primero la sesión (`iam.sessions` no tiene RLS), fija **el tenant de esa sesión** con `set_config(..., true)`, lee la membresía por `tenant_isolation` y restaura el valor del llamador. La cláusula `SET app.tenant_id = ''` en `CREATE FUNCTION`, que restauraría sola, no sirve: con un rol no superusuario, PostgreSQL 16 responde `permission denied to set parameter "app.tenant_id"`. Si la función falla, la transacción o el savepoint se revierten, y con ellos el `set_config`. **Verificado en PostgreSQL 16** con los roles reales: `data/rls-check/`.
  Solo encuentra la fila cuyo hash coincide, y quien llama necesita el token para calcularlo. Devuelve la versión **actual** de la membresía. No hay caché antes de esa lectura. **Si una sesión de tenant no impersonada (con `membership_id`) no tiene versión actual** (membresía borrada) **o la membresía no está activa, se revoca y se responde 401** (las sesiones impersonadas no tienen membresía: se validan contra su grant, que debe seguir vigente y sin revocar): una versión ausente nunca se trata como "sin cambios", así que nunca se escribe NULL en la sesión. Si la versión actual difiere de la guardada en la sesión (`iam.sessions.authz_version`), el middleware re-resuelve el rol y los permisos (ya dentro de `InTenantTx`), aplica el control de 2FA y actualiza la versión de la sesión. La caché de permisos se consulta con la clave `membership_id + versión actual`, así que una entrada caliente con la versión vieja nunca se usa. Con ese rol compara: si `requires_mfa = true` y la sesión tiene `mfa_method = none`, responde `403 mfa_required` y **revoca esa sesión de tenant**. El BFF manda al usuario a `/login`, donde se le pide enrolarse.
  - Esto cubre el cajero ascendido a administrador y el rol que pasa a exigir 2FA.
  - Como cambiar el rol incrementa `authz_version`, el efecto es inmediato: no espera a la caché ni al vencimiento del refresh.
  - Las pruebas corren **contra PostgreSQL con RLS activo y con el rol real `wanka_app`**, nunca con un superusuario ni con el dueño de las tablas. `data/rls-check/run.sh` lo hace en un contenedor desechable de PostgreSQL 16.
  - Prueba de integración: borrar o suspender la membresía hace que la siguiente petición responda 401 y revoque la sesión.
  - **Dónde se decide** (precisión del 2026-11-18, épica #154): en el paso de sesión del middleware, justo después de `iam.session_lookup` y **antes** de los entitlements, para toda ruta (también las que no necesitan tenant, como `GET /v1/me`). No cuesta otra consulta ni depende de la caché: usa `current_authz_version` y `membership_status`, que la búsqueda ya devolvió. Evaluarla después de los entitlements haría que un ex miembro de una empresa morosa recibiera `402` en vez de `401`, que revela el estado de la empresa y no lo lleva a `/login`. La re-resolución por versión cambiada y el control de 2FA siguen después de los entitlements (`sequences/08-entitlement-enforcement.mmd`).
  - **Si la base falla en la búsqueda** (error de conexión, `statement_timeout` o el plazo de 1 s del contexto), la respuesta es `503 dependency_unavailable` con `Retry-After: 1` (`api/conventions.md` §4), nunca `401` ni `500`: sin la búsqueda no se sabe si la sesión es válida, y un `401` haría que el BFF refresque contra la misma base caída y mande al usuario a `/login`. Un pool sin conexión libre es `503 overloaded`. El resolver distingue «sin fila» (`401`) de «error» (`503`).
  - Prueba e2e **con la caché caliente** (dos peticiones previas del cajero, que llenan la caché con la versión vieja): ascenderlo a administrador sin TOTP; su siguiente petición responde 403 y la sesión queda revocada.
- **Sesiones impersonadas** (ADR 0001, camino 6): los permisos salen del alcance del grant (`read_only` o `full`), nunca de un rol del tenant. `full` excluye `identidad.support_access:*`, `identidad.users:*`, `identidad.roles:*`, `identidad.sessions:*` y `empresas.einvoicing:update`. El staff no tiene TOTP en el plano tenant: su 2FA ya se cumplió en el plano plataforma, y la sesión lleva `mfa_method = platform`. Como se exige por rol, quien es cajero en una empresa y administrador en otra debe tener 2FA para entrar a la segunda. El mock muestra `sms`: **SMS no se propone** como segundo factor (SIM swapping); pregunta abierta al PO.

**Recuperación**: `forgot` responde 202 siempre; token de un uso, 15 min, hash en base; `reset` revoca todas las sesiones y exige 2FA si estaba activo.

**Selección de tenant y sede**: tras 2FA la sesión no tiene tenant; `POST /v1/auth/session/tenant` valida la membresía activa, el estado del tenant y el alcance de sede, y emite tokens nuevos ligados a `tenant_id`, `site_id`. Cambiar de empresa = tokens nuevos. Se elimina la cookie `tenant_id` del frontend como fuente de autorización.

**RBAC**
- Permisos con código `modulo.recurso:accion` (acciones `read | create | update | void | export | approve`), catálogo global versionado en migraciones; cada módulo declara los suyos en su `module.go`.
- Roles por tenant (`iam.roles`); al aprovisionar se copian plantillas por rubro (p. ej. "Cajero POS / Recepción", "Administrador de sede", "Contador externo / Auditor" del mock). El tenant puede crear roles propios.
- Membresía = usuario × tenant × rol × sedes. El middleware resuelve permisos (caché en memoria por `membership_id + authz_version`, donde la versión es la **actual**, leída de `iam.memberships` en el `JOIN` de la búsqueda de sesión; TTL 60 s) y cada handler declara el permiso que exige en el OpenAPI (`x-permission`), verificado en una prueba que recorre todas las rutas.
- El plan/módulo/flag se comprueba antes que el permiso (ADR 0005).

**Super-admin (plano plataforma)**: usuarios en `platform.platform_users`, sesiones en `platform.platform_sessions`, tokens `wka_`, servicio `platform-api` separado, 2FA TOTP **obligatorio** siempre, sesión más corta (acceso 10 min, refresh 12 h absolutos), allowlist de IP opcional. Roles de staff del mock (`roleMatrix`: SuperAdmin, Soporte, Finanzas, Ventas, Ingeniería). SSO (OIDC con el IdP corporativo) queda para un ADR posterior; el diseño lo admite como otro método de login que termina en la misma `platform_session`. Ningún token de un plano es válido en el otro (tablas, prefijos y servicios distintos).

## Consecuencias

- Buenas: revocación inmediata; sin proveedor externo ni claves de firma; el modelo cubre N empresas con roles distintos.
- Malas: el equipo es responsable de la seguridad del flujo completo (requiere revisión de seguridad y pruebas dedicadas); una lectura extra de base por petición (aceptable con los números de `capacity.md`, a medir).
- Qué haría cambiar la decisión: necesidad de SSO de clientes empresariales (SAML/OIDC por tenant) o de clientes móviles/terceros con OAuth: se evaluaría un servidor OIDC dedicado.
