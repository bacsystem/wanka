#!/bin/sh
# Check de CI de RLS (ADR 0001): corre docs/architecture/data/rls-check/run.sh y
# comprueba que
#   1. sin mutación termina con exit 0;
#   2. cada mutación de rls-check/mutations que se lista abajo falla exactamente en el paso que
#      debe romperla (exit 1 y la línea "FAILED at: <paso>");
#   3. una mutación que no existe NO cuenta como detectada: run.sh también sale con
#      exit 1 (falla en el paso de la mutación), y un check que aceptara cualquier
#      fallo daría por buena una mutación que ya no prueba nada.
# Uso: sh apps/api/scripts/rls-check.sh   (necesita Docker; se puede correr desde cualquier carpeta)
set -u

root="$(cd "$(dirname "$0")/../../.." && pwd)"
run="$root/docs/architecture/data/rls-check/run.sh"
log="$(mktemp)"
trap 'rm -f "$log"' EXIT

# detected <mutación> <paso>: éxito solo si run.sh sale con 1 y falla en ese paso.
detected() {
  rc=0
  MUTATION="$1" sh "$run" > "$log" 2>&1 || rc=$?
  [ "$rc" -eq 1 ] && grep -qxF "FAILED at: $2" "$log"
}

echo "== rls-check sin mutación: debe pasar =="
sh "$run" || { echo "FAIL: rls-check/run.sh falló sin mutación" >&2; exit 1; }

# Cada mutación rompe una línea clave y debe hacer fallar el paso que la prueba.
for m in "naive_session_lookup|wanka_app" \
         "session_lookup_no_restore|wanka_app (membresia borrada)" \
         "drop_authz_check|wanka_app" \
         "revoke_app_sessions_insert|wanka_app" \
         "grant_app_update_tenants|wanka_app" \
         "drop_impersonated_check|superusuario: sesiones impersonadas" \
         "revoke_worker_session_columns|wanka_worker" \
         "grant_app_update_all_sessions|wanka_app" \
         "drop_guard_trigger|wanka_app" \
         "drop_user_check|wanka_app" \
         "drop_force_rls|wanka_migrator: borra la membresia"; do
  name="${m%%|*}" step="${m#*|}"
  echo "== MUTATION=${name}: debe fallar en «${step}» =="
  if ! detected "$name" "$step"; then
    cat "$log"
    echo "FAIL: la mutación ${name} no hizo fallar el paso «${step}»" >&2
    exit 1
  fi
  echo "OK: la mutación ${name} se detectó en «${step}»"
done

echo "== caso negativo: una mutación inexistente no cuenta como detectada =="
if detected naive_sesion_lookup "wanka_app"; then
  echo "FAIL: una mutación inexistente se dio por detectada" >&2
  exit 1
fi
grep '^FAILED at:' "$log"
echo "OK: una mutación inexistente se rechazó"
