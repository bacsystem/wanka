// Package postgres implementa el acceso a la base de datos para el módulo de empresas (#47).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
			siteCode   string
		)
		err := tx.QueryRow(ctx, `
			SELECT s.id, s.doc_type, s.series, s.last_number, s.provider_conflict_at, st.code
			FROM org.document_series s
			JOIN org.sites st ON st.id = s.site_id
			WHERE s.doc_type = $1 AND s.series = $2
		`, r.DocType, r.Series).Scan(&seriesID, &docType, &seriesCode, &lastNum, &conflictAt, &siteCode)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return report, fmt.Errorf("querying series %s %s: %w", r.DocType, r.Series, err)
		}

		// 1. Verificación de discrepancia de anexo (aplica a todos los tipos, incluida 09)
		if r.Anexo != "" && r.Anexo != siteCode {
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
