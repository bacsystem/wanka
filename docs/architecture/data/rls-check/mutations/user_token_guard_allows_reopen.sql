-- Mutacion: el trigger de iam.user_tokens deja reabrir un token consumido (consumed_at a NULL) y volver a consumirlo (#231).
-- Uso: MUTATION=user_token_guard_allows_reopen sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DO $$ DECLARE d text := pg_get_functiondef('iam.guard_user_token_writes()'::regprocedure); m text;
BEGIN
  m := replace(d, $q$IF OLD.consumed_at IS NOT NULL THEN$q$, $q$IF OLD.consumed_at IS NOT NULL AND NEW.consumed_at IS NOT NULL THEN$q$);
  IF m = d THEN RAISE EXCEPTION 'la mutacion no cambia nada: ya no coincide con la funcion'; END IF;
  EXECUTE m;
END $$;
