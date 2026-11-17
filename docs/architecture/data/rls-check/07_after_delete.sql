\set ON_ERROR_STOP on
\echo '== A8. (wanka_app) membresia borrada: session_lookup sin version actual -> el middleware revoca =='
SELECT test.assert((SELECT session_authz_version = 1 AND current_authz_version IS NULL AND membership_status IS NULL AND membership_id IS NOT NULL
                      FROM iam.session_lookup(sha256('tok-cajero'))), 'membresia borrada: sin version actual (revocar)');
