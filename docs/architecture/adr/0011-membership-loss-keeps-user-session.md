# ADR 0011: Perder la membresía termina la sesión en esa empresa, no la sesión del usuario

- Estado: propuesto
- Fecha: 2026-11-18
- Modifica: [ADR 0004](0004-in-house-auth.md), solo en lo que lista «Qué reemplaza del ADR 0004». El resto del ADR 0004 sigue vigente. Su texto no se reescribe ([README §9](../README.md)): ante una diferencia, manda este ADR.

## Contexto

ADR 0004 revoca la sesión cuando la membresía de su tenant está borrada o no activa, y la API responde `401`. Como `POST /v1/auth/session/tenant` revoca la sesión sin tenant y emite una nueva ligada a la empresa ([secuencia 01](../sequences/01-login-2fa-tenant.mmd), paso «revoca tokens actuales, nueva sesion ligada a tenant_id»), el usuario ya no tiene ninguna otra sesión: la revocación lo saca de todas sus empresas y el BFF lo manda a `/login`.

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

**Qué reemplaza del ADR 0004** (el texto del 0004 queda como estaba; estas frases dejan de valer):
- «Revocación: … suspensión de usuario **o de membresía** ⇒ `revoked_at`». Ahora solo revoca la suspensión del usuario, definida abajo. Quitar o suspender una membresía desliga la sesión.
- «Si una sesión de tenant no impersonada … no tiene versión actual … o la membresía no está activa, **se revoca y se responde 401**». Ahora se desliga, y la ruta de tenant responde `403 tenant_access_denied`. La frase «nunca se escribe NULL en la sesión» se refiere a `authz_version` cuando la versión actual falta y la sesión sigue ligada. Desligar sí escribe `NULL`, en todas las columnas del tenant a la vez.
- «Prueba de integración: borrar o suspender la membresía hace que la siguiente petición responda 401 y revoque la sesión». La reemplaza la prueba de #174: `403 tenant_access_denied` en una ruta de tenant, sesión sin tenant y sin revocar, y `200` en `GET /v1/me/memberships`.
- En «Dónde se decide», «recibiera `402` en vez de `401` … y no lo lleva a `/login`». El lugar y el orden de la decisión no cambian; cambia el código, que ahora es `403 tenant_access_denied`, y el destino, que ahora es `/seleccionar-empresa`.

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
- No se agrega un `code`. Se amplía `403 tenant_access_denied`, que ahora también lo emite el kernel. Su HTTP no cambia y su significado sigue siendo «sin membresía activa en esa empresa». Para un cliente, la misma situación pasa de `401` a `403`. Eso no está en la lista de cambios compatibles de `conventions.md` §0, así que se registra allí como excepción: se aplica sin `/v2` porque todavía no hay consumidores (ni la API ni el BFF existen).
- **Re-resolución por versión cambiada** (secuencia 08, #176): su `UPDATE iam.sessions SET authz_version = :current` lleva la misma guarda, `WHERE id = $1 AND membership_id = $2 AND revoked_at IS NULL`. Si un desligue concurrente ganó, actualiza 0 filas y no deja una sesión sin tenant con versión: el estado desligado es uno solo, con todas las columnas del tenant en `NULL`. Con 0 filas, esa petición responde `403 tenant_access_denied` y el handler no se ejecuta: la membresía ya se perdió y no se atiende nada de A después de eso.
- **Códigos `401` del paso de sesión (aclaración):** `iam.session_lookup` **no distingue** una sesión inexistente de una vencida o revocada, porque filtra `revoked_at` y `access_expires_at` y en los tres casos no devuelve fila. Por eso «sin fila» es siempre `401 session_expired`, sea inexistente, vencida o revocada, e incluye la impersonada cuyo grant se revocó o venció. `401 unauthenticated` queda para lo que se rechaza **sin consultar la base**: falta `Authorization`, el formato es inválido o el prefijo es de otro plano. El BFF trata los dos igual: un refresh y, si falla, `/login`.
- **Refresh:** no evalúa la membresía y sigue igual (misma familia, rotación y gracia de 10 s). El único punto de decisión es el paso de sesión de la siguiente petición. Una sesión desligada se refresca como cualquier sesión sin tenant.
- **Suspender o quitar la membresía ya no escribe `revoked_at`** en las sesiones de esa membresía. El efecto sigue siendo inmediato, porque la búsqueda lee el estado vigente en cada petición. Suspender al **usuario**, cambiar la contraseña, cerrar sesión y «cerrar otras sesiones» siguen revocando, igual que en ADR 0004.
- **Qué es cada suspensión.** La **suspensión del usuario** bloquea la cuenta global (`iam.users.status = bloqueado`, [er-01](../data/er-01-identidad-empresas.mmd)). Vale para todas sus empresas, revoca todas sus sesiones y no la puede hacer un administrador de una empresa: es del plano plataforma, y su endpoint no existe todavía. Lo que un administrador de empresa hace desde `/configuracion/usuarios` (`PATCH /v1/users/{id}`, [`resources.md`](../api/resources.md)) es suspender **la membresía** en su empresa, que desliga y no revoca.
- **Si la membresía vuelve a estar activa** antes de la siguiente petición del usuario, la sesión sigue en A. No hay estado intermedio que limpiar.

**Sesiones impersonadas** (`impersonator_id`, ADR 0001, camino 6): son del staff (`user_id` es el usuario de plataforma del grant) y el grant es por empresa, no por usuario. No tienen membresía (`membership_id IS NULL`), así que nunca se desligan y el `UPDATE` las excluye. Borrar o suspender cualquier membresía de esa empresa no las cambia. Si su grant se revoca o vence, la sesión se **revoca entera**, como hasta ahora (ADR 0001: «la revocación corta la sesión»; acceso de 60 min desde el canje). La siguiente búsqueda no devuelve fila y la API responde `401 session_expired`: el staff no tiene una sesión de usuario del plano tenant a la que volver, y su salida es la consola.

**En el BFF**
- `403 tenant_access_denied` en cualquier ruta lleva a `/seleccionar-empresa` con el aviso «Ya no tienes acceso a esa empresa». No hace refresh ni escribe cookies: los tokens no cambiaron. Por eso funciona también desde un Server Component, que solo hace `redirect`.
- **No revelar nada de A en la lista.** `GET /v1/me/memberships` devuelve **solo las membresías `activo`**. Una membresía borrada o suspendida no aparece, ni con su empresa, ni con su estado, ni con el estado de la empresa, ni con un motivo. Así `/seleccionar-empresa` no se vuelve otra vía para que un ex miembro sepa si A está morosa, suspendida o cerrada, lo mismo que el orden del middleware evita con el `402`. El criterio de #27 que muestra una empresa suspendida «deshabilitada con el motivo» vale solo para quien **sigue** siendo miembro activo: es la empresa la que está suspendida, no su membresía. Las invitaciones (`invitado`) no cambian con este ADR.
- En `/seleccionar-empresa`, con una o más membresías activas, el usuario elige y `POST /v1/auth/session/tenant` emite tokens nuevos para B, como siempre. El aviso «Ya no tienes acceso a esa empresa» no nombra a A.
- **Si queda una sola empresa activa** (decisión del dueño, 2026-11-18, PR #185): se muestra `/seleccionar-empresa` con el aviso y esa única empresa; no se entra directo. Cuesta un clic más, pero el usuario ve por qué salió de A y no lo cambian de empresa sin que lo elija. Esto prevalece, en este caso, sobre el criterio «una sola empresa entra directo» de #27, que sigue valiendo en el login. Se descartó entrar directo: solo funciona con una sede y como mucho una caja (`session/tenant` exige `siteId`) y exige un route handler intermedio que escriba las cookies.
- **Sin membresías activas:** el BFF cierra la sesión (`POST /v1/auth/logout` desde un route handler, que sí puede borrar cookies) y lleva a `/login` con el aviso «Ya no perteneces a ninguna empresa».
- `401 session_expired` y `401 unauthenticated` mantienen su comportamiento: un refresh y, si falla, `/login`.
- Los criterios del BFF están en #27.

## Consecuencias

- Buenas: cumple la decisión del dueño sin un segundo juego de tokens ni cambios en la rotación del refresh. No agrega `code`, consultas ni funciones `SECURITY DEFINER`, y el texto del ADR 0004 no se reescribe. Mantiene el orden del middleware, así que un ex miembro no ve `402` ni `403` de entitlements. También define qué responde una ruta de tenant con una sesión sin tenant.
- Malas:
  - El access token sobrevive a la pérdida de la membresía. Solo sirve en las rutas sin tenant, y para entrar a otra empresa se necesita una membresía activa allí.
  - Un usuario sin membresías activas conserva, hasta que el BFF lo desloguea o la sesión vence, una sesión que solo puede leer su propio perfil y su lista vacía de membresías.
  - `tenant_access_denied` cubre dos casos: se eligió una empresa sin membresía, y se perdió la membresía de la empresa en uso. Para el BFF la acción es la misma (`/seleccionar-empresa`). El `detail` del problema los distingue para el aviso y para los logs.
  - El `UPDATE` de desligar corre en el paso de sesión, fuera de `InTenantTx`. Es una escritura por sesión afectada, una sola vez.
- Qué haría cambiar la decisión: necesitar sesiones simultáneas en dos empresas en el mismo navegador, o auditar por separado la vida de cada sesión de tenant. Ahí convendría la opción 2.
