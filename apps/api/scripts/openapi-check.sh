#!/bin/sh
# Comprueba el contrato OpenAPI (ADR 0009, #166). Uso: sh scripts/openapi-check.sh [<ref-base>]
#   1. Redocly lint de openapi/openapi.yaml (un $ref roto falla).
#   2. Empaqueta el spec actual y el de <ref-base> y corre `oasdiff breaking`: un cambio
#      incompatible solo pasa si info.version sube de minor en 0.x (de major desde 1.0).
#      Si <ref-base> aún no tiene apps/api/openapi/, compara con docs/architecture/api/openapi.yaml
#      (el esqueleto que esta historia partió). Sin <ref-base>, solo el lint.
#   3. Prueba sus casos malos: un camino quitado debe fallar, y pasar con la versión subida.
# Se puede correr desde cualquier carpeta. Necesita node (npx), git y, en Linux x86-64, curl.
set -eu

# Versiones fijadas (ADR 0002): se cambian a propósito. El checksum es el de checksums.txt del release.
REDOCLY="@redocly/cli@2.82.0"
OASDIFF_VERSION="1.39.0"
OASDIFF_LINUX_AMD64_SHA256="71a85930728f367a2086077b7ae1411319421549d18473a3e8258b6451520433"

root="$(cd "$(dirname "$0")/.." && pwd)"
repo="$(git -C "$root" rev-parse --show-toplevel)"
base_ref="${1:-}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"; git -C "$repo" worktree prune' EXIT

redocly() { npx --yes "$REDOCLY" "$@"; }

oasdiff_bin() {
  if [ -n "${OASDIFF:-}" ]; then echo "$OASDIFF"; return; fi
  if command -v oasdiff >/dev/null 2>&1 && oasdiff --version | grep -q "$OASDIFF_VERSION"; then
    command -v oasdiff; return
  fi
  if [ "$(uname -s)-$(uname -m)" != "Linux-x86_64" ]; then
    echo "oasdiff $OASDIFF_VERSION is not installed: put it in PATH or set OASDIFF" >&2
    return 1
  fi
  curl -fsSL -o "$tmp/oasdiff.tgz" \
    "https://github.com/oasdiff/oasdiff/releases/download/v$OASDIFF_VERSION/oasdiff_${OASDIFF_VERSION}_linux_amd64.tar.gz"
  echo "$OASDIFF_LINUX_AMD64_SHA256  $tmp/oasdiff.tgz" | sha256sum -c - >&2
  tar -xzf "$tmp/oasdiff.tgz" -C "$tmp" oasdiff
  echo "$tmp/oasdiff"
}

# version <archivo>: info.version del spec empaquetado.
version() { awk '/^info:/ {i=1; next} i && /^  version:/ {print $2; exit}' "$1" | tr -d "'\""; }

# bumped <base> <head>: 0 si head sube de minor en 0.x, o de major desde 1.0.
bumped() {
  bmaj="${1%%.*}"; hmaj="${2%%.*}"
  brest="${1#*.}"; hrest="${2#*.}"; bmin="${brest%%.*}"; hmin="${hrest%%.*}"
  if [ "$hmaj" -gt "$bmaj" ]; then return 0; fi
  if [ "$bmaj" -eq 0 ] && [ "$hmaj" -eq 0 ] && [ "$hmin" -gt "$bmin" ]; then return 0; fi
  return 1
}

# breaking <base.yaml> <head.yaml>: falla ante un cambio ERR sin versión nueva.
breaking() {
  if "$OASDIFF_BIN" breaking "$1" "$2" --fail-on ERR >"$tmp/oasdiff.out" 2>&1; then return 0; fi
  cat "$tmp/oasdiff.out"
  if bumped "$(version "$1")" "$(version "$2")"; then
    echo "breaking change accepted: info.version went from $(version "$1") to $(version "$2")"
    return 0
  fi
  echo "breaking change without a new version (info.version $(version "$1") -> $(version "$2"))" >&2
  return 1
}

echo "== redocly lint =="
redocly lint --config "$root/openapi/redocly.yaml" "$root/openapi/openapi.yaml"

[ -n "$base_ref" ] || exit 0

OASDIFF_BIN="$(oasdiff_bin)"
echo "== bundle (head) =="
redocly bundle --config "$root/openapi/redocly.yaml" "$root/openapi/openapi.yaml" -o "$tmp/head.yaml"

echo "== bundle (base $base_ref) =="
if git -C "$repo" cat-file -e "$base_ref:apps/api/openapi/openapi.yaml" 2>/dev/null; then
  git -C "$repo" worktree add --detach "$tmp/base" "$base_ref" >/dev/null 2>&1
  redocly bundle --config "$tmp/base/apps/api/openapi/redocly.yaml" "$tmp/base/apps/api/openapi/openapi.yaml" -o "$tmp/base.yaml"
else
  # La historia #166 partió el esqueleto: la base es el archivo único de docs/.
  git -C "$repo" show "$base_ref:docs/architecture/api/openapi.yaml" > "$tmp/base.yaml"
fi

echo "== oasdiff breaking =="
breaking "$tmp/base.yaml" "$tmp/head.yaml"

echo "== casos malos: un camino quitado debe fallar =="
sed 's|^paths:|paths:\n  /v1/zz-selftest:\n    get:\n      operationId: zzSelftest\n      responses:\n        "200":\n          description: ok|' "$tmp/head.yaml" > "$tmp/selftest-base.yaml"
if breaking "$tmp/selftest-base.yaml" "$tmp/head.yaml" >/dev/null 2>&1; then
  echo "FAIL: oasdiff accepted a removed path without a new version" >&2
  exit 1
fi
echo "OK: the removed path was rejected"
sed 's|^  version: .*|  version: 0.99.0|' "$tmp/head.yaml" > "$tmp/selftest-head.yaml"
breaking "$tmp/selftest-base.yaml" "$tmp/selftest-head.yaml" >/dev/null
echo "OK: the same change passes with a new minor version"
