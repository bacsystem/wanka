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
| `make seed` | empresa y usuarios de prueba: **pendiente de #24** (falla con un mensaje claro hasta que exista `wanka dev-seed`) |
| `make api` / `make web` | API o web en primer plano, para depurar |
| `make test` / `make test-api` | pruebas de web y API / solo API contra el PostgreSQL local |
| `make rls-check` | `docs/architecture/data/rls-check/run.sh` (su propio contenedor) |
| `make lint` | eslint, `gofmt` y `go vet` |
| `make prune-volumes` | borra solo volúmenes Docker **anónimos colgados** (nombre de 64 hex y sin contenedor; nunca los con nombre, como `wanka-local-pgdata`) con `CONFIRM=1`; limpia restos de corridas viejas |
| `make logs` | sigue PostgreSQL, API y web (los de API y web están en `.local/*.log`) |

## Volúmenes

El único volumen de este entorno es `wanka-local-pgdata` (nombre fijo). El servicio `roles` y `rls-check` usan `tmpfs` para el directorio de datos, así que no dejan volúmenes anónimos. `make prune-volumes` limpia los que dejaron versiones anteriores.

## Pruebas sueltas

- Una prueba unitaria: `pnpm --filter web exec vitest run src/lib/format.test.ts`.
- Un proyecto E2E: `pnpm --filter web exec playwright test --project=mobile -g "sidebar"`.
- Primera corrida de E2E: `pnpm --filter web exec playwright install chromium`.

## Claves

`make env` (lo llama `make up`) copia `.env.example` a `.env.local` y reemplaza los `CHANGE_ME` por valores aleatorios. `.env.local` y `.local/` están en `.gitignore`: nunca se versionan. Todos los roles de base comparten la misma clave local; no uses estas claves fuera de tu máquina.

## Resetear

`make reset` y luego `make up`. Solo toca el proyecto `wanka-local`; los demás contenedores del equipo no se tocan. Si cambiás `WANKA_PG_PORT` o las claves después del primer `make up`, hacé `make reset` (la clave queda guardada en el volumen; `make up` lo detecta y lo dice).

## Prueba de punta a punta

Cuando estén el login (#24) y la semilla (`make seed`):

1. `make reset && make up && make seed`.
2. Abrí `http://localhost:3400/login` e ingresá con cada usuario de prueba (uno por rol base: dueño, administrador, contador, cajero, vendedor, almacenero; las contraseñas las imprime `make seed`).
3. Comprobá que el shell vacío carga y que cada rol ve solo lo que sus permisos permiten.
4. `curl http://127.0.0.1:58080/readyz` debe devolver `{"status":"ready"}`; con `docker compose -p wanka-local stop postgres` debe dar 503 sin matar la API.
5. `make down` al terminar.

Hoy (sin login ni semilla) se puede comprobar `make up`, las rutas de salud, el shell vacío y `make test-api`.
