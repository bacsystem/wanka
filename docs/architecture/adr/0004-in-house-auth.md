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
- Rotación del refresh en cada uso; reuso de un refresh ya rotado ⇒ se revoca toda la familia (`refresh_family`) y se avisa al usuario, **salvo dentro de la gracia de 10 s**: si llega el refresh inmediatamente anterior (`prev_refresh_hash`) antes de 10 s desde `rotated_at`, la API devuelve el par vigente en vez de revocar. Esto cubre dos pestañas o dos peticiones que refrescan a la vez.
- **Dónde se refresca en el BFF (Next 16).** Los Server Components **no pueden escribir cookies**: si refrescaran durante el render, el token nuevo se perdería y el siguiente render reusaría el viejo, lo que revocaría la familia. Por eso hay solo dos lugares que refrescan, igual que el portal de Khipu (`bacsystem/khipu`, `portal/`: `middleware.ts` + `app/api/proxy/[...path]/route.ts` con `refrescarUnaVez`):
  1. `proxy.ts` (antes `middleware.ts`) refresca **de forma proactiva antes del render** cuando `wk_at_exp` (cookie no secreta con el vencimiento del acceso, porque el token es opaco) indica menos de 60 s. Escribe las cookies en la respuesta y en la petición que sigue al render.
  2. El route handler proxy (`app/api/proxy/[...path]`) y los route handlers de mutación reintentan **una vez** ante un 401, refrescando con **single-flight**: un candado por sesión a nivel de módulo, para que dos peticiones concurrentes compartan una sola rotación.

  La gracia de 10 s cubre lo que el single-flight no ve, como dos instancias de `web` o dos pestañas. Los Server Components solo leen la cookie, y un 401 a mitad del render redirige a `/login?next=`.
- Revocación: cerrar sesión, "cerrar otras sesiones", cambio de contraseña, suspensión de usuario o de membresía ⇒ `revoked_at`, efecto inmediato.
- `authz_version` en la membresía: cambiar el rol invalida la caché de permisos de esa membresía.
- Cookies (las pone el BFF, solo en `proxy.ts` y route handlers): `wk_at`, `wk_rt`, `wk_at_exp`, con `httpOnly`, `Secure` y `SameSite=Lax`. `wk_rt` lleva `Path=/`, porque `proxy.ts` lo necesita en cualquier ruta para refrescar antes del render. El riesgo de enviarlo en cada petición se acota con `httpOnly`, la rotación y la detección de reuso. CSRF: SameSite + comprobación de `Origin` en mutaciones del BFF.

**2FA**
- TOTP (RFC 6238, 30 s, ventana ±1, rechazo de código reutilizado), secreto cifrado con AES-GCM (`TOTP_ENC_KEY`), 10 códigos de recuperación de un uso (hash).
- **Obligatorio** (decisión del usuario) para propietario, administrador y contador (`iam.roles.requires_mfa = true` en esas plantillas) y para todo el staff de plataforma. Para los demás roles es opcional, y la empresa puede volverlo obligatorio. Si un usuario con rol obligatorio no tiene 2FA activo, el login le exige enrolarse antes de elegir empresa. Como se exige por rol, quien es cajero en una empresa y administrador en otra debe tener 2FA para entrar a la segunda. El mock muestra `sms`: **SMS no se propone** como segundo factor (SIM swapping); pregunta abierta al PO.

**Recuperación**: `forgot` responde 202 siempre; token de un uso, 15 min, hash en base; `reset` revoca todas las sesiones y exige 2FA si estaba activo.

**Selección de tenant y sede**: tras 2FA la sesión no tiene tenant; `POST /v1/auth/session/tenant` valida la membresía activa, el estado del tenant y el alcance de sede, y emite tokens nuevos ligados a `tenant_id`, `site_id`. Cambiar de empresa = tokens nuevos. Se elimina la cookie `tenant_id` del frontend como fuente de autorización.

**RBAC**
- Permisos con código `modulo.recurso:accion` (acciones `read | create | update | void | export | approve`), catálogo global versionado en migraciones; cada módulo declara los suyos en su `module.go`.
- Roles por tenant (`iam.roles`); al aprovisionar se copian plantillas por rubro (p. ej. "Cajero POS / Recepción", "Administrador de sede", "Contador externo / Auditor" del mock). El tenant puede crear roles propios.
- Membresía = usuario × tenant × rol × sedes. El middleware resuelve permisos (caché en memoria por `membership_id + authz_version`, TTL 60 s) y cada handler declara el permiso que exige en el OpenAPI (`x-permission`), verificado en una prueba que recorre todas las rutas.
- El plan/módulo/flag se comprueba antes que el permiso (ADR 0005).

**Super-admin (plano plataforma)**: usuarios en `platform.platform_users`, sesiones en `platform.platform_sessions`, tokens `wka_`, servicio `platform-api` separado, 2FA TOTP **obligatorio** siempre, sesión más corta (acceso 10 min, refresh 12 h absolutos), allowlist de IP opcional. Roles de staff del mock (`roleMatrix`: SuperAdmin, Soporte, Finanzas, Ventas, Ingeniería). SSO (OIDC con el IdP corporativo) queda para un ADR posterior; el diseño lo admite como otro método de login que termina en la misma `platform_session`. Ningún token de un plano es válido en el otro (tablas, prefijos y servicios distintos).

## Consecuencias

- Buenas: revocación inmediata; sin proveedor externo ni claves de firma; el modelo cubre N empresas con roles distintos.
- Malas: el equipo es responsable de la seguridad del flujo completo (requiere revisión de seguridad y pruebas dedicadas); una lectura extra de base por petición (aceptable con los números de `capacity.md`, a medir).
- Qué haría cambiar la decisión: necesidad de SSO de clientes empresariales (SAML/OIDC por tenant) o de clientes móviles/terceros con OAuth: se evaluaría un servidor OIDC dedicado.
