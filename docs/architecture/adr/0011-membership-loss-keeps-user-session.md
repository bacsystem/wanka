# ADR 0011: Perder la membresía termina la sesión en esa empresa, no la sesión del usuario

- Estado: propuesto
- Fecha: 2026-11-18
- Modifica: [ADR 0004](0004-in-house-auth.md) (revocación por membresía y «Dónde se decide»). El resto de ADR 0004 sigue vigente.

## Contexto

ADR 0004 revoca la sesión cuando la membresía de su tenant está borrada o no activa, y la API responde `401 session_expired`. Como `POST /v1/auth/session/tenant` revoca la sesión sin tenant y emite una nueva ligada a la empresa ([secuencia 01](../sequences/01-login-2fa-tenant.mmd), paso «revoca tokens actuales, nueva sesion ligada a tenant_id»), el usuario ya no tiene ninguna otra sesión: la revocación lo saca de todas sus empresas y el BFF lo manda a `/login`.

**Decisión del dueño (2026-11-18, pregunta 3 del PR #181):** a quien le quitan o suspenden la membresía de la empresa A mientras la usa **no se le lleva a `/login`, sino a `/seleccionar-empresa`**, donde elige otra empresa con membresía activa. Si no le queda ninguna, va a `/login`. La sesión del usuario sigue viva; solo termina su sesión en A.

También pesa la autoridad: quien quita la membresía es un administrador de A, que no tiene por qué poder cerrar la sesión con la que el usuario trabaja en B.

Restricciones que se mantienen:
- La decisión ocurre en el paso de sesión, justo después de `iam.session_lookup` y antes de los entitlements: un ex miembro nunca ve `402` ni `403` que revelen el estado de la empresa (ADR 0004, «Dónde se decide»).
- Sin consultas extra ni caché: se usa `current_authz_version` y `membership_status`, que ya devuelve la búsqueda.
- Los Server Components del BFF no pueden escribir cookies (ADR 0004, «Dónde se refresca en el BFF»).

## Opciones

1. **No hacer nada:** revocar y `401 session_expired`. Contradice la decisión del dueño.
2. **Sesión de usuario padre y sesión de tenant hija:** `session/tenant` deja de revocar la sesión sin tenant y crea una hija. Perder la membresía revoca solo la hija y el BFF vuelve a usar los tokens de la padre. Exige dos juegos de cookies y dos familias de refresh que se rotan por separado, cambia la secuencia 01 y la detección de reuso, y duplica los casos de carrera del refresh. Es mucho esfuerzo y mucho riesgo para un caso raro.
3. **Responder `401 membership_revoked` con tokens nuevos sin tenant en el cuerpo del error:** el BFF tendría que escribir cookies al recibir un error, y un Server Component no puede. Además, un error que entrega credenciales es un patrón que nadie espera. Agrega un `code`.
4. **Desligar la sesión en su sitio (elegida):** la misma fila de `iam.sessions`, con los mismos tokens y la misma familia de refresh, pierde el tenant (`tenant_id`, `membership_id`, `site_id`, `cash_register_id` y `authz_version` pasan a `NULL`). Queda como queda una sesión recién salida de la 2FA: solo sirve para `/v1/auth/*`, `GET /v1/me` y `GET /v1/me/memberships`. Una ruta de tenant con una sesión sin tenant responde `403 tenant_access_denied`, un código que ya existe y que ya significa «no tienes membresía activa en esa empresa».

## Decisión

Opción 4.

**En la API (paso de sesión del middleware, issue #174)**
- Si `iam.session_lookup` devuelve una sesión **no impersonada** con `membership_id` y (`current_authz_version IS NULL` o `membership_status <> 'activo'`), el middleware **desliga** la sesión con un `UPDATE` condicional, como `wanka_app` (`iam.sessions` no tiene RLS y `wanka_app` ya tiene `UPDATE`):
  ```sql
  -- Desliga la sesión de su empresa: conserva tokens, familia de refresh y 2FA.
  UPDATE iam.sessions
     SET tenant_id = NULL, membership_id = NULL, site_id = NULL, cash_register_id = NULL, authz_version = NULL
   WHERE id = $1 AND membership_id = $2 AND impersonator_id IS NULL AND revoked_at IS NULL;
  ```
  Es idempotente: si dos peticiones concurrentes lo detectan, la segunda actualiza 0 filas. Los `CHECK` de `iam.sessions` ya admiten la fila resultante y obligan a borrar `tenant_id` junto con `membership_id`. `data/rls-check/` lo verifica (caso A9).
- El contexto de la petición queda **sin tenant**, sea cual sea el resultado del `UPDATE`. Si el `UPDATE` falla, la decisión no cambia, y la siguiente búsqueda lo vuelve a detectar.
- Después sigue la rama normal «¿ruta sin tenant?». Las rutas sin tenant (`/v1/auth/*`, `GET /v1/me`, `GET /v1/me/memberships`) se atienden. **Toda ruta de tenant con una sesión sin tenant responde `403 tenant_access_denied`**, antes de los entitlements. Esto también cubre un caso que antes no tenía respuesta definida: una ruta de tenant llamada con una sesión que nunca eligió empresa.
- No se revoca nada y no se usa `401`: el usuario sigue autenticado, solo perdió el acceso a A. Como es un `403`, el BFF no intenta un refresh, que no haría falta.
- No se agrega un `code`. Se amplía `403 tenant_access_denied`, que ahora también lo emite el kernel. Su HTTP y su significado no cambian (`conventions.md` §4).
- **Refresh:** no evalúa la membresía y sigue igual (misma familia, rotación y gracia de 10 s). El único punto de decisión es el paso de sesión de la siguiente petición. Una sesión desligada se refresca como cualquier sesión sin tenant.
- **Suspender o quitar la membresía ya no escribe `revoked_at`** en las sesiones de esa membresía. El efecto sigue siendo inmediato, porque la búsqueda lee el estado vigente en cada petición. Suspender al **usuario**, cambiar la contraseña, cerrar sesión y «cerrar otras sesiones» siguen revocando, igual que en ADR 0004.
- **Si la membresía vuelve a estar activa** antes de la siguiente petición del usuario, la sesión sigue en A. No hay estado intermedio que limpiar.

**Sesiones impersonadas** (`impersonator_id`, ADR 0001, camino 6): nunca se desligan, porque no tienen membresía (`membership_id IS NULL`) y el `UPDATE` las excluye. Si su grant se revoca o vence, la sesión se **revoca entera**, como hasta ahora (ADR 0001: «la revocación corta la sesión»; acceso de 60 min desde el canje), y la API responde `401` como a cualquier sesión revocada o vencida: el staff no tiene una sesión de usuario del plano tenant a la que volver, y su salida es la consola. Que el usuario impersonado pierda una membresía no las afecta.

**En el BFF**
- `403 tenant_access_denied` en cualquier ruta lleva a `/seleccionar-empresa` con el aviso «Ya no tienes acceso a esa empresa». No hace refresh ni escribe cookies: los tokens no cambiaron. Por eso funciona también desde un Server Component, que solo hace `redirect`.
- En `/seleccionar-empresa`, `GET /v1/me/memberships` devuelve las membresías del usuario con su estado, y solo las activas se pueden elegir: A ya no lo está. Con una o más activas, el usuario elige y `POST /v1/auth/session/tenant` emite tokens nuevos para B, como siempre.
- **Sin membresías activas:** el BFF cierra la sesión (`POST /v1/auth/logout` desde un route handler, que sí puede borrar cookies) y lleva a `/login` con el aviso «Ya no perteneces a ninguna empresa».
- `401 session_expired` mantiene su comportamiento: un refresh y, si falla, `/login`.

## Consecuencias

- Buenas: cumple la decisión del dueño sin un segundo juego de tokens ni cambios en la rotación del refresh. No agrega `code`, consultas ni funciones `SECURITY DEFINER`. Mantiene el orden del middleware, así que un ex miembro no ve `402` ni `403` de entitlements. También define qué responde una ruta de tenant con una sesión sin tenant.
- Malas:
  - El access token sobrevive a la pérdida de la membresía. Solo sirve en las rutas sin tenant, y para entrar a otra empresa se necesita una membresía activa allí.
  - Un usuario sin membresías activas conserva, hasta que el BFF lo desloguea o la sesión vence, una sesión que solo puede leer su propio perfil y su lista vacía de membresías.
  - `tenant_access_denied` cubre dos casos: se eligió una empresa sin membresía, y se perdió la membresía de la empresa en uso. Para el BFF la acción es la misma (`/seleccionar-empresa`). El `detail` del problema los distingue para el aviso y para los logs.
  - El `UPDATE` de desligar corre en el paso de sesión, fuera de `InTenantTx`. Es una escritura por sesión afectada, una sola vez.
- Qué haría cambiar la decisión: necesitar sesiones simultáneas en dos empresas en el mismo navegador, o auditar por separado la vida de cada sesión de tenant. Ahí convendría la opción 2.
