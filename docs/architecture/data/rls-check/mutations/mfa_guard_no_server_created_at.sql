-- Mutacion: el trigger deja de fijar created_at en el INSERT: un created_at futuro haria pasar el CHECK del TTL con un token vigente semanas.
-- Uso: MUTATION=mfa_guard_no_server_created_at sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DO $$ DECLARE d text := pg_get_functiondef('iam.guard_mfa_enrollment_writes()'::regprocedure); m text;
BEGIN
  m := replace(d, $q$NEW.created_at := pg_catalog.now();$q$, $q$NULL;$q$);
  IF m = d THEN RAISE EXCEPTION 'la mutacion no cambia nada: ya no coincide con la funcion'; END IF;
  EXECUTE m;
END $$;
