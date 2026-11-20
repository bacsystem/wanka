-- Mutacion: el trigger de iam.user_tokens deja a la app fijar created_at (con un created_at futuro un token duraria semanas, #231).
-- Uso: MUTATION=user_token_guard_no_server_created_at sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DO $$ DECLARE d text := pg_get_functiondef('iam.guard_user_token_writes()'::regprocedure); m text;
BEGIN
  m := replace(d, $q$NEW.created_at := pg_catalog.now();$q$, $q$NULL;$q$);
  IF m = d THEN RAISE EXCEPTION 'la mutacion no cambia nada: ya no coincide con la funcion'; END IF;
  EXECUTE m;
END $$;
