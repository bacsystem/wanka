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

# has_no_tx <archivo>: devuelve 0 si el archivo lleva la anotación NO TRANSACTION
# de goose, 1 si no la lleva y 2 si alguna línea de anotación no se puede
# reconocer con seguridad ("unrecognized goose annotation"). Una línea candidata
# es la que contiene "--" y "+goose". goose la lee con TrimSpace y EqualFold de
# Unicode (internal/sqlparser/parser.go), que awk y grep no replican: NO
# TRANſACTION (U+017F), \f, \v, NBSP o EM SPACE valen para goose. Por eso toda
# línea candidata con un byte no ASCII o un control distinto de tab y CR se
# rechaza (2), y las demás se normalizan en ASCII (se quitan todos los "--", el
# primer "+goose" y los espacios, y se pasa a minúsculas) y deben quedar en una
# anotación conocida de goose. Es más estricto que goose, que es el lado seguro.
has_no_tx() {
  cand="$(LC_ALL=C grep -i -F -- '+goose' "$1" | LC_ALL=C grep -F -- '--' || true)"
  [ -n "$cand" ] || return 1
  if printf '%s\n' "$cand" | LC_ALL=C grep -q "$(printf '[^[:print:]\t\r]')"; then
    return 2
  fi
  printf '%s\n' "$cand" | LC_ALL=C awk '
    { line = tolower($0); gsub(/--/, "", line); sub(/\+goose/, "", line); gsub(/[ \t\r]/, "", line)
      if (line == "notransaction") nt = 1
      else if (line != "up" && line != "down" && line != "statementbegin" && line != "statementend" \
               && line != "envsubston" && line != "envsubstoff") bad = 1 }
    END { if (bad) exit 2; exit !nt }'
}

# Modo de prueba: "--detect <archivo>" sale con 0 si el check lo trata como NO
# TRANSACTION, 2 si no reconoce la anotación (también lo bloquea) y 1 si no lo
# bloquea (lo usa un test Go que lo contrasta con el comportamiento real de goose).
if [ "${1:-}" = "--detect" ]; then
  rc=0
  has_no_tx "$2" || rc=$?
  exit "$rc"
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
      rc=0
      has_no_tx "$f" || rc=$?
      if [ "$rc" = 2 ]; then
        echo "FAIL: $name tiene una línea de anotación goose que no se reconoce (unrecognized goose annotation)" >&2
        bad=1
      elif [ "$rc" = 0 ]; then
        # Sin comentarios, las sentencias separadas por ';' deben ser todas CREATE [UNIQUE] INDEX CONCURRENTLY
        # sin IF NOT EXISTS: si el build concurrente falla deja un índice INVALID, y con IF NOT EXISTS
        # la siguiente corrida lo saltaría y el índice inválido quedaría para siempre.
        # Los espacios y saltos de línea se colapsan antes de comparar (varias líneas, sangrías).
        if ! sed 's/--.*$//' "$f" | tr '\n' ' ' | awk '
            # Quita los comentarios /* ... */ (uno sin cerrar deja un marcador que no es una sentencia válida).
            { s = $0; out = ""
              while ((i = index(s, "/*")) > 0) {
                out = out substr(s, 1, i - 1) " "; s = substr(s, i + 2)
                j = index(s, "*/")
                if (j == 0) { s = "unterminated_block_comment"; break }
                s = substr(s, j + 2)
              }
              printf "%s", out s }' | awk '
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
for variant in 'NO TRAN\305\277ACTION' 'NO TRANSACTION\f' '+goose\v NO TRANSACTION' 'NO TRANSACTION\302\240' '+goose\342\200\203 NO TRANSACTION'; do
  case "$variant" in '+goose'*) line="-- $variant" ;; *) line="-- +goose $variant" ;; esac
  new_repo; printf -- "$line\n-- +goose Up\nCREATE INDEX CONCURRENTLY i ON t (a);\nALTER TABLE t ADD COLUMN c int;\n" > "$mdir/20260102000000_unicode.sql"
  expect fail "anotación con bytes que goose acepta y awk no ve ($variant)" "$tmp/r" main
done
new_repo; printf -- '-- +goose NO TRANSACTION\n-- +goose Up\nCREATE INDEX CONCURRENTLY i ON t (a); -- +goose raro\n' > "$mdir/20260102000000_unknown.sql"
expect fail "línea +goose con anotación desconocida" "$tmp/r" main
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
new_repo; printf -- '-- +goose NO TRANSACTION\n-- +goose Up\nCREATE INDEX CONCURRENTLY /* nota */ IF NOT EXISTS i1 ON t (a);\n' > "$mdir/20260102000000_cm_ifne.sql"
expect fail "IF NOT EXISTS separado por un comentario /* */" "$tmp/r" main
new_repo; printf -- '-- +goose NO TRANSACTION\n-- +goose Up\nCREATE INDEX /* a; b */ CONCURRENTLY i1 ON t (a); /* sin cerrar\n' > "$mdir/20260102000000_cm_open.sql"
expect fail "comentario /* sin cerrar" "$tmp/r" main
new_repo; printf -- '-- +goose NO TRANSACTION\n-- +goose Up\nCREATE INDEX /* nota */ CONCURRENTLY i1 ON t (a);\n' > "$mdir/20260102000000_cm_ok.sql"
expect pass "comentario /* */ dentro de un CREATE INDEX CONCURRENTLY válido" "$tmp/r" main
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
