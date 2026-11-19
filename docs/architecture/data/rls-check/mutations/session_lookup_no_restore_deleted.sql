-- Mutacion: session_lookup restaura app.tenant_id solo si la membresia existe; con la membresia borrada deja fijado el tenant de la sesion.
-- Demuestra que el caso A9 de 07_after_delete.sql distingue por si solo (05_app.sql usa una membresia viva y no lo ve).
-- Uso: MUTATION=session_lookup_no_restore_deleted sh run.sh   (debe fallar en el paso «wanka_app (membresia borrada)»)
\set ON_ERROR_STOP on
CREATE OR REPLACE FUNCTION iam.session_lookup(p_access_hash bytea)
  RETURNS TABLE (session_id uuid, user_id uuid, tenant_id uuid, site_id uuid, cash_register_id uuid, membership_id uuid,
                 session_authz_version bigint, mfa_satisfied boolean, mfa_method text, impersonator_id uuid,
                 current_authz_version bigint, membership_status text)
  LANGUAGE plpgsql STABLE SECURITY DEFINER
  SET search_path = pg_catalog, pg_temp
AS $$
DECLARE s iam.sessions%ROWTYPE; v bigint; st text; prev text := current_setting('app.tenant_id', true);
BEGIN
  SELECT * INTO s FROM iam.sessions x
   WHERE x.access_hash = p_access_hash AND x.revoked_at IS NULL AND x.access_expires_at > now() AND x.mfa_satisfied;
  IF NOT FOUND THEN RETURN; END IF;
  IF s.membership_id IS NOT NULL THEN
    PERFORM set_config('app.tenant_id', s.tenant_id::text, true);
    SELECT m.authz_version, m.status INTO v, st FROM iam.memberships m WHERE m.id = s.membership_id AND m.user_id = s.user_id;
    IF v IS NOT NULL THEN PERFORM set_config('app.tenant_id', coalesce(prev, ''), true); END IF;   -- defecto: no restaura si la membresia no existe
  END IF;
  RETURN QUERY SELECT s.id, s.user_id, s.tenant_id, s.site_id, s.cash_register_id, s.membership_id, s.authz_version,
                      s.mfa_satisfied, s.mfa_method, s.impersonator_id, v, st;
END $$;
