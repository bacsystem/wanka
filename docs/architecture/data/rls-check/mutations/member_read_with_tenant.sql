-- Mutacion: own_memberships (iam.memberships) y member_read (org.sites) sin la condicion app.current_tenant() IS NULL: dentro de
-- InTenantTx(A), un miembro de A y B ve su membresia y las sedes de B. Las ramas por usuario solo valen sin tenant fijado (ADR 0001).
-- Uso: MUTATION=member_read_with_tenant sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DROP POLICY own_memberships ON iam.memberships;
CREATE POLICY own_memberships ON iam.memberships FOR SELECT USING (user_id = app.current_user_id() AND status IN ('invitado','activo'));
DROP POLICY member_read ON org.sites;
CREATE POLICY member_read ON org.sites FOR SELECT USING (
  tenant_id = app.current_tenant()
  OR EXISTS (SELECT 1 FROM iam.memberships m
             WHERE m.user_id = app.current_user_id() AND m.status = 'activo' AND m.tenant_id = sites.tenant_id
               AND (m.all_sites OR EXISTS (SELECT 1 FROM iam.membership_sites ms
                                           WHERE ms.membership_id = m.id AND ms.site_id = sites.id))));
