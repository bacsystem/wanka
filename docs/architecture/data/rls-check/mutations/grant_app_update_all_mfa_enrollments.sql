-- Mutacion: UPDATE de toda la fila de iam.mfa_enrollments para wanka_app (permitiria alargar expires_at o mover el token a otro usuario, #25).
-- Uso: MUTATION=grant_app_update_all_mfa_enrollments sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT UPDATE ON iam.mfa_enrollments TO wanka_app;
