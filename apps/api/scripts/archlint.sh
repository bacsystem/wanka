#!/bin/sh
# Corre go-arch-lint sobre apps/api y demuestra que una importación prohibida
# entre módulos lo hace fallar. Se puede correr desde cualquier carpeta; necesita go en el PATH.
set -eu

# Versión fijada: se cambia a propósito (.github/workflows/api.yml usa este mismo script).
VERSION="v1.19.2"

root="$(cd "$(dirname "$0")/.." && pwd)"
bin="$(go env GOPATH)/bin/go-arch-lint"
if [ ! -x "$bin" ] || ! "$bin" version 2>/dev/null | grep -q "$VERSION"; then
  # La herramienta exige un Go más nuevo que el del proyecto; auto deja que Go baje esa
  # toolchain solo para instalarla (la CI fija GOTOOLCHAIN=local). El proyecto sigue
  # compilando con el Go de go.mod.
  GOTOOLCHAIN=auto go install "github.com/fe3dback/go-arch-lint@$VERSION"
fi

echo "== apps/api =="
"$bin" check --project-path "$root"

echo "== fixture: allowed imports must pass =="
"$bin" check --project-path "$root/testdata/archlint"

echo "== fixture: forbidden import must fail =="
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cp -R "$root/testdata/archlint/." "$tmp/"
printf 'package app\n\nimport _ "example.com/archlint/internal/alpha/domain"\n' > "$tmp/internal/beta/app/forbidden.go"
if "$bin" check --project-path "$tmp"; then
  echo "FAIL: go-arch-lint accepted beta importing alpha/domain" >&2
  exit 1
fi
echo "OK: forbidden import was rejected"

# Criterio de ADR 0001: ningún paquete fuera de internal/platform/db abre conexiones
# por su cuenta. Se prueba sobre una copia del módulo real, con un archivo prohibido
# en cada lugar donde podría colarse: otro paquete del kernel y un módulo.
# Lo que go-arch-lint no ve: pgx.Connect y pgconn.Connect usan paquetes que los
# módulos sí necesitan importar (pgx.Tx, pgconn.PgError), así que no se prohíben por
# importación. Esas llamadas las detecta un test de AST
# (internal/platform/db/rawconn_test.go); tx.Conn() guardado fuera de fn no se
# detecta y queda para la revisión de código (ver el doc del paquete db).
forbidden() { # forbidden <paquete> <importación>
  copy="$(mktemp -d)"
  cp -R "$root/." "$copy/"
  printf 'package %s\n\nimport _ "%s"\n' "$(basename "$1")" "$2" > "$copy/$1/forbidden_import.go"
  if "$bin" check --project-path "$copy" >/dev/null 2>&1; then
    rm -rf "$copy"
    echo "FAIL: go-arch-lint accepted $2 imported from $1" >&2
    exit 1
  fi
  rm -rf "$copy"
  echo "OK: $2 imported from $1 was rejected"
}
echo "== apps/api: pgxpool and pgx/stdlib outside internal/platform/db must fail =="
forbidden internal/platform/httpx github.com/jackc/pgx/v5/pgxpool
forbidden cmd/wanka github.com/jackc/pgx/v5/pgxpool
forbidden internal/platform/logging github.com/jackc/pgx/v5/stdlib
forbidden cmd/wanka github.com/jackc/pgx/v5/stdlib

# ADR 0006: River solo vive en internal/platform/jobs (y su driver en db). Los módulos
# encolan con jobs.Enqueuer, así que el outbox propio puede sustituirlo sin tocarlos.
echo "== apps/api: River outside internal/platform/jobs must fail =="
forbidden internal/platform/httpx github.com/riverqueue/river
forbidden cmd/wanka github.com/riverqueue/river
forbidden internal/platform/logging github.com/riverqueue/river/rivertype
forbidden internal/platform/httpx github.com/riverqueue/river/riverdriver/riverpgxv5
