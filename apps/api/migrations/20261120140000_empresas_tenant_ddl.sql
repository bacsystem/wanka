-- +goose Up
-- Empresas de la ola 1, F3: DDL de empresas (issue #265, épica #4; ADR 0001, ADR 0005, ADR 0013; er-01).
-- Corre como wanka_migrator, dueño de todo lo que crea, y solo hacia adelante (expandir).
-- 1. Políticas con rama por usuario pasan TO wanka_app (decisión #246, ADR 0001 nota del 2026-11-20).
-- 2. org.tenants: edición de trade_name por wanka_app (#32).
-- 3. org.sites: ampliación ADR 0013 (#33), anexo SUNAT, dirección, versión y trigger de entitlements.
-- 4. org.cash_registers: claves compuestas (tenant_id, id) y (tenant_id, site_id, id) para FK de series, timestamps y grants.
-- 5. org.tenant_settings: configuración fiscal y comercial (#32), versión y trigger de entitlements.
-- 6. org.document_series: series de comprobantes y documentos internos (#33, #47), trigger guard_series_conflict y FK compuesta.
-- 7. einv.company_sync_state: estado y arrendamiento de sincronización con Khipu (ADR 0013).

-- ===== 1. Políticas con rama por usuario TO wanka_app =====
-- Las seis políticas de la lista blanca con rama por usuario pasan TO wanka_app.
-- Permite a wanka_worker leer org.sites y org.cash_registers solo con tenant_isolation
-- sin necesidad de SELECT ON iam.membership_sites.
ALTER POLICY own_memberships ON iam.memberships TO wanka_app;
ALTER POLICY own_memberships ON iam.membership_sites TO wanka_app;
ALTER POLICY member_read ON org.tenants TO wanka_app;
ALTER POLICY member_read ON iam.roles TO wanka_app;
ALTER POLICY member_read ON org.sites TO wanka_app;
ALTER POLICY member_read ON org.cash_registers TO wanka_app;

-- ===== 2. org.tenants: edición de nombre comercial (#32) =====
GRANT UPDATE (trade_name, updated_at) ON org.tenants TO wanka_app;

-- ===== 3. org.sites: ampliación (#33, ADR 0013) =====
ALTER TABLE org.sites
  ADD COLUMN sunat_annex text CONSTRAINT sites_sunat_annex_format CHECK (sunat_annex ~ '^[0-9]{4}$'),
  ADD COLUMN address text CONSTRAINT sites_address_len CHECK (char_length(btrim(address)) BETWEEN 1 AND 300),
  ADD COLUMN version bigint NOT NULL DEFAULT 1 CONSTRAINT sites_version_positive CHECK (version > 0),
  ADD COLUMN provider_synced_version bigint CONSTRAINT sites_synced_version_non_neg CHECK (provider_synced_version >= 0);

CREATE UNIQUE INDEX sites_tenant_annex_key
  ON org.sites (tenant_id, sunat_annex)
  WHERE sunat_annex IS NOT NULL;

GRANT INSERT (sunat_annex, address, version) ON org.sites TO wanka_app;
GRANT UPDATE (sunat_annex, address, version) ON org.sites TO wanka_app;
GRANT SELECT ON org.sites TO wanka_worker;
GRANT UPDATE (provider_synced_version) ON org.sites TO wanka_worker;

CREATE TRIGGER sites_entitlements_notify
  AFTER INSERT OR UPDATE OF active OR DELETE ON org.sites
  FOR EACH ROW EXECUTE FUNCTION platform.notify_tenant_entitlements('tenant_id');

-- ===== 4. org.cash_registers: claves compuestas y timestamps =====
ALTER TABLE org.cash_registers
  ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(),
  ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
  ADD CONSTRAINT cash_registers_tenant_id_key UNIQUE (tenant_id, id),
  ADD CONSTRAINT cash_registers_tenant_site_id_key UNIQUE (tenant_id, site_id, id);

GRANT UPDATE (updated_at) ON org.cash_registers TO wanka_app;
GRANT SELECT ON org.cash_registers TO wanka_worker;

-- ===== 5. org.tenant_settings: configuración de empresa (#32) =====
CREATE TABLE org.tenant_settings (
  tenant_id uuid PRIMARY KEY REFERENCES org.tenants,
  tax_regime text CONSTRAINT tenant_settings_tax_regime_values
    CHECK (tax_regime IS NULL OR tax_regime IN ('rus', 'rer', 'mype', 'general')),
  ubigeo text CONSTRAINT tenant_settings_ubigeo_format CHECK (ubigeo ~ '^[0-9]{6}$'),
  fiscal_address text CONSTRAINT tenant_settings_fiscal_address_len CHECK (char_length(btrim(fiscal_address)) BETWEEN 3 AND 200),
  fiscal_urbanization text CONSTRAINT tenant_settings_fiscal_urb_len CHECK (char_length(btrim(fiscal_urbanization)) BETWEEN 1 AND 25),
  default_currency char(3) NOT NULL DEFAULT 'PEN' CONSTRAINT tenant_settings_currency_values CHECK (default_currency IN ('PEN', 'USD')),
  allow_negative_stock boolean NOT NULL DEFAULT false,
  print_settings jsonb NOT NULL DEFAULT '{"format":"a4"}'::jsonb,
  version bigint NOT NULL DEFAULT 1 CONSTRAINT tenant_settings_version_positive CHECK (version > 0),
  provider_sync_seq bigint NOT NULL DEFAULT 0 CONSTRAINT tenant_settings_sync_seq_non_neg CHECK (provider_sync_seq >= 0),
  provider_synced_version bigint CONSTRAINT tenant_settings_synced_version_non_neg CHECK (provider_synced_version >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE org.tenant_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE org.tenant_settings FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON org.tenant_settings
  USING (tenant_id = app.current_tenant())
  WITH CHECK (tenant_id = app.current_tenant());

CREATE TRIGGER tenant_settings_entitlements_notify
  AFTER INSERT OR UPDATE OF allow_negative_stock, default_currency OR DELETE ON org.tenant_settings
  FOR EACH ROW EXECUTE FUNCTION platform.notify_tenant_entitlements('tenant_id');

GRANT SELECT ON org.tenant_settings TO wanka_app, wanka_worker;
GRANT INSERT (tenant_id, tax_regime, ubigeo, fiscal_address, fiscal_urbanization, default_currency, allow_negative_stock, print_settings, version, provider_sync_seq)
  ON org.tenant_settings TO wanka_app;
GRANT UPDATE (tax_regime, ubigeo, fiscal_address, fiscal_urbanization, default_currency, allow_negative_stock, print_settings, version, provider_sync_seq, updated_at)
  ON org.tenant_settings TO wanka_app;
GRANT UPDATE (provider_synced_version) ON org.tenant_settings TO wanka_worker;

-- ===== 6. org.document_series: series de comprobantes y documentos internos (#33, #47) =====
CREATE TABLE org.document_series (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL REFERENCES org.tenants,
  site_id uuid NOT NULL,
  cash_register_id uuid,
  doc_type text NOT NULL CONSTRAINT document_series_doc_type_values
    CHECK (doc_type IN ('01', '03', '07', '08', '09', 'PRO', 'OC', 'Z', 'OT')),
  series text NOT NULL,
  last_number bigint NOT NULL DEFAULT 0 CONSTRAINT document_series_last_number_non_neg CHECK (last_number >= 0),
  active boolean NOT NULL DEFAULT true,
  provider_synced_at timestamptz,
  provider_conflict_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT document_series_tenant_type_series_key UNIQUE (tenant_id, doc_type, series),
  CONSTRAINT document_series_tenant_site_fkey FOREIGN KEY (tenant_id, site_id) REFERENCES org.sites (tenant_id, id),
  CONSTRAINT document_series_tenant_site_cash_register_fkey FOREIGN KEY (tenant_id, site_id, cash_register_id) REFERENCES org.cash_registers (tenant_id, site_id, id),
  CONSTRAINT document_series_internal_no_sync CHECK (
    doc_type IN ('01', '03', '07', '08', '09') OR (provider_synced_at IS NULL AND provider_conflict_at IS NULL)
  ),
  CONSTRAINT document_series_format CHECK (
    CASE doc_type
      WHEN '01' THEN series ~ '^F[A-Z0-9]{3}$'
      WHEN '03' THEN series ~ '^B[A-Z0-9]{3}$'
      WHEN '07' THEN series ~ '^[FB][A-Z0-9]{3}$'
      WHEN '08' THEN series ~ '^[FB][A-Z0-9]{3}$'
      WHEN '09' THEN series ~ '^T[A-Z0-9]{3}$'
      ELSE series ~ '^[A-Z0-9]{2,10}$'
    END
  )
);

CREATE INDEX document_series_site_idx ON org.document_series (tenant_id, site_id);
CREATE INDEX document_series_cash_register_idx ON org.document_series (tenant_id, site_id, cash_register_id);
CREATE UNIQUE INDEX document_series_active_cash_doc_key
  ON org.document_series (tenant_id, cash_register_id, doc_type)
  WHERE cash_register_id IS NOT NULL AND active AND provider_conflict_at IS NULL;

-- +goose StatementBegin
CREATE FUNCTION org.guard_series_conflict() RETURNS trigger
  LANGUAGE plpgsql
  SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
  IF OLD.provider_conflict_at IS NOT NULL AND NEW.provider_conflict_at IS DISTINCT FROM OLD.provider_conflict_at THEN
    RAISE EXCEPTION 'a frozen series stays frozen' USING ERRCODE = '42501';
  END IF;
  IF OLD.provider_conflict_at IS NULL AND NEW.provider_conflict_at IS NOT NULL THEN
    NEW.provider_conflict_at := now();
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TRIGGER document_series_guard_conflict
  BEFORE UPDATE OF provider_conflict_at ON org.document_series
  FOR EACH ROW EXECUTE FUNCTION org.guard_series_conflict();

ALTER TABLE org.document_series ENABLE ROW LEVEL SECURITY;
ALTER TABLE org.document_series FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON org.document_series
  USING (tenant_id = app.current_tenant())
  WITH CHECK (tenant_id = app.current_tenant());

GRANT SELECT ON org.document_series TO wanka_app, wanka_worker;
GRANT INSERT (id, tenant_id, site_id, cash_register_id, doc_type, series, last_number, active)
  ON org.document_series TO wanka_app;
GRANT UPDATE (site_id, cash_register_id, last_number, active, provider_conflict_at, updated_at)
  ON org.document_series TO wanka_app;
GRANT UPDATE (provider_synced_at, provider_conflict_at)
  ON org.document_series TO wanka_worker;

-- ===== 7. einv.company_sync_state: arrendamiento y estado de sincronización (ADR 0013) =====
CREATE TABLE einv.company_sync_state (
  tenant_id uuid PRIMARY KEY REFERENCES org.tenants,
  holder text,
  lease_until timestamptz,
  synced_seq bigint NOT NULL DEFAULT 0 CONSTRAINT company_sync_state_synced_seq_non_neg CHECK (synced_seq >= 0)
);

ALTER TABLE einv.company_sync_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE einv.company_sync_state FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON einv.company_sync_state
  USING (tenant_id = app.current_tenant())
  WITH CHECK (tenant_id = app.current_tenant());

GRANT USAGE ON SCHEMA einv TO wanka_app, wanka_worker;
GRANT SELECT, INSERT, UPDATE ON einv.company_sync_state TO wanka_worker;
