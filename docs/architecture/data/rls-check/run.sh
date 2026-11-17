#!/bin/sh
# Ejecuta la verificacion de RLS y de las funciones SECURITY DEFINER (ADR 0001, ADR 0004)
# en un contenedor DESECHABLE de PostgreSQL 16, con los roles reales:
#   - wanka_migrator (NOBYPASSRLS) crea esquema, politicas y funciones;
#   - wanka_definer (NOLOGIN, NOBYPASSRLS) es dueno de todas las funciones SECURITY DEFINER;
#   - wanka_app, wanka_platform, wanka_webhooks y wanka_worker ejecutan cada caso.
# El superusuario solo crea los roles (01_schema.sql). No usa datos reales.
# Uso: sh docs/architecture/data/rls-check/run.sh
set -u
D=$(cd "$(dirname "$0")" && pwd)
N=wanka-rls-check
docker rm -f "$N" >/dev/null 2>&1
docker run -d --name "$N" -e POSTGRES_PASSWORD=x -e POSTGRES_DB=wanka -e POSTGRES_HOST_AUTH_METHOD=trust \
  -v "$D":/sql:ro postgres:16-alpine >/dev/null
for i in $(seq 1 30); do docker exec "$N" pg_isready -U postgres -d wanka -h 127.0.0.1 >/dev/null 2>&1 && break; sleep 1; done
P="docker exec $N psql -h 127.0.0.1 -d wanka -X -q -v VERBOSITY=terse"
docker exec "$N" psql -h 127.0.0.1 -U postgres -d wanka -X -At -c 'select version()'
$P -U postgres -f /sql/01_schema.sql
$P -U wanka_migrator -f /sql/02_migrate.sql
echo "### wanka_platform"; $P -U wanka_platform -f /sql/03_platform.sql
echo "### wanka_migrator (datos de prueba)"; $P -U wanka_migrator -f /sql/04_seed.sql
echo "### wanka_app"; $P -U wanka_app -f /sql/05_app.sql
echo "### wanka_migrator (borra la membresia)"; $P -U wanka_migrator -f /sql/06_delete_and_others.sql
echo "### wanka_app"; $P -U wanka_app -f /sql/07_after_delete.sql
echo "### wanka_webhooks"; $P -U wanka_webhooks -f /sql/08_webhooks_worker.sql
echo "### wanka_worker"; $P -U wanka_worker -f /sql/09_worker.sql
docker rm -f "$N" >/dev/null
