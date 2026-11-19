-- Mutacion: quita la comparacion de mu con s.user_id de iam.session_lookup: una sesion cruzada (usuario A con la membresia de B) veria la membresia ajena como vigente.
-- Uso: MUTATION=lookup_without_user_check sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DO $$ DECLARE d text := pg_get_functiondef('iam.session_lookup(bytea)'::regprocedure); m text;
BEGIN
  m := replace(d, $q$IF mu IS DISTINCT FROM s.user_id THEN$q$, $q$IF false THEN$q$);
  IF m = d THEN RAISE EXCEPTION 'la mutacion no cambia nada: ya no coincide con la funcion'; END IF;
  EXECUTE m;
END $$;
