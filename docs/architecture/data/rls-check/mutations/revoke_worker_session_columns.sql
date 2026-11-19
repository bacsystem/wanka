-- Mutacion: quita el SELECT por columna de wanka_worker en iam.sessions. Sin el, el WHERE del DELETE de limpieza da permission denied.
-- Uso: MUTATION=revoke_worker_session_columns sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
REVOKE SELECT (id, created_at, revoked_at, access_expires_at, refresh_expires_at) ON iam.sessions FROM wanka_worker;
