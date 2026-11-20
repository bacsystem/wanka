-- Mutacion: iam.valid_recovery_codes acepta cualquier arreglo (codigos en claro, vacios o con relleno entrarian, #231).
-- Uso: MUTATION=weaken_recovery_codes_function sh run.sh   (debe fallar en el paso wanka_app)
\set ON_ERROR_STOP on
CREATE OR REPLACE FUNCTION iam.valid_recovery_codes(codes text[]) RETURNS boolean
  LANGUAGE sql IMMUTABLE STRICT SET search_path = pg_catalog, pg_temp AS 'SELECT true';
