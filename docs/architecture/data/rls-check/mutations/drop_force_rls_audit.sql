-- Mutacion: quita FORCE ROW LEVEL SECURITY de audit.events (el duenio volveria a saltarse la politica).
-- Uso: MUTATION=drop_force_rls_audit sh run.sh   (debe fallar en el paso «wanka_migrator: borra la membresia»)
\set ON_ERROR_STOP on
ALTER TABLE audit.events NO FORCE ROW LEVEL SECURITY;
