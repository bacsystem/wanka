-- Verificacion de ADR 0001 y ADR 0004: esquema reducido con las mismas politicas, FORCE RLS, roles y funciones SECURITY DEFINER.
-- Ejecutado como wanka_migrator (NOBYPASSRLS), DESPUES de las migraciones reales del kernel (run.sh las aplica antes:
-- esquemas, app.current_tenant(), org.tenants, iam.users, iam.roles, iam.memberships, iam.sessions, iam.session_lookup
-- y sus GRANT; issue #195). Aqui queda solo lo que esas migraciones no crean todavia: lo que agregan identidad (#24, #27),
-- empresas (#31) y plataforma (ola 6), y las ayudas de prueba. Asi la verificacion no puede diferir de la migracion real.
\set ON_ERROR_STOP on
CREATE SCHEMA platform_fn;

-- Columnas que la migracion del kernel deja a identidad y a empresas (expandir, data-model §6); las usan las funciones de abajo.
ALTER TABLE org.tenants ADD COLUMN ruc text UNIQUE NOT NULL, ADD COLUMN name text NOT NULL;
ALTER TABLE iam.users ADD COLUMN email text UNIQUE NOT NULL;
-- Enrolamiento obligatorio del login con emailCode (#25, data-model §6): contador de segundo paso del usuario y una fila por
-- enrollmentToken. Sin tenant_id, asi que sin RLS (identidad global, como iam.users e iam.sessions).
ALTER TABLE iam.users ADD COLUMN mfa_failed_attempts smallint NOT NULL DEFAULT 0
    CONSTRAINT users_mfa_failed_attempts_range CHECK (mfa_failed_attempts BETWEEN 0 AND 4),
  ADD COLUMN mfa_locked_until timestamptz,
  ADD COLUMN mfa_lockouts smallint NOT NULL DEFAULT 0
    CONSTRAINT users_mfa_lockouts_range CHECK (mfa_lockouts BETWEEN 0 AND 3);  -- 3: segundo paso cerrado hasta restablecer la contrasena
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
-- Equivalente de iam.guard_session_writes() (revision del PR #203): los GRANT por columna no bastan, porque el INSERT es
-- de fila completa y consumed_at esta en el UPDATE. Sin SECURITY DEFINER (corre con el rol que escribe); solo un
-- superusuario (las semillas de prueba) lo salta. Todo rechazo es 42501.
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
    NEW.email_sent_at := pg_catalog.now();   -- los 60 s entre envios no dependen de lo que escriba la app
  ELSE
    NEW.email_sent_at := OLD.email_sent_at;
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER mfa_enrollments_guard_writes BEFORE INSERT OR UPDATE ON iam.mfa_enrollments
  FOR EACH ROW EXECUTE FUNCTION iam.guard_mfa_enrollment_writes();

CREATE TABLE org.sites (id uuid PRIMARY KEY, tenant_id uuid NOT NULL REFERENCES org.tenants, name text NOT NULL, UNIQUE (tenant_id, id));
CREATE TABLE ops.webhook_routes (route_id text PRIMARY KEY, tenant_id uuid NOT NULL, provider text NOT NULL, webhook_secret_enc bytea NOT NULL, revoked_at timestamptz);
CREATE TABLE platform.impersonation_grants (id uuid PRIMARY KEY, platform_user_id uuid NOT NULL, tenant_id uuid NOT NULL, ticket_id uuid NOT NULL,
  scope text NOT NULL, reason text, status text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), approved_by uuid, approved_at timestamptz,
  consent_expires_at timestamptz, exchange_token_hash bytea UNIQUE, redeemed_at timestamptz, revoked_at timestamptz);
CREATE TABLE platform.audit_events (id bigserial PRIMARY KEY, platform_user_id uuid, target_tenant_id uuid, action text NOT NULL);

-- RLS habilitado y FORZADO en las demas tablas de tenant (ADR 0001); iam.roles, iam.memberships, org.tenants y audit.events
-- ya lo traen de la migracion del kernel, con su tenant_isolation.
DO $$ DECLARE t text; BEGIN
  FOREACH t IN ARRAY ARRAY['org.sites'] LOOP
    EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %s USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant())', t);
  END LOOP; END $$;
-- Solo las membresias invitado/activo dan lectura sin tenant (ADR 0001, issue #187): una suspendida no ve ni su empresa.
CREATE POLICY member_read ON org.tenants FOR SELECT USING (app.current_tenant() IS NULL AND id IN
  (SELECT tenant_id FROM iam.memberships WHERE user_id = app.current_user_id() AND status IN ('invitado','activo')));
CREATE POLICY platform_rw ON org.tenants TO wanka_platform USING (true) WITH CHECK (true);
CREATE POLICY own_memberships ON iam.memberships FOR SELECT USING (app.current_tenant() IS NULL AND user_id = app.current_user_id()
  AND status IN ('invitado','activo'));
CREATE POLICY member_read ON iam.roles FOR SELECT USING (app.current_tenant() IS NULL AND id IN
  (SELECT role_id FROM iam.memberships WHERE user_id = app.current_user_id() AND status IN ('invitado','activo')));
CREATE POLICY member_read ON org.sites FOR SELECT USING (app.current_tenant() IS NULL AND EXISTS
  (SELECT 1 FROM iam.memberships m WHERE m.user_id = app.current_user_id() AND m.status = 'activo' AND m.tenant_id = sites.tenant_id AND m.all_sites));

-- Vista security_invoker (no es SECURITY DEFINER): corre con los permisos y RLS de quien llama
CREATE VIEW iam.list_my_memberships WITH (security_invoker = true) AS
  SELECT t.name AS tenant, r.name AS role, r.requires_mfa, s.name AS site, m.status
    FROM iam.memberships m JOIN org.tenants t ON t.id = m.tenant_id JOIN iam.roles r ON r.id = m.role_id
    LEFT JOIN org.sites s ON s.tenant_id = m.tenant_id AND m.all_sites;

-- ===== Funciones SECURITY DEFINER: dueño wanka_definer (NOLOGIN, NOBYPASSRLS, no es dueño de tablas) =====
-- Patrón para las que tocan tablas con RLS sin tenant fijado por el llamador: guardan el valor del llamador,
-- fijan el tenant con set_config(..., true) y lo RESTAURAN antes de salir. Si la función falla, la transacción
-- (o el savepoint) se revierte y con ella el set_config, así que tampoco se filtra.
-- (La cláusula SET app.tenant_id = '' en CREATE FUNCTION no sirve: un rol no superusuario recibe
--  'permission denied to set parameter app.tenant_id' en PG 16.)

-- iam.session_lookup(bytea) la crea la migracion del kernel (ADR 0004); las mutaciones de ./mutations la reemplazan.

CREATE FUNCTION ops.resolve_webhook_route(p_route_id text)
  RETURNS TABLE (tenant_id uuid, webhook_secret_enc bytea)
  LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp
AS $$ SELECT r.tenant_id, r.webhook_secret_enc FROM ops.webhook_routes r WHERE r.route_id = p_route_id AND r.revoked_at IS NULL $$;

CREATE FUNCTION iam.list_support_requests()
  RETURNS TABLE (id uuid, ticket_id uuid, scope text, status text, created_at timestamptz)
  LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp
AS $$ SELECT g.id, g.ticket_id, g.scope, g.status, g.created_at FROM platform.impersonation_grants g
      WHERE app.current_tenant() IS NOT NULL AND g.tenant_id = app.current_tenant() ORDER BY g.created_at DESC LIMIT 200 $$;

-- Llamada dentro de InTenantTx (tenant del llamador fijado): no necesita fijar tenant; falla cerrada por session_kind.
CREATE FUNCTION iam.decide_impersonation(p_grant_id uuid, p_approve boolean)
  RETURNS text LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp
AS $$
DECLARE n int;
BEGIN
  IF NULLIF(current_setting('app.session_kind', true), '') IS DISTINCT FROM 'user'
     OR NULLIF(current_setting('app.impersonator_id', true), '') IS NOT NULL THEN
    RAISE EXCEPTION 'decide_impersonation: session kind must be user' USING ERRCODE = 'insufficient_privilege';
  END IF;
  UPDATE platform.impersonation_grants g
     SET status = CASE WHEN p_approve THEN 'aprobado' ELSE 'rechazado' END,
         approved_by = app.current_user_id(), approved_at = now(), consent_expires_at = now() + interval '24 hours'
   WHERE g.id = p_grant_id AND g.status = 'solicitado' AND g.tenant_id = app.current_tenant();
  GET DIAGNOSTICS n = ROW_COUNT;
  IF n = 0 THEN RAISE EXCEPTION 'grant not found for this tenant' USING ERRCODE = 'no_data_found'; END IF;
  INSERT INTO audit.events (tenant_id, actor_type, actor_user_id, action) VALUES (app.current_tenant(), 'user', app.current_user_id(), 'support_access:' || CASE WHEN p_approve THEN 'approve' ELSE 'reject' END);
  RETURN CASE WHEN p_approve THEN 'aprobado' ELSE 'rechazado' END;
END $$;

-- Canje: sin tenant fijado por el llamador; fija el del grant internamente (patrón SET).
CREATE FUNCTION iam.redeem_impersonation(p_token_hash bytea, p_session_id uuid, p_access_hash bytea)
  RETURNS uuid LANGUAGE plpgsql SECURITY DEFINER
  SET search_path = pg_catalog, pg_temp
AS $$
DECLARE g platform.impersonation_grants%ROWTYPE; prev text := current_setting('app.tenant_id', true);
BEGIN
  SELECT * INTO g FROM platform.impersonation_grants x WHERE x.exchange_token_hash = p_token_hash FOR UPDATE;
  IF NOT FOUND OR g.status <> 'aprobado' OR g.consent_expires_at <= now() OR g.revoked_at IS NOT NULL THEN
    RAISE EXCEPTION 'grant not redeemable' USING ERRCODE = 'insufficient_privilege';
  END IF;
  UPDATE platform.impersonation_grants SET status = 'canjeado', redeemed_at = now() WHERE id = g.id;
  INSERT INTO iam.sessions (id, user_id, tenant_id, membership_id, access_hash, access_expires_at, authz_version, mfa_satisfied, mfa_method, impersonator_id)
       VALUES (p_session_id, NULL, g.tenant_id, NULL, p_access_hash, now() + interval '60 minutes', NULL, true, 'platform', g.platform_user_id);
  PERFORM set_config('app.tenant_id', g.tenant_id::text, true);
  INSERT INTO audit.events (tenant_id, actor_type, impersonator_id, action) VALUES (g.tenant_id, 'platform_staff', g.platform_user_id, 'impersonation:redeem');
  PERFORM set_config('app.tenant_id', coalesce(prev, ''), true);
  RETURN g.tenant_id;
END $$;

CREATE FUNCTION platform_fn.provision_tenant(p_tenant uuid, p_ruc text, p_name text, p_owner uuid, p_owner_email text, p_role uuid, p_site uuid)
  RETURNS uuid LANGUAGE plpgsql SECURITY DEFINER
  SET search_path = pg_catalog, pg_temp
AS $$
DECLARE prev text := current_setting('app.tenant_id', true);
BEGIN
  PERFORM set_config('app.tenant_id', p_tenant::text, true);
  INSERT INTO org.tenants (id, ruc, name) VALUES (p_tenant, p_ruc, p_name);
  INSERT INTO org.sites (id, tenant_id, name) VALUES (p_site, p_tenant, 'Sede principal');
  INSERT INTO iam.roles (id, tenant_id, name, requires_mfa) VALUES (p_role, p_tenant, 'Propietario', true);
  INSERT INTO iam.users (id, email) VALUES (p_owner, p_owner_email) ON CONFLICT (email) DO NOTHING;
  INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status) VALUES (gen_random_uuid(), p_tenant, p_owner, p_role, 'invitado');
  INSERT INTO audit.events (tenant_id, actor_type, impersonator_id, action) VALUES (p_tenant, 'platform_staff', app.current_user_id(), 'tenants:create');
  PERFORM set_config('app.tenant_id', coalesce(prev, ''), true);
  RETURN p_tenant;
END $$;

CREATE FUNCTION platform_fn.request_impersonation(p_grant uuid, p_staff uuid, p_tenant uuid, p_ticket uuid, p_scope text, p_token_hash bytea)
  RETURNS uuid LANGUAGE plpgsql SECURITY DEFINER
  SET search_path = pg_catalog, pg_temp
AS $$
DECLARE prev text := current_setting('app.tenant_id', true);
BEGIN
  INSERT INTO platform.impersonation_grants (id, platform_user_id, tenant_id, ticket_id, scope, status, exchange_token_hash)
       VALUES (p_grant, p_staff, p_tenant, p_ticket, p_scope, 'solicitado', p_token_hash);
  PERFORM set_config('app.tenant_id', p_tenant::text, true);
  INSERT INTO audit.events (tenant_id, actor_type, impersonator_id, action) VALUES (p_tenant, 'platform_staff', p_staff, 'impersonation:request');
  PERFORM set_config('app.tenant_id', coalesce(prev, ''), true);
  RETURN p_grant;
END $$;

CREATE FUNCTION platform_fn.suspend_tenant(p_tenant uuid)
  RETURNS void LANGUAGE plpgsql SECURITY DEFINER
  SET search_path = pg_catalog, pg_temp
AS $$
DECLARE prev text := current_setting('app.tenant_id', true);
BEGIN
  PERFORM set_config('app.tenant_id', p_tenant::text, true);
  UPDATE org.tenants SET status = 'suspendida' WHERE id = p_tenant;
  IF NOT FOUND THEN RAISE EXCEPTION 'tenant not found'; END IF;
  INSERT INTO audit.events (tenant_id, actor_type, impersonator_id, action) VALUES (p_tenant, 'platform_staff', app.current_user_id(), 'tenants:suspend');
  PERFORM set_config('app.tenant_id', coalesce(prev, ''), true);
END $$;

CREATE FUNCTION platform_fn.purge_closed_tenant(p_tenant uuid)
  RETURNS int LANGUAGE plpgsql SECURITY DEFINER
  SET search_path = pg_catalog, pg_temp
AS $$
DECLARE n int; prev text := current_setting('app.tenant_id', true);
BEGIN
  PERFORM set_config('app.tenant_id', p_tenant::text, true);
  DELETE FROM org.sites WHERE tenant_id = p_tenant;
  GET DIAGNOSTICS n = ROW_COUNT;
  PERFORM set_config('app.tenant_id', coalesce(prev, ''), true);
  RETURN n;
END $$;

-- Dueño explícito de cada función. ALTER ... OWNER exige que el nuevo dueño tenga CREATE en el esquema:
-- se concede solo para el cambio de dueño y se revoca enseguida, en UNA transacción (si algo falla, el
-- GRANT CREATE no queda aplicado). Por eso esta migración nunca usa `-- +goose NO TRANSACTION`.
BEGIN;
GRANT CREATE ON SCHEMA iam, ops, platform_fn TO wanka_definer;  -- como en la migracion del kernel: solo para el cambio de dueño
DO $$ DECLARE f text; BEGIN
  FOREACH f IN ARRAY ARRAY['ops.resolve_webhook_route(text)','iam.list_support_requests()',
     'iam.decide_impersonation(uuid,boolean)','iam.redeem_impersonation(bytea,uuid,bytea)',
     'platform_fn.provision_tenant(uuid,text,text,uuid,text,uuid,uuid)','platform_fn.request_impersonation(uuid,uuid,uuid,uuid,text,bytea)',
     'platform_fn.suspend_tenant(uuid)','platform_fn.purge_closed_tenant(uuid)'] LOOP
    EXECUTE format('ALTER FUNCTION %s OWNER TO wanka_definer', f);
    EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC', f);
  END LOOP; END $$;
REVOKE CREATE ON SCHEMA iam, ops, platform_fn FROM wanka_definer;
COMMIT;
-- app: lo da esta referencia y NO la migracion del kernel. Las funciones de plataforma (ola 6, aun no migradas) llaman a
-- app.current_tenant() en su cuerpo y necesitan USAGE; su migracion tendra que concederlo. iam.session_lookup no lo necesita
-- (la politica RLS se evalua sin el), y el test de la mutacion que lo quitaba lo confirmo. iam y org: migracion del kernel.
GRANT USAGE ON SCHEMA app, audit, ops, platform TO wanka_definer;
GRANT EXECUTE ON FUNCTION app.current_tenant(), app.current_user_id() TO wanka_definer, wanka_app, wanka_platform, wanka_worker, wanka_webhooks;
-- Privilegios de escritura de wanka_definer que necesitan las funciones de plataforma (ola 6); la migracion del kernel
-- le da solo SELECT en iam.sessions e iam.memberships, lo que necesita iam.session_lookup.
GRANT INSERT ON iam.sessions TO wanka_definer;
GRANT INSERT ON iam.memberships, iam.users, iam.roles TO wanka_definer;
GRANT SELECT ON iam.users, iam.roles TO wanka_definer;
GRANT SELECT, INSERT, UPDATE ON org.tenants TO wanka_definer;
GRANT SELECT, INSERT, DELETE ON org.sites TO wanka_definer;
GRANT INSERT ON audit.events TO wanka_definer;   -- el esquema audit y sus GRANT para app y worker: migracion del kernel (#173)
GRANT SELECT ON ops.webhook_routes TO wanka_definer;
GRANT SELECT, INSERT, UPDATE ON platform.impersonation_grants TO wanka_definer;

-- Permisos de los roles de aplicación (extracto de la matriz del ADR 0001)
-- app, audit, iam y org, y los GRANT de iam.*, org.tenants y audit.events: migracion del kernel
GRANT SELECT, INSERT, UPDATE ON org.sites TO wanka_app;
GRANT SELECT ON iam.list_my_memberships TO wanka_app;
-- iam.mfa_enrollments (#25): la app no mueve user_id, token_hash, expires_at ni created_at, ni borra; el worker solo limpia.
GRANT SELECT, INSERT ON iam.mfa_enrollments TO wanka_app;
GRANT UPDATE (email_code_hmac, email_sends, email_sent_at, totp_secret_enc, consumed_at) ON iam.mfa_enrollments TO wanka_app;
GRANT DELETE ON iam.mfa_enrollments TO wanka_worker;
GRANT SELECT (id, expires_at) ON iam.mfa_enrollments TO wanka_worker;
GRANT EXECUTE ON FUNCTION iam.list_support_requests(), iam.decide_impersonation(uuid,boolean),
  iam.redeem_impersonation(bytea,uuid,bytea) TO wanka_app;
GRANT USAGE ON SCHEMA ops TO wanka_webhooks;
GRANT EXECUTE ON FUNCTION ops.resolve_webhook_route(text) TO wanka_webhooks;
GRANT USAGE ON SCHEMA platform_fn, org TO wanka_platform;
GRANT EXECUTE ON FUNCTION platform_fn.provision_tenant(uuid,text,text,uuid,text,uuid,uuid), platform_fn.request_impersonation(uuid,uuid,uuid,uuid,text,bytea),
  platform_fn.suspend_tenant(uuid) TO wanka_platform;
GRANT USAGE ON SCHEMA platform_fn TO wanka_worker;
GRANT EXECUTE ON FUNCTION platform_fn.purge_closed_tenant(uuid) TO wanka_worker;

-- wanka_platform lee y suspende tenants por la politica platform_rw
GRANT SELECT, UPDATE ON org.tenants TO wanka_platform;

-- ===== Ayudas de prueba (solo en esta verificación; no forman parte de la migración real) =====
CREATE SCHEMA test;
CREATE FUNCTION test.assert(p_ok boolean, p_msg text) RETURNS text LANGUAGE plpgsql AS $$
BEGIN
  IF p_ok IS NOT TRUE THEN RAISE EXCEPTION 'ASSERT FAILED: %', p_msg; END IF;
  RETURN 'ok: ' || p_msg;
END $$;
-- Ejecuta p_sql con los privilegios de quien llama y exige que falle con p_sqlstate (sin efectos: subtransacción).
CREATE FUNCTION test.expect_error(p_sql text, p_sqlstate text, p_msg text) RETURNS text LANGUAGE plpgsql AS $$
DECLARE failed boolean := false; got text;
BEGIN
  BEGIN
    EXECUTE p_sql;
  EXCEPTION WHEN OTHERS THEN
    failed := true; got := SQLSTATE;
  END;
  IF NOT failed THEN RAISE EXCEPTION 'ASSERT FAILED: % (se esperaba error %, no fallo)', p_msg, p_sqlstate; END IF;
  IF got <> p_sqlstate THEN RAISE EXCEPTION 'ASSERT FAILED: % (se esperaba %, llego %)', p_msg, p_sqlstate, got; END IF;
  RETURN 'ok: ' || p_msg || ' -> ' || got;
END $$;
-- Tablas de River (ADR 0001, matriz de GRANT; apps/api/migrations/20261119143700_kernel_river_jobs.sql). Los tres servicios que
-- encolan (wanka_app, wanka_webhooks, wanka_platform) solo insertan en ops.river_job y no leen nada de la cola: la tabla no
-- tiene RLS y sus argumentos son de todos los tenants. Se llama como el rol que se prueba; el INSERT se deshace.
CREATE FUNCTION test.river_enqueue_only() RETURNS text LANGUAGE plpgsql AS $fn$
BEGIN
  BEGIN
    INSERT INTO ops.river_job (kind, args, queue) VALUES ('rls.probe', '{}', 'maintenance');
    RAISE EXCEPTION 'rollback' USING ERRCODE = 'P0001';
  EXCEPTION WHEN SQLSTATE 'P0001' THEN NULL;
  END;
  PERFORM test.expect_error('SELECT args FROM ops.river_job', '42501', current_user || ' no lee ops.river_job');
  PERFORM test.expect_error('SELECT count(*) FROM ops.river_job', '42501', current_user || ' no cuenta ops.river_job');
  PERFORM test.expect_error($$UPDATE ops.river_job SET queue = 'cpe'$$, '42501', current_user || ' no actualiza ops.river_job');
  PERFORM test.expect_error('DELETE FROM ops.river_job', '42501', current_user || ' no borra ops.river_job');
  PERFORM test.expect_error('SELECT * FROM ops.river_leader', '42501', current_user || ' no lee river_leader');
  PERFORM test.expect_error($$INSERT INTO ops.river_queue (name) VALUES ('x')$$, '42501', current_user || ' no escribe river_queue');
  PERFORM test.expect_error('SELECT * FROM ops.river_migration', '42501', current_user || ' no lee river_migration');
  RETURN 'ok: ' || current_user || ' solo encola';
END $fn$;
-- El worker corre, reintenta, limpia y elige lider: todo el DML de las tablas de River, pero nunca river_migration.
CREATE FUNCTION test.river_worker_full() RETURNS text LANGUAGE plpgsql AS $fn$
DECLARE n int;
BEGIN
  BEGIN
    INSERT INTO ops.river_job (kind, args, queue) VALUES ('rls.probe', '{}', 'maintenance');
    PERFORM test.assert((SELECT count(*) FROM ops.river_job WHERE kind = 'rls.probe') = 1, 'wanka_worker lee lo que encola');
    UPDATE ops.river_job SET state = 'completed', finalized_at = now() WHERE kind = 'rls.probe';
    DELETE FROM ops.river_job WHERE kind = 'rls.probe' AND state = 'completed'; GET DIAGNOSTICS n = ROW_COUNT;
    PERFORM test.assert(n = 1, 'wanka_worker actualiza y borra jobs (limpieza de terminados)');
    INSERT INTO ops.river_leader (elected_at, expires_at, leader_id) VALUES (now(), now() + interval '10 seconds', 'rls-probe');
    UPDATE ops.river_leader SET expires_at = now() + interval '20 seconds';
    DELETE FROM ops.river_leader;
    INSERT INTO ops.river_queue (name) VALUES ('rls-probe');
    UPDATE ops.river_queue SET paused_at = now() WHERE name = 'rls-probe';
    DELETE FROM ops.river_queue WHERE name = 'rls-probe';
    INSERT INTO ops.river_notification (payload, topic) VALUES ('{}', 'rls-probe');
    DELETE FROM ops.river_notification WHERE topic = 'rls-probe';
    RAISE EXCEPTION 'rollback' USING ERRCODE = 'P0001';
  EXCEPTION WHEN SQLSTATE 'P0001' THEN NULL;
  END;
  PERFORM test.expect_error('SELECT * FROM ops.river_migration', '42501', 'wanka_worker no lee river_migration');
  PERFORM test.expect_error($$DELETE FROM ops.river_migration$$, '42501', 'wanka_worker no toca river_migration');
  RETURN 'ok: wanka_worker con todo el DML de la cola';
END $fn$;
GRANT USAGE ON SCHEMA test TO wanka_app, wanka_platform, wanka_webhooks, wanka_worker;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA test TO wanka_app, wanka_platform, wanka_webhooks, wanka_worker;

-- ===== Bootstrap fuera de goose (ADR 0001, «Creación de los roles») =====
\echo '== M1. wanka_migrator no puede crear roles: los crea el bootstrap del superusuario, no la migracion =='
SELECT test.expect_error('CREATE ROLE wanka_intruso LOGIN', '42501', 'wanka_migrator sin CREATEROLE');
