-- Mutacion: quita el trigger iam.guard_role_writes: wanka_app podria crear otro dueno, renombrar o borrar un rol base y bajar su requires_mfa.
-- Uso: MUTATION=drop_role_guard_trigger sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP TRIGGER roles_guard_writes ON iam.roles;
