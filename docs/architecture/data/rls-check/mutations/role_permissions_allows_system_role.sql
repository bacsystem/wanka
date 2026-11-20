-- Mutacion: quita el trigger de iam.role_permissions: un rol base (system) recibe filas propias de permisos, ademas de los de su plantilla.
-- Uso: MUTATION=role_permissions_allows_system_role sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP TRIGGER role_permissions_guard_writes ON iam.role_permissions;
