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
         "session_lookup_no_restore|wanka_app" \
         "session_lookup_no_restore_deleted|wanka_app (membresia borrada)" \
         "member_read_tenants_no_status|wanka_app" \
         "member_read_with_suspended|wanka_app" \
         "own_memberships_no_status|wanka_app" \
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
         "grant_app_write_subscriptions|wanka_app" \
         "enable_rls_platform_subscriptions|wanka_app" \
         "grant_worker_write_subscriptions|wanka_worker" \
         "revoke_worker_usage_write|wanka_worker" \
         "grant_webhooks_platform|wanka_webhooks" \
         "grant_app_write_permissions|wanka_app" \
         "grant_worker_write_permissions|wanka_worker" \
         "revoke_platform_permissions|wanka_platform" \
         "grant_webhooks_permissions|wanka_webhooks" \
         "drop_force_rls|wanka_migrator: borra la membresia" \
         "grant_app_write_exchange_rates|wanka_app" \
         "grant_worker_read_exchange_rates|wanka_worker" \
         "drop_exchange_rates_currency_check|wanka_worker" \
         "grant_app_select_river_job|wanka_app" \
         "revoke_app_river_sequence|wanka_app" \
         "revoke_platform_river_insert|wanka_platform" \
         "grant_webhooks_update_river_job|wanka_webhooks" \
         "revoke_worker_river_delete|wanka_worker" \
         "grant_worker_river_migration|wanka_worker" \
         "grant_app_delete_idempotency_keys|wanka_app" \
         "grant_app_update_idempotency_hash|wanka_app" \
         "idempotency_open_policy|wanka_app" \
         "drop_idempotency_2xx_check|wanka_app" \
         "grant_worker_update_series|wanka_worker" \
         "revoke_worker_series_sync|wanka_worker" \
         "grant_app_update_series_sync|wanka_app" \
         "drop_series_conflict_guard|wanka_app" \
         "purge_policy_always|wanka_worker" \
         "purge_policy_any_tenant|wanka_worker" \
         "idempotency_pk_without_principal|wanka_app" \
         "drop_purge_policy|wanka_worker" \
         "grant_worker_select_idempotency_response|wanka_worker" \
         "revoke_worker_idempotency_delete|wanka_worker" \
         "grant_app_update_audit|wanka_app" \
         "grant_app_delete_audit|wanka_app" \
         "grant_worker_delete_audit|wanka_worker" \
         "grant_webhooks_insert_audit|wanka_webhooks" \
         "audit_policy_allows_all|wanka_app" \
         "drop_force_rls_audit|wanka_migrator: borra la membresia" \
         "drop_audit_user_check|wanka_app" \
         "drop_audit_system_check|wanka_app" \
         "drop_audit_staff_check|wanka_app" \
         "drop_audit_action_check|wanka_app" \
         "drop_audit_impersonator_check|wanka_app" \
         "ensure_partitions_to_public|wanka_app" \
         "ensure_partitions_no_search_path|wanka_worker" \
         "grant_app_update_all_user_tokens|wanka_app" \
         "grant_app_delete_user_tokens|wanka_app" \
         "revoke_worker_user_token_columns|wanka_worker" \
         "grant_worker_select_user_token_hash|wanka_worker" \
         "grant_worker_user_totp|wanka_worker" \
         "grant_app_delete_user_totp|wanka_app" \
         "grant_webhooks_user_tokens|wanka_webhooks" \
         "grant_platform_user_tokens|wanka_platform" \
         "drop_user_token_guard_trigger|wanka_app" \
         "user_token_guard_no_server_created_at|wanka_app" \
         "user_token_guard_allows_reopen|wanka_app" \
         "user_token_guard_moves_consumed_at|wanka_app" \
         "user_token_guard_trusts_consumed_at|wanka_app" \
         "drop_user_tokens_ttl_check|wanka_app" \
         "drop_totp_recovery_format_check|wanka_app" \
         "weaken_recovery_codes_function|wanka_app" \
         "drop_user_tokens_membership_check|wanka_app" \
         "drop_user_tokens_new_email_check|wanka_app" \
         "grant_app_partition_select|wanka_app" \
         "disable_rls_cash_registers|wanka_app" \
         "drop_membership_guard_trigger|wanka_app" \
         "drop_role_guard_trigger|wanka_app" \
         "drop_roles_lower_name_unique|wanka_app" \
         "drop_roles_system_check|wanka_app" \
         "drop_roles_template_unique|wanka_app" \
         "grant_app_delete_cash_registers|wanka_app" \
         "grant_app_delete_sites|wanka_app" \
         "grant_app_write_role_template_permissions|wanka_app" \
         "grant_app_write_role_templates|wanka_app" \
         "grant_platform_read_role_templates|wanka_platform" \
         "grant_platform_write_role_templates|wanka_platform" \
         "grant_webhooks_read_role_templates|wanka_webhooks" \
         "grant_worker_read_role_templates|wanka_worker" \
         "member_read_sites_no_scope|wanka_app" \
         "member_read_with_tenant|wanka_app" \
         "membership_sites_own_open|wanka_app" \
         "revoke_app_read_role_templates|wanka_app" \
         "revoke_definer_read_role_templates|wanka_app" \
         "role_guard_allows_delete_system|wanka_app" \
         "role_guard_allows_low_mfa_insert|wanka_app" \
         "role_guard_allows_mfa_downgrade|wanka_app" \
         "role_guard_allows_system_change|wanka_app" \
         "role_guard_allows_system_rename|wanka_app" \
         "role_permissions_open_policy|wanka_app" \
         "membership_guard_allows_reinvite|wanka_app" \
         "role_permissions_allows_system_role|wanka_app" \
         "role_guard_allows_system_id_change|wanka_app" \
         "view_security_definer|wanka_app" \
         "view_without_status_filter|wanka_app" \
         "view_without_user_filter|wanka_app"; do
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
