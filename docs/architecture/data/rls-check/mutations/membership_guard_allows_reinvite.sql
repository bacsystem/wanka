-- Mutacion: quita el trigger iam.memberships_guard_status: una membresia activa o suspendida vuelve a invitado y luego se borra,
-- saltando iam.guard_membership_deletes (revision del PR #245).
-- Uso: MUTATION=membership_guard_allows_reinvite sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP TRIGGER memberships_guard_status ON iam.memberships;
