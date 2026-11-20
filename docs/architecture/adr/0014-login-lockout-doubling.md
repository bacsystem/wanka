# ADR 0014: Bloqueo creciente del login, sin cierre permanente

- Estado: aceptado (2026-11-20, decisión del dueño; el architect fija el reinicio y los bordes). **Corregido el mismo día** por una segunda decisión del dueño, que agrega un techo de 100 fallos seguidos: ver la nota al final. Ante una diferencia, manda la nota.
- Fecha: 2026-11-20
- Modifica: [ADR 0004](0004-in-house-auth.md), solo el «Techo absoluto» de la viñeta «Bloqueo progresivo» de su nota del 2026-11-19 y la mención del «ingreso cerrado» en «Qué ve quien llega bloqueado». El resto de esa nota sigue vigente: el orden semáforo → cobro → hash, la ventana fija de 15 min, `failed_logins` de 0 a 5, la devolución del cobro con la contraseña correcta, la respuesta `401` genérica y el aviso por el outbox. Ante una diferencia, manda este ADR ([README §9](../README.md)).

## Contexto

La nota del 2026-11-19 del ADR 0004 cerraba el ingreso con contraseña al llegar a `login_lockouts = 10` (50 fallos seguidos) hasta restablecerla. Era un techo absoluto, pensado para el límite de NIST SP 800-63B §5.2.2 (no más de 100 fallos seguidos por cuenta). Su costo: cualquiera que conozca el correo puede, con 50 intentos repartidos en unas 2,5 h, obligar al titular a restablecer la contraseña, y repetirlo cuantas veces quiera.

**Decisión del dueño (2026-11-20):** no habrá cierre permanente. Cada bloqueo dura el doble que el anterior, con un tope de 24 h, y el titular recibe un aviso en cada uno.

## Opciones (para el reinicio del contador)

1. **Solo un ingreso correcto** (y el restablecimiento o el cambio de contraseña) vuelve `login_lockouts` a 0, como hoy.
2. **Además, N horas sin fallos** lo vuelven a 0.
3. **Decaimiento:** una unidad menos cada N horas sin fallos.

## Decisión

Bloqueo creciente con la opción 1.

- **Duración.** Al 5.º fallo de la ventana, `n = LEAST(login_lockouts + 1, 10)` y `locked_until = now() + LEAST(interval '15 minutes' * 2 ^ (n - 1), interval '24 hours')`:

  | n | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 a 10 |
  |---|---|---|---|---|---|---|---|---|
  | duración | 15 min | 30 min | 1 h | 2 h | 4 h | 8 h | 16 h | 24 h |

  En la misma transacción: `failed_logins = 0`, `login_lockouts = n` y el aviso en el outbox, como hasta ahora.
- **Contador.** `login_lockouts` se **satura** en 10 y no sigue creciendo: desde el 8.º bloqueo la duración ya es el tope, así que más rango no cambia nada. El `CHECK BETWEEN 0 AND 10` de la migración de #231 sirve tal cual, sin una migración nueva. El cobro del intento deja de llevar `AND login_lockouts < 10`: el ingreso nunca se cierra.
- **Reinicio, solo con un ingreso correcto** (la contraseña correcta, con o sin el segundo paso después; devuelve también el cobro), con `password/reset` o con `password/change`. Se descartaron la opción 2 y la 3 por tres razones:
  - Un reinicio por tiempo le devuelve al atacante paciente los bloqueos cortos. Si espera N horas, recupera el escalón de 15 min y vuelve a probar unas 35 contraseñas en el día que dura la subida de 15 min a 16 h, en vez de 5.
  - Al titular no le cuesta nada. Su primer ingreso correcto lo limpia, y mientras no hay fallos el contador no se nota.
  - El aviso de cada bloqueo le dice que alguien está probando, y la salida (restablecer o cambiar la contraseña) también limpia.
- **Avisos.** Uno por bloqueo, idempotente por `user_id` y `locked_until`. El techo de correos sale solo: unos 8 en el primer día y después 1 por día.
- **Qué ve quien llega bloqueado:** sin cambios, `401 invalid_credentials` con el mismo cuerpo y tiempo que una contraseña mala. Ya no existe el caso «ingreso cerrado».

## Consecuencias

- **Buenas:**
  - Nadie puede dejar a un titular sin acceso más de 24 h seguidas solo por conocer su correo. Antes podía obligarlo a restablecer la contraseña.
  - No hace falta ninguna migración, ni columna, ni estado nuevo.
  - En régimen, el ritmo de prueba de un atacante baja a 5 contraseñas por día y por cuenta.
- **Malas:**
  - **Ya no hay techo absoluto.** Un atacante paciente suma unos 35 intentos el primer día y 5 por día después, unos 1 800 al año por cuenta. Eso **no cumple** el límite de NIST SP 800-63B §5.2.2 (100 fallos seguidos), que se alcanza en unas dos semanas, y contradice la regla «techo absoluto» de `review-checklist.md` para este caso. Lo mitigan:
    - la lista de contraseñas filtradas (#238) y el mínimo de 10 caracteres, que dejan fuera las contraseñas que se adivinan con pocos intentos;
    - las cuotas por IP de `conventions.md` §7;
    - el aviso al titular en cada bloqueo;
    - la 2FA obligatoria en los roles con más privilegios.
  - Un titular que nunca vuelve a entrar sigue sufriendo bloqueos de 24 h mientras alguien pruebe. Es lo pedido: nunca queda cerrado.
- **Qué la haría cambiar:** que la medición de los avisos muestre ataques sostenidos contra cuentas sin 2FA. En ese caso se volvería a un techo, por ejemplo exigir el restablecimiento al llegar a 100 fallos seguidos, como pide NIST, con un ADR nuevo.

## Nota del 2026-11-20: techo en 100 fallos seguidos (decisión del dueño, «creciente + techo en 100»)

Cambia la decisión de arriba en un punto: **sí** hay cierre, a los 100 fallos seguidos. El bloqueo creciente se mantiene igual: 15 min que se duplican hasta 24 h, con un aviso en cada bloqueo. Con esto se cumple NIST SP 800-63B §5.2.2, y desaparece la «consecuencia mala» de arriba, que dejaba la cuenta sin techo absoluto.

**El contador: columna nueva `iam.users.login_failure_streak`.** Es `smallint NOT NULL DEFAULT 0`, con `CHECK BETWEEN 0 AND 100`, y llega con una migración nueva en el PR #251, sin editar la de #231.
- Se descartó derivarlo de `login_lockouts` (5 × bloqueos + `failed_logins`) porque **no es correcto**. `failed_logins` vive en la ventana fija de 15 min y vuelve a 1 cuando la ventana vence. Un atacante que prueba 4 contraseñas por ventana nunca provoca un bloqueo, así que ni el bloqueo ni un contador derivado ven esos fallos: son 384 intentos por día, sin aviso y sin techo.
- La columna cuenta **todo** fallo, con cualquier espaciado.
- Solo la ponen en 0 una contraseña correcta (al devolver el cobro), `password/reset` y `password/change`. No la reinician ni la ventana ni los bloqueos.

**Qué dispara el bloqueo.** Por lo mismo, el bloqueo se dispara por la racha y no por la ventana: cada 5 fallos seguidos hay un bloqueo, con cualquier espaciado. La ventana de 15 min y `failed_logins` siguen sirviendo para lo que ya hacían en el cobro: acotar los intentos en vuelo (5 por ventana) y descartar un cobro colgado.

**Cifras.**
- **Cuánto le cuesta a un atacante cerrar una cuenta.** 100 fallos son 20 bloqueos. Los bloqueos 1 a 7 suman 15 min + 30 min + 1 h + 2 h + 4 h + 8 h + 16 h = 31 h 45 min. Los bloqueos 8 a 19, de 24 h cada uno, suman 12 días. El 20.º es el cierre. En total son unos **13,3 días: unas 2 semanas**, siempre que pruebe en cuanto vence cada bloqueo.
- **Avisos que recibe el titular antes del cierre.** **19 avisos de bloqueo** (uno por bloqueo, del 1 al 19) y después **un aviso de cierre**. En total son 20 correos en unas 2 semanas, y el primero llega en los primeros minutos del ataque.
- **Cuántas contraseñas prueba el atacante.** 100 como máximo por cada contraseña que tenga la cuenta. Las cuotas por IP de `conventions.md` §7 solo bajan el ritmo.
- **La salida es #26** (restablecer la contraseña desde «Olvidé mi contraseña», que exige el buzón) o #30 (cambiarla con una sesión abierta). Las dos ponen la racha en 0. Como el segundo paso, el cierre solo lo puede provocar quien insiste durante 2 semanas, y el titular lo ve venir por los avisos.

**Qué ve quien llega cerrado:** lo mismo que con un bloqueo, `401 invalid_credentials` con el mismo cuerpo y tiempo que una contraseña mala, sin cobrar el intento.
