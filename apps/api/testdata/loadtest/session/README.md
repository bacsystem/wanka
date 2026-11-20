# Carga con sesiones reales (#196)

Mide la cadena de sesión completa del plano tenant (`pgsession` + `auth.NewRoutes` con
los límites por defecto + `httpx.Chain`) contra PostgreSQL 16 real, sin tocar código de
producción. Complementa `../` (sobrecarga de la cadena sin sesión).

- `main.go seed`: crea la base `wanka_load`, aplica el bootstrap de roles y las
  migraciones y siembra `TENANTS` empresas (24) con `PER_TENANT` sesiones (15), una
  sesión por usuario y membresía, con la forma de `dbtest.SeedSession` (sesión ligada).
  Escribe los tokens, uno por línea, en `TOKENS_FILE` (fuera del repo).
- `main.go serve`: ruta `GET /v1/loadtest/ping` de clase `Tenant` que no hace otra
  consulta; `GET /ctl` es el control (misma cadena, sin sesión ni base). El log de
  acceso va a `ACCESS_LOG`; `STATS_ADDR` (`/stats?reset=1`) da el tiempo de cada
  `Lookup` (pool + consulta) y las conexiones en uso del pool muestreadas cada 20 ms.
- `load.js` (k6): llegadas constantes, turno rotatorio entre todos los tokens.
- `run.sh`: escalones; cada escalón de sesión va seguido de su control al mismo rate.
- `fail.sh`: carga constante y `docker stop`/`docker start` de la base.
- `analyze.py`, `timeline.py`: percentiles de `duration_ms` y línea de tiempo del log de acceso.

Entorno (los valores no se versionan): `LOAD_ADMIN_URL` (superusuario, base `postgres`),
`LOAD_DB_HOST` (`host:puerto`), `LOAD_ROLE_PASSWORD` (clave de `wanka_migrator` y
`wanka_app`), `TOKENS_FILE`, `OUT`, y para `fail.sh` `CONTAINER`.

```sh
docker run -d --name wanka-load-pg --tmpfs /var/lib/postgresql/data -e POSTGRES_PASSWORD=$SUPERUSER_PW -p 127.0.0.1:55433:5432 postgres:16-alpine
cd apps/api
go run ./testdata/loadtest/session seed
OUT=/tmp/run1 sh testdata/loadtest/session/run.sh              # RATES="50 100 200 300 500 700 1000" DUR=30s
OUT=/tmp/fail CONTAINER=wanka-load-pg sh testdata/loadtest/session/fail.sh   # RATE=300 PRE=20 DOWN=30 POST=40
```

`duration_ms` del log de acceso es un entero en milisegundos (truncado): un p95 de `1`
significa entre 1 y 2 ms. Los resultados vigentes están en el PR #201.
