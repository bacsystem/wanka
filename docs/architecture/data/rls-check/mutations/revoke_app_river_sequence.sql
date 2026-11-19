-- Mutacion: wanka_app pierde el USAGE de la secuencia de ids de ops.river_job. Sin ella no puede encolar (nextval).
-- Uso: MUTATION=revoke_app_river_sequence sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
REVOKE USAGE ON SEQUENCE ops.river_job_id_seq FROM wanka_app;
