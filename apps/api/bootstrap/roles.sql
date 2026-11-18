-- Bootstrap de roles (ADR 0001, "Creacion de los roles"). Lo ejecuta el SUPERUSUARIO, fuera de goose:
--   psql -v ON_ERROR_STOP=1 -d <base> -f bootstrap/roles.sql
-- una vez por entorno y otra vez en cada release que agregue un rol. Es idempotente: crea los roles
-- que faltan y vuelve a fijar sus atributos en cada corrida. Referencia: docs/architecture/data/rls-check/01_schema.sql.
--
-- No fija contrasenas: el admin las pone con \password <rol> en psql (envia el hash SCRAM, el texto no
-- queda en archivos ni en el historial). Modelo: cada servicio se conecta con SU rol, que es LOGIN; el unico
-- NOLOGIN es wanka_definer (duenio de las funciones SECURITY DEFINER; nadie se conecta con el).
--
-- Exige SUPERUSER: CREATEROLE no basta, porque en PostgreSQL 16 solo un superusuario puede fijar
-- NOSUPERUSER, NOREPLICATION y NOBYPASSRLS, aunque sea para quitarlos.
DO $$
DECLARE r text;
BEGIN
  FOREACH r IN ARRAY ARRAY['wanka_migrator','wanka_definer','wanka_app','wanka_worker','wanka_webhooks','wanka_platform'] LOOP
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = r) THEN
      EXECUTE format('CREATE ROLE %I', r);
    END IF;
    EXECUTE format('ALTER ROLE %I NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS %s', r,
                   CASE WHEN r = 'wanka_definer' THEN 'NOLOGIN' ELSE 'LOGIN' END);
  END LOOP;

  -- para poder hacer ALTER FUNCTION ... OWNER TO wanka_definer
  EXECUTE 'GRANT wanka_definer TO wanka_migrator';
  -- el migrador crea los esquemas de la aplicacion
  EXECUTE format('GRANT CREATE ON DATABASE %I TO wanka_migrator', current_database());
  -- Nadie crea objetos en "public": una sentencia sin esquema en una migracion falla con 42501 en vez de
  -- dejar un objeto que todos los roles verian. PostgreSQL 15+ ya lo trae asi; el REVOKE cubre las bases
  -- restauradas o actualizadas desde versiones anteriores (idempotente).
  EXECUTE 'REVOKE CREATE ON SCHEMA public FROM PUBLIC';
  -- goose guarda sus versiones en goose.goose_db_version, un esquema propio del migrador (no en public).
  -- Lo crea el bootstrap porque goose crea su tabla antes de la primera migracion.
  EXECUTE 'CREATE SCHEMA IF NOT EXISTS goose AUTHORIZATION wanka_migrator';
  EXECUTE 'ALTER SCHEMA goose OWNER TO wanka_migrator';
  -- api/conventions.md §7; ALTER ROLE ... SET exige superusuario o CREATEROLE
  EXECUTE 'ALTER ROLE wanka_app SET statement_timeout = ''5s''';
END $$;

-- El bootstrap se verifica a si mismo contra pg_roles: un atributo fuera de lo esperado aborta.
DO $$
BEGIN
  IF (SELECT count(*) FROM pg_roles WHERE rolname LIKE 'wanka\_%') <> 6 THEN
    RAISE EXCEPTION 'missing or extra wanka_* roles';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname LIKE 'wanka\_%'
             AND (rolsuper OR rolbypassrls OR rolcreaterole OR rolcreatedb OR rolreplication)) THEN
    RAISE EXCEPTION 'a wanka_* role has SUPERUSER, BYPASSRLS, CREATEROLE, CREATEDB or REPLICATION';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname LIKE 'wanka\_%' AND rolcanlogin <> (rolname <> 'wanka_definer')) THEN
    RAISE EXCEPTION 'LOGIN: only wanka_definer is NOLOGIN';
  END IF;
  IF NOT pg_has_role('wanka_migrator', 'wanka_definer', 'MEMBER') THEN
    RAISE EXCEPTION 'wanka_migrator is not a member of wanka_definer';
  END IF;
  IF NOT has_database_privilege('wanka_migrator', current_database(), 'CREATE') THEN
    RAISE EXCEPTION 'wanka_migrator lacks CREATE on the database';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_db_role_setting s JOIN pg_roles r ON r.oid = s.setrole
                 WHERE r.rolname = 'wanka_app' AND s.setdatabase = 0 AND s.setconfig @> ARRAY['statement_timeout=5s']) THEN
    RAISE EXCEPTION 'wanka_app lacks statement_timeout = 5s';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname LIKE 'wanka\_%' AND has_schema_privilege(rolname, 'public', 'CREATE')) THEN
    RAISE EXCEPTION 'a wanka_* role can create in schema public';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_namespace n WHERE n.nspname = 'goose' AND pg_get_userbyid(n.nspowner) = 'wanka_migrator') THEN
    RAISE EXCEPTION 'schema goose is missing or not owned by wanka_migrator';
  END IF;
  RAISE NOTICE 'bootstrap OK: 5 LOGIN roles + wanka_definer NOLOGIN, none with BYPASSRLS or CREATEROLE';
END $$;
