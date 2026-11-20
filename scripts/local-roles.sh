#!/bin/sh
# Entorno local: corre el bootstrap de roles (idempotente) y fija la misma clave local
# a cada rol LOGIN. La clave llega por WANKA_ROLE_PASSWORD (de .env.local); no se guarda aquí.
set -eu
psql -h postgres -U postgres -d wanka -v ON_ERROR_STOP=1 -f /bootstrap/roles.sql
psql -h postgres -U postgres -d wanka -v ON_ERROR_STOP=1 -v pw="$WANKA_ROLE_PASSWORD" <<'SQL'
ALTER ROLE wanka_migrator PASSWORD :'pw';
ALTER ROLE wanka_app PASSWORD :'pw';
ALTER ROLE wanka_worker PASSWORD :'pw';
ALTER ROLE wanka_webhooks PASSWORD :'pw';
ALTER ROLE wanka_platform PASSWORD :'pw';
SQL
echo "roles OK"
