-- Mutacion: revocar una sesion ya revocada lanza 42501 en vez de ser un no-op: un UPDATE masivo que revoca por usuario fallaria si alguna ya estaba revocada.
-- Uso: MUTATION=guard_rerevoke_raises sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DO $$ DECLARE d text := pg_get_functiondef('iam.guard_session_writes()'::regprocedure); m text;
BEGIN
  m := replace(d, $q$NEW.revoked_at := OLD.revoked_at;$q$, $q$RAISE EXCEPTION 'x' USING ERRCODE = 'insufficient_privilege';$q$);
  IF m = d THEN RAISE EXCEPTION 'la mutacion no cambia nada: ya no coincide con la funcion'; END IF;
  EXECUTE m;
END $$;
