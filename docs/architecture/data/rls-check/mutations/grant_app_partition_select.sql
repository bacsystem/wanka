-- Mutacion: wanka_app lee una particion directo (se salta la politica del padre).
-- Uso: MUTATION=grant_app_partition_select sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
DO $$ BEGIN EXECUTE format('GRANT SELECT ON audit.%I TO wanka_app', 'events_y' || to_char(now() AT TIME ZONE 'UTC', 'YYYY') || 'm' || to_char(now() AT TIME ZONE 'UTC', 'MM')); END $$;
