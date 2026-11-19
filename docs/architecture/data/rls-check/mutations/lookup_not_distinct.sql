-- Mutacion: cambia «mu IS DISTINCT FROM s.user_id» por «mu <> s.user_id» en iam.session_lookup. Con NULL (una sesion impersonada
-- con membership_id y sin user_id) la comparacion da NULL y la membresia se veria como vigente.
-- Uso: MUTATION=lookup_not_distinct sh run.sh   (debe fallar en el paso «superusuario: sesiones impersonadas»)
\set ON_ERROR_STOP on
DO $$ DECLARE d text := pg_get_functiondef('iam.session_lookup(bytea)'::regprocedure); m text;
BEGIN
  m := replace(d, $q$IF mu IS DISTINCT FROM s.user_id THEN$q$, $q$IF mu <> s.user_id THEN$q$);
  IF m = d THEN RAISE EXCEPTION 'la mutacion no cambia nada: ya no coincide con la funcion'; END IF;
  EXECUTE m;
END $$;
