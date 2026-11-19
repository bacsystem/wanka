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
         "grant_app_update_all_mfa_enrollments|wanka_app" \
         "drop_mfa_guard_trigger|wanka_app" \
         "mfa_guard_no_server_created_at|wanka_app" \
         "mfa_guard_allows_reopen|wanka_app" \
         "revoke_worker_mfa_enrollment_columns|wanka_worker" \
         "drop_guard_trigger|wanka_app" \
         "drop_user_check|wanka_app" \
         "lookup_without_user_check|wanka_app" \
         "lookup_not_distinct|superusuario: sesiones impersonadas" \
         "guard_allows_unrevoke|wanka_app" \
         "guard_no_server_created_at|wanka_app" \
         "guard_rerevoke_raises|wanka_app" \
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

# run.sh aplica solo la seccion Up de cada migracion (como goose) y respeta NO TRANSACTION: una migracion extra cuyo
# Down borraria el esquema, y otra NO TRANSACTION con CREATE INDEX CONCURRENTLY, no deben romper la verificacion.
echo "== caso negativo: run.sh no ejecuta la seccion Down ni falla con NO TRANSACTION =="
extra="$(mktemp)"
printf -- '-- +goose Up\nSELECT 1;\n-- +goose Down\nDROP SCHEMA iam CASCADE;\n' > "$extra"
RLS_CHECK_EXTRA_MIGRATION="$extra" sh "$run" > "$log" 2>&1 || { cat "$log"; echo "FAIL: una migracion con Down destructivo rompio rls-check/run.sh" >&2; rm -f "$extra"; exit 1; }
printf -- '-- +goose NO TRANSACTION\n-- +goose Up\nCREATE INDEX CONCURRENTLY roles_name_idx ON iam.roles (name);\n-- +goose Down\nDROP SCHEMA iam CASCADE;\n' > "$extra"
RLS_CHECK_EXTRA_MIGRATION="$extra" sh "$run" > "$log" 2>&1 || { cat "$log"; echo "FAIL: una migracion NO TRANSACTION con Down destructivo rompio rls-check/run.sh" >&2; rm -f "$extra"; exit 1; }
rm -f "$extra"
echo "OK: la seccion Down no se ejecuta y NO TRANSACTION se respeta"

echo "== caso negativo: una mutación inexistente no cuenta como detectada =="
if detected naive_sesion_lookup "wanka_app"; then
  echo "FAIL: una mutación inexistente se dio por detectada" >&2
  exit 1
fi
grep '^FAILED at:' "$log"
echo "OK: una mutación inexistente se rechazó"
