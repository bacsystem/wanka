-- Mutacion: le da a wanka_worker el SELECT de password_hash en iam.users. El worker solo necesita el correo (#24).
-- Uso: MUTATION=grant_worker_select_user_password sh run.sh   (debe fallar en el paso wanka_worker)
\set ON_ERROR_STOP on
GRANT SELECT (password_hash) ON iam.users TO wanka_worker;
