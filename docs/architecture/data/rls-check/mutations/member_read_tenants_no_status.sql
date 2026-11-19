-- Mutacion: member_read de org.tenants sin el filtro de estado de la membresia (la politica original del ADR 0001).
-- Con own_memberships filtrando, la subconsulta ya no ve la suspendida y ninguna fila cambia: es una defensa en profundidad
-- que solo el caso de catalogo de 05_app.sql (pg_policies.qual) distingue.
-- Uso: MUTATION=member_read_tenants_no_status sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP POLICY member_read ON org.tenants;
CREATE POLICY member_read ON org.tenants FOR SELECT USING (app.current_tenant() IS NULL AND id IN
  (SELECT tenant_id FROM iam.memberships WHERE user_id = app.current_user_id()));
