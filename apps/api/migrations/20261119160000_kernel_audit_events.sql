-- +goose Up
-- Kernel: bitácora de auditoría audit.events (issue #173, ADR 0001, data-model §1 y §5, er-05). Tabla de
-- tenant, de solo inserción y particionada por mes sobre "at". La escribe la capa de aplicación en la MISMA
-- transacción que el cambio (audit.Writer.Record); la consulta y el exporte son de #79 y #80. Corre como
-- wanka_migrator en UNA transacción.
--
-- Sin FK a org.tenants a propósito: la bitácora sobrevive a la empresa (retención de 5 años, data-model §5) y
-- una FK obligaría a cascadear o a bloquear la purga. El aislamiento lo da la política, no una referencia.
-- La clave primaria incluye "at" porque PostgreSQL exige la columna de partición en toda clave única.
CREATE TABLE audit.events (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  actor_type text NOT NULL,
  -- Usuario de la empresa. NULL en el sistema (un job no tiene usuario; su origen va en action o diff, ADR 0001
  -- «Workers») y en el staff de plataforma (va en impersonator_id).
  actor_user_id uuid,
  impersonator_id uuid,
  action text NOT NULL,
  entity_type text,
  entity_id uuid,
  result text NOT NULL DEFAULT 'ok',
  ip inet,
  request_id text,
  diff jsonb,
  at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, at),
  -- Con nombre propio para que rls-check/mutations pueda quitarlos uno a uno.
  CONSTRAINT events_actor_type_check CHECK (actor_type IN ('user', 'platform_staff', 'system')),
  CONSTRAINT events_result_check CHECK (result IN ('ok', 'denegado')),
  -- Una fila de tipo user sin usuario es imposible (decisión del architect, #173).
  CONSTRAINT events_user_has_user CHECK (actor_type <> 'user' OR actor_user_id IS NOT NULL),
  -- El sistema nunca lleva un usuario inventado.
  CONSTRAINT events_system_has_no_user CHECK (actor_type <> 'system' OR actor_user_id IS NULL),
  -- El staff de plataforma va siempre como impersonador y nunca como usuario de la empresa, y solo él lleva impersonador.
  CONSTRAINT events_staff_has_staff CHECK (actor_type <> 'platform_staff' OR (actor_user_id IS NULL AND impersonator_id IS NOT NULL)),
  CONSTRAINT events_impersonator_is_staff CHECK (impersonator_id IS NULL OR actor_type = 'platform_staff'),
  CONSTRAINT events_action_check CHECK (char_length(action) BETWEEN 3 AND 120),
  CONSTRAINT events_request_id_check CHECK (char_length(request_id) <= 128)
) PARTITION BY RANGE (at);

ALTER TABLE audit.events ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit.events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON audit.events
  USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());

-- Solo SELECT e INSERT (sin UPDATE, DELETE ni TRUNCATE): los eventos no se modifican ni se borran. Las
-- particiones viejas se separan y se descartan enteras (data-model §5), nunca con DELETE.
GRANT USAGE ON SCHEMA audit TO wanka_app, wanka_worker;
GRANT SELECT, INSERT ON audit.events TO wanka_app, wanka_worker;

-- Crea las particiones mensuales de p_from hasta p_ahead meses después (en UTC), las que falten. Es idempotente
-- y el worker la llama a diario (job audit.ensure_partitions, cola maintenance).
--
-- Excepción acotada del ADR 0001, «Excepción: DDL de particiones» (architect, 2026-11-19): es SECURITY DEFINER y su dueño es wanka_migrator, no
-- wanka_definer. Crear o enganchar una partición exige ser dueño de la tabla padre, y wanka_definer no es dueño
-- de tablas (si lo fuera, el migrador dejaría de poder alterar audit.events). Por eso la función hace lo mínimo:
-- ningún texto libre (una fecha y un entero), rango acotado, search_path fijo, y EXECUTE solo para wanka_worker.
--
-- CREATE TABLE ... PARTITION OF toma ACCESS EXCLUSIVE sobre el padre y frenaría los INSERT de todos los
-- tenants; ATTACH PARTITION solo toma SHARE UPDATE EXCLUSIVE, que no choca con ellos. El lock_timeout evita
-- hacer cola detrás de otro DDL. El lock de aviso serializa dos workers.
-- +goose StatementBegin
CREATE FUNCTION audit.ensure_partitions(p_from date, p_ahead integer) RETURNS integer
  LANGUAGE plpgsql SECURITY DEFINER
  SET search_path = pg_catalog, pg_temp
AS $$
DECLARE
  first_month date; m date; part text; made integer := 0;
BEGIN
  IF p_ahead IS NULL OR p_ahead < 0 OR p_ahead > 12 THEN
    RAISE EXCEPTION 'ensure_partitions: p_ahead must be between 0 and 12' USING ERRCODE = 'invalid_parameter_value';
  END IF;
  IF p_from IS NULL OR p_from < date '2026-01-01' OR p_from > (pg_catalog.now() AT TIME ZONE 'UTC')::date + 366 THEN
    RAISE EXCEPTION 'ensure_partitions: p_from out of range' USING ERRCODE = 'invalid_parameter_value';
  END IF;
  PERFORM pg_catalog.set_config('lock_timeout', '3s', true);
  PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtext('audit.ensure_partitions'));
  first_month := pg_catalog.date_trunc('month', p_from::timestamp)::date;
  FOR i IN 0..p_ahead LOOP
    m := (first_month + pg_catalog.make_interval(months => i))::date;
    part := 'events_y' || pg_catalog.to_char(m, 'YYYY') || 'm' || pg_catalog.to_char(m, 'MM');
    IF pg_catalog.to_regclass(pg_catalog.format('audit.%I', part)) IS NULL THEN
      EXECUTE pg_catalog.format('CREATE TABLE audit.%I (LIKE audit.events INCLUDING ALL)', part);
      EXECUTE pg_catalog.format('ALTER TABLE audit.events ATTACH PARTITION audit.%I FOR VALUES FROM (%L) TO (%L)', part,
        pg_catalog.to_char(m, 'YYYY-MM-DD') || ' 00:00:00+00',
        pg_catalog.to_char((m + pg_catalog.make_interval(months => 1))::date, 'YYYY-MM-DD') || ' 00:00:00+00');
      made := made + 1;
    END IF;
  END LOOP;
  RETURN made;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION audit.ensure_partitions(date, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION audit.ensure_partitions(date, integer) TO wanka_worker;

-- Particiones iniciales: el mes actual y los 3 siguientes.
SELECT audit.ensure_partitions((now() AT TIME ZONE 'UTC')::date, 3);
