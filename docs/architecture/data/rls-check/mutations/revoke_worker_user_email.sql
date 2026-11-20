-- Mutacion: quita el SELECT por columna de wanka_worker en iam.users. Sin el, el aviso del bloqueo del login no puede leer el correo (#24).
-- Uso: MUTATION=revoke_worker_user_email sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
REVOKE SELECT (id, email) ON iam.users FROM wanka_worker;
