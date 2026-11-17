\set ON_ERROR_STOP on
\echo '== 7. (wanka_app) membresia borrada: session_lookup sin version actual -> revocar =='
SELECT session_authz_version, current_authz_version, membership_status,
       CASE WHEN membership_id IS NOT NULL AND (current_authz_version IS NULL OR membership_status <> 'activo') THEN 'revocar'
            WHEN current_authz_version <> session_authz_version THEN 're-resolver rol'
            ELSE 'sin cambios' END AS decision_middleware
  FROM iam.session_lookup(sha256('tok-cajero'));
