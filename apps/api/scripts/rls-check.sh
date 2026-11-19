#!/bin/sh
# Check de CI de RLS (ADR 0001): corre docs/architecture/data/rls-check/run.sh y
# comprueba que
#   1. sin mutación termina con exit 0;
#   2. con MUTATION=naive_session_lookup falla exactamente en el paso "wanka_app",
#      donde la mutación debe romper un caso (exit 1 y la línea "FAILED at: wanka_app");
#   3. una mutación que no existe NO cuenta como detectada: run.sh también sale con
#      exit 1 (falla en el paso de la mutación), y un check que aceptara cualquier
#      fallo daría por buena una mutación que ya no prueba nada.
# Uso: sh apps/api/scripts/rls-check.sh   (necesita Docker; se puede correr desde cualquier carpeta)
set -u

root="$(cd "$(dirname "$0")/../../.." && pwd)"
run="$root/docs/architecture/data/rls-check/run.sh"
log="$(mktemp)"
trap 'rm -f "$log"' EXIT

# detected <mutación>: éxito solo si run.sh sale con 1 y falla en el paso wanka_app.
detected() {
  rc=0
  MUTATION="$1" sh "$run" > "$log" 2>&1 || rc=$?
  [ "$rc" -eq 1 ] && grep -qx 'FAILED at: wanka_app' "$log"
}

echo "== rls-check sin mutación: debe pasar =="
sh "$run" || { echo "FAIL: rls-check/run.sh falló sin mutación" >&2; exit 1; }

echo "== MUTATION=naive_session_lookup: debe fallar en wanka_app =="
if ! detected naive_session_lookup; then
  cat "$log"
  echo "FAIL: la mutación naive_session_lookup no hizo fallar el paso wanka_app" >&2
  exit 1
fi
echo "OK: la mutación se detectó en wanka_app"

echo "== caso negativo: una mutación inexistente no cuenta como detectada =="
if detected naive_sesion_lookup; then
  echo "FAIL: una mutación inexistente se dio por detectada" >&2
  exit 1
fi
grep '^FAILED at:' "$log"
echo "OK: una mutación inexistente se rechazó"
