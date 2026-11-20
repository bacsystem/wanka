-- Mutacion: iam.list_my_memberships sin el WHERE por app.user_id: con un tenant fijado devuelve las membresias de todos sus miembros.
-- Uso: MUTATION=view_without_user_filter sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
CREATE OR REPLACE VIEW iam.list_my_memberships WITH (security_invoker = true) AS
SELECT m.id AS membership_id, m.tenant_id, coalesce(t.trade_name, t.legal_name) AS tenant_name, t.industry, t.status AS tenant_status,
       m.status AS membership_status, r.id AS role_id, r.name AS role_name, r.requires_mfa, m.all_sites,
       coalesce(s.sites, '[]'::jsonb) AS sites, coalesce(c.cash_registers, '[]'::jsonb) AS cash_registers
  FROM iam.memberships m
  JOIN org.tenants t ON t.id = m.tenant_id
  JOIN iam.roles r ON r.tenant_id = m.tenant_id AND r.id = m.role_id
  LEFT JOIN LATERAL (
    SELECT jsonb_agg(jsonb_build_object('id', x.id, 'code', x.code, 'name', x.name) ORDER BY x.code, x.id) AS sites
      FROM org.sites x
     WHERE x.tenant_id = m.tenant_id AND x.active AND m.status = 'activo'
       AND (m.all_sites OR EXISTS (SELECT 1 FROM iam.membership_sites ms
                                    WHERE ms.tenant_id = m.tenant_id AND ms.membership_id = m.id AND ms.site_id = x.id))
  ) s ON true
  LEFT JOIN LATERAL (
    SELECT jsonb_agg(jsonb_build_object('id', k.id, 'site_id', k.site_id, 'code', k.code) ORDER BY k.code, k.id) AS cash_registers
      FROM org.cash_registers k
      JOIN org.sites x ON x.tenant_id = k.tenant_id AND x.id = k.site_id
     WHERE k.tenant_id = m.tenant_id AND k.active AND x.active AND m.status = 'activo'
       AND (m.all_sites OR EXISTS (SELECT 1 FROM iam.membership_sites ms
                                    WHERE ms.tenant_id = m.tenant_id AND ms.membership_id = m.id AND ms.site_id = x.id))
  ) c ON true
 WHERE m.status IN ('invitado', 'activo');
