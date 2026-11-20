-- Mutacion: quita el trigger iam.guard_membership_deletes: wanka_app borra una membresia activa o suspendida (ADR 0011).
-- Uso: MUTATION=drop_membership_guard_trigger sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP TRIGGER memberships_guard_deletes ON iam.memberships;
