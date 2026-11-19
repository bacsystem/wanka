#!/bin/sh
# Verificacion de RLS y de las funciones SECURITY DEFINER (ADR 0001, ADR 0004) en un contenedor
# DESECHABLE de PostgreSQL 16, con los roles reales:
#   - wanka_migrator (NOBYPASSRLS) aplica las migraciones reales de apps/api/migrations y crea el resto del esquema reducido;
#   - wanka_definer (NOLOGIN, NOBYPASSRLS) es duenio de todas las funciones SECURITY DEFINER;
#   - los demas son LOGIN: cada servicio se conecta con su rol (ADR 0001, «Creacion de los roles»);
#   - wanka_app, wanka_platform, wanka_webhooks y wanka_worker ejecutan cada caso.
# El superusuario solo corre el bootstrap de roles (01_schema.sql, dos veces: es idempotente). No usa datos reales.
# Cada caso compara contra su valor esperado (test.assert / test.expect_error); el primer fallo aborta
# psql (ON_ERROR_STOP) y el script termina con exit distinto de 0.
# Uso:  sh docs/architecture/data/rls-check/run.sh                                  -> exit 0
#       MUTATION=naive_session_lookup sh docs/architecture/data/rls-check/run.sh    -> exit != 0
set -u
D=$(cd "$(dirname "$0")" && pwd)
M=$(cd "$D/../../../../apps/api/migrations" && pwd)   # migraciones reales del kernel (issue #195)
N=wanka-rls-check
cleanup() { docker rm -f "$N" >/dev/null 2>&1; }
cleanup
docker run -d --name "$N" -e POSTGRES_PASSWORD=x -e POSTGRES_DB=wanka -e POSTGRES_HOST_AUTH_METHOD=trust \
  -v "$D":/sql:ro -v "$M":/migrations:ro postgres:16-alpine >/dev/null || exit 2
for i in $(seq 1 30); do docker exec "$N" pg_isready -U postgres -d wanka -h 127.0.0.1 >/dev/null 2>&1 && break; sleep 1; done
P="docker exec $N psql -h 127.0.0.1 -d wanka -X -q -v VERBOSITY=terse"
step() { label=$1; shift; echo "### $label"; "$@" || { echo "FAILED at: $label"; cleanup; exit 1; }; }
docker exec "$N" psql -h 127.0.0.1 -U postgres -d wanka -X -At -c 'select version()'
step "superusuario: bootstrap de roles"    $P -U postgres -f /sql/01_schema.sql
step "superusuario: bootstrap otra vez (idempotente)" $P -U postgres -f /sql/01_schema.sql
# Las migraciones reales del kernel, en orden y como wanka_migrator, cada una en UNA transaccion (-1) como en goose:
# lo que esta verificacion prueba es lo que se despliega, no una copia. 02_migrate.sql agrega solo lo que todavia no es del kernel.
for f in $(ls "$M"/*.sql | sort); do
  step "wanka_migrator: migracion real $(basename "$f")" $P -U wanka_migrator -1 -v ON_ERROR_STOP=1 -f "/migrations/$(basename "$f")"
done
step "wanka_migrator: migracion"           $P -U wanka_migrator -f /sql/02_migrate.sql
if [ -n "${MUTATION:-}" ]; then
  step "wanka_migrator: MUTACION $MUTATION" $P -U wanka_migrator -f "/sql/mutations/$MUTATION.sql"
fi
step "wanka_platform"                      $P -U wanka_platform -f /sql/03_platform.sql
step "wanka_migrator: datos de prueba"     $P -U wanka_migrator -f /sql/04_seed.sql
step "wanka_app"                           $P -U wanka_app -f /sql/05_app.sql
step "wanka_migrator: borra la membresia"  $P -U wanka_migrator -f /sql/06_delete_and_others.sql
step "wanka_app (membresia borrada)"       $P -U wanka_app -f /sql/07_after_delete.sql
step "wanka_webhooks"                      $P -U wanka_webhooks -f /sql/08_webhooks_worker.sql
step "wanka_worker"                        $P -U wanka_worker -f /sql/09_worker.sql
cleanup
echo "ALL CHECKS PASSED"
exit 0
