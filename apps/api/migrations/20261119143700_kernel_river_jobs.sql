-- +goose Up
-- Tablas de River en el esquema ops (ADR 0006, ADR 0001 «Workers»). No son de tenant: sin tenant_id
-- ni RLS; el tenant va en los argumentos de cada job y el handler abre InTenantTx (internal/platform/jobs).
--
-- Es el esquema final de las migraciones 1 a 7 de la línea «main» de River v0.44.1
-- (riverdriver/riverpgxv5/migration/main), con el esquema fijado en ops. goose es el único que migra
-- (solo hacia adelante): "wanka migrate up" no llama al migrador de River. Para que las herramientas de
-- River (river migrate-list) vean la base al día, al final se registran las siete versiones en
-- ops.river_migration. Al subir de versión de River se agrega otra migración goose con sus cambios
-- (un test compara este esquema con el que crea rivermigrate y falla si se desvían).
--
-- Diferencias deliberadas con la secuencia de River, sin efecto en el esquema final:
--   - river_job_state se crea con los ocho valores de una vez (la migración 4 agrega «pending» con
--     ALTER TYPE ADD VALUE, que no se puede usar en la misma transacción que lo crea);
--   - river_migration nace ya con (line, version), sin el paso intermedio de la migración 4;
--   - river_client y river_client_queue (migración 5) no se crean: la migración 7 las borra;
--   - el trigger river_notify y su función (migración 2) no se crean: la migración 4 los borra
--     (el NOTIFY lo hace el cliente).

CREATE TABLE ops.river_migration (
  line text NOT NULL,
  version bigint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT line_length CHECK (char_length(line) > 0 AND char_length(line) < 128),
  CONSTRAINT version_gte_1 CHECK (version >= 1),
  PRIMARY KEY (line, version)
);

CREATE TYPE ops.river_job_state AS ENUM (
  'available',
  'cancelled',
  'completed',
  'discarded',
  'pending',
  'retryable',
  'running',
  'scheduled'
);

CREATE TABLE ops.river_job (
  id bigserial PRIMARY KEY,
  state ops.river_job_state NOT NULL DEFAULT 'available',
  attempt smallint NOT NULL DEFAULT 0,
  max_attempts smallint NOT NULL DEFAULT 25,
  attempted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  finalized_at timestamptz,
  scheduled_at timestamptz NOT NULL DEFAULT NOW(),
  priority smallint NOT NULL DEFAULT 1,
  args jsonb NOT NULL,
  attempted_by text[],
  errors jsonb[],
  kind text NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}',
  queue text NOT NULL DEFAULT 'default',
  tags varchar(255)[] NOT NULL DEFAULT '{}',
  unique_key bytea,
  unique_states bit(8),
  CONSTRAINT finalized_or_finalized_at_null CHECK (
    (finalized_at IS NULL AND state NOT IN ('cancelled', 'completed', 'discarded')) OR
    (finalized_at IS NOT NULL AND state IN ('cancelled', 'completed', 'discarded'))
  ),
  CONSTRAINT max_attempts_is_positive CHECK (max_attempts > 0),
  CONSTRAINT priority_in_range CHECK (priority >= 1 AND priority <= 4),
  CONSTRAINT queue_length CHECK (char_length(queue) > 0 AND char_length(queue) < 128),
  CONSTRAINT kind_length CHECK (char_length(kind) > 0 AND char_length(kind) < 128)
);

CREATE INDEX river_job_kind ON ops.river_job USING btree(kind);
CREATE INDEX river_job_state_and_finalized_at_index ON ops.river_job USING btree(state, finalized_at) WHERE finalized_at IS NOT NULL;
CREATE INDEX river_job_prioritized_fetching_index ON ops.river_job USING btree(state, queue, priority, scheduled_at, id);
CREATE INDEX river_job_args_index ON ops.river_job USING GIN(args);
CREATE INDEX river_job_metadata_index ON ops.river_job USING GIN(metadata);

-- +goose StatementBegin
CREATE FUNCTION ops.river_job_state_in_bitmask(bitmask BIT(8), state ops.river_job_state)
RETURNS boolean
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT CASE state
        WHEN 'available' THEN get_bit(bitmask, 7)
        WHEN 'cancelled' THEN get_bit(bitmask, 6)
        WHEN 'completed' THEN get_bit(bitmask, 5)
        WHEN 'discarded' THEN get_bit(bitmask, 4)
        WHEN 'pending'   THEN get_bit(bitmask, 3)
        WHEN 'retryable' THEN get_bit(bitmask, 2)
        WHEN 'running'   THEN get_bit(bitmask, 1)
        WHEN 'scheduled' THEN get_bit(bitmask, 0)
        ELSE 0
    END = 1;
$$;
-- +goose StatementEnd

CREATE UNIQUE INDEX river_job_unique_idx ON ops.river_job (unique_key)
    WHERE unique_key IS NOT NULL
      AND unique_states IS NOT NULL
      AND ops.river_job_state_in_bitmask(unique_states, state);

CREATE UNLOGGED TABLE ops.river_leader (
  elected_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  leader_id text NOT NULL,
  name text PRIMARY KEY DEFAULT 'default',
  CONSTRAINT name_length CHECK (name = 'default'),
  CONSTRAINT leader_id_length CHECK (char_length(leader_id) > 0 AND char_length(leader_id) < 128)
);

CREATE TABLE ops.river_queue (
  name text PRIMARY KEY NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  metadata jsonb NOT NULL DEFAULT '{}' ::jsonb,
  paused_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE ops.river_notification (
  id bigserial PRIMARY KEY,
  created_at timestamptz NOT NULL DEFAULT now(),
  payload text NOT NULL,
  topic text NOT NULL,
  CONSTRAINT topic_length CHECK (length(topic) > 0 AND length(topic) < 128)
);

CREATE INDEX river_notification_created_at_idx ON ops.river_notification (created_at);
CREATE INDEX river_notification_topic_id_idx ON ops.river_notification (topic, id);

INSERT INTO ops.river_migration (line, version) VALUES
  ('main', 1), ('main', 2), ('main', 3), ('main', 4), ('main', 5), ('main', 6), ('main', 7);

-- GRANT (ADR 0001, «Roles de base de datos y permisos»). Los servicios que encolan (api, webhooks,
-- platform-api) solo hacen INSERT en river_job y usan su secuencia: el Enqueuer inserta con COPY y no lee
-- la fila de vuelta, así que no necesitan SELECT (y no ven los argumentos de los jobs de otros tenants,
-- porque river_job no tiene RLS). El worker corre, reintenta, limpia y elige líder: todo el DML.
-- Nadie más toca ops.river_migration; el migrador, dueño de las tablas, la mantiene.
GRANT INSERT ON ops.river_job TO wanka_app, wanka_webhooks, wanka_platform;
GRANT USAGE ON SEQUENCE ops.river_job_id_seq TO wanka_app, wanka_webhooks, wanka_platform;

GRANT SELECT, INSERT, UPDATE, DELETE ON ops.river_job, ops.river_leader, ops.river_queue, ops.river_notification TO wanka_worker;
GRANT USAGE, SELECT ON SEQUENCE ops.river_job_id_seq, ops.river_notification_id_seq TO wanka_worker;
