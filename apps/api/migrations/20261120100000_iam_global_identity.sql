-- +goose Up
-- Identidad global de la ola 1 (F1, historia #231; data-model §6 «DDL de identidad de la ola 1», ADR 0001 y 0004, notas
-- del 2026-11-19). Solo expande: ninguna migración anterior se edita. Corre como wanka_migrator, dueño de todo lo que crea,
-- en UNA transacción. Ninguna tabla de este archivo lleva tenant_id ni RLS (identidad global, como iam.users e iam.sessions),
-- así que la lista blanca de la prueba de pg_catalog no cambia. Los índices son de tablas nuevas y vacías (sin CONCURRENTLY);
-- iam.sessions no se toca. No rellena usuarios: ningún entorno tiene filas reales en iam.users, y con filas el
-- «email text NOT NULL» fallaría a propósito en vez de inventar correos.

-- Credenciales, bloqueos y segundo paso del usuario. Los nombres de las restricciones son fijos para que rls-check/mutations
-- pueda quitarlas una a una.
ALTER TABLE iam.users
  -- Siempre en minúsculas y único (Go lo normaliza con TrimSpace y ToLower antes de buscar y de guardar): sin citext.
  ADD COLUMN email text NOT NULL
    CONSTRAINT users_email_key UNIQUE
    CONSTRAINT users_email_format CHECK (email = lower(email) AND length(email) BETWEEN 3 AND 254),
  ADD COLUMN full_name text CONSTRAINT users_full_name_len CHECK (length(full_name) BETWEEN 1 AND 120),
  ADD COLUMN phone text CONSTRAINT users_phone_format CHECK (phone ~ '^\+?[0-9]{6,15}$'),
  ADD COLUMN password_hash text,   -- PHC de argon2id; NULL = no puede iniciar sesión hasta definirla
  ADD COLUMN email_verified boolean NOT NULL DEFAULT false,
  ADD COLUMN password_changed_at timestamptz,
  -- Intentos cobrados en la ventana fija de 15 min (incluidos los que están en vuelo); el cobro exacto está en el ADR 0004.
  ADD COLUMN failed_logins smallint NOT NULL DEFAULT 0 CONSTRAINT users_failed_logins_range CHECK (failed_logins BETWEEN 0 AND 5),
  ADD COLUMN login_window_started_at timestamptz,   -- inicio de la ventana: solo se mueve cuando la anterior venció
  -- Códigos malos del segundo factor en el restablecimiento de contraseña, acumulados; con 10 el restablecimiento con segundo factor queda cerrado.
  ADD COLUMN reset_mfa_failures smallint NOT NULL DEFAULT 0 CONSTRAINT users_reset_mfa_failures_range CHECK (reset_mfa_failures BETWEEN 0 AND 10),
  ADD COLUMN locked_until timestamptz,
  ADD COLUMN login_lockouts smallint NOT NULL DEFAULT 0 CONSTRAINT users_login_lockouts_range CHECK (login_lockouts BETWEEN 0 AND 10),
  -- Segundo paso (#25), compartido con POST /v1/auth/mfa/verify: el 5.º fallo bloquea y vuelve a 0 los intentos.
  ADD COLUMN mfa_failed_attempts smallint NOT NULL DEFAULT 0 CONSTRAINT users_mfa_failed_attempts_range CHECK (mfa_failed_attempts BETWEEN 0 AND 4),
  ADD COLUMN mfa_locked_until timestamptz,
  ADD COLUMN mfa_lockouts smallint NOT NULL DEFAULT 0 CONSTRAINT users_mfa_lockouts_range CHECK (mfa_lockouts BETWEEN 0 AND 3);

-- Formato de los códigos de recuperación fijado por el contrato (PR #230): cada elemento es «vN:» + HMAC-SHA256 en base64url
-- sin relleno (43 caracteres). Ningún código en claro, ningún NULL ni elemento vacío entra; el arreglo vacío es válido. Un CHECK
-- no admite subconsultas, de ahí la función. Sin SECURITY DEFINER; el CHECK exige EXECUTE al rol que escribe, por eso solo
-- wanka_app (el único que escribe iam.user_totp) lo recibe.
-- +goose StatementBegin
CREATE FUNCTION iam.valid_recovery_codes(codes text[]) RETURNS boolean
  LANGUAGE sql IMMUTABLE STRICT
  SET search_path = pg_catalog, pg_temp
AS $$
  SELECT NOT EXISTS (SELECT 1 FROM unnest(codes) AS c WHERE c IS NULL OR c !~ '^v[1-9][0-9]*:[A-Za-z0-9_-]{43}$')
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION iam.valid_recovery_codes(text[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION iam.valid_recovery_codes(text[]) TO wanka_app;

-- Segundo factor TOTP: a lo sumo una fila por usuario (la PK es la FK). confirmed_at NULL = sin 2FA activa.
CREATE TABLE iam.user_totp (
  user_id uuid PRIMARY KEY REFERENCES iam.users,
  secret_enc bytea NOT NULL,                  -- AES-GCM, clave en el entorno; nunca el secreto en claro
  confirmed_at timestamptz,
  last_used_step bigint,                      -- último paso TOTP aceptado: rechaza el código reutilizado
  -- Un elemento por código: «vN:» + HMAC-SHA256 con la clave derivada de la versión N de TOTP_ENC_KEY (ADR 0004, nota del
  -- 2026-11-19; er-01); se quita el usado. Arreglo vacío = sin códigos. El prefijo de versión es lo que permite rotar la clave.
  recovery_codes_hash text[] NOT NULL DEFAULT '{}' CONSTRAINT user_totp_recovery_codes_format CHECK (iam.valid_recovery_codes(recovery_codes_hash))
);
GRANT SELECT, INSERT, UPDATE ON iam.user_totp TO wanka_app;

-- Enrolamiento obligatorio del login con emailCode (#25, data-model §6): una fila por enrollmentToken. Pasó de #25 a esta
-- historia para que #24 y #26 la usen sin esperar al 2FA. Los intentos son del usuario (columnas mfa_* de arriba), no del token.
CREATE TABLE iam.mfa_enrollments (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES iam.users,
  token_hash bytea NOT NULL UNIQUE CONSTRAINT mfa_enrollments_token_hash_len CHECK (octet_length(token_hash) = 32),
  email_code_hmac bytea NOT NULL CONSTRAINT mfa_enrollments_code_hmac_len CHECK (octet_length(email_code_hmac) = 32),
  email_sends smallint NOT NULL DEFAULT 1 CONSTRAINT mfa_enrollments_sends_range CHECK (email_sends BETWEEN 1 AND 3),
  email_sent_at timestamptz NOT NULL DEFAULT now(),
  totp_secret_enc bytea,
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT mfa_enrollments_ttl CHECK (expires_at > created_at AND expires_at <= created_at + interval '10 minutes'),
  CONSTRAINT mfa_enrollments_consumed_after CHECK (consumed_at IS NULL OR consumed_at >= created_at)
);
CREATE INDEX mfa_enrollments_user_created_idx ON iam.mfa_enrollments (user_id, created_at);  -- FK y tope de correos por hora
CREATE INDEX mfa_enrollments_expires_at_idx ON iam.mfa_enrollments (expires_at);             -- limpieza (data-model §5)

-- Equivalente de iam.guard_session_writes() (revisión del PR #203): los GRANT por columna no bastan, porque el INSERT es de
-- fila completa y consumed_at está en el UPDATE. Sin SECURITY DEFINER (corre con el rol que escribe); solo un superusuario
-- (las semillas de prueba) lo salta. Todo rechazo es 42501.
-- +goose StatementBegin
CREATE FUNCTION iam.guard_mfa_enrollment_writes() RETURNS trigger
  LANGUAGE plpgsql
  SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
  IF coalesce((SELECT r.rolsuper FROM pg_catalog.pg_roles r WHERE r.rolname = current_user), false) THEN
    RETURN NEW;
  END IF;
  IF TG_OP = 'INSERT' THEN
    NEW.created_at := pg_catalog.now();      -- la fija el servidor: el CHECK del TTL se mide contra el reloj real
    NEW.email_sent_at := pg_catalog.now();
    NEW.email_sends := 1;
    NEW.consumed_at := NULL;
    RETURN NEW;
  END IF;
  IF OLD.consumed_at IS NOT NULL THEN
    -- Un token consumido no se reabre ni cambia; consumir otra vez es un no-op que conserva el instante viejo.
    IF NEW.consumed_at IS NULL THEN
      RAISE EXCEPTION 'a consumed enrollment stays consumed' USING ERRCODE = 'insufficient_privilege';
    END IF;
    NEW.consumed_at := OLD.consumed_at;
    IF pg_catalog.to_jsonb(NEW) <> pg_catalog.to_jsonb(OLD) THEN
      RAISE EXCEPTION 'a consumed enrollment cannot change' USING ERRCODE = 'insufficient_privilege';
    END IF;
    RETURN NEW;
  END IF;
  IF NEW.consumed_at IS NOT NULL THEN
    NEW.consumed_at := pg_catalog.now();     -- lo fija el servidor
  END IF;
  IF NEW.email_sends < OLD.email_sends THEN
    RAISE EXCEPTION 'email_sends never decreases' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF OLD.totp_secret_enc IS NOT NULL AND NEW.totp_secret_enc IS DISTINCT FROM OLD.totp_secret_enc THEN
    RAISE EXCEPTION 'the pending TOTP secret does not rotate' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF NEW.email_code_hmac IS DISTINCT FROM OLD.email_code_hmac AND NEW.email_sends <> OLD.email_sends + 1 THEN
    RAISE EXCEPTION 'a new email code is a new send' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF NEW.email_sends > OLD.email_sends THEN
    NEW.email_sent_at := pg_catalog.now();   -- los 60 s entre envíos no dependen de lo que escriba la app
  ELSE
    NEW.email_sent_at := OLD.email_sent_at;
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER mfa_enrollments_guard_writes BEFORE INSERT OR UPDATE ON iam.mfa_enrollments
  FOR EACH ROW EXECUTE FUNCTION iam.guard_mfa_enrollment_writes();

-- Tokens de un solo uso enviados por correo (reemplaza a iam.password_resets del diseño original). Sin FK de membership_id
-- ni tenant_id, como iam.sessions: el tenant se lee de la membresía con own_memberships. token_hash es el sha256 del token
-- completo con su prefijo (wk_pr_, wk_iv_ o wk_ec_).
CREATE TABLE iam.user_tokens (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES iam.users,
  purpose text NOT NULL CONSTRAINT user_tokens_purpose CHECK (purpose IN ('password_reset', 'invitation', 'email_change')),
  token_hash bytea NOT NULL UNIQUE CONSTRAINT user_tokens_token_hash_len CHECK (octet_length(token_hash) = 32),
  membership_id uuid,   -- solo y obligatorio en invitation
  new_email text,       -- solo y obligatorio en email_change; mismo formato que iam.users.email
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT user_tokens_membership_iff_invitation CHECK ((membership_id IS NOT NULL) = (purpose = 'invitation')),
  CONSTRAINT user_tokens_new_email_iff_email_change CHECK ((new_email IS NOT NULL) = (purpose = 'email_change')),
  CONSTRAINT user_tokens_new_email_format CHECK (new_email = lower(new_email) AND length(new_email) BETWEEN 3 AND 254),
  -- Vigencia máxima por propósito: 15 min, 7 días y 24 h (en horas, para no depender de la zona horaria de la sesión).
  CONSTRAINT user_tokens_ttl CHECK (expires_at > created_at AND expires_at <= created_at + CASE purpose
    WHEN 'password_reset' THEN interval '15 minutes' WHEN 'invitation' THEN interval '168 hours' ELSE interval '24 hours' END),
  CONSTRAINT user_tokens_consumed_after CHECK (consumed_at IS NULL OR consumed_at >= created_at)
);
CREATE INDEX user_tokens_user_purpose_created_idx ON iam.user_tokens (user_id, purpose, created_at);  -- FK, topes y «el nuevo invalida al anterior»
CREATE INDEX user_tokens_expires_at_idx ON iam.user_tokens (expires_at);                              -- limpieza (data-model §5)

-- Como iam.guard_mfa_enrollment_writes(): created_at lo fija el servidor y consumed_at es irreversible. Volver a consumir es
-- un no-op que conserva el instante viejo, así que el consumo del flujo es siempre condicional
-- (UPDATE … WHERE consumed_at IS NULL AND expires_at > now() RETURNING …, ADR 0001 nota del 2026-11-19). Todo rechazo es 42501.
-- +goose StatementBegin
CREATE FUNCTION iam.guard_user_token_writes() RETURNS trigger
  LANGUAGE plpgsql
  SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
  IF coalesce((SELECT r.rolsuper FROM pg_catalog.pg_roles r WHERE r.rolname = current_user), false) THEN
    RETURN NEW;
  END IF;
  IF TG_OP = 'INSERT' THEN
    NEW.created_at := pg_catalog.now();      -- la fija el servidor: el CHECK del TTL se mide contra el reloj real
    NEW.consumed_at := NULL;
    RETURN NEW;
  END IF;
  IF OLD.consumed_at IS NOT NULL THEN
    IF NEW.consumed_at IS NULL THEN
      RAISE EXCEPTION 'a consumed token stays consumed' USING ERRCODE = 'insufficient_privilege';
    END IF;
    NEW.consumed_at := OLD.consumed_at;
  ELSIF NEW.consumed_at IS NOT NULL THEN
    NEW.consumed_at := pg_catalog.now();     -- lo fija el servidor
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER user_tokens_guard_writes BEFORE INSERT OR UPDATE ON iam.user_tokens
  FOR EACH ROW EXECUTE FUNCTION iam.guard_user_token_writes();

-- GRANT del ADR 0001 (matriz y nota del 2026-11-19), y nada más. wanka_webhooks, wanka_platform y wanka_definer no reciben nada.
-- Enrolamiento: la app no mueve user_id, token_hash, expires_at ni created_at, ni borra; el worker solo limpia.
GRANT SELECT, INSERT ON iam.mfa_enrollments TO wanka_app;
GRANT UPDATE (email_code_hmac, email_sends, email_sent_at, totp_secret_enc, consumed_at) ON iam.mfa_enrollments TO wanka_app;
GRANT DELETE ON iam.mfa_enrollments TO wanka_worker;
GRANT SELECT (id, expires_at) ON iam.mfa_enrollments TO wanka_worker;
-- Tokens de un uso: la app solo consume (UPDATE de consumed_at); el worker borra por expires_at sin ver nunca el hash.
GRANT SELECT, INSERT ON iam.user_tokens TO wanka_app;
GRANT UPDATE (consumed_at) ON iam.user_tokens TO wanka_app;
GRANT DELETE ON iam.user_tokens TO wanka_worker;
GRANT SELECT (id, expires_at) ON iam.user_tokens TO wanka_worker;
