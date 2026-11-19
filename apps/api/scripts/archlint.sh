#!/bin/sh
# Runs go-arch-lint on apps/api and proves that a forbidden cross-module import
# makes it fail. Run from anywhere; needs go on PATH.
set -eu

# Pinned: bump on purpose (see .github/workflows/api.yml, which uses the same version).
VERSION="v1.19.2"

root="$(cd "$(dirname "$0")/.." && pwd)"
bin="$(go env GOPATH)/bin/go-arch-lint"
if [ ! -x "$bin" ] || ! "$bin" version 2>/dev/null | grep -q "$VERSION"; then
  # The tool requires a newer Go than the project; auto lets Go fetch that toolchain
  # for the install only (CI sets GOTOOLCHAIN=local). The project still builds with go.mod's Go.
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

# Criterio de ADR 0001: ningún paquete fuera de internal/platform/db usa pgxpool.
# Se prueba sobre una copia del módulo real, con un archivo prohibido en cada
# lugar donde podría colarse el pool: otro paquete del kernel y un módulo.
echo "== apps/api: pgxpool outside internal/platform/db must fail =="
for place in internal/platform/httpx cmd/wanka; do
  copy="$(mktemp -d)"
  cp -R "$root/." "$copy/"
  printf 'package %s\n\nimport _ "github.com/jackc/pgx/v5/pgxpool"\n' "$(basename "$place")" > "$copy/$place/forbidden_pool.go"
  if "$bin" check --project-path "$copy" >/dev/null 2>&1; then
    rm -rf "$copy"
    echo "FAIL: go-arch-lint accepted pgxpool imported from $place" >&2
    exit 1
  fi
  rm -rf "$copy"
  echo "OK: pgxpool imported from $place was rejected"
done
