-- Mutacion para demostrar que la verificacion falla sola: la version LANGUAGE sql con JOIN directo
-- (el defecto H37). Con FORCE RLS y un duenio NOBYPASSRLS ve la membresia sin tenant y devuelve NULL.
-- Uso: MUTATION=naive_session_lookup sh run.sh   (debe terminar con exit distinto de 0)
\set ON_ERROR_STOP on
CREATE OR REPLACE FUNCTION iam.session_lookup(p_access_hash bytea)
  RETURNS TABLE (session_id uuid, user_id uuid, tenant_id uuid, site_id uuid, cash_register_id uuid, membership_id uuid,
                 session_authz_version bigint, mfa_satisfied boolean, mfa_method text, impersonator_id uuid,
                 current_authz_version bigint, membership_status text)
  LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp
AS $$
  SELECT s.id, s.user_id, s.tenant_id, s.site_id, s.cash_register_id, s.membership_id, s.authz_version,
         s.mfa_satisfied, s.mfa_method, s.impersonator_id, m.authz_version, m.status
    FROM iam.sessions s LEFT JOIN iam.memberships m ON m.id = s.membership_id AND m.tenant_id = s.tenant_id AND m.user_id = s.user_id
   WHERE s.access_hash = p_access_hash AND s.revoked_at IS NULL AND s.access_expires_at > now() AND s.mfa_satisfied
$$;
