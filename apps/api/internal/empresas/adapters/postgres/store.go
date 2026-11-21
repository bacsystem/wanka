// Package postgres implementa el acceso a la base de datos para el módulo de empresas (#47).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/empresas/domain"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

// Querier abstrae operaciones de consulta tanto en un pool como dentro de una transacción.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconnTag, error)
}

type pgconnTag interface {
	RowsAffected() int64
}

// Store provee métodos de acceso a datos para series y numeración correlativa en org.document_series.
type Store struct {
	pool *db.Pool
}

// NewStore construye una nueva instancia de Store.
func NewStore(pool *db.Pool) *Store {
	return &Store{pool: pool}
}

// NextNumber asigna atómicamente el siguiente correlativo para la caja y tipo de documento especificados dentro de tx.
func (s *Store) NextNumber(ctx context.Context, tx pgx.Tx, cashRegisterID pgtype.UUID, docType string) (domain.AssignedNumber, error) {
	if docType == domain.DocTypeGuiaRemision {
		return domain.AssignedNumber{}, domain.ErrGREUsesProviderNumbering
	}

	var res domain.AssignedNumber
	err := tx.QueryRow(ctx, `
		UPDATE org.document_series
		SET last_number = last_number + 1, updated_at = now()
		WHERE cash_register_id = $1 AND doc_type = $2 AND active AND provider_conflict_at IS NULL
		  AND (doc_type NOT IN ('01', '03', '07', '08', '09') OR provider_synced_at IS NOT NULL)
		RETURNING id, doc_type, series, last_number
	`, cashRegisterID, docType).Scan(&res.SeriesID, &res.DocType, &res.Series, &res.Number)
	if err == nil {
		return res, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.AssignedNumber{}, err
	}

	// Diagnóstico de la causa del rechazo
	var (
		active     bool
		syncedAt   *time.Time
		conflictAt *time.Time
	)
	diagErr := tx.QueryRow(ctx, `
		SELECT active, provider_synced_at, provider_conflict_at
		FROM org.document_series
		WHERE cash_register_id = $1 AND doc_type = $2
	`, cashRegisterID, docType).Scan(&active, &syncedAt, &conflictAt)
	if errors.Is(diagErr, pgx.ErrNoRows) {
		return domain.AssignedNumber{}, domain.ErrSeriesNotFound
	}
	if diagErr != nil {
		return domain.AssignedNumber{}, diagErr
	}

	switch {
	case conflictAt != nil:
		return domain.AssignedNumber{}, domain.ErrSeriesConflict
	case !active:
		return domain.AssignedNumber{}, domain.ErrSeriesInactive
	case domain.IsCPEDocType(docType) && syncedAt == nil:
		return domain.AssignedNumber{}, domain.ErrSeriesNotSynced
	default:
		return domain.AssignedNumber{}, domain.ErrSeriesNotFound
	}
}

// NextNumberBySeriesID asigna atómicamente el siguiente correlativo referenciando directamente la serie por ID.
func (s *Store) NextNumberBySeriesID(ctx context.Context, tx pgx.Tx, seriesID pgtype.UUID) (domain.AssignedNumber, error) {
	var res domain.AssignedNumber
	err := tx.QueryRow(ctx, `
		UPDATE org.document_series
		SET last_number = last_number + 1, updated_at = now()
		WHERE id = $1 AND active AND provider_conflict_at IS NULL
		  AND (doc_type NOT IN ('01', '03', '07', '08', '09') OR provider_synced_at IS NOT NULL)
		  AND doc_type != '09'
		RETURNING id, doc_type, series, last_number
	`, seriesID).Scan(&res.SeriesID, &res.DocType, &res.Series, &res.Number)
	if err == nil {
		return res, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.AssignedNumber{}, err
	}

	// Diagnóstico de la causa del rechazo
	var (
		docType    string
		active     bool
		syncedAt   *time.Time
		conflictAt *time.Time
	)
	diagErr := tx.QueryRow(ctx, `
		SELECT doc_type, active, provider_synced_at, provider_conflict_at
		FROM org.document_series
		WHERE id = $1
	`, seriesID).Scan(&docType, &active, &syncedAt, &conflictAt)
	if errors.Is(diagErr, pgx.ErrNoRows) {
		return domain.AssignedNumber{}, domain.ErrSeriesNotFound
	}
	if diagErr != nil {
		return domain.AssignedNumber{}, diagErr
	}

	switch {
	case docType == domain.DocTypeGuiaRemision:
		return domain.AssignedNumber{}, domain.ErrGREUsesProviderNumbering
	case conflictAt != nil:
		return domain.AssignedNumber{}, domain.ErrSeriesConflict
	case !active:
		return domain.AssignedNumber{}, domain.ErrSeriesInactive
	case domain.IsCPEDocType(docType) && syncedAt == nil:
		return domain.AssignedNumber{}, domain.ErrSeriesNotSynced
	default:
		return domain.AssignedNumber{}, domain.ErrSeriesNotFound
	}
}

// FreezeSeries congela una serie marcando provider_conflict_at = now().
func (s *Store) FreezeSeries(ctx context.Context, tx pgx.Tx, seriesID pgtype.UUID, reason string) error {
	tag, err := tx.Exec(ctx, `
		UPDATE org.document_series
		SET provider_conflict_at = now(), updated_at = now()
		WHERE id = $1 AND provider_conflict_at IS NULL
	`, seriesID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var conflictAt *time.Time
		err := tx.QueryRow(ctx, `
			SELECT provider_conflict_at
			FROM org.document_series
			WHERE id = $1
		`, seriesID).Scan(&conflictAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrSeriesNotFound
		}
		if err != nil {
			return err
		}
		if conflictAt != nil {
			return domain.ErrSeriesAlreadyFrozen
		}
	}
	return nil
}

// GetDocumentSeries consulta una serie de comprobantes por ID dentro de la transacción / contexto RLS.
func (s *Store) GetDocumentSeries(ctx context.Context, tx pgx.Tx, id pgtype.UUID) (domain.DocumentSeries, error) {
	var ds domain.DocumentSeries
	var cashRegisterID pgtype.UUID
	err := tx.QueryRow(ctx, `
		SELECT id, tenant_id, site_id, cash_register_id, doc_type, series, last_number, active,
		       provider_synced_at, provider_conflict_at, created_at, updated_at
		FROM org.document_series
		WHERE id = $1
	`, id).Scan(
		&ds.ID,
		&ds.TenantID,
		&ds.SiteID,
		&cashRegisterID,
		&ds.DocType,
		&ds.Series,
		&ds.LastNumber,
		&ds.Active,
		&ds.ProviderSyncedAt,
		&ds.ProviderConflictAt,
		&ds.CreatedAt,
		&ds.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DocumentSeries{}, domain.ErrSeriesNotFound
	}
	if err != nil {
		return domain.DocumentSeries{}, err
	}
	ds.CashRegisterID = cashRegisterID
	ds.ProviderStatus = domain.ComputeProviderStatus(ds.DocType, ds.ProviderSyncedAt, ds.ProviderConflictAt)
	return ds, nil
}

// ReconcileSeries procesa el reporte de Khipu y congela o alerta según discrepancias de anexo y correlativo (#47 AC6).
func (s *Store) ReconcileSeries(ctx context.Context, tx pgx.Tx, remote []domain.RemoteSeries) (domain.ReconcileReport, error) {
	report := domain.ReconcileReport{}
	for _, r := range remote {
		report.Processed++
		var (
			seriesID   pgtype.UUID
			docType    string
			seriesCode string
			lastNum    int64
			conflictAt *time.Time
			siteAnnex  string
		)
		err := tx.QueryRow(ctx, `
			SELECT s.id, s.doc_type, s.series, s.last_number, s.provider_conflict_at, COALESCE(NULLIF(st.sunat_annex, ''), st.code)
			FROM org.document_series s
			JOIN org.sites st ON st.id = s.site_id
			WHERE s.doc_type = $1 AND s.series = $2
		`, r.DocType, r.Series).Scan(&seriesID, &docType, &seriesCode, &lastNum, &conflictAt, &siteAnnex)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return report, fmt.Errorf("querying series %s %s: %w", r.DocType, r.Series, err)
		}

		// 1. Verificación de discrepancia de anexo (aplica a todos los tipos, incluida 09)
		if r.Anexo != "" && r.Anexo != siteAnnex {
			if conflictAt == nil {
				if err := s.FreezeSeries(ctx, tx, seriesID, "annex mismatch with provider"); err != nil && !errors.Is(err, domain.ErrSeriesAlreadyFrozen) {
					return report, fmt.Errorf("freezing series on annex mismatch: %w", err)
				}
				report.FrozenCount++
			}
			continue
		}

		// 2. Verificación de correlativo
		if r.UltimoNumero > lastNum {
			if docType == domain.DocTypeGuiaRemision {
				// GRE (09) usa numeración de Khipu: no congelar por número, solo alertar a soporte (ADR 0013 §5)
				report.Alerts = append(report.Alerts, fmt.Sprintf("GRE series %s remote number %d > local %d", seriesCode, r.UltimoNumero, lastNum))
			} else {
				// Comprobantes 01, 03, 07, 08: congelar inmediatamente
				if conflictAt == nil {
					if err := s.FreezeSeries(ctx, tx, seriesID, "provider number ahead of local"); err != nil && !errors.Is(err, domain.ErrSeriesAlreadyFrozen) {
						return report, fmt.Errorf("freezing series on number disparity: %w", err)
					}
					report.FrozenCount++
				}
			}
		}
	}
	return report, nil
}

func mapPgError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.ConstraintName {
		case "sites_code_format":
			return domain.ErrInvalidCode
		case "sites_sunat_annex_format":
			return domain.ErrInvalidAnnexFormat
		case "sites_tenant_id_code_key", "sites_tenant_code_key", "sites_code_key":
			return domain.ErrDuplicateCode
		case "sites_tenant_annex_key":
			return domain.ErrDuplicateAnnex
		case "cash_registers_code_format":
			return domain.ErrInvalidCode
		case "cash_registers_tenant_id_code_key", "cash_registers_tenant_code_key", "cash_registers_code_key":
			return domain.ErrDuplicateCode
		case "document_series_tenant_type_series_key", "document_series_active_cash_doc_key":
			return domain.ErrDuplicateSeries
		case "document_series_format":
			return domain.ErrInvalidSeriesFormat
		}
	}
	return err
}

// CreateSite registra una nueva sede o local en org.sites (#33 AC1).
func (s *Store) CreateSite(ctx context.Context, tx pgx.Tx, site domain.Site) (domain.Site, error) {
	if err := domain.ValidateSunatAnnex(site.SunatAnnex); err != nil {
		return domain.Site{}, err
	}

	var res domain.Site
	err := tx.QueryRow(ctx, `
		INSERT INTO org.sites (id, tenant_id, code, name, sunat_annex, address, active, version)
		VALUES ($1, app.current_tenant(), $2, $3, $4, $5, $6, 1)
		RETURNING id, tenant_id, code, name, sunat_annex, address, active, version, provider_synced_version, created_at, updated_at
	`, site.ID, site.Code, site.Name, site.SunatAnnex, site.Address, site.Active).Scan(
		&res.ID, &res.TenantID, &res.Code, &res.Name, &res.SunatAnnex, &res.Address, &res.Active,
		&res.Version, &res.ProviderSyncedVersion, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		return domain.Site{}, mapPgError(err)
	}

	// Si se asignó un anexo distinto de '0000', se incrementa provider_sync_seq para que el job lo registre en Khipu (#33 AC1)
	if res.SunatAnnex != nil && *res.SunatAnnex != "" && *res.SunatAnnex != "0000" {
		_, _ = tx.Exec(ctx, `
			UPDATE org.tenant_settings
			SET provider_sync_seq = provider_sync_seq + 1, updated_at = now()
			WHERE tenant_id = app.current_tenant()
		`)
	}

	return res, nil
}

// GetSite obtiene una sede por su ID.
func (s *Store) GetSite(ctx context.Context, tx pgx.Tx, id pgtype.UUID) (domain.Site, error) {
	var res domain.Site
	err := tx.QueryRow(ctx, `
		SELECT id, tenant_id, code, name, sunat_annex, address, active, version, provider_synced_version, created_at, updated_at
		FROM org.sites
		WHERE id = $1
	`, id).Scan(
		&res.ID, &res.TenantID, &res.Code, &res.Name, &res.SunatAnnex, &res.Address, &res.Active,
		&res.Version, &res.ProviderSyncedVersion, &res.CreatedAt, &res.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Site{}, domain.ErrSiteNotFound
	}
	if err != nil {
		return domain.Site{}, err
	}
	return res, nil
}

// ListSites lista todas las sedes del tenant, opcionalmente filtradas por activas.
func (s *Store) ListSites(ctx context.Context, tx pgx.Tx, active *bool) ([]domain.Site, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, tenant_id, code, name, sunat_annex, address, active, version, provider_synced_version, created_at, updated_at
		FROM org.sites
		WHERE ($1::boolean IS NULL OR active = $1)
		ORDER BY code ASC
	`, active)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sites []domain.Site
	for rows.Next() {
		var res domain.Site
		if err := rows.Scan(
			&res.ID, &res.TenantID, &res.Code, &res.Name, &res.SunatAnnex, &res.Address, &res.Active,
			&res.Version, &res.ProviderSyncedVersion, &res.CreatedAt, &res.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sites = append(sites, res)
	}
	return sites, rows.Err()
}

// UpdateSite actualiza una sede existente (#33 AC6, AC10).
func (s *Store) UpdateSite(ctx context.Context, tx pgx.Tx, id pgtype.UUID, name *string, address *string, sunatAnnex *string, active *bool) (domain.Site, error) {
	var current domain.Site
	err := tx.QueryRow(ctx, `
		SELECT id, tenant_id, code, name, sunat_annex, address, active, version, provider_synced_version, created_at, updated_at
		FROM org.sites
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(
		&current.ID, &current.TenantID, &current.Code, &current.Name, &current.SunatAnnex, &current.Address,
		&current.Active, &current.Version, &current.ProviderSyncedVersion, &current.CreatedAt, &current.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Site{}, domain.ErrSiteNotFound
	}
	if err != nil {
		return domain.Site{}, err
	}

	// AC10: Cambio de anexo SUNAT
	if sunatAnnex != nil {
		currentAnnex := ""
		if current.SunatAnnex != nil {
			currentAnnex = *current.SunatAnnex
		}
		if *sunatAnnex != currentAnnex {
			// Si el local tiene alguna serie ya sincronizada con Khipu (incluido 0000), rechazar con 409
			var hasSyncedSeries bool
			err := tx.QueryRow(ctx, `
				SELECT EXISTS(
					SELECT 1 FROM org.document_series
					WHERE site_id = $1 AND provider_synced_at IS NOT NULL
				)
			`, id).Scan(&hasSyncedSeries)
			if err != nil {
				return domain.Site{}, err
			}
			if hasSyncedSeries {
				return domain.Site{}, domain.ErrAnnexImmutableWithSyncedSeries
			}
			if currentAnnex == "0000" {
				return domain.Site{}, domain.ErrAnnexImmutableWithSyncedSeries
			}
			if err := domain.ValidateSunatAnnex(sunatAnnex); err != nil {
				return domain.Site{}, err
			}
		}
	}

	// AC6: Si se desactiva el local, inactivar sus cajas y series en Wanka en la misma transacción
	if active != nil && !*active && current.Active {
		if _, err := tx.Exec(ctx, `
			UPDATE org.cash_registers
			SET active = false, updated_at = now()
			WHERE site_id = $1 AND active = true
		`, id); err != nil {
			return domain.Site{}, fmt.Errorf("deactivating site cash registers: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE org.document_series
			SET active = false, updated_at = now()
			WHERE site_id = $1 AND active = true
		`, id); err != nil {
			return domain.Site{}, fmt.Errorf("deactivating site document series: %w", err)
		}
	}

	var res domain.Site
	err = tx.QueryRow(ctx, `
		UPDATE org.sites
		SET name = COALESCE($2, name),
		    address = CASE WHEN $3::text IS NOT NULL THEN $3 ELSE address END,
		    sunat_annex = CASE WHEN $4::text IS NOT NULL THEN NULLIF($4, '') ELSE sunat_annex END,
		    active = COALESCE($5, active),
		    version = version + 1,
		    updated_at = now()
		WHERE id = $1
		RETURNING id, tenant_id, code, name, sunat_annex, address, active, version, provider_synced_version, created_at, updated_at
	`, id, name, address, sunatAnnex, active).Scan(
		&res.ID, &res.TenantID, &res.Code, &res.Name, &res.SunatAnnex, &res.Address, &res.Active,
		&res.Version, &res.ProviderSyncedVersion, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		return domain.Site{}, mapPgError(err)
	}

	if sunatAnnex != nil && *sunatAnnex != "" && *sunatAnnex != "0000" {
		_, _ = tx.Exec(ctx, `
			UPDATE org.tenant_settings
			SET provider_sync_seq = provider_sync_seq + 1, updated_at = now()
			WHERE tenant_id = app.current_tenant()
		`)
	}

	return res, nil
}

// CreateCashRegister registra una nueva caja en un local de la empresa.
func (s *Store) CreateCashRegister(ctx context.Context, tx pgx.Tx, cr domain.CashRegister) (domain.CashRegister, error) {
	var siteActive bool
	err := tx.QueryRow(ctx, `SELECT active FROM org.sites WHERE id = $1`, cr.SiteID).Scan(&siteActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CashRegister{}, domain.ErrSiteNotFound
	}
	if err != nil {
		return domain.CashRegister{}, err
	}
	if !siteActive {
		return domain.CashRegister{}, domain.ErrSiteInactive
	}

	var res domain.CashRegister
	err = tx.QueryRow(ctx, `
		INSERT INTO org.cash_registers (id, tenant_id, site_id, code, active)
		VALUES ($1, app.current_tenant(), $2, $3, $4)
		RETURNING id, tenant_id, site_id, code, active, created_at, updated_at
	`, cr.ID, cr.SiteID, cr.Code, cr.Active).Scan(
		&res.ID, &res.TenantID, &res.SiteID, &res.Code, &res.Active, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		return domain.CashRegister{}, mapPgError(err)
	}
	return res, nil
}

// GetCashRegister consulta una caja por su ID.
func (s *Store) GetCashRegister(ctx context.Context, tx pgx.Tx, id pgtype.UUID) (domain.CashRegister, error) {
	var res domain.CashRegister
	err := tx.QueryRow(ctx, `
		SELECT id, tenant_id, site_id, code, active, created_at, updated_at
		FROM org.cash_registers
		WHERE id = $1
	`, id).Scan(&res.ID, &res.TenantID, &res.SiteID, &res.Code, &res.Active, &res.CreatedAt, &res.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CashRegister{}, domain.ErrCashRegisterNotFound
	}
	if err != nil {
		return domain.CashRegister{}, err
	}
	return res, nil
}

// ListCashRegisters lista las cajas de la empresa, con filtros opcionales.
func (s *Store) ListCashRegisters(ctx context.Context, tx pgx.Tx, siteID *pgtype.UUID, active *bool) ([]domain.CashRegister, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, tenant_id, site_id, code, active, created_at, updated_at
		FROM org.cash_registers
		WHERE ($1::uuid IS NULL OR site_id = $1)
		  AND ($2::boolean IS NULL OR active = $2)
		ORDER BY code ASC
	`, siteID, active)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.CashRegister
	for rows.Next() {
		var res domain.CashRegister
		if err := rows.Scan(&res.ID, &res.TenantID, &res.SiteID, &res.Code, &res.Active, &res.CreatedAt, &res.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, res)
	}
	return list, rows.Err()
}

// UpdateCashRegister actualiza una caja registradora.
func (s *Store) UpdateCashRegister(ctx context.Context, tx pgx.Tx, id pgtype.UUID, code *string, active *bool) (domain.CashRegister, error) {
	if active != nil && !*active {
		if _, err := tx.Exec(ctx, `
			UPDATE org.document_series
			SET active = false, updated_at = now()
			WHERE cash_register_id = $1 AND active = true
		`, id); err != nil {
			return domain.CashRegister{}, fmt.Errorf("deactivating cash register series: %w", err)
		}
	}

	var res domain.CashRegister
	err := tx.QueryRow(ctx, `
		UPDATE org.cash_registers
		SET code = COALESCE($2, code),
		    active = COALESCE($3, active),
		    updated_at = now()
		WHERE id = $1
		RETURNING id, tenant_id, site_id, code, active, created_at, updated_at
	`, id, code, active).Scan(&res.ID, &res.TenantID, &res.SiteID, &res.Code, &res.Active, &res.CreatedAt, &res.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CashRegister{}, domain.ErrCashRegisterNotFound
	}
	if err != nil {
		return domain.CashRegister{}, mapPgError(err)
	}
	return res, nil
}

// CreateDocumentSeries registra una nueva serie en org.document_series (#33 AC2, AC3, AC4).
func (s *Store) CreateDocumentSeries(ctx context.Context, tx pgx.Tx, ds domain.DocumentSeries) (domain.DocumentSeries, error) {
	if err := domain.ValidateSeriesFormat(ds.DocType, ds.Series); err != nil {
		return domain.DocumentSeries{}, err
	}

	var siteAnnex *string
	var siteActive bool
	err := tx.QueryRow(ctx, `
		SELECT sunat_annex, active
		FROM org.sites
		WHERE id = $1
	`, ds.SiteID).Scan(&siteAnnex, &siteActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DocumentSeries{}, domain.ErrSiteNotFound
	}
	if err != nil {
		return domain.DocumentSeries{}, err
	}
	if !siteActive {
		return domain.DocumentSeries{}, domain.ErrSiteInactive
	}

	// AC2: Series de CPE exigen que el local tenga anexo SUNAT
	if domain.IsCPEDocType(ds.DocType) && (siteAnnex == nil || *siteAnnex == "") {
		return domain.DocumentSeries{}, domain.ErrCPERequiresSunatAnnex
	}

	if ds.CashRegisterID.Valid {
		var crSiteID pgtype.UUID
		var crActive bool
		err := tx.QueryRow(ctx, `
			SELECT site_id, active
			FROM org.cash_registers
			WHERE id = $1
		`, ds.CashRegisterID).Scan(&crSiteID, &crActive)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DocumentSeries{}, domain.ErrCashRegisterNotFound
		}
		if err != nil {
			return domain.DocumentSeries{}, err
		}
		if !crActive {
			return domain.DocumentSeries{}, domain.ErrCashRegisterInactive
		}
		if crSiteID != ds.SiteID {
			return domain.DocumentSeries{}, domain.ErrCashRegisterSiteMismatch
		}
	}

	var res domain.DocumentSeries
	var cashRegisterID pgtype.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO org.document_series (id, tenant_id, site_id, cash_register_id, doc_type, series, last_number, active)
		VALUES ($1, app.current_tenant(), $2, $3, $4, $5, $6, $7)
		RETURNING id, tenant_id, site_id, cash_register_id, doc_type, series, last_number, active,
		          provider_synced_at, provider_conflict_at, created_at, updated_at
	`, ds.ID, ds.SiteID, ds.CashRegisterID, ds.DocType, ds.Series, ds.LastNumber, ds.Active).Scan(
		&res.ID, &res.TenantID, &res.SiteID, &cashRegisterID, &res.DocType, &res.Series, &res.LastNumber, &res.Active,
		&res.ProviderSyncedAt, &res.ProviderConflictAt, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		return domain.DocumentSeries{}, mapPgError(err)
	}
	res.CashRegisterID = cashRegisterID
	res.ProviderStatus = domain.ComputeProviderStatus(res.DocType, res.ProviderSyncedAt, res.ProviderConflictAt)

	// Si es comprobante electrónico, se encola sincronización subiendo provider_sync_seq
	if domain.IsCPEDocType(res.DocType) {
		_, _ = tx.Exec(ctx, `
			UPDATE org.tenant_settings
			SET provider_sync_seq = provider_sync_seq + 1, updated_at = now()
			WHERE tenant_id = app.current_tenant()
		`)
	}

	return res, nil
}

// ListDocumentSeries lista las series de comprobantes con filtros opcionales.
func (s *Store) ListDocumentSeries(ctx context.Context, tx pgx.Tx, siteID *pgtype.UUID, cashRegisterID *pgtype.UUID, docType *string, active *bool) ([]domain.DocumentSeries, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, tenant_id, site_id, cash_register_id, doc_type, series, last_number, active,
		       provider_synced_at, provider_conflict_at, created_at, updated_at
		FROM org.document_series
		WHERE ($1::uuid IS NULL OR site_id = $1)
		  AND ($2::uuid IS NULL OR cash_register_id = $2)
		  AND ($3::text IS NULL OR doc_type = $3)
		  AND ($4::boolean IS NULL OR active = $4)
		ORDER BY doc_type ASC, series ASC
	`, siteID, cashRegisterID, docType, active)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.DocumentSeries
	for rows.Next() {
		var res domain.DocumentSeries
		var crID pgtype.UUID
		if err := rows.Scan(
			&res.ID, &res.TenantID, &res.SiteID, &crID, &res.DocType, &res.Series, &res.LastNumber, &res.Active,
			&res.ProviderSyncedAt, &res.ProviderConflictAt, &res.CreatedAt, &res.UpdatedAt,
		); err != nil {
			return nil, err
		}
		res.CashRegisterID = crID
		res.ProviderStatus = domain.ComputeProviderStatus(res.DocType, res.ProviderSyncedAt, res.ProviderConflictAt)
		list = append(list, res)
	}
	return list, rows.Err()
}

// UpdateDocumentSeries actualiza una serie existente (#33 AC7, AC8, AC9, AC11).
func (s *Store) UpdateDocumentSeries(ctx context.Context, tx pgx.Tx, id pgtype.UUID, siteID *pgtype.UUID, cashRegisterID *pgtype.UUID, lastNumber *int64, active *bool, client domain.ProviderClient) (domain.DocumentSeries, error) {
	current, err := s.GetDocumentSeries(ctx, tx, id)
	if err != nil {
		return domain.DocumentSeries{}, err
	}

	// AC11: last_number solo se puede editar mientras no esté sincronizada con Khipu
	if lastNumber != nil && *lastNumber != current.LastNumber {
		if current.ProviderSyncedAt != nil {
			return domain.DocumentSeries{}, domain.ErrLastNumberImmutableWhenSynced
		}
	}

	targetSiteID := current.SiteID
	if siteID != nil && *siteID != current.SiteID {
		// AC9: site_id no puede cambiar si ya fue registrada en Khipu
		if current.ProviderSyncedAt != nil {
			return domain.DocumentSeries{}, domain.ErrSiteImmutableWhenSynced
		}
		var siteActive bool
		var siteAnnex *string
		err := tx.QueryRow(ctx, `SELECT active, sunat_annex FROM org.sites WHERE id = $1`, *siteID).Scan(&siteActive, &siteAnnex)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DocumentSeries{}, domain.ErrSiteNotFound
		}
		if err != nil {
			return domain.DocumentSeries{}, err
		}
		if !siteActive {
			return domain.DocumentSeries{}, domain.ErrSiteInactive
		}
		if domain.IsCPEDocType(current.DocType) && (siteAnnex == nil || *siteAnnex == "") {
			return domain.DocumentSeries{}, domain.ErrCPERequiresSunatAnnex
		}
		targetSiteID = *siteID
	}

	targetCashRegisterID := current.CashRegisterID
	if cashRegisterID != nil {
		if cashRegisterID.Valid {
			var crSiteID pgtype.UUID
			var crActive bool
			err := tx.QueryRow(ctx, `SELECT site_id, active FROM org.cash_registers WHERE id = $1`, *cashRegisterID).Scan(&crSiteID, &crActive)
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.DocumentSeries{}, domain.ErrCashRegisterNotFound
			}
			if err != nil {
				return domain.DocumentSeries{}, err
			}
			if !crActive {
				return domain.DocumentSeries{}, domain.ErrCashRegisterInactive
			}
			// AC9: la caja sí puede cambiar pero dentro del mismo local
			if crSiteID != targetSiteID {
				return domain.DocumentSeries{}, domain.ErrCashRegisterSiteMismatch
			}
			targetCashRegisterID = *cashRegisterID
		} else {
			targetCashRegisterID = pgtype.UUID{}
		}
	}

	// AC8: Reactivación de serie
	if active != nil && *active && !current.Active {
		if domain.IsCPEDocType(current.DocType) && current.ProviderSyncedAt != nil {
			if client != nil {
				var siteAnnex string
				_ = tx.QueryRow(ctx, `SELECT COALESCE(sunat_annex, '') FROM org.sites WHERE id = $1`, targetSiteID).Scan(&siteAnnex)
				remoteList, err := client.GetSeries(ctx, current.TenantID)
				if err == nil {
					for _, r := range remoteList {
						if r.DocType == current.DocType && r.Series == current.Series {
							if r.UltimoNumero > current.LastNumber || (r.Anexo != "" && r.Anexo != siteAnnex) {
								if s.pool != nil {
									actor := db.Actor{TenantID: current.TenantID, Kind: db.KindSystem}
									_ = s.pool.InTenantTx(ctx, actor, func(freezeTx pgx.Tx) error {
										return s.FreezeSeries(ctx, freezeTx, id, "provider conflict on reactivation")
									})
								} else {
									_ = s.FreezeSeries(ctx, tx, id, "provider conflict on reactivation")
								}
								return domain.DocumentSeries{}, domain.ErrSeriesConflict
							}
						}
					}
				}
			}
		}
	}

	var res domain.DocumentSeries
	var resCRID pgtype.UUID
	err = tx.QueryRow(ctx, `
		UPDATE org.document_series
		SET site_id = $2,
		    cash_register_id = $3,
		    last_number = COALESCE($4, last_number),
		    active = COALESCE($5, active),
		    updated_at = now()
		WHERE id = $1
		RETURNING id, tenant_id, site_id, cash_register_id, doc_type, series, last_number, active,
		          provider_synced_at, provider_conflict_at, created_at, updated_at
	`, id, targetSiteID, targetCashRegisterID, lastNumber, active).Scan(
		&res.ID, &res.TenantID, &res.SiteID, &resCRID, &res.DocType, &res.Series, &res.LastNumber, &res.Active,
		&res.ProviderSyncedAt, &res.ProviderConflictAt, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		return domain.DocumentSeries{}, mapPgError(err)
	}
	res.CashRegisterID = resCRID
	res.ProviderStatus = domain.ComputeProviderStatus(res.DocType, res.ProviderSyncedAt, res.ProviderConflictAt)
	return res, nil
}
