-- Ejecutado como superusuario SOLO para crear roles (lo que en Railway hace el admin una vez).
\set ON_ERROR_STOP on
CREATE ROLE wanka_migrator LOGIN NOBYPASSRLS PASSWORD 'x';
CREATE ROLE wanka_definer NOLOGIN NOBYPASSRLS;           -- dueño de TODAS las funciones SECURITY DEFINER
CREATE ROLE wanka_app LOGIN NOBYPASSRLS PASSWORD 'x';
CREATE ROLE wanka_worker LOGIN NOBYPASSRLS PASSWORD 'x';
CREATE ROLE wanka_webhooks LOGIN NOBYPASSRLS PASSWORD 'x';
CREATE ROLE wanka_platform LOGIN NOBYPASSRLS PASSWORD 'x';
GRANT wanka_definer TO wanka_migrator;                    -- para poder hacer ALTER FUNCTION ... OWNER TO wanka_definer
GRANT CREATE ON DATABASE wanka TO wanka_migrator;
