-- Mutacion: wanka_app puede leer ops.river_job. La tabla no tiene RLS: veria los argumentos de los jobs de todos los tenants.
-- Demuestra que test.river_enqueue_only() (05_app.sql, A10) distingue.
-- Uso: MUTATION=grant_app_select_river_job sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
GRANT SELECT ON ops.river_job TO wanka_app;
