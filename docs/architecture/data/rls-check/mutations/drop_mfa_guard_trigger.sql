-- Mutacion: quita el trigger iam.guard_mfa_enrollment_writes. wanka_app podria insertar un created_at futuro (token vigente
-- semanas) y reabrir un token consumido (#25, revision del PR #203).
-- Uso: MUTATION=drop_mfa_guard_trigger sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP TRIGGER mfa_enrollments_guard_writes ON iam.mfa_enrollments;
