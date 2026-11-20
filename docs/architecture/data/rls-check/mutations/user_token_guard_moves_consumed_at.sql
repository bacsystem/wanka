-- Mutacion: el trigger de iam.user_tokens deja mover el instante de un token ya consumido (consumir otra vez lo pisa, #231).
-- Uso: MUTATION=user_token_guard_moves_consumed_at sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DO $$ DECLARE d text := pg_get_functiondef('iam.guard_user_token_writes()'::regprocedure); m text;
BEGIN
  m := replace(d, $q$NEW.consumed_at := OLD.consumed_at;$q$, $q$NEW.consumed_at := pg_catalog.now();$q$);
  IF m = d THEN RAISE EXCEPTION 'la mutacion no cambia nada: ya no coincide con la funcion'; END IF;
  EXECUTE m;
END $$;
