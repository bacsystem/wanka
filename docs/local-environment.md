# Entorno local

Levanta Wanka completo en una máquina de desarrollo con un comando (historia #250). Es solo para local: el despliegue (Railway, ADR 0007), las imágenes de producción y la CI no entran.

## Requisitos

Docker con Compose v2, Go 1.25 o superior, Node 24 o superior con pnpm 9 (`pnpm install` una vez), `curl` y `make`.

## Qué levanta

| Pieza | Cómo corre | URL / puerto |
|---|---|---|
| PostgreSQL 16 (`postgres:16-alpine`) | contenedor `wanka-local-pg`, proyecto compose `wanka-local`, volumen propio | `127.0.0.1:55432` |
| Bootstrap de roles | servicio de una sola vez (`roles`, perfil `tools`): corre `apps/api/bootstrap/roles.sql` y fija una clave local a cada rol LOGIN | |
| Migraciones | `wanka migrate up` con `go run`, como `wanka_migrator` | |
| API | binario `wanka serve --plane=tenant` compilado con tu Go, en segundo plano | `http://127.0.0.1:58080` (`/healthz`, `/readyz`) |
| Web | `next dev` en segundo plano, con `WANKA_API_URL` apuntando a la API | `http://localhost:3400` |

La API no va en un contenedor: `go build` local evita mantener una imagen y reutiliza la caché de módulos. Los puertos (55432, 58080, 3400) no chocan con `wanka-shared-pg` (55400) ni con `rls-check` (que usa contenedores con nombre único); se cambian en `.env.local`.

## Comandos

`make help` los lista.

| Comando | Qué hace |
|---|---|
| `make up` | genera `.env.local` si falta, levanta PostgreSQL, aplica roles y migraciones, arranca API y web y termina diciendo las URL |
| `make down` | detiene API, web y PostgreSQL; conserva los datos |
| `make reset` | borra contenedor, volumen de datos y estado local (pide confirmación; `CONFIRM=1 make reset` la salta) |
| `make migrate` / `make roles` | aplica migraciones / el bootstrap de roles (idempotentes) |
| `make seed` | empresa de prueba y un usuario por rol base (idempotente; corre `wanka dev-seed`, que se niega si no es `WANKA_ENV=local` con una base en loopback) |
| `make api` / `make web` | API o web en primer plano, para depurar |
| `make test` / `make test-api` | pruebas de web y API / solo API contra el PostgreSQL local |
| `make rls-check` | `docs/architecture/data/rls-check/run.sh` (su propio contenedor) |
| `make lint` | eslint, `gofmt` y `go vet` |
| `make prune-volumes` | **solo lista** los volúmenes Docker anónimos colgados con su fecha; no borra nada |
| `make logs` | sigue PostgreSQL, API y web (los de API y web están en `.local/*.log`) |

## Volúmenes

El único volumen de este entorno es `wanka-local-pgdata` (nombre fijo). El servicio `roles` y `rls-check` usan `tmpfs` para el directorio de datos, así que no dejan volúmenes anónimos. Las fugas viejas venían de `rls-check/run.sh` (un volumen anónimo por corrida). `make prune-volumes` los lista, pero en esta máquina también pueden ser de otros proyectos: revisalos antes de borrar y hacelo de a uno.

## Pruebas sueltas

- Una prueba unitaria: `pnpm --filter web exec vitest run src/lib/format.test.ts`.
- Un proyecto E2E: `pnpm --filter web exec playwright test --project=mobile -g "sidebar"`.
- Primera corrida de E2E: `pnpm --filter web exec playwright install chromium`.

## Claves

`make env` (lo llama `make up`) copia `.env.example` a `.env.local` y reemplaza los `CHANGE_ME` por valores aleatorios. `.env.local` y `.local/` están en `.gitignore`: nunca se versionan. Todos los roles de base comparten la misma clave local; no uses estas claves fuera de tu máquina.

## Resetear

`make reset` y luego `make up`. Solo toca el proyecto `wanka-local`; los demás contenedores del equipo no se tocan. Si cambiás `WANKA_PG_PORT` o las claves después del primer `make up`, hacé `make reset` (la clave queda guardada en el volumen; `make up` lo detecta y lo dice).

## Prueba de punta a punta

`make seed` crea la empresa «Empresa Demo SAC» (RUC inventado 20999999990), sus roles base, un local `PRINCIPAL` con la caja `CAJA-01` y un usuario activo por rol. Todos comparten la contraseña de prueba `Demo-Wanka-2026-local` (solo local; datos inventados, sin correo real: el dominio `demo.local` no se resuelve).

| Rol | Correo | Login hoy |
|---|---|---|
| dueño | `dueno@demo.local` | 503 hasta que #25 permita enrolar el 2FA (el rol lo exige) |
| administrador | `administrador@demo.local` | 503, igual |
| contador | `contador@demo.local` | 503, igual |
| cajero | `cajero@demo.local` | 200 |
| vendedor | `vendedor@demo.local` | 200 |
| almacenero | `almacenero@demo.local` | 200 |

`make seed` escribe como el **superusuario** del PostgreSQL local (`DATABASE_URL_SEED`, que solo fija el Makefile): ningún rol de servicio puede crear empresas ni usuarios globales, y el superusuario se salta la RLS (incluida `FORCE`). Por eso `wanka dev-seed` solo corre con `WANKA_ENV=local` y con todos los hosts de la URL en loopback o socket Unix. Al resembrar, cada usuario queda sin bloqueos ni racha de fallos, y la empresa y las membresías, activas.

Pasos:

1. `CONFIRM=1 make reset && make up && make seed` (la semilla se puede repetir: repone contraseñas y desbloqueos).
2. `curl -X POST http://127.0.0.1:58080/v1/auth/login -H 'Content-Type: application/json' -d '{"email":"cajero@demo.local","password":"Demo-Wanka-2026-local"}'` debe dar 200 con los tokens; repetilo con cada rol de la tabla.
3. Abrí `http://localhost:3400` y comprobá que el shell vacío carga.
4. `curl http://127.0.0.1:58080/readyz` debe devolver `{"status":"ready"}`; con `docker compose -p wanka-local stop postgres` debe dar 503 sin matar la API.
5. `make down` al terminar.

Nota: `make reset` solo detiene la API y la web cuyo pid está en el `.local/` de ese árbol de trabajo; si levantaste la API desde otro worktree, detenela allí.

## E2E del login (#24)

`pnpm --filter web test:e2e login` corre contra la API real: si `WANKA_API_URL` (por defecto `http://127.0.0.1:58080`) responde en `/readyz`, `e2e/global-setup.ts` siembra por `docker exec wanka-local-pg psql` (se cambia con `E2E_DB_CONTAINER`) una empresa y, por proyecto de Playwright, cuatro usuarios (`e2e-{ok,bad,nomembership,mfa}-{desktop,tablet,mobile}@e2e.wanka.test`) con una contraseña aleatoria de esa corrida; no hay credenciales en el repositorio. Sin la API, los specs se saltan. Son usuarios propios del E2E, aparte de los de `make seed`.
