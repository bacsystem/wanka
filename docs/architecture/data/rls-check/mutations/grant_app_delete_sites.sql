-- Mutacion: da DELETE sobre org.sites a wanka_app (el DELETE esta acotado a role_permissions, membership_sites, roles y memberships).
-- Uso: MUTATION=grant_app_delete_sites sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT DELETE ON org.sites TO wanka_app;
