-- Mutacion: el trigger de roles deja cambiar system o template_code (un rol propio pasaria a base: UPDATE iam.roles SET template_code = 'dueno', system = true).
-- Uso: MUTATION=role_guard_allows_system_change sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
CREATE OR REPLACE FUNCTION iam.guard_role_writes() RETURNS trigger
  LANGUAGE plpgsql
  SET search_path = pg_catalog, pg_temp
AS $$
DECLARE template_mfa boolean;
BEGIN
  IF coalesce((SELECT r.rolsuper FROM pg_catalog.pg_roles r WHERE r.rolname = current_user), false) THEN
    IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
  END IF;
  IF TG_OP = 'DELETE' THEN
    IF OLD.system THEN
      RAISE EXCEPTION 'a system role is never deleted' USING ERRCODE = 'insufficient_privilege';
    END IF;
    RETURN OLD;
  END IF;
  IF TG_OP = 'UPDATE' THEN
    IF OLD.system AND NEW.name IS DISTINCT FROM OLD.name THEN
      RAISE EXCEPTION 'a system role is not renamed' USING ERRCODE = 'insufficient_privilege';
    END IF;
  END IF;
  IF NEW.template_code IS NOT NULL THEN
    SELECT t.requires_mfa INTO template_mfa FROM iam.role_templates t WHERE t.code = NEW.template_code;
    IF FOUND AND template_mfa AND NOT NEW.requires_mfa THEN
      RAISE EXCEPTION 'requires_mfa of a system role cannot be lower than its template' USING ERRCODE = 'insufficient_privilege';
    END IF;
  END IF;
  RETURN NEW;
END $$;
