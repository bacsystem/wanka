-- Mutacion: la plantilla cajero recibe finanzas.cash:approve (aprobar caja), que la matriz del dueno le niega (#233).
-- Uso: MUTATION=seed_cajero_approves_cash sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
INSERT INTO iam.role_template_permissions (template_code, permission_code) VALUES ('cajero', 'finanzas.cash:approve');
