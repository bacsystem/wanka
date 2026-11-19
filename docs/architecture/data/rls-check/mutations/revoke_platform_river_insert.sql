-- Mutacion: wanka_platform pierde el INSERT sobre ops.river_job (la consola ya no puede encolar).
-- Uso: MUTATION=revoke_platform_river_insert sh run.sh   (debe fallar en el paso wanka_platform)
\set ON_ERROR_STOP on
REVOKE INSERT ON ops.river_job FROM wanka_platform;
