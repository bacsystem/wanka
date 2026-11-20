-- Mutacion: la plantilla dueno pierde un permiso del catalogo (el dueno siempre tiene el catalogo completo, #233).
-- Uso: MUTATION=seed_owner_loses_a_permission sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DELETE FROM iam.role_template_permissions WHERE template_code = 'dueno' AND permission_code = 'catalogo.costs:read';
