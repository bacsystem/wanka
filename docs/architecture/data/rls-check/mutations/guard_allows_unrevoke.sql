-- Mutacion: el trigger deja de impedir que revoked_at deje de ser un instante: una sesion revocada se podria reactivar.
-- Uso: MUTATION=guard_allows_unrevoke sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DO $$ DECLARE d text := pg_get_functiondef('iam.guard_session_writes()'::regprocedure); m text;
BEGIN
  m := replace(d, $q$IF NEW.revoked_at IS NULL THEN$q$, $q$IF false THEN$q$);
  IF m = d THEN RAISE EXCEPTION 'la mutacion no cambia nada: ya no coincide con la funcion'; END IF;
  EXECUTE m;
END $$;
