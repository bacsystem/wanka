// Package empresas implementa el módulo de empresas (épica #4, ADR 0002).
// Esta es su fachada pública; los demás módulos no importan domain, adapters ni store directamente.
package empresas

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	apigen "github.com/bacsystem/wanka/apps/api/internal/apigen/empresas"
	"github.com/bacsystem/wanka/apps/api/internal/empresas/adapters/httpapi"
	"github.com/bacsystem/wanka/apps/api/internal/empresas/adapters/postgres"
	"github.com/bacsystem/wanka/apps/api/internal/empresas/domain"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

// Re-exportación de tipos de dominio
type (
	AssignedNumber  = domain.AssignedNumber
	DocumentSeries  = domain.DocumentSeries
	Site            = domain.Site
	CashRegister    = domain.CashRegister
	RemoteSeries    = domain.RemoteSeries
	ReconcileReport = domain.ReconcileReport
	ProviderClient  = domain.ProviderClient
)

// Re-exportación de errores tipados
var (
	ErrSeriesNotFound                 = domain.ErrSeriesNotFound
	ErrSeriesInactive                 = domain.ErrSeriesInactive
	ErrSeriesNotSynced                = domain.ErrSeriesNotSynced
	ErrSeriesConflict                 = domain.ErrSeriesConflict
	ErrGREUsesProviderNumbering       = domain.ErrGREUsesProviderNumbering
	ErrSeriesAlreadyFrozen            = domain.ErrSeriesAlreadyFrozen
	ErrSiteNotFound                   = domain.ErrSiteNotFound
	ErrCashRegisterNotFound           = domain.ErrCashRegisterNotFound
	ErrDuplicateCode                  = domain.ErrDuplicateCode
	ErrDuplicateAnnex                 = domain.ErrDuplicateAnnex
	ErrDuplicateSeries                = domain.ErrDuplicateSeries
	ErrAnnexImmutableWithSyncedSeries = domain.ErrAnnexImmutableWithSyncedSeries
	ErrCPERequiresSunatAnnex          = domain.ErrCPERequiresSunatAnnex
	ErrSiteInactive                   = domain.ErrSiteInactive
	ErrCashRegisterInactive           = domain.ErrCashRegisterInactive
	ErrCashRegisterSiteMismatch       = domain.ErrCashRegisterSiteMismatch
	ErrLastNumberImmutableWhenSynced  = domain.ErrLastNumberImmutableWhenSynced
	ErrSiteImmutableWhenSynced        = domain.ErrSiteImmutableWhenSynced
	ErrInvalidSeriesFormat            = domain.ErrInvalidSeriesFormat
	ErrInvalidAnnexFormat             = domain.ErrInvalidAnnexFormat
	ErrInvalidCode                    = domain.ErrInvalidCode
	ErrPlanLimitReached               = domain.ErrPlanLimitReached
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

// CreateSite registra una nueva sede (#33 AC1).
func CreateSite(ctx context.Context, tx pgx.Tx, s Site) (Site, error) {
	return defaultStore.CreateSite(ctx, tx, s)
}

// GetSite consulta una sede por ID (#33).
func GetSite(ctx context.Context, tx pgx.Tx, id pgtype.UUID) (Site, error) {
	return defaultStore.GetSite(ctx, tx, id)
}

// ListSites lista sedes (#33).
func ListSites(ctx context.Context, tx pgx.Tx, active *bool) ([]Site, error) {
	return defaultStore.ListSites(ctx, tx, active)
}

// UpdateSite actualiza una sede (#33 AC6, AC10).
func UpdateSite(ctx context.Context, tx pgx.Tx, id pgtype.UUID, name, address, sunatAnnex *string, active *bool) (Site, error) {
	return defaultStore.UpdateSite(ctx, tx, id, name, address, sunatAnnex, active)
}

// CreateCashRegister registra una nueva caja (#33).
func CreateCashRegister(ctx context.Context, tx pgx.Tx, cr CashRegister) (CashRegister, error) {
	return defaultStore.CreateCashRegister(ctx, tx, cr)
}

// GetCashRegister consulta una caja por ID (#33).
func GetCashRegister(ctx context.Context, tx pgx.Tx, id pgtype.UUID) (CashRegister, error) {
	return defaultStore.GetCashRegister(ctx, tx, id)
}

// ListCashRegisters lista cajas (#33).
func ListCashRegisters(ctx context.Context, tx pgx.Tx, siteID *pgtype.UUID, active *bool) ([]CashRegister, error) {
	return defaultStore.ListCashRegisters(ctx, tx, siteID, active)
}

// UpdateCashRegister actualiza una caja (#33).
func UpdateCashRegister(ctx context.Context, tx pgx.Tx, id pgtype.UUID, code *string, active *bool) (CashRegister, error) {
	return defaultStore.UpdateCashRegister(ctx, tx, id, code, active)
}

// CreateDocumentSeries registra una serie (#33 AC2, AC3, AC4).
func CreateDocumentSeries(ctx context.Context, tx pgx.Tx, ds DocumentSeries) (DocumentSeries, error) {
	return defaultStore.CreateDocumentSeries(ctx, tx, ds)
}

// ListDocumentSeries lista series (#33).
func ListDocumentSeries(ctx context.Context, tx pgx.Tx, siteID, cashRegisterID *pgtype.UUID, docType *string, active *bool) ([]DocumentSeries, error) {
	return defaultStore.ListDocumentSeries(ctx, tx, siteID, cashRegisterID, docType, active)
}

// UpdateDocumentSeries actualiza una serie (#33 AC7, AC8, AC9, AC11).
func UpdateDocumentSeries(ctx context.Context, tx pgx.Tx, id pgtype.UUID, siteID, cashRegisterID *pgtype.UUID, lastNumber *int64, active *bool, client ProviderClient) (DocumentSeries, error) {
	return defaultStore.UpdateDocumentSeries(ctx, tx, id, siteID, cashRegisterID, lastNumber, active, client)
}

// Mount registra los endpoints HTTP de empresas en la cadena de rutas de tenant (#33, #47).
func Mount(routes *auth.Routes, pool *db.Pool, client ...ProviderClient) error {
	var c ProviderClient
	if len(client) > 0 {
		c = client[0]
	}

	h := httpapi.NewHandler(pool, c)
	siw := apigen.ServerInterfaceWrapper{
		Handler: h,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			httpx.WriteProblem(w, r, "malformed_request", err.Error())
		},
	}

	// Locales (Sites)
	routes.HandleFunc("GET /v1/sites", auth.Tenant, auth.Spec{
		Module:     "empresas",
		Permission: string(permission.EmpresasSitesRead),
	}, siw.ListSites)
	routes.HandleFunc("POST /v1/sites", auth.Tenant, auth.Spec{
		Module:     "empresas",
		Permission: string(permission.EmpresasSitesCreate),
		Limit:      entitlements.LimitSites,
	}, siw.CreateSite)
	routes.HandleFunc("GET /v1/sites/{id}", auth.Tenant, auth.Spec{
		Module:     "empresas",
		Permission: string(permission.EmpresasSitesRead),
	}, siw.GetSite)
	routes.HandleFunc("PATCH /v1/sites/{id}", auth.Tenant, auth.Spec{
		Module:     "empresas",
		Permission: string(permission.EmpresasSitesUpdate),
	}, siw.UpdateSite)

	// Cajas (Cash Registers)
	routes.HandleFunc("GET /v1/cash-registers", auth.Tenant, auth.Spec{
		Module:     "empresas",
		Permission: string(permission.EmpresasSitesRead),
	}, siw.ListCashRegisters)
	routes.HandleFunc("POST /v1/cash-registers", auth.Tenant, auth.Spec{
		Module:     "empresas",
		Permission: string(permission.EmpresasSitesUpdate),
	}, siw.CreateCashRegister)
	routes.HandleFunc("GET /v1/cash-registers/{id}", auth.Tenant, auth.Spec{
		Module:     "empresas",
		Permission: string(permission.EmpresasSitesRead),
	}, siw.GetCashRegister)
	routes.HandleFunc("PATCH /v1/cash-registers/{id}", auth.Tenant, auth.Spec{
		Module:     "empresas",
		Permission: string(permission.EmpresasSitesUpdate),
	}, siw.UpdateCashRegister)

	// Series (Document Series)
	routes.HandleFunc("GET /v1/document-series", auth.Tenant, auth.Spec{
		Module:     "empresas",
		Permission: string(permission.EmpresasSeriesRead),
	}, siw.ListDocumentSeries)
	routes.HandleFunc("POST /v1/document-series", auth.Tenant, auth.Spec{
		Module:     "empresas",
		Permission: string(permission.EmpresasSeriesCreate),
	}, siw.CreateDocumentSeries)
	routes.HandleFunc("GET /v1/document-series/{id}", auth.Tenant, auth.Spec{
		Module:     "empresas",
		Permission: string(permission.EmpresasSeriesRead),
	}, siw.GetDocumentSeries)
	routes.HandleFunc("PATCH /v1/document-series/{id}", auth.Tenant, auth.Spec{
		Module:     "empresas",
		Permission: string(permission.EmpresasSeriesUpdate),
	}, siw.UpdateDocumentSeries)

	return nil
}
