-- Mutacion: quita el SELECT de las plantillas a wanka_app: el resolver de permisos y el trigger de roles no podrian leerlas.
-- Uso: MUTATION=revoke_app_read_role_templates sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
REVOKE SELECT ON iam.role_templates, iam.role_template_permissions FROM wanka_app;
