# apps/api

Backend de Wanka: un binario Go (`wanka`) con módulos hexagonales. Decisiones en
[ADR 0002](../../docs/architecture/adr/0002-modular-monolith-go.md). Módulo Go:
`github.com/bacsystem/wanka/apps/api` (Go 1.25 o superior).

Estado: kernel de la ola 0 completo (épica #154, release v0.19.0). Incluye la
cadena `/v1` (sesión, tenant, entitlements, RBAC e idempotencia), la auditoría,
los jobs, `pricing.Compute` y el contrato OpenAPI con su código generado. La ola 1
está en curso (release v0.20.0): el módulo de identidad (épica #3) ya tiene el DDL
de identidad (#231, #232), el resolver de permisos con los roles base (#233), la
lista de contraseñas filtradas (#238) y el login (#24, ADR 0014). Aún no hay
despliegue (Railway queda para más adelante).

## Estructura

```
cmd/wanka/            main: serve, worker, migrate up
internal/identidad/    módulo de identidad (fachada): resolver de permisos de PostgreSQL (`NewPermissionResolver`) y `SeedTenantRoles`, la siembra de los roles base que llama el alta (#31);
internal/platform/    kernel: config, passwordpolicy (lista de contraseñas filtradas, #238), logging, httpx, auth (+ auth/pgsession, el resolver de sesión), metrics, clock, db (+ db/dbtest para tests), entitlements, permission, jobs en uso;
                      fx (tipo de cambio: ops.exchange_rates) en uso;
                      money, idgen (paquetes vacíos por ahora)
internal/identidad/   módulo de identidad (ola 1): login hoy; fachada + domain, app, adapters (argon2, postgres, httpapi)
bootstrap/roles.sql   bootstrap de roles: lo corre el superusuario con psql, fuera de goose
migrations/           goose, SQL con timestamp, embebido en el binario, solo hacia adelante
openapi/              contrato OpenAPI 3.1 partido por módulo (ver «Servidor generado»)
internal/apigen/      servidor generado de openapi/ (un paquete por módulo + components); no se edita a mano
testdata/archlint/    ejemplo de la regla entre módulos para go-arch-lint
scripts/archlint.sh   corre go-arch-lint y demuestra que una importación prohibida (y un pgxpool fuera de internal/platform/db) falla
scripts/openapi-check.sh     Redocly lint y oasdiff breaking contra la base (versiones fijadas en el script)
scripts/migrations-check.sh  check de migraciones solo hacia adelante y sus casos malos
scripts/breachedgen/         genera internal/platform/passwordpolicy/breached.bin (lista de contraseñas filtradas); ver docs/architecture/data/breached-passwords.md
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
go generate ./... && git status --short .   # el servidor generado no debe cambiar
sh scripts/openapi-check.sh origin/main      # lint del contrato y cambios incompatibles (necesita node)
```

Los tests de integración usan una base PostgreSQL 16 real (sin mocks). Si no
está `WANKA_TEST_ADMIN_URL` se saltan en local; en CI (`CI` definida) fallan.
Cada test crea y borra su propia base, así que los paquetes no se pisan.

## Base de datos local

Para levantar todo (PostgreSQL, roles, migraciones, API y web) con un comando: `make up`; ver [docs/local-environment.md](../../docs/local-environment.md). A mano:

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
wanka worker                    # ejecuta los jobs (colas de River); sin la base sale con código 1
wanka migrate up                # aplica las migraciones pendientes y termina
wanka dev-seed                  # solo local (WANKA_ENV=local): empresa y usuarios de prueba; ver docs/local-environment.md
```

`serve` expone en todos los planos:

| Ruta | Respuesta |
|---|---|
| `GET /healthz` (alias `GET /livez`) | 200 siempre; no consulta dependencias |
| `GET /readyz` | 200 si la base responde; si no, 503 `dependency_unavailable` (`application/problem+json`, `Retry-After: 1`) y el proceso sigue vivo |

## Base HTTP (`internal/platform/httpx`)

`httpx.Chain` (request id, log de acceso, recuperación de `panic`, límite de
cuerpo) envuelve a un `httpx.Router`. Las rutas que no son `/v1` (salud) se
registran con `rt.Handle("GET /healthz", h, opts...)`; `Handle` entra en pánico
con cualquier patrón que `/v1` pueda alcanzar (`/v1`, `/v1/...` o un comodín
inicial como `/{version}/x`) y con cualquier `%` (ServeMux desescapa `/v%31/x` a `/v1/x`). Las rutas `/v1` van por `auth.Routes` (ver «Sesión»).

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
del handler y el registrador `auth.Routes`. El adaptador de PostgreSQL es
`internal/platform/auth/pgsession`: `Lookup` corre `SELECT ... FROM
iam.session_lookup($1)` en `db.InNoTenantReadTx` y `Detach` el `UPDATE` del ADR
0011 en `db.InNoTenantTx`; ni otra consulta ni caché. `wanka serve --plane=tenant`
lo cablea con los limitadores de `conventions.md` §7 y una goroutine que llama a
`Routes.Sweep()` cada 10 s y termina en el apagado; los planos `platform` y
`webhooks` no montan nada de esto.

- **Registro**: los módulos registran sus rutas `/v1` con
  `routes.Handle(pattern, class, spec, h, opts...)`, `class` de tipo `auth.RouteClass`
  (`Public`, `NoTenant`, `Tenant`; sin valor cero válido). Nadie recibe el
  `httpx.Router`: `Router.Handle` rechaza `/v1` y solo `auth.Routes` llama a
  `Router.HandleV1`, así que ninguna ruta `/v1` queda sin clase. Una clase omitida o que no cumple `conventions.md` §2 entra en
  pánico al arrancar. `Public` solo bajo `/v1/auth/`; `NoTenant` es `/v1/auth/*`,
  `GET /v1/me` y `GET /v1/me/memberships`; el resto es `Tenant`. Una ruta tiene
  una sola clase (el enrolamiento TOTP son dos rutas). `spec` es un `auth.Spec`
  con `Module` y `Permission` (el `x-module` y el `x-permission` del spec; para
  «ninguno», las constantes `auth.NoModule` y `auth.NoPermission`, que valen `none`)
  y, si la ruta consume un límite del plan, `Limit` (`entitlements.LimitUsers` o
  `LimitCPEPerMonth`; `LimitSites` hace fallar el arranque hasta que alguien mida
  las sedes en uso). Un string vacío, un módulo en una ruta
  `Public` o `NoTenant`, `NoModule` en una ruta `Tenant` que no sea
  `GET /v1/exports` ni `GET /v1/me/entitlements` (y un módulo en esas dos), o un
  límite desconocido entran en pánico al arrancar.
- **Token**: `Authorization: Bearer wk_at_<43 caracteres>`, una sola cabecera.
  `auth.ParseAuthorization` comprueba los 43 caracteres de `[A-Za-z0-9_-]` uno por
  uno antes de decodificar con `Strict()` (Go ignora `\r` y `\n`) y devuelve el
  `sha256` del token completo. Lo demás es `401 unauthenticated` sin consultar la
  base. El token y su hash no salen de ahí y no se registran.
- **Orden**: token, búsqueda (plazo de 1 s, fase `session_lookup`; la fase no llega
  al handler), `MFASatisfied` y coherencia de la fila, desligue de la membresía
  perdida (ADR 0011, una llamada a `Detach` y el contador
  `auth_session_detach_total{outcome}`), `403 tenant_access_denied` en rutas
  `Tenant` sin empresa, límite por sesión y, solo en `Tenant`, por tenant,
  entitlements (abajo) y los pasos de la ruta (`httpx.Use`; ahí va RBAC, #176). Sin fila o `MFASatisfied = false` es
  `401 session_expired`; un fallo de la base o el plazo vencido es `503`; un error
  no tipado o una fila incoherente es `500`. Todo `401` lleva
  `WWW-Authenticate: Bearer`. Una impersonada llega con `UserID` vacío y su
  `Actor.UserID` es el `ImpersonatorID`.
- **Límites por defecto** (`auth.Options` los reemplaza en los tests): sesión 10/s
  con ráfaga de 20, tenant 100/s con ráfaga de 200, con topes de 50 000 y 10 000
  claves por réplica. Son supuestos que ajusta la prueba de carga de #196.
  `Routes.Sweep()` elimina los buckets llenos; `serve` lo llama cada 10 s.
- El handler lee la sesión con `auth.FromContext(ctx)`. Solo un handler de ruta
  `Tenant` pasa su `Actor` a `db.InTenantTx`: ahí corren los entitlements, la
  versión, la 2FA, el RBAC y el límite del tenant. En una ruta `NoTenant` el
  tenant de la sesión (si lo tiene) es informativo y no se entra a datos de la
  empresa. Los módulos registran rutas solo por `auth.Routes`; el `httpx.Router`
  no sale de `cmd/wanka` (`mountPlane`).
- **Entitlements** (solo rutas `Tenant`, después de los limitadores): `For(ctx, tenant)`
  con la fase `entitlements` solo en el contexto de esa lectura (un error del handler
  lleva `phase=handler`), y las reglas de `entitlements.Entitlements.Check` en el
  orden de la secuencia 08: empresa cerrada y método distinto de `GET` y `HEAD` →
  `403 tenant_read_only`; si no está cerrada, suscripción `morosa`, `suspendida` o
  `cancelada` → `402 subscription_inactive`, salvo las rutas de la lista fija en el
  código (`GET /v1/exports`, `GET /v1/me/entitlements`; `/v1/auth/*`, `GET /v1/me` y
  `GET /v1/me/memberships` no pasan por aquí); módulo fuera de la lista →
  `403 module_not_enabled` con `module`; `usage >= limit` →
  `403 plan_limit_reached` con `limit` y `used`. Una empresa sin suscripción
  (`sin_plan`) no es un `402`: no tiene módulos, así que cae en
  `module_not_enabled`. Si no se pueden leer, `503 dependency_unavailable` (o
  `overloaded`) con `Retry-After: 1` y la petición no pasa. Un job de una empresa
  usa la misma comprobación con `Service.CheckJob`.
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

## Jobs (`internal/platform/jobs`)

Trabajo diferido sobre River (ADR 0006), con un solo backing service: PostgreSQL. Las
tablas están en el esquema `ops` (no son de tenant: sin `tenant_id` ni RLS). Los
módulos usan solo los tipos de este paquete (`Enqueuer`, `Args`, `Job`, `Registry`,
`Cancel`, `Snooze`); ninguno es de River, y un test revisa que la API exportada no
los nombre, para poder sustituirlo por el outbox propio del ADR 0006 sin tocarlos.

```go
// Encolar: en la misma transacción del cambio de datos. COMMIT los guarda a ambos; ROLLBACK, a ninguno.
err := pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
    // ... el cambio de negocio ...
    return enq.InsertTx(ctx, tx, SendReceipt{TenantID: actor.TenantID, SaleID: id},
        jobs.WithQueue(jobs.QueueNotifications), jobs.WithUnique())
})

// Ejecutar: el módulo registra su handler en el Registry del worker.
jobs.Register(reg, func(ctx context.Context, job jobs.Job[SendReceipt]) error {
    // 1. lee en una transacción corta; 2. llama a la red SIN conexión tomada; 3. guarda en otra.
    return job.InTenantTx(ctx, func(tx pgx.Tx) error { ... })
})
```

- **Argumentos**: `Kind() string` estable (está en cada fila) y, si son de tenant, `Tenant()`
  y el campo `json:"tenant_id"`. Sin secretos ni datos personales: la tabla de jobs se ve desde la consola.
- **Colas**: `WithQueue` es obligatoria (`cpe`, `webhooks`, `notifications`, `reports`, `maintenance`).
  `WithMaxAttempts` (por defecto 10), `WithScheduledAt` y `WithUnique` (uno solo en curso por clave:
  los argumentos serializados, todos los campos; un duplicado no falla ni aborta la transacción).
- **Resultados de un handler**: `nil` (completado, se limpia a los 7 días), un error cualquiera
  (transitorio: backoff exponencial de 10 s duplicando hasta 1 h, con jitter; al agotar los intentos
  queda `discarded` y se conserva 30 días), `jobs.Cancel(err)` (definitivo: sin reintentos; `cancelled`, 7 días)
  o `jobs.Snooze(d)` (vuelve a correr pasado `d` sin gastar un intento).
- **Entrega al menos una vez**: un job puede correr más de una vez, así que todo handler es idempotente
  (transiciones condicionadas por estado, claves únicas, `idem_key` hacia terceros).
- **Tenant**: `Job.InTenantTx` abre `InTenantTx(Actor{TenantID del job, Kind: system})` **sin `UserID`**
  (ADR 0001, «Workers»): `app.session_kind` es `system`, `app.current_user_id()` es `NULL` y solo se ven las filas
  del tenant del job. Un job sin `tenant_id` (o con argumentos que no son de tenant) recibe `db.ErrNoTenant` y se
  cancela al primer intento. Ningún job mantiene una conexión durante una llamada de red.
- **Periódicos**: `Registry.AddPeriodic(every, newArgs, opts...)`. Solo el líder de River los encola, así que corren
  una vez por ciclo aunque haya N réplicas.
- **Roles**: `wanka_app`, `wanka_webhooks` y `wanka_platform` solo hacen `INSERT` en `ops.river_job` (encolan con
  `COPY`, sin leer la fila de vuelta); `wanka_worker` tiene todo el DML. Los casos están en `docs/architecture/data/rls-check/`.
- **Conexiones**: River usa el pool del worker (`DB_POOL_MAX_CONNS`, 10) por su cuenta: una conexión para `LISTEN` y
  consultas cortas para tomar y terminar jobs, además de las `InTenantTx` de los handlers.
- **Comando `worker`**: sin handlers de módulo registra solo `kernel.noop` (River no arranca con un Workers vacío).
  El reindexado automático de River está desactivado: exige ser dueño del índice, y el dueño es `wanka_migrator`.

### Migraciones de River con goose

River trae su propio migrador (`rivermigrate`), pero aquí **solo migra goose** (solo hacia adelante,
embebido, `wanka migrate up`). `20261119143700_kernel_river_jobs.sql` es el esquema final de las migraciones
1 a 7 de la línea `main` de River v0.44.1 con el esquema `ops`, más las siete filas en `ops.river_migration`
para que las herramientas de River vean la base al día. Para subir de versión de River: leer su CHANGELOG, agregar
**otra** migración goose con los cambios (no se edita la existente) y registrar sus versiones; el test
`TestSchemaMatchesRiversOwnMigrations` falla mientras el esquema de `ops` no coincida con el que crea `rivermigrate`.
River v0.44.1 es la última versión compatible con Go 1.25 (las siguientes piden Go 1.26).

## Pruebas del catálogo RLS

`internal/platform/db/catalog_test.go` revisa `pg_catalog` tras `migrate up` (ADR
0001, «Migraciones y pruebas» y «Funciones `SECURITY DEFINER`»). Las reglas no
listan tablas: corren sobre todo lo que haya migrado, así que una tabla nueva de
cualquier módulo queda cubierta sin tocar el test.

| Regla | Falla si |
|---|---|
| `tenant-tables-have-force-rls-and-policy` | una tabla con `tenant_id` (o `org.tenants`) no tiene RLS, `FORCE` y `tenant_isolation` (`FOR ALL`) cuyo `USING` y `WITH CHECK` son exactamente `(tenant_id = app.current_tenant())` (`id` en `org.tenants`) |
| `partitions-of-tenant-tables` | una partición de una tabla de tenant tiene algún privilegio (de tabla o de columna) para un rol `wanka_*` de ejecución y no tiene su propia RLS, `FORCE` y `tenant_isolation`: una consulta directa a la partición no pasa por las políticas de la madre |
| `extra-policies-are-whitelisted` | hay una política que no es `tenant_isolation` ni está en `policyAllow` (tabla, comando, `TO`, rama `app.current_tenant() IS NULL` y, donde el ADR escribe el `USING`, la expresión exacta con el filtro de estado `invitado`/`activo`) |
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

- `noRLSTables`: tabla con `tenant_id` que por diseño no es de tenant (clave `esquema.tabla`). Hoy `iam.sessions` y las cuatro tablas `platform.*` de entitlements con `tenant_id` (`subscriptions`, `tenant_module_overrides`, `tenant_flag_overrides`, `tenant_usage`). Una tabla `platform.*` sin RLS por ADR entra aquí.
- `uniqueAllow`: `UNIQUE` global marcado (clave `esquema.tabla(col1,col2)`).
- `fkAllow`: FK entre tablas de tenant sin `tenant_id` (clave `esquema.tabla.restricción`).
- `policyAllow`: política adicional (clave `esquema.tabla:política`), con comandos, roles, si exige la rama `IS NULL`, fragmentos obligatorios (`mustContain`) y, si el ADR escribe el `USING`, el texto exacto (`qual`, tal como lo deparsa PostgreSQL 16; los espacios se colapsan). El número de ADR del motivo tiene que existir en `docs/architecture/adr/`.
- `tenantRoots`: tabla de tenant que se aísla por otra columna que `tenant_id` (hoy `org.tenants`, por `id`).

**PR que toca políticas, roles, grants o funciones definer.** Además de estas
pruebas, actualiza `docs/architecture/data/rls-check/` (el SQL y, si cambia lo
que se debe detectar, sus mutaciones) y corre `sh apps/api/scripts/rls-check.sh`
con salida 0. Lo exige el revisor del PR (`/pr-review`) y la sección «Verificación»
del ADR 0001; el workflow `api.yml` corre ese script cuando cambia
`apps/api/` o `rls-check/`.

## Servidor generado (`openapi/`, `internal/apigen`)

La fuente del contrato es `openapi/openapi.yaml` (ADR 0009). Es la raíz: `info`, seguridad, los tags y un `$ref` por camino, agrupados por módulo con un comentario `# --- <módulo> ---`. Cada módulo edita **solo** su `openapi/paths/<módulo>.yaml` (uno por código de `platform.modules`; los vacíos tienen `paths: {}`) y agrega la línea de su camino en su bloque de la raíz. Lo compartido está en `openapi/components/` (errores, paginación, dinero, cabeceras, comprobantes). El tag de una operación es el módulo de su archivo.

Para agregar un endpoint:

1. Escribe la operación en `paths/<módulo>.yaml` con `tags: [<módulo>]`, `operationId`, `x-module` y `x-permission`. Las dos extensiones son siempre un string: un código del catálogo o el literal `none` (ADR 0009, «Cómo se declara ninguno»). Ausente, `null` o vacío falla la prueba.
2. Agrega su `$ref` en `openapi.yaml`, en el bloque de tu módulo.
3. Si el módulo no tenía operaciones, crea `internal/apigen/<módulo>/` con `cfg.yaml` y `generate.go` (copia el de `ventas`) y regístralo en `opsByModule` de `openapi/contract_test.go`.
4. `go generate ./...` y commitea el resultado: la CI falla si hay diff. No edites `*.gen.go`.

Reglas del contrato (las comprueba `openapi/contract_test.go`; Redocly lint valida la sintaxis y los `$ref`):

- **Pertenencia:** ningún parámetro ni propiedad se llama `tenantId`: el tenant sale de la sesión (ADR 0001).
- **Dinero:** `type: string` con su patrón (`components/money.yaml`). Prohibido `type: number` (salvo `x-wanka-not-money: true` en la misma línea) y `x-go-type: decimal.Decimal`: su `MarshalJSON` no fija la escala. La conversión va en el handler, con `internal/platform/money`.
- **Tiempo:** un día civil es `type: string, format: date` (llega como `openapi_types.Date`, sin zona); un instante es `format: date-time` con offset. Un filtro por día de un documento fiscal (`issuedFrom`, `issuedTo`) es un día civil, nunca `date-time`. Es regla de revisión: Redocly no la expresa.
- **Nulos:** `oneOf` con `type: 'null'` y `type: [T, 'null']` generan un puntero opcional; la API no distingue «ausente» de `null`.

Cómo se usa el código generado:

- **Nunca `HandlerWithOptions`, `HandlerFromMux` ni `Handler`:** hacen `mux.HandleFunc` a secas y las rutas `/v1` solo se registran con `auth.Routes.Handle` (clase + `Spec`). Cada módulo registra el método del `ServerInterfaceWrapper` generado: `routes.HandleFunc("POST /v1/sales", auth.Tenant, auth.Spec{...}, w.CreateSale)`. Una prueba falla si otro código nombra esas funciones.
- **Errores:** todo `ServerInterfaceWrapper` y todo `NewStrictHandlerWithOptions` se construyen con `ErrorHandlerFunc`, `RequestErrorHandlerFunc` y `ResponseErrorHandlerFunc` que escriben con `httpx`: parámetro mal formado → `400 malformed_request`; cuerpo que no es JSON → `400 malformed_request`; `*http.MaxBytesError` → `413 payload_too_large`; error del handler → `httpx.WriteError`. Lo que trae el generado por defecto es `http.Error` de texto plano y no sirve. `internal/apigen/ventas/errorhandling_test.go` es el ejemplo.
- `internal/apigen` no lo importa `platform` (go-arch-lint).

## Entitlements (`internal/platform/entitlements`)

`entitlements.New(pool, Options{}).For(ctx, tenantID)` devuelve lo que una empresa
tiene habilitado (ADR 0005): `Modules` y `Flags` (ordenados), `Limits` y `Usage`
(`Limits`: `users`, `sites`, `cpe_per_month`; todo plan declara las tres y `Limit(key)` de una ausente es 0. `Usage` solo trae `users` y `cpe_per_month`: `sites` no se mide), `Status`, `ReadOnly` y `Version`. Es solo
lectura; el middleware que responde `402`, `403 tenant_read_only`,
`403 module_not_enabled` o `403 plan_limit_reached` está en `auth.Routes` (ver «Sesión»,
#175), y `GET /v1/me/entitlements` lo cablea identidad
(contrato en `apps/api/openapi/paths/identidad.yaml`).

- **Una consulta** agregada en `InTenantReadTx` (rol `wanka_app`, `Actor` de tipo
  `KindSystem`): `org.tenants`, la suscripción, `plan_modules`, overrides, flags y
  `tenant_usage`. Las tablas de `platform` no tienen RLS: cada una se filtra por
  `app.current_tenant()`, nunca por un parámetro.
- **Módulos** = los del plan, menos los overrides en `false`, más los overrides en
  `true`. **Flags**: un override de la empresa gana; si no, `default_on` lo enciende para
  todas; si no, entra la empresa cuyo `sha256(tenant_id, 0x00, flag)` (8 bytes, módulo
  100) es menor que `rollout_pct`, igual en cualquier réplica.
- **Falla cerrado**: sin suscripción `Status` es `sin_plan` (no el plan `semilla`); con
  `morosa`, `suspendida` o `cancelada` (`Status.Inactive()`), `Modules` y `Flags` vienen
  vacíos, salvo en la empresa cerrada, que conserva los de su último plan y tiene `ReadOnly`. Una empresa que no existe es
  `ErrTenantNotFound`; un error de la base sale como `db.ErrOverloaded`,
  `db.ErrUnavailable` o `db.ErrStatementTimeout` y nunca se guarda.
- **Caché** por empresa, TTL 30 s, con tope de 10 000 entradas; las lecturas en frío de
  la misma empresa comparten una sola consulta. `For` devuelve una copia.
  `Service.Run(ctx)` escucha `entitlements_changed` (`db.Pool.Listen`, en una conexión
  propia fuera del pool) e invalida la entrada del tenant del payload, o todo si va vacío o
  al reconectar. Triggers de la migración notifican los cambios de suscripción,
  overrides, catálogo, planes, flags y el cierre de una empresa; el uso no notifica. `serve`
  (plano tenant) lanza `Run` en una goroutine suya, igual que el barrido de los
  limitadores, y en el apagado cancela su contexto y espera a que salga.
- El plan `semilla` (todos los módulos, límites holgados, `non_production`) existe para
  desarrollo y pruebas; asignarlo a una empresa real lo decide el dueño (#31).
- Medir: `go test -run '^$' -bench 'HotCache|ColdRead' -benchmem ./internal/platform/entitlements`.

## Catálogo de permisos (`internal/platform/permission`)

Los permisos de tenant (`modulo.recurso:accion`) salen de un solo catálogo: la fuente de verdad es
`docs/architecture/api/resources.md`, se siembra en `iam.permissions` (sin RLS; SELECT para `wanka_app`,
`wanka_worker` y `wanka_platform`, nada de escritura para nadie en runtime) y el paquete `permission` es su copia
en Go: un `permission.Code` por permiso (`permission.VentasPosCreate`). Un módulo declara los suyos con
`Permissions() []permission.Code` y el arranque llama a `permission.Check(modulo, m)`: un código inventado no
compila (constante) o aborta el arranque (`Code("...")`). El catálogo de plataforma (`plataforma.*`) es otro y no está aquí.

Un permiso nuevo es un PR a `resources.md` más su migración hacia adelante (un `INSERT` en `iam.permissions`) más su
constante en `catalog.go` y en `all`, a cargo de identidad. `go test ./internal/platform/permission` falla si
`resources.md`, las migraciones y el código Go no tienen exactamente los mismos códigos, si una acción no está en
la lista `read | read_all | create | update | void | export | import | approve | send | retry`, si una migración siembra un código
que `resources.md` no lista o si la correspondencia de historias o el ADR 0004 nombran un permiso que no existe.

## Identidad: login (`internal/identidad`)

Primer módulo de la ola 1 (story #24, parte backend). Fachada `identidad` (`Mount`,
`RegisterJobs`) y capas internas `domain` (correo y topes), `app` (el caso de uso, el
semáforo del hash y el job del aviso), `adapters/argon2`, `adapters/postgres` y
`adapters/httpapi`. `mountPlane` lo monta con `identidad.Mount`; el worker registra
`identidad.RegisterJobs`.

`POST /v1/auth/login` (clase `Public`) sigue el orden del ADR 0004 (nota del 2026-11-19):
cuerpo y esquema (400/422), cuotas (5/min por IP+email y 30/min por IP, en memoria por
réplica; la IP es la de `X-Client-Ip` solo con `API_TRUST_CLIENT_IP=true`), semáforo del
hash (`GOMAXPROCS` cupos, 2 x núcleos en cola y 300 ms; sin cupo `503 overloaded` y no se
cobró nada), cobro del intento (un `UPDATE` con `RETURNING`, ventana fija de 15 min), hash
(ficticio, con el mismo semáforo, si no hay cuenta que cobrar) y resultado en una sola
transacción con `iam.users FOR UPDATE`. El hash nunca corre dentro de una transacción.
Contraseña mala, correo inexistente, cuenta bloqueada o cerrada, usuario bloqueado, sin
`password_hash` y segundo paso cerrado dan el mismo `401 invalid_credentials`.

- **Bloqueo** (ADR 0014): `iam.users.login_failure_streak` cuenta todo fallo seguido, con cualquier espaciado; cada 5
  hay un bloqueo de 15 min que se duplica hasta 24 h (`locked_until`, `login_lockouts = min(n, 10)`, `failed_logins = 0`) y se
  encola el job `identidad.login_lockout_notice` (cola `notifications`, único por usuario y fin del bloqueo; solo `user_id`
  y `locked_until` en los argumentos). Con 100 el ingreso con contraseña queda cerrado hasta restablecer o cambiar la
  contraseña (#26, #30) y se encola el aviso de cierre. La ventana de 15 min y `failed_logins` solo acotan los intentos en
  vuelo. Una contraseña correcta pone en 0 la racha, `failed_logins` y `login_lockouts`.
- **Respuestas 200**: `authenticated` (acceso 15 min, refresh 30 días, `mfa_method = 'none'`, sin
  empresa), `mfa_required` (con TOTP confirmado; sesión a medio login con `challengeToken`, 5 min) y,
  para quien debe enrolar el segundo factor, `503 dependency_unavailable` con `Retry-After: 60` hasta #25.
- **Aviso**: el worker lee el correo con `GRANT SELECT (id, email) ON iam.users TO wanka_worker`
  (migración `20261120130000`) y lo manda por el puerto `Mailer`; en R1 el adaptador es el stub y
  no sale ningún correo real.
- **argon2id**: `m = 65536 KiB (64 MiB), t = 2, p = 2` (`argon2.Default`; el ADR 0004 decía t = 3 y dejaba ajustarlo con la medición de #24). Cada hash guarda sus parámetros,
  así que cambiarlos no encierra a nadie. Se mide con la prueba de carga (no corre en la suite):
  `WANKA_LOAD=1 GOMAXPROCS=2 go test ./internal/identidad -run TestLoginLoad -v -count=1` (con
  `WANKA_LOAD_M`, `WANKA_LOAD_T` y `WANKA_LOAD_P` para otros parámetros).

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
| `JOBS_CONCURRENCY_<COLA>` | no | Jobs en ejecución a la vez por réplica del worker, 1 a 1000. Colas `CPE` (20), `WEBHOOKS` (10), `NOTIFICATIONS` (10), `REPORTS` (2), `MAINTENANCE` (2). Solo `worker` |
| `JOBS_JOB_TIMEOUT` | no (`1m`) | Duración de Go, de `1s` a `1h`: plazo de cada ejecución de un job; `jobs.WithJobTimeout` lo cambia por tipo. Solo `worker` |
| `JOBS_SHUTDOWN_TIMEOUT` | no (`30s`) | Duración de Go, de `1s` a `10m`: cuánto espera el worker a los jobs en curso tras `SIGTERM`; vencida, cancela su contexto y el job se reintenta. El plazo de drenaje de Railway debe ser mayor. Solo `worker` |
| `DB_POOL_MAX_CONNS` | no | Tamaño máximo del pool, 1 a 200. Por defecto: api 20, worker 10, webhooks 5, platform 5 (`capacity.md`) |
| `HTTP_ADDR` | no (`:8080`) | Dirección donde escucha `serve` (con `127.0.0.1:0` el sistema elige el puerto y se informa en el log `listening`) |
| `LOG_LEVEL` | no (`info`) | `debug`, `info`, `warn` o `error` |
| `MAILER` | no (`stub`) | Adaptador de correo: `stub` (no envía; guarda en memoria y registra destinatario enmascarado y asunto) o `none` (rechaza todo envío). Otro valor impide arrancar |
| `APP_ENV` | no (`production`) | `production`, `staging` o `development`. Sin la variable se asume producción. En `production` y `staging`, `MAILER=stub` o `none` arrancan con un `WARN` hasta que exista un adaptador real. Otro valor impide arrancar |
| `API_TRUST_CLIENT_IP` | no (`false`) | `true` o `false`. Con `true` la API toma la IP del cliente de la cabecera `X-Client-Ip` (cuotas por IP de las rutas `Public`, `iam.sessions.ip`, auditoría); solo donde el BFF es el único cliente que alcanza la API por la red privada (ADR 0007). Con `false` la cabecera se ignora y la IP es la del socket. Otro valor impide arrancar |

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
