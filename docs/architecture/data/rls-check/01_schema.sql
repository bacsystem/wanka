-- Bootstrap de roles (ADR 0001, «Creación de los roles»). Lo ejecuta el superusuario, FUERA de goose:
-- en Railway, el admin con el usuario administrador del servicio Postgres, una vez por entorno y otra vez
-- en cada release que agregue un rol. Es idempotente: crea lo que falta y vuelve a fijar los atributos
-- (run.sh lo corre dos veces). No fija contraseñas: el admin las pone con \password (psql envía el hash
-- SCRAM, el texto no queda en archivos ni en el historial). Aquí el contenedor usa trust y no las necesita.
-- Modelo: cada servicio se conecta con SU rol, que es LOGIN; no hay roles de grupo con usuarios aparte.
-- El único NOLOGIN es wanka_definer (dueño de las funciones SECURITY DEFINER; nadie se conecta con él).
\set ON_ERROR_STOP on
DO $$
DECLARE r text;
BEGIN
  FOREACH r IN ARRAY ARRAY['wanka_migrator','wanka_definer','wanka_app','wanka_worker','wanka_webhooks','wanka_platform'] LOOP
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = r) THEN
      EXECUTE format('CREATE ROLE %I', r);
    END IF;
    -- fijar NOSUPERUSER, NOREPLICATION y NOBYPASSRLS exige SUPERUSER: CREATEROLE no basta (ADR 0001)
    EXECUTE format('ALTER ROLE %I NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS %s', r,
                   CASE WHEN r = 'wanka_definer' THEN 'NOLOGIN' ELSE 'LOGIN' END);
  END LOOP;
END $$;
GRANT wanka_definer TO wanka_migrator;                    -- para poder hacer ALTER FUNCTION ... OWNER TO wanka_definer
GRANT CREATE ON DATABASE wanka TO wanka_migrator;
ALTER ROLE wanka_app SET statement_timeout = '5s';        -- api/conventions.md §7; ALTER ROLE ... SET exige superusuario o CREATEROLE

-- El bootstrap se verifica a sí mismo: un atributo fuera de lo esperado aborta.
DO $$
BEGIN
  IF (SELECT count(*) FROM pg_roles WHERE rolname LIKE 'wanka\_%') <> 6 THEN RAISE EXCEPTION 'faltan o sobran roles wanka_*'; END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname LIKE 'wanka\_%' AND (rolsuper OR rolbypassrls OR rolcreaterole OR rolcreatedb OR rolreplication)) THEN
    RAISE EXCEPTION 'un rol wanka_* tiene SUPERUSER, BYPASSRLS, CREATEROLE, CREATEDB o REPLICATION'; END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname LIKE 'wanka\_%' AND rolcanlogin <> (rolname <> 'wanka_definer')) THEN
    RAISE EXCEPTION 'LOGIN: solo wanka_definer es NOLOGIN'; END IF;
  RAISE NOTICE 'bootstrap OK: 5 roles LOGIN + wanka_definer NOLOGIN, ninguno con BYPASSRLS ni CREATEROLE';
END $$;
