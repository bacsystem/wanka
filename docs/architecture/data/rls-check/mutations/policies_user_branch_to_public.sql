-- Mutación: devuelve las 6 políticas con rama por usuario a PUBLIC (como nacieron en #232)
-- Debe hacer fallar el paso wanka_worker al consultar org.sites sin SELECT sobre iam.membership_sites (42501).
ALTER POLICY own_memberships ON iam.memberships TO PUBLIC;
ALTER POLICY own_memberships ON iam.membership_sites TO PUBLIC;
ALTER POLICY member_read ON org.tenants TO PUBLIC;
ALTER POLICY member_read ON iam.roles TO PUBLIC;
ALTER POLICY member_read ON org.sites TO PUBLIC;
ALTER POLICY member_read ON org.cash_registers TO PUBLIC;
