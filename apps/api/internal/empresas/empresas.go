// Package empresas implementa el módulo de empresas (épica #4, ADR 0002).
// Esta es su fachada pública; los demás módulos no importan domain, adapters ni store directamente.
package empresas

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/empresas/adapters/httpapi"
	"github.com/bacsystem/wanka/apps/api/internal/empresas/adapters/postgres"
	"github.com/bacsystem/wanka/apps/api/internal/empresas/domain"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

// Re-exportación de tipos de dominio
type (
	AssignedNumber  = domain.AssignedNumber
	DocumentSeries  = domain.DocumentSeries
	RemoteSeries    = domain.RemoteSeries
	ReconcileReport = domain.ReconcileReport
)

// Re-exportación de errores tipados
var (
	ErrSeriesNotFound           = domain.ErrSeriesNotFound
	ErrSeriesInactive           = domain.ErrSeriesInactive
	ErrSeriesNotSynced          = domain.ErrSeriesNotSynced
	ErrSeriesConflict           = domain.ErrSeriesConflict
	ErrGREUsesProviderNumbering = domain.ErrGREUsesProviderNumbering
	ErrSeriesAlreadyFrozen      = domain.ErrSeriesAlreadyFrozen
)

// Store por defecto para invocaciones directas a nivel de paquete
var defaultStore = postgres.NewStore(nil)

// NextNumber asigna el siguiente correlativo para una caja y tipo de comprobante (#47 AC1).
func NextNumber(ctx context.Context, tx pgx.Tx, cashRegisterID pgtype.UUID, docType string) (AssignedNumber, error) {
	return defaultStore.NextNumber(ctx, tx, cashRegisterID, docType)
}

// NextNumberBySeriesID asigna el siguiente correlativo referenciando la serie directamente por su ID (#47 AC7).
func NextNumberBySeriesID(ctx context.Context, tx pgx.Tx, seriesID pgtype.UUID) (AssignedNumber, error) {
	return defaultStore.NextNumberBySeriesID(ctx, tx, seriesID)
}

// FreezeSeries congela una serie ante conflicto o discrepancia (#47 AC5).
func FreezeSeries(ctx context.Context, tx pgx.Tx, seriesID pgtype.UUID, reason string) error {
	return defaultStore.FreezeSeries(ctx, tx, seriesID, reason)
}

// GetDocumentSeries consulta el estado y numeración de una serie dentro de la transacción (#47 AC8).
func GetDocumentSeries(ctx context.Context, tx pgx.Tx, id pgtype.UUID) (DocumentSeries, error) {
	return defaultStore.GetDocumentSeries(ctx, tx, id)
}

// ReconcileSeries procesa las series reportadas por Khipu y congela o alerta según discrepancias (#47 AC6).
func ReconcileSeries(ctx context.Context, tx pgx.Tx, remote []RemoteSeries) (ReconcileReport, error) {
	return defaultStore.ReconcileSeries(ctx, tx, remote)
}

// Mount registra los endpoints HTTP de empresas en la cadena de rutas de tenant (#47 AC8).
func Mount(routes *auth.Routes, pool *db.Pool) error {
	handler := httpapi.NewSeriesHandler(pool)
	routes.Handle("GET /v1/document-series/{id}", auth.Tenant, auth.Spec{
		Module:     "empresas",
		Permission: string(permission.EmpresasSeriesRead),
	}, handler)
	return nil
}
