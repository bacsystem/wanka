-- Mutacion: iam.list_my_memberships sin el filtro de estado: con tenant B fijado, la membresia suspendida del ex miembro aparece.
-- Uso: MUTATION=view_without_status_filter sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
CREATE OR REPLACE VIEW iam.list_my_memberships WITH (security_invoker = true) AS
SELECT m.id AS membership_id, m.tenant_id, coalesce(t.trade_name, t.legal_name) AS tenant_name, t.industry, t.status AS tenant_status,
       m.status AS membership_status, r.id AS role_id, r.name AS role_name, r.requires_mfa, m.all_sites,
       '[]'::jsonb AS sites, '[]'::jsonb AS cash_registers
  FROM iam.memberships m
  JOIN org.tenants t ON t.id = m.tenant_id
  JOIN iam.roles r ON r.tenant_id = m.tenant_id AND r.id = m.role_id
 WHERE m.user_id = app.current_user_id();
