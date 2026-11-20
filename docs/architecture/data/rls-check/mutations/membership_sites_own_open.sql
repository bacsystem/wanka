-- Mutacion: own_memberships de iam.membership_sites deja de acotar por la membresia propia: cualquier usuario sin tenant lee el
-- alcance de todas las membresias (y, por la politica de las sedes, deja de importar el filtro de estado de la membresia).
-- Uso: MUTATION=membership_sites_own_open sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP POLICY own_memberships ON iam.membership_sites;
CREATE POLICY own_memberships ON iam.membership_sites FOR SELECT USING (app.current_tenant() IS NULL);
