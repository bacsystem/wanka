#!/bin/sh
# Entorno local: corre el bootstrap de roles (idempotente) y fija la misma clave local
# a cada rol LOGIN. La clave llega por WANKA_ROLE_PASSWORD (de .env.local); no se guarda aquí.
set -eu
# La clave se prueba por red (por el socket local el servidor confia en cualquiera): un volumen creado con otra clave falla aqui.
if ! PGCONNECT_TIMEOUT=5 psql -h postgres -U postgres -d wanka -Atc 'select 1' >/dev/null 2>&1; then
  echo "ERROR: la clave de .env.local no coincide con la del volumen wanka-local-pgdata (se creo con otra clave)." >&2
  echo "Solucion: make reset (borra los datos locales) y luego make up." >&2
  exit 1
fi
psql -h postgres -U postgres -d wanka -v ON_ERROR_STOP=1 -f /bootstrap/roles.sql
psql -h postgres -U postgres -d wanka -v ON_ERROR_STOP=1 -v pw="$WANKA_ROLE_PASSWORD" <<'SQL'
ALTER ROLE wanka_migrator PASSWORD :'pw';
ALTER ROLE wanka_app PASSWORD :'pw';
ALTER ROLE wanka_worker PASSWORD :'pw';
ALTER ROLE wanka_webhooks PASSWORD :'pw';
ALTER ROLE wanka_platform PASSWORD :'pw';
SQL
echo "roles OK"
