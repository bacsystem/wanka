-- Mutacion: member_read de org.tenants deja pasar tambien la membresia 'suspendido' (el filtro existe pero esta mal puesto).
-- La regex vieja de 05_app.sql lo daba por bueno; el caso compara el conjunto exacto de estados.
-- Uso: MUTATION=member_read_with_suspended sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP POLICY member_read ON org.tenants;
CREATE POLICY member_read ON org.tenants FOR SELECT USING (app.current_tenant() IS NULL AND id IN
  (SELECT tenant_id FROM iam.memberships WHERE user_id = app.current_user_id() AND status IN ('invitado','activo','suspendido')));
