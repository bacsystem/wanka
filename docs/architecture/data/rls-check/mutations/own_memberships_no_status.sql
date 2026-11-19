-- Mutacion: own_memberships de iam.memberships sin el filtro de estado: el ex miembro vuelve a leer su membresia suspendida.
-- Uso: MUTATION=own_memberships_no_status sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP POLICY own_memberships ON iam.memberships;
CREATE POLICY own_memberships ON iam.memberships FOR SELECT USING (app.current_tenant() IS NULL AND user_id = app.current_user_id());
