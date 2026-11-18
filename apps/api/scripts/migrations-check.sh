#!/bin/sh
# Check de CI de migraciones solo hacia adelante (data-model §6). Uso:
#   sh scripts/migrations-check.sh [base-ref]     (por defecto origin/main)
# Falla si, respecto del ancestro común con la base:
#   - un archivo de migración que ya estaba se modificó, se borró o se renombró;
#   - un archivo no se llama <yyyymmddhhmmss>_<nombre>.sql;
#   - un archivo con "-- +goose NO TRANSACTION" contiene algo más que
#     CREATE [UNIQUE] INDEX CONCURRENTLY (sin IF NOT EXISTS).
# Antes de revisar el repo real, demuestra con repos temporales que el check
# rechaza cada caso malo y acepta los buenos (como scripts/archlint.sh).
set -eu

root="$(cd "$(dirname "$0")/../../.." && pwd)"
mdir="apps/api/migrations"

# has_no_tx <archivo>: devuelve 0 si alguna línea es la anotación NO TRANSACTION
# tal como la lee goose (internal/sqlparser/parser.go): una línea que, sin
# espacios iniciales, empieza con "--" y contiene "+goose"; se quitan todos los
# "--" y el primer "+goose", y lo que queda se compara sin distinguir
# mayúsculas con "NO TRANSACTION". Este check es más estricto que goose: quita
# todos los espacios, así que cualquier variante que goose acepte la detecta.
has_no_tx() {
  awk '
    { line = tolower($0); sub(/^[ \t\r]+/, "", line)
      if (substr(line, 1, 2) == "--" && index(line, "+goose") > 0) {
        gsub(/--/, "", line); sub(/\+goose/, "", line); gsub(/[ \t\r]/, "", line)
        if (line == "notransaction") found = 1
      } }
    END { exit !found }' "$1"
}

# Modo de prueba: "--detect <archivo>" dice si el check lo trata como NO TRANSACTION
# (lo usa un test Go que lo contrasta con el comportamiento real de goose).
if [ "${1:-}" = "--detect" ]; then
  has_no_tx "$2"
  exit $?
fi

# lint <repo> <base-ref>: imprime los problemas y devuelve 1 si hay alguno.
lint() {
  repo=$1 base=$2
  (
    cd "$repo"
    mb="$(git merge-base "$base" HEAD)"
    bad=0
    # Archivos que ya estaban en la base: no se tocan (--no-renames: un renombre cuenta como borrado).
    git diff --name-status --no-renames "$mb" -- "$mdir/*.sql" | while IFS="$(printf '\t')" read -r status path; do
      if [ "$status" != "A" ]; then
        echo "FAIL: $path ya estaba en la base y fue modificado, borrado o renombrado (solo hacia adelante)" >&2
        echo x
      fi
    done | grep -q x && bad=1
    for f in "$mdir"/*.sql; do
      [ -e "$f" ] || continue
      name="$(basename "$f")"
      if ! printf '%s\n' "$name" | grep -Eq '^[0-9]{14}_[a-z0-9_]+\.sql$'; then
        echo "FAIL: $name debe llamarse <yyyymmddhhmmss>_<nombre>.sql" >&2
        bad=1
      fi
      if has_no_tx "$f"; then
        # Sin comentarios, las sentencias separadas por ';' deben ser todas CREATE [UNIQUE] INDEX CONCURRENTLY
        # sin IF NOT EXISTS: si el build concurrente falla deja un índice INVALID, y con IF NOT EXISTS
        # la siguiente corrida lo saltaría y el índice inválido quedaría para siempre.
        # Los espacios y saltos de línea se colapsan antes de comparar (varias líneas, sangrías).
        if ! sed 's/--.*$//' "$f" | tr '\n' ' ' | awk '
            BEGIN { RS = ";"; n = 0; bad = 0 }
            { s = tolower($0); gsub(/[ \t\r]+/, " ", s); gsub(/^ | $/, "", s); if (s == "") next; n++
              if (s !~ /^create (unique )?index concurrently / || s ~ /^create (unique )?index concurrently if not exists /) bad = 1 }
            END { exit (n == 0 || bad) }'; then
          echo "FAIL: $name usa NO TRANSACTION pero contiene algo más que CREATE INDEX CONCURRENTLY" >&2
          bad=1
        fi
      fi
    done
    exit "$bad"
  )
}

# expect <pass|fail> <descripción> <repo> <base>
expect() {
  want=$1 what=$2 repo=$3 base=$4
  if lint "$repo" "$base" 2>/dev/null; then got=pass; else got=fail; fi
  if [ "$got" != "$want" ]; then
    echo "FAIL: caso '$what': se esperaba $want y fue $got" >&2
    exit 1
  fi
  echo "OK: $what -> $got"
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

new_repo() { # crea un repo con una migración en la base y vuelve a ella
  rm -rf "$tmp/r" && mkdir -p "$tmp/r/$mdir" && cd "$tmp/r"
  git init -q -b main . && git config user.email t@example.com && git config user.name t
  printf -- '-- +goose Up\nSELECT 1;\n' > "$mdir/20260101000000_first.sql"
  git add -A && git commit -q -m base
  git checkout -q -b change
}

echo "== casos de prueba del check =="
new_repo; printf -- '-- +goose Up\nSELECT 2;\n' > "$mdir/20260102000000_second.sql"
expect pass "migración nueva" "$tmp/r" main
new_repo; printf -- '-- +goose Up\nSELECT 99;\n' > "$mdir/20260101000000_first.sql"
expect fail "editar una migración ya presente" "$tmp/r" main
new_repo; rm "$mdir/20260101000000_first.sql"
expect fail "borrar una migración ya presente" "$tmp/r" main
new_repo; git mv "$mdir/20260101000000_first.sql" "$mdir/20260101000000_renamed.sql"
expect fail "renombrar una migración ya presente" "$tmp/r" main
new_repo; printf -- '-- +goose Up\nSELECT 2;\n' > "$mdir/second.sql"
expect fail "nombre sin timestamp" "$tmp/r" main
new_repo; printf -- '-- +goose NO TRANSACTION\n-- +goose Up\n-- índice grande\nCREATE INDEX CONCURRENTLY i1 ON t (a);\nCREATE UNIQUE INDEX CONCURRENTLY i2 ON t (b);\n' > "$mdir/20260102000000_idx.sql"
expect pass "NO TRANSACTION solo con CREATE INDEX CONCURRENTLY" "$tmp/r" main
new_repo; printf -- '-- +goose NO TRANSACTION\n-- +goose Up\nCREATE INDEX CONCURRENTLY i1 ON t (a);\nALTER TABLE t ADD COLUMN c int;\n' > "$mdir/20260102000000_mixed.sql"
expect fail "NO TRANSACTION con otra sentencia" "$tmp/r" main
new_repo; printf -- '-- +goose no transaction\n-- +goose Up\nCREATE INDEX CONCURRENTLY i ON t (a);\nALTER TABLE t ADD COLUMN c int;\n' > "$mdir/20260102000000_lower.sql"
expect fail "NO TRANSACTION en minúsculas (goose no distingue mayúsculas)" "$tmp/r" main
new_repo; printf -- '--+goose No Transaction\n-- +goose Up\nCREATE INDEX CONCURRENTLY i ON t (a);\nALTER TABLE t ADD COLUMN c int;\n' > "$mdir/20260102000000_mixed_case.sql"
expect fail "NO TRANSACTION con mayúsculas mezcladas y sin espacio" "$tmp/r" main
new_repo; printf -- '-- +gooseNO TRANSACTION\n-- +goose Up\nCREATE INDEX CONCURRENTLY i ON t (a);\nALTER TABLE t ADD COLUMN c int;\n' > "$mdir/20260102000000_glued.sql"
expect fail "NO TRANSACTION con +goose pegado a la palabra" "$tmp/r" main
new_repo; printf -- '-- NO TRANSACTION +goose\n-- +goose Up\nCREATE INDEX CONCURRENTLY i ON t (a);\nALTER TABLE t ADD COLUMN c int;\n' > "$mdir/20260102000000_swapped.sql"
expect fail "NO TRANSACTION antes de +goose" "$tmp/r" main
new_repo; printf -- '-- +goose NO TRANSACTION\n-- +goose Up\nCREATE INDEX CONCURRENTLY\n    IF NOT EXISTS i1 ON t (a);\n' > "$mdir/20260102000000_ml_ifne.sql"
expect fail "IF NOT EXISTS en una segunda línea sangrada" "$tmp/r" main
new_repo; printf -- '-- +goose NO TRANSACTION\n-- +goose Up\nCREATE INDEX CONCURRENTLY  IF  NOT EXISTS i1 ON t (a);\n' > "$mdir/20260102000000_sp_ifne.sql"
expect fail "IF NOT EXISTS con espacios dobles" "$tmp/r" main
new_repo; printf -- '-- +goose NO TRANSACTION\n-- +goose Up\nCREATE INDEX\n    CONCURRENTLY i1\n    ON t (a);\nCREATE UNIQUE   INDEX\tCONCURRENTLY i2 ON t (b);\n' > "$mdir/20260102000000_ml_ok.sql"
expect pass "CREATE INDEX CONCURRENTLY repartido en varias líneas" "$tmp/r" main
new_repo; printf -- '-- +goose NO TRANSACTION\n-- +goose Up\nCREATE INDEX CONCURRENTLY\n    i1 ON t (a);\nALTER\n TABLE t ADD COLUMN c int;\n' > "$mdir/20260102000000_ml_mixed.sql"
expect fail "varias líneas con una sentencia que no es de índice" "$tmp/r" main
new_repo; printf -- '-- +goose NO TRANSACTION\n-- +goose Up\nCREATE INDEX CONCURRENTLY IF NOT EXISTS i1 ON t (a);\n' > "$mdir/20260102000000_ifne.sql"
expect fail "CONCURRENTLY IF NOT EXISTS (un índice INVALID quedaría para siempre)" "$tmp/r" main
new_repo; printf -- '-- +goose NO TRANSACTION\n-- +goose Up\nSELECT 1;\n' > "$mdir/20260102000000_other.sql"
expect fail "NO TRANSACTION sin índice" "$tmp/r" main
new_repo; printf -- '-- +goose NO TRANSACTION\n-- +goose Up\n' > "$mdir/20260102000000_empty.sql"
expect fail "NO TRANSACTION vacío" "$tmp/r" main

echo "== repo real =="
cd "$root"
base="${1:-origin/main}"
git rev-parse --verify -q "$base" >/dev/null || base=main
lint "$root" "$base"
echo "OK: migraciones solo hacia adelante respecto de $base"
