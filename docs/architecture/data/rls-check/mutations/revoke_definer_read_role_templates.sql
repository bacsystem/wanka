-- Mutacion: quita el SELECT de las plantillas a wanka_definer: el alta desde la consola (su trigger de roles lee la plantilla) fallaria.
-- Uso: MUTATION=revoke_definer_read_role_templates sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
REVOKE SELECT ON iam.role_templates FROM wanka_definer;
