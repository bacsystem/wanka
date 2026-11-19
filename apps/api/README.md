# apps/api

Backend de Wanka: un binario Go (`wanka`) con módulos hexagonales. Decisiones en
[ADR 0002](../../docs/architecture/adr/0002-modular-monolith-go.md). Módulo Go:
`github.com/bacsystem/wanka/apps/api` (Go 1.25 o superior).

Estado: kernel con conexión a PostgreSQL 16, migraciones goose y bootstrap de
roles. Aún no hay tablas de negocio, OpenAPI ni despliegue (Railway queda para
más adelante).

## Estructura

```
cmd/wanka/            main: serve, worker, migrate up
internal/platform/    kernel: config, logging, httpx, auth, metrics, clock, db (+ db/dbtest para tests) en uso;
                      jobs, money, entitlements, idgen (paquetes vacíos por ahora)
bootstrap/roles.sql   bootstrap de roles: lo corre el superusuario con psql, fuera de goose
migrations/           goose, SQL con timestamp, embebido en el binario, solo hacia adelante
openapi/              contrato (vacío)
testdata/archlint/    ejemplo de la regla entre módulos para go-arch-lint
scripts/archlint.sh   corre go-arch-lint y demuestra que una importación prohibida (y un pgxpool fuera de internal/platform/db) falla
scripts/migrations-check.sh  check de migraciones solo hacia adelante y sus casos malos
```

## Compilar y probar

```sh
cd apps/api
go build -o bin/wanka ./cmd/wanka
gofmt -l .               # no debe imprimir nada
go vet ./...
go test -race ./...
sh scripts/archlint.sh   # go-arch-lint (versión fijada en el script)
sh scripts/migrations-check.sh   # solo hacia adelante (base: origin/main, o main)
```

Los tests de integración usan una base PostgreSQL 16 real (sin mocks). Si no
está `WANKA_TEST_ADMIN_URL` se saltan en local; en CI (`CI` definida) fallan.
Cada test crea y borra su propia base, así que los paquetes no se pisan.

## Base de datos local

```sh
docker run -d --name wanka-pg -e POSTGRES_PASSWORD=<superusuario> -e POSTGRES_DB=wanka \
  -p 127.0.0.1:55432:5432 postgres:16-alpine
# 1. bootstrap de roles, como superusuario (idempotente, sin contraseñas)
psql "postgres://postgres:<superusuario>@127.0.0.1:55432/wanka" -v ON_ERROR_STOP=1 -f bootstrap/roles.sql
# 2. contraseñas: a mano, una por rol (en psql: \password wanka_app, etc.)
# 3. migraciones, como wanka_migrator
DATABASE_URL_MIGRATOR=postgres://wanka_migrator:<clave>@127.0.0.1:55432/wanka ./bin/wanka migrate up
# 4. tests de integración (todos los roles con la misma clave de prueba)
WANKA_TEST_ADMIN_URL=postgres://postgres:<superusuario>@127.0.0.1:55432/wanka \
WANKA_TEST_ROLE_PASSWORD=<clave de prueba> go test -race ./...
```

En la CI (`.github/workflows/api.yml`) el mismo flujo corre contra un servicio
PostgreSQL 16 con contraseñas de prueba que solo existen allí.

## Ejecutar

Todos los subcomandos exigen la configuración de abajo y terminan limpio con
`SIGTERM` o `SIGINT` (código 0). Un subcomando o `--plane` desconocido sale con
código 2 y un mensaje en stderr.

```sh
wanka serve --plane=tenant      # API de la empresa
wanka serve --plane=platform    # consola de administración
wanka serve --plane=webhooks    # receptor de webhooks
wanka worker                    # stub: abre su pool, arranca y espera la señal
wanka migrate up                # aplica las migraciones pendientes y termina
```

`serve` expone en todos los planos:

| Ruta | Respuesta |
|---|---|
| `GET /healthz` (alias `GET /livez`) | 200 siempre; no consulta dependencias |
| `GET /readyz` | 200 si la base responde; si no, 503 `dependency_unavailable` (`application/problem+json`, `Retry-After: 1`) y el proceso sigue vivo |

## Base HTTP (`internal/platform/httpx`)

`httpx.Chain` (request id, log de acceso, recuperación de `panic`, límite de
cuerpo) envuelve a un `httpx.Router`; los módulos registran ahí sus rutas con
`rt.Handle("GET /v1/items/{id}", h, opts...)`.

- **Errores**: todo error es `application/problem+json` (RFC 9457) con `type`,
  `title`, `status`, `code` y `requestId`. El `code` sale de la lista cerrada de
  `conventions.md` §4 y su HTTP de una sola tabla (`httpx.Codes()`); el handler
  devuelve `httpx.NewError(code, detail, fields...)` y llama a `httpx.WriteError`.
  Un `code` fuera del catálogo, un error sin tipar o un `panic` dan `500 internal`
  sin detalle (la causa va al log).
- **Rutas**: método + camino. Lo no declarado, otro método y `OPTIONS` son
  `404 not_found` (no hay `405` ni CORS).
- **Cuerpo**: máximo 1 MB. Con `Content-Length` mayor, `413 payload_too_large` sin
  leer el cuerpo; sin él, la lectura se corta en 1 MB + 1 byte. `httpx.DecodeJSON`
  da `400 malformed_request` (con `errors[].field` si un campo tiene el tipo
  incorrecto) ante JSON inválido, datos sobrantes o `Content-Type` distinto de JSON.
- **Plazos**: cada ruta corre con 10 s; si vence, `503 overloaded` con
  `Retry-After: 1`, el contexto del handler se cancela (la sentencia en curso se
  cancela y `InTenantTx` hace `ROLLBACK`) y se descarta lo que el handler hubiera
  escrito: la respuesta se guarda en un buffer y `Flush` no envía nada hasta
  terminar. Las descargas y los exportes se registran con `httpx.Streaming()`:
  sin buffer ni plazo de 10 s, con `Flush` y un contexto de 25 s desde la llegada
  (si vence a mitad del envío, la conexión se corta).
- **Errores de base** (`httpx.WriteDBError`): `db.ErrOverloaded` (pool sin
  conexión libre en 200 ms) es `503 overloaded`; `db.ErrUnavailable` (la base no
  responde) y `db.ErrStatementTimeout` (SQLSTATE 57014) son `503
  dependency_unavailable`, los tres con `Retry-After: 1` y sin la causa. El
  `statement_timeout` deja una línea `warn` (ruta, fase, módulo, sin SQL) y suma
  al contador `db_statement_timeouts_total` por fase y ruta del puerto
  `internal/platform/metrics`. Con el contexto de la petición ya terminado no
  escribe nada.
- **Rate limit**: `httpx.NewLimiter` es un token bucket en memoria con clave
  inyectable (`httpx.KeyFunc`) y reloj inyectable (`internal/platform/clock`). Se
  engancha en la ruta con `httpx.Use(limiter.Middleware())`; varios limitadores
  encadenados no consumen ficha si uno rechaza. `429 rate_limited` con
  `Retry-After`, y `RateLimit-Limit`, `-Remaining` y `-Reset` en toda respuesta de
  la ruta. Con `MaxKeys` buckets no llenos, una clave nueva recibe `503 overloaded`.
  Las claves de sesión y tenant y los valores por defecto están en la sección «Sesión».
- La sobrecarga de la cadena se repite con `go test -run '^$' -bench Overhead -benchmem ./internal/platform/httpx` y, con carga real, con `testdata/loadtest` (servidor + script k6).
- Una conexión perdida del pool (reinicio o failover de la base) es `503 dependency_unavailable`, no `500`. El log de acceso marca con `aborted` una conexión cortada con `http.ErrAbortHandler` (status 0 si no se envió nada). Las peticiones que `net/http` rechaza antes de la cadena (`/%zz`, línea de petición inválida) las responde Go en `text/plain` (conventions.md §4).
- Todo `429` y `503` del kernel lleva `Retry-After` en segundos enteros >= 1.

Cada respuesta lleva `X-Request-Id`: se conserva el entrante si son de 1 a 128
caracteres de `[A-Za-z0-9_-]` y, si falta o no cumple, se reemplaza por uno nuevo
(nunca `400`).

## Sesión (`internal/platform/auth`)

`auth` define el puerto `SessionResolver` (`Lookup` por `sha256` del token y
`Detach`), la `SessionRecord` que devuelve `iam.session_lookup`, la `Session`
del handler y el registrador `auth.Routes`. El adaptador de PostgreSQL llega con
#196; el middleware se prueba con un doble.

- **Registro**: los módulos registran sus rutas `/v1` con
  `routes.Handle(pattern, class, h, opts...)`, `class` de tipo `auth.RouteClass`
  (`Public`, `NoTenant`, `Tenant`; sin valor cero válido). Nadie recibe el
  `httpx.Router` (`Router.Handle` sigue siendo público: el cableado de #196 no lo expone). Una clase omitida o que no cumple `conventions.md` §2 entra en
  pánico al arrancar. `Public` solo bajo `/v1/auth/`; `NoTenant` es `/v1/auth/*`,
  `GET /v1/me` y `GET /v1/me/memberships`; el resto es `Tenant`. Una ruta tiene
  una sola clase (el enrolamiento TOTP son dos rutas).
- **Token**: `Authorization: Bearer wk_at_<43 caracteres>`, una sola cabecera.
  `auth.ParseAuthorization` comprueba los 43 caracteres de `[A-Za-z0-9_-]` uno por
  uno antes de decodificar con `Strict()` (Go ignora `\r` y `\n`) y devuelve el
  `sha256` del token completo. Lo demás es `401 unauthenticated` sin consultar la
  base. El token y su hash no salen de ahí y no se registran.
- **Orden**: token, búsqueda (plazo de 1 s, fase `session_lookup`; la fase no llega
  al handler), `MFASatisfied` y coherencia de la fila, desligue de la membresía
  perdida (ADR 0011, una llamada a `Detach` y el contador
  `auth_session_detach_total{outcome}`), `403 tenant_access_denied` en rutas
  `Tenant` sin empresa, límite por sesión y, solo en `Tenant`, por tenant, y los
  pasos de la ruta (`httpx.Use`). Sin fila o `MFASatisfied = false` es
  `401 session_expired`; un fallo de la base o el plazo vencido es `503`; un error
  no tipado o una fila incoherente es `500`. Todo `401` lleva
  `WWW-Authenticate: Bearer`. Una impersonada llega con `UserID` vacío y su
  `Actor.UserID` es el `ImpersonatorID`.
- **Límites por defecto** (`auth.Options` los reemplaza en los tests): sesión 10/s
  con ráfaga de 20, tenant 100/s con ráfaga de 200, con topes de 50 000 y 10 000
  claves por réplica. Son supuestos que ajusta la prueba de carga de #196.
  `Routes.Sweep()` elimina los buckets llenos; llamarlo cada 10 s es de `serve`.
- El handler lee la sesión con `auth.FromContext(ctx)`. Solo un handler de ruta
  `Tenant` pasa su `Actor` a `db.InTenantTx`: ahí corren los entitlements, la
  versión, la 2FA, el RBAC y el límite del tenant. En una ruta `NoTenant` el
  tenant de la sesión (si lo tiene) es informativo y no se entra a datos de la
  empresa. Los módulos registran rutas solo por `auth.Routes`; el `httpx.Router`
  no sale de `cmd/wanka`. Los entitlements (fase `entitlements`) llegan con #175.
- Sobrecarga: `go test -run '^$' -bench Session -benchmem -count 5 ./internal/platform/auth`.

## Transacciones de tenant (`InTenantTx`)

Todo acceso a tablas de tenant pasa por `db.Pool.InTenantTx` (lectura y
escritura) o `InTenantReadTx` (`BEGIN READ ONLY`); el aislamiento lo da la RLS
(ADR 0001), sin `WHERE tenant_id` en el código. `Actor` lleva `TenantID`,
`UserID`, `Kind` (`KindUser`, `KindImpersonated`, `KindSystem`; no hay valor
cero válido) e `ImpersonatorID` (solo con `KindImpersonated`). Los UUID son
`pgtype.UUID` de pgx. Errores: `ErrNoTenant` (sesión sin empresa o UUID cero,
para todo `Kind`), `ErrInvalidActor` (error de programación: `Kind` indefinido, `ImpersonatorID` incoherente o, salvo en `KindSystem`, `UserID` vacío) y
`ErrStatementTimeout` (5 s del rol `wanka_app`); ninguno de los dos primeros
abre transacción.

Hay una segunda puerta para todo acceso a la base sin tenant fijado, de
cualquier servicio (`docs/architecture/data/data-model.md`, «Puerta sin
tenant»): `InNoTenantTx` e `InNoTenantReadTx` con `NoTenantScope{UserID}`
(opcional). En `api`: la búsqueda y el desligue de sesión (#174, #196), el login
(#24), `iam.list_my_memberships` con `UserID` (#27) y el canje
`iam.redeem_impersonation`; en `webhooks`, `ops.resolve_webhook_route`; en
`platform-api`, las `platform_fn.*` y las tablas `platform.*`; en `worker`, la
limpieza de `iam.sessions`. Los datos de un tenant van siempre por
`InTenantTx`. Antes de `fn` limpia `app.tenant_id`, `app.session_kind` y
`app.impersonator_id` (y deja `app.user_id` en el `UserID` del scope o en `''`)
en la misma sentencia que fija las variables, sin una ida y vuelta más; un valor
de sesión que otra petición dejó en la conexión no se hereda. Los errores son
los de `InTenantTx`. En las dos puertas sigue prohibido `set_config(..., false)`
dentro de `fn`. Las pruebas usan la tabla `rlstest.items`, que solo existe en
`internal/platform/db/testdata/` y se aplica con `dbtest.MigrateWith`.

## Pruebas del catálogo RLS

`internal/platform/db/catalog_test.go` revisa `pg_catalog` tras `migrate up` (ADR
0001, «Migraciones y pruebas» y «Funciones `SECURITY DEFINER`»). Las reglas no
listan tablas: corren sobre todo lo que haya migrado, así que una tabla nueva de
cualquier módulo queda cubierta sin tocar el test.

| Regla | Falla si |
|---|---|
| `tenant-tables-have-force-rls-and-policy` | una tabla con `tenant_id` (o `org.tenants`) no tiene RLS, `FORCE` y `tenant_isolation` (`FOR ALL`, `USING` y `WITH CHECK` con `app.current_tenant()`) |
| `extra-policies-are-whitelisted` | hay una política que no es `tenant_isolation` ni está en `policyAllow` (tabla, comando, `TO` y rama `app.current_tenant() IS NULL`) |
| `unique-and-pk-start-with-tenant-id` | un `UNIQUE` o PK de una tabla con `tenant_id` no empieza por `tenant_id` (la PK sustituta `id uuid` o identidad no cuenta) |
| `policies-wrap-current-setting-in-nullif` | una política usa `current_setting` sin `NULLIF` |
| `foreign-keys-are-indexed` | una FK no tiene índice que empiece por sus columnas |
| `tenant-foreign-keys-carry-tenant-id` | una FK entre dos tablas de tenant no lleva `tenant_id` en ambos lados |
| `security-definer-functions` | una función `SECURITY DEFINER` no es de `wanka_definer`, no fija `search_path = pg_catalog, pg_temp` o `PUBLIC` ejecuta |
| `application-roles` | un rol `wanka_*` tiene `BYPASSRLS` o es superusuario, o uno que no es `wanka_migrator` es dueño de tablas |

`TestCatalogRulesCatchMutations` aplica, por cada regla, una migración de prueba
(en memoria, nunca en `migrations/`) que la viola y exige que la regla nombre el
objeto; los defectos de roles se inyectan como superusuario y se restauran. Al
agregar una regla, agrega su mutación.

**Cómo agregar una excepción.** Una excepción es un PR de arquitectura: cita el
ADR (o su erratum) que la justifica y, si el ADR 0001 no la nombra, se agrega
allí primero. En la parte alta de `catalog_test.go`, una entrada con su motivo,
que tiene que contener «ADR NNNN» (`TestCatalogRules` lo comprueba, y que el
objeto exista, para que no queden entradas viejas):

- `noRLSTables`: tabla con `tenant_id` que por diseño no es de tenant (clave `esquema.tabla`). Hoy solo `iam.sessions`. Una tabla `platform.*` sin RLS por ADR entra aquí.
- `uniqueAllow`: `UNIQUE` global marcado (clave `esquema.tabla(col1,col2)`).
- `fkAllow`: FK entre tablas de tenant sin `tenant_id` (clave `esquema.tabla.restricción`).
- `policyAllow`: política adicional (clave `esquema.tabla:política`), con comandos, roles y si exige la rama `IS NULL`.
- `tenantRoots`: tabla de tenant que se aísla por otra columna que `tenant_id` (hoy `org.tenants`, por `id`).

**PR que toca políticas, roles, grants o funciones definer.** Además de estas
pruebas, actualiza `docs/architecture/data/rls-check/` (el SQL y, si cambia lo
que se debe detectar, sus mutaciones) y corre `sh apps/api/scripts/rls-check.sh`
con salida 0. Lo exige el revisor del PR (`/pr-review`) y la sección «Verificación»
del ADR 0001; el workflow `api.yml` corre ese script cuando cambia
`apps/api/` o `rls-check/`.

## Variables de entorno

Solo nombres; los valores no se versionan (no hay `.env` en el repo).

Cada servicio se conecta con su propio rol de base de datos (ADR 0001, ADR
0007), así que no hay un `DATABASE_URL` único: cada comando exige la URL de su
rol y no lee las de los demás.

| Variable | Obligatoria para | Para qué |
|---|---|---|
| `DATABASE_URL_APP` | `serve --plane=tenant` | URL del rol `wanka_app`. Secreto |
| `DATABASE_URL_PLATFORM` | `serve --plane=platform` | URL del rol `wanka_platform`. Secreto |
| `DATABASE_URL_WEBHOOKS` | `serve --plane=webhooks` | URL del rol `wanka_webhooks`. Secreto |
| `DATABASE_URL_WORKER` | `worker` | URL del rol `wanka_worker`. Secreto |
| `DATABASE_URL_MIGRATOR` | `migrate up` | URL del rol `wanka_migrator`. Secreto |
| `DB_POOL_MAX_CONNS` | no | Tamaño máximo del pool, 1 a 200. Por defecto: api 20, worker 10, webhooks 5, platform 5 (`capacity.md`) |
| `HTTP_ADDR` | no (`:8080`) | Dirección donde escucha `serve` (con `127.0.0.1:0` el sistema elige el puerto y se informa en el log `listening`) |
| `LOG_LEVEL` | no (`info`) | `debug`, `info`, `warn` o `error` |

Solo para tests: `WANKA_TEST_ADMIN_URL` (URL del superusuario de una base de
pruebas) y `WANKA_TEST_ROLE_PASSWORD` (clave común de los cinco roles LOGIN en
esa base).

Si falta una obligatoria el proceso no arranca (código 1) y el mensaje nombra la
variable, nunca su valor.

## Logs

JSON de `log/slog` en stdout. Incluyen `request_id` cuando existe. Las claves
sensibles (token, secret, password, authorization, cookie, `database_url`,
email, dni, teléfono...) se reemplazan por `[REDACTED]`, y el log de acceso
registra la ruta pero nunca la query ni las cabeceras.

## Arquitectura verificada (go-arch-lint)

`.go-arch-lint.yml` define las reglas del árbol real. La regla entre módulos
(un módulo importa de otro solo su fachada, nunca `domain`, `app` ni
`adapters`) está ejercitada en `testdata/archlint`; al agregar un módulo en
`internal/<modulo>/` se agrega su par de componentes al archivo de reglas.
