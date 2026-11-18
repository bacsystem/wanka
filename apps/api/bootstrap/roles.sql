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
  -- PostgreSQL 15+ ya no deja crear en "public" a nadie salvo al duenio de la base: goose guarda ahi su
  -- tabla de versiones (goose_db_version). Solo el migrador recibe el permiso.
  EXECUTE 'GRANT CREATE ON SCHEMA public TO wanka_migrator';
  -- api/conventions.md §7; ALTER ROLE ... SET exige superusuario o CREATEROLE
  EXECUTE 'ALTER ROLE wanka_app SET statement_timeout = ''5s''';
END $$;

-- El bootstrap se verifica a si mismo contra pg_roles: un atributo fuera de lo esperado aborta.
DO $$
BEGIN
  IF (SELECT count(*) FROM pg_roles WHERE rolname LIKE 'wanka\_%') <> 6 THEN
    RAISE EXCEPTION 'faltan o sobran roles wanka_*';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname LIKE 'wanka\_%'
             AND (rolsuper OR rolbypassrls OR rolcreaterole OR rolcreatedb OR rolreplication)) THEN
    RAISE EXCEPTION 'un rol wanka_* tiene SUPERUSER, BYPASSRLS, CREATEROLE, CREATEDB o REPLICATION';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname LIKE 'wanka\_%' AND rolcanlogin <> (rolname <> 'wanka_definer')) THEN
    RAISE EXCEPTION 'LOGIN: solo wanka_definer es NOLOGIN';
  END IF;
  IF NOT pg_has_role('wanka_migrator', 'wanka_definer', 'MEMBER') THEN
    RAISE EXCEPTION 'wanka_migrator no es miembro de wanka_definer';
  END IF;
  IF NOT has_database_privilege('wanka_migrator', current_database(), 'CREATE') THEN
    RAISE EXCEPTION 'wanka_migrator no tiene CREATE sobre la base';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_db_role_setting s JOIN pg_roles r ON r.oid = s.setrole
                 WHERE r.rolname = 'wanka_app' AND s.setdatabase = 0 AND s.setconfig @> ARRAY['statement_timeout=5s']) THEN
    RAISE EXCEPTION 'wanka_app no tiene statement_timeout = 5s';
  END IF;
  RAISE NOTICE 'bootstrap OK: 5 roles LOGIN + wanka_definer NOLOGIN, ninguno con BYPASSRLS ni CREATEROLE';
END $$;
