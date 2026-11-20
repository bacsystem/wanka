-- Mutacion: member_read de org.sites ignora el alcance de la membresia (membership_sites): un cajero de una sede ve todas.
-- Uso: MUTATION=member_read_sites_no_scope sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP POLICY member_read ON org.sites;
CREATE POLICY member_read ON org.sites FOR SELECT USING (
  tenant_id = app.current_tenant()
  OR (app.current_tenant() IS NULL AND EXISTS (SELECT 1 FROM iam.memberships m
             WHERE m.user_id = app.current_user_id() AND m.status = 'activo' AND m.tenant_id = sites.tenant_id)));
