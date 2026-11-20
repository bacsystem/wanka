// Package httpapi provee adaptadores HTTP para el módulo de empresas (#47).
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/empresas/adapters/postgres"
	"github.com/bacsystem/wanka/apps/api/internal/empresas/domain"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// SeriesHandler maneja las solicitudes a GET /v1/document-series/{id}.
type SeriesHandler struct {
	pool  *db.Pool
	store *postgres.Store
}

// NewSeriesHandler inicializa un nuevo SeriesHandler.
func NewSeriesHandler(pool *db.Pool) *SeriesHandler {
	return &SeriesHandler{
		pool:  pool,
		store: postgres.NewStore(pool),
	}
}

type documentSeriesResponse struct {
	ID             string    `json:"id"`
	SiteID         string    `json:"siteId"`
	CashRegisterID *string   `json:"cashRegisterId"`
	DocType        string    `json:"docType"`
	Series         string    `json:"series"`
	LastNumber     int64     `json:"lastNumber"`
	Active         bool      `json:"active"`
	ProviderStatus string    `json:"providerStatus"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (h *SeriesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rawID := r.PathValue("id")
	var seriesID pgtype.UUID
	if err := seriesID.Scan(rawID); err != nil || !seriesID.Valid {
		httpx.WriteProblem(w, r, "malformed_request", "invalid document series id")
		return
	}

	session, ok := auth.FromContext(r.Context())
	if !ok || !session.HasTenant() {
		httpx.WriteProblem(w, r, "unauthorized", "tenant session required")
		return
	}

	var ds domain.DocumentSeries
	err := h.pool.InTenantReadTx(r.Context(), session.Actor, func(tx pgx.Tx) error {
		var txErr error
		ds, txErr = h.store.GetDocumentSeries(r.Context(), tx, seriesID)
		return txErr
	})
	if errors.Is(err, domain.ErrSeriesNotFound) {
		httpx.WriteProblem(w, r, "not_found", "document series not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	var cashRegisterID *string
	if ds.CashRegisterID.Valid && ds.CashRegisterID.Bytes != [16]byte{} {
		crStr := ds.CashRegisterID.String()
		cashRegisterID = &crStr
	}

	resp := documentSeriesResponse{
		ID:             ds.ID.String(),
		SiteID:         ds.SiteID.String(),
		CashRegisterID: cashRegisterID,
		DocType:        ds.DocType,
		Series:         ds.Series,
		LastNumber:     ds.LastNumber,
		Active:         ds.Active,
		ProviderStatus: ds.ProviderStatus,
		CreatedAt:      ds.CreatedAt,
		UpdatedAt:      ds.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
