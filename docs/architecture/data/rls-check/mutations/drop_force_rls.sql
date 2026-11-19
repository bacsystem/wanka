-- Mutacion: quita FORCE ROW LEVEL SECURITY de iam.memberships. El dueno (wanka_migrator) volveria a saltarse la politica.
-- Demuestra que el caso de FORCE RLS de 06_delete_and_others.sql distingue.
-- Uso: MUTATION=drop_force_rls sh run.sh   (debe fallar en el paso «wanka_migrator: borra la membresia»)
\set ON_ERROR_STOP on
ALTER TABLE iam.memberships NO FORCE ROW LEVEL SECURITY;
