-- Mutacion: quita el SELECT por columna de wanka_worker en iam.mfa_enrollments. Sin el, el WHERE del DELETE de limpieza da permission denied (#25).
-- Uso: MUTATION=revoke_worker_mfa_enrollment_columns sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
REVOKE SELECT (id, expires_at) ON iam.mfa_enrollments FROM wanka_worker;
