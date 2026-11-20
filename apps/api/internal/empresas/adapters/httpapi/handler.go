// Package httpapi provee adaptadores HTTP para el módulo de empresas (#33, #47).
package httpapi

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	apigen "github.com/bacsystem/wanka/apps/api/internal/apigen/empresas"
	"github.com/bacsystem/wanka/apps/api/internal/empresas/adapters/postgres"
	"github.com/bacsystem/wanka/apps/api/internal/empresas/domain"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

func newUUID() pgtype.UUID {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return pgtype.UUID{Bytes: b, Valid: true}
}

// Handler implementa apigen.ServerInterface para el módulo de empresas.
type Handler struct {
	pool   *db.Pool
	store  *postgres.Store
	client domain.ProviderClient
}

// NewHandler construye una nueva instancia del Handler.
func NewHandler(pool *db.Pool, client domain.ProviderClient) *Handler {
	return &Handler{
		pool:   pool,
		store:  postgres.NewStore(pool),
		client: client,
	}
}

func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrSiteNotFound):
		httpx.WriteProblem(w, r, "not_found", "site not found")
	case errors.Is(err, domain.ErrCashRegisterNotFound):
		httpx.WriteProblem(w, r, "not_found", "cash register not found")
	case errors.Is(err, domain.ErrSeriesNotFound):
		httpx.WriteProblem(w, r, "not_found", "document series not found")
	case errors.Is(err, domain.ErrDuplicateCode):
		httpx.WriteProblem(w, r, "duplicate", "code already in use")
	case errors.Is(err, domain.ErrDuplicateAnnex):
		httpx.WriteProblem(w, r, "duplicate", "sunat annex already in use")
	case errors.Is(err, domain.ErrDuplicateSeries):
		httpx.WriteProblem(w, r, "duplicate", "series already in use")
	case errors.Is(err, domain.ErrAnnexImmutableWithSyncedSeries):
		httpx.WriteProblem(w, r, "invalid_state_transition", "cannot change sunat annex of site with synced series")
	case errors.Is(err, domain.ErrLastNumberImmutableWhenSynced):
		httpx.WriteProblem(w, r, "invalid_state_transition", "last number cannot be changed when synced or in conflict")
	case errors.Is(err, domain.ErrSiteImmutableWhenSynced):
		httpx.WriteProblem(w, r, "invalid_state_transition", "site cannot be changed when series is synced")
	case errors.Is(err, domain.ErrSeriesConflict):
		httpx.WriteProblem(w, r, "invalid_state_transition", "series in conflict with provider")
	case errors.Is(err, domain.ErrCPERequiresSunatAnnex):
		httpx.WriteProblem(w, r, "validation_failed", "cpe series requires site with sunat annex")
	case errors.Is(err, domain.ErrSiteInactive):
		httpx.WriteProblem(w, r, "validation_failed", "site is inactive")
	case errors.Is(err, domain.ErrCashRegisterInactive):
		httpx.WriteProblem(w, r, "validation_failed", "cash register is inactive")
	case errors.Is(err, domain.ErrCashRegisterSiteMismatch):
		httpx.WriteProblem(w, r, "validation_failed", "cash register belongs to another site")
	case errors.Is(err, domain.ErrInvalidSeriesFormat):
		httpx.WriteProblem(w, r, "validation_failed", "invalid series format for document type")
	case errors.Is(err, domain.ErrInvalidAnnexFormat):
		httpx.WriteProblem(w, r, "validation_failed", "invalid sunat annex format")
	case errors.Is(err, domain.ErrInvalidCode):
		httpx.WriteProblem(w, r, "validation_failed", "invalid code format")
	case errors.Is(err, domain.ErrPlanLimitReached):
		httpx.WriteProblem(w, r, "plan_limit_reached", "maximum sites limit reached for current subscription")
	default:
		httpx.WriteError(w, r, err)
	}
}

func toAPISite(s domain.Site) apigen.Site {
	return apigen.Site{
		Id:         openapi_types.UUID(s.ID.Bytes),
		Code:       s.Code,
		Name:       s.Name,
		Address:    s.Address,
		SunatAnnex: s.SunatAnnex,
		Active:     s.Active,
		Version:    s.Version,
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
	}
}

func toAPICashRegister(cr domain.CashRegister) apigen.CashRegister {
	return apigen.CashRegister{
		Id:        openapi_types.UUID(cr.ID.Bytes),
		SiteId:    openapi_types.UUID(cr.SiteID.Bytes),
		Code:      cr.Code,
		Active:    cr.Active,
		CreatedAt: cr.CreatedAt,
		UpdatedAt: cr.UpdatedAt,
	}
}

func toAPIDocumentSeries(ds domain.DocumentSeries) apigen.DocumentSeries {
	var crID *openapi_types.UUID
	if ds.CashRegisterID.Valid && ds.CashRegisterID.Bytes != [16]byte{} {
		id := openapi_types.UUID(ds.CashRegisterID.Bytes)
		crID = &id
	}

	return apigen.DocumentSeries{
		Id:             openapi_types.UUID(ds.ID.Bytes),
		SiteId:         openapi_types.UUID(ds.SiteID.Bytes),
		CashRegisterId: crID,
		DocType:        ds.DocType,
		Series:         ds.Series,
		LastNumber:     ds.LastNumber,
		Active:         ds.Active,
		ProviderStatus: apigen.DocumentSeriesProviderStatus(ds.ProviderStatus),
		CreatedAt:      ds.CreatedAt,
		UpdatedAt:      ds.UpdatedAt,
	}
}

// ==================== Sites Handlers ====================

// ListSites (GET /v1/sites)
func (h *Handler) ListSites(w http.ResponseWriter, r *http.Request, params apigen.ListSitesParams) {
	session, ok := auth.FromContext(r.Context())
	if !ok || !session.HasTenant() {
		httpx.WriteProblem(w, r, "unauthorized", "tenant session required")
		return
	}

	var sites []domain.Site
	err := h.pool.InTenantReadTx(r.Context(), session.Actor, func(tx pgx.Tx) error {
		var txErr error
		sites, txErr = h.store.ListSites(r.Context(), tx, params.Active)
		return txErr
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	resp := make([]apigen.Site, 0, len(sites))
	for _, s := range sites {
		resp = append(resp, toAPISite(s))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CreateSite (POST /v1/sites)
func (h *Handler) CreateSite(w http.ResponseWriter, r *http.Request, _ apigen.CreateSiteParams) {
	session, ok := auth.FromContext(r.Context())
	if !ok || !session.HasTenant() {
		httpx.WriteProblem(w, r, "unauthorized", "tenant session required")
		return
	}

	var req apigen.CreateSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, "malformed_request", "invalid request body")
		return
	}

	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	if req.Code == "" || req.Name == "" {
		httpx.WriteProblem(w, r, "validation_failed", "code and name are required")
		return
	}

	site := domain.Site{
		ID:         newUUID(),
		TenantID:   session.Actor.TenantID,
		Code:       req.Code,
		Name:       req.Name,
		Address:    req.Address,
		SunatAnnex: req.SunatAnnex,
		Active:     true,
	}

	var created domain.Site
	err := h.pool.InTenantTx(r.Context(), session.Actor, func(tx pgx.Tx) error {
		var txErr error
		created, txErr = h.store.CreateSite(r.Context(), tx, site)
		return txErr
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPISite(created))
}

// GetSite (GET /v1/sites/{id})
func (h *Handler) GetSite(w http.ResponseWriter, r *http.Request, id openapi_types.UUID, _ apigen.GetSiteParams) {
	session, ok := auth.FromContext(r.Context())
	if !ok || !session.HasTenant() {
		httpx.WriteProblem(w, r, "unauthorized", "tenant session required")
		return
	}

	siteID := pgtype.UUID{Bytes: [16]byte(id), Valid: true}
	var site domain.Site
	err := h.pool.InTenantReadTx(r.Context(), session.Actor, func(tx pgx.Tx) error {
		var txErr error
		site, txErr = h.store.GetSite(r.Context(), tx, siteID)
		return txErr
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toAPISite(site))
}

// UpdateSite (PATCH /v1/sites/{id})
func (h *Handler) UpdateSite(w http.ResponseWriter, r *http.Request, id openapi_types.UUID, _ apigen.UpdateSiteParams) {
	session, ok := auth.FromContext(r.Context())
	if !ok || !session.HasTenant() {
		httpx.WriteProblem(w, r, "unauthorized", "tenant session required")
		return
	}

	var req apigen.UpdateSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, "malformed_request", "invalid request body")
		return
	}

	siteID := pgtype.UUID{Bytes: [16]byte(id), Valid: true}
	var updated domain.Site
	err := h.pool.InTenantTx(r.Context(), session.Actor, func(tx pgx.Tx) error {
		var txErr error
		updated, txErr = h.store.UpdateSite(r.Context(), tx, siteID, req.Name, req.Address, req.SunatAnnex, req.Active)
		return txErr
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toAPISite(updated))
}

// ==================== Cash Registers Handlers ====================

// ListCashRegisters (GET /v1/cash-registers)
func (h *Handler) ListCashRegisters(w http.ResponseWriter, r *http.Request, params apigen.ListCashRegistersParams) {
	session, ok := auth.FromContext(r.Context())
	if !ok || !session.HasTenant() {
		httpx.WriteProblem(w, r, "unauthorized", "tenant session required")
		return
	}

	var siteID *pgtype.UUID
	if params.SiteId != nil {
		sid := pgtype.UUID{Bytes: [16]byte(*params.SiteId), Valid: true}
		siteID = &sid
	}

	var list []domain.CashRegister
	err := h.pool.InTenantReadTx(r.Context(), session.Actor, func(tx pgx.Tx) error {
		var txErr error
		list, txErr = h.store.ListCashRegisters(r.Context(), tx, siteID, params.Active)
		return txErr
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	resp := make([]apigen.CashRegister, 0, len(list))
	for _, cr := range list {
		resp = append(resp, toAPICashRegister(cr))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CreateCashRegister (POST /v1/cash-registers)
func (h *Handler) CreateCashRegister(w http.ResponseWriter, r *http.Request, _ apigen.CreateCashRegisterParams) {
	session, ok := auth.FromContext(r.Context())
	if !ok || !session.HasTenant() {
		httpx.WriteProblem(w, r, "unauthorized", "tenant session required")
		return
	}

	var req apigen.CreateCashRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, "malformed_request", "invalid request body")
		return
	}

	req.Code = strings.TrimSpace(req.Code)
	if req.Code == "" {
		httpx.WriteProblem(w, r, "validation_failed", "code is required")
		return
	}

	cr := domain.CashRegister{
		ID:       newUUID(),
		TenantID: session.Actor.TenantID,
		SiteID:   pgtype.UUID{Bytes: [16]byte(req.SiteId), Valid: true},
		Code:     req.Code,
		Active:   true,
	}

	var created domain.CashRegister
	err := h.pool.InTenantTx(r.Context(), session.Actor, func(tx pgx.Tx) error {
		var txErr error
		created, txErr = h.store.CreateCashRegister(r.Context(), tx, cr)
		return txErr
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPICashRegister(created))
}

// GetCashRegister (GET /v1/cash-registers/{id})
func (h *Handler) GetCashRegister(w http.ResponseWriter, r *http.Request, id openapi_types.UUID, _ apigen.GetCashRegisterParams) {
	session, ok := auth.FromContext(r.Context())
	if !ok || !session.HasTenant() {
		httpx.WriteProblem(w, r, "unauthorized", "tenant session required")
		return
	}

	crID := pgtype.UUID{Bytes: [16]byte(id), Valid: true}
	var cr domain.CashRegister
	err := h.pool.InTenantReadTx(r.Context(), session.Actor, func(tx pgx.Tx) error {
		var txErr error
		cr, txErr = h.store.GetCashRegister(r.Context(), tx, crID)
		return txErr
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toAPICashRegister(cr))
}

// UpdateCashRegister (PATCH /v1/cash-registers/{id})
func (h *Handler) UpdateCashRegister(w http.ResponseWriter, r *http.Request, id openapi_types.UUID, _ apigen.UpdateCashRegisterParams) {
	session, ok := auth.FromContext(r.Context())
	if !ok || !session.HasTenant() {
		httpx.WriteProblem(w, r, "unauthorized", "tenant session required")
		return
	}

	var req apigen.UpdateCashRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, "malformed_request", "invalid request body")
		return
	}

	crID := pgtype.UUID{Bytes: [16]byte(id), Valid: true}
	var updated domain.CashRegister
	err := h.pool.InTenantTx(r.Context(), session.Actor, func(tx pgx.Tx) error {
		var txErr error
		updated, txErr = h.store.UpdateCashRegister(r.Context(), tx, crID, req.Code, req.Active)
		return txErr
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toAPICashRegister(updated))
}

// ==================== Document Series Handlers ====================

// ListDocumentSeries (GET /v1/document-series)
func (h *Handler) ListDocumentSeries(w http.ResponseWriter, r *http.Request, params apigen.ListDocumentSeriesParams) {
	session, ok := auth.FromContext(r.Context())
	if !ok || !session.HasTenant() {
		httpx.WriteProblem(w, r, "unauthorized", "tenant session required")
		return
	}

	var siteID *pgtype.UUID
	if params.SiteId != nil {
		sid := pgtype.UUID{Bytes: [16]byte(*params.SiteId), Valid: true}
		siteID = &sid
	}

	var crID *pgtype.UUID
	if params.CashRegisterId != nil {
		crid := pgtype.UUID{Bytes: [16]byte(*params.CashRegisterId), Valid: true}
		crID = &crid
	}

	var list []domain.DocumentSeries
	err := h.pool.InTenantReadTx(r.Context(), session.Actor, func(tx pgx.Tx) error {
		var txErr error
		list, txErr = h.store.ListDocumentSeries(r.Context(), tx, siteID, crID, params.DocType, params.Active)
		return txErr
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	resp := make([]apigen.DocumentSeries, 0, len(list))
	for _, ds := range list {
		resp = append(resp, toAPIDocumentSeries(ds))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CreateDocumentSeries (POST /v1/document-series)
func (h *Handler) CreateDocumentSeries(w http.ResponseWriter, r *http.Request, _ apigen.CreateDocumentSeriesParams) {
	session, ok := auth.FromContext(r.Context())
	if !ok || !session.HasTenant() {
		httpx.WriteProblem(w, r, "unauthorized", "tenant session required")
		return
	}

	var req apigen.CreateDocumentSeriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, "malformed_request", "invalid request body")
		return
	}

	req.Series = strings.TrimSpace(req.Series)
	if req.Series == "" {
		httpx.WriteProblem(w, r, "validation_failed", "series is required")
		return
	}

	var lastNum int64
	if req.InitialNumber != nil {
		lastNum = *req.InitialNumber
	}

	var cashRegisterID pgtype.UUID
	if req.CashRegisterId != nil {
		cashRegisterID = pgtype.UUID{Bytes: [16]byte(*req.CashRegisterId), Valid: true}
	}

	ds := domain.DocumentSeries{
		ID:             newUUID(),
		TenantID:       session.Actor.TenantID,
		SiteID:         pgtype.UUID{Bytes: [16]byte(req.SiteId), Valid: true},
		CashRegisterID: cashRegisterID,
		DocType:        string(req.DocType),
		Series:         req.Series,
		LastNumber:     lastNum,
		Active:         true,
	}

	var created domain.DocumentSeries
	err := h.pool.InTenantTx(r.Context(), session.Actor, func(tx pgx.Tx) error {
		var txErr error
		created, txErr = h.store.CreateDocumentSeries(r.Context(), tx, ds)
		return txErr
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPIDocumentSeries(created))
}

// GetDocumentSeries (GET /v1/document-series/{id})
func (h *Handler) GetDocumentSeries(w http.ResponseWriter, r *http.Request, id openapi_types.UUID, _ apigen.GetDocumentSeriesParams) {
	session, ok := auth.FromContext(r.Context())
	if !ok || !session.HasTenant() {
		httpx.WriteProblem(w, r, "unauthorized", "tenant session required")
		return
	}

	seriesID := pgtype.UUID{Bytes: [16]byte(id), Valid: true}
	var ds domain.DocumentSeries
	err := h.pool.InTenantReadTx(r.Context(), session.Actor, func(tx pgx.Tx) error {
		var txErr error
		ds, txErr = h.store.GetDocumentSeries(r.Context(), tx, seriesID)
		return txErr
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toAPIDocumentSeries(ds))
}

// UpdateDocumentSeries (PATCH /v1/document-series/{id})
func (h *Handler) UpdateDocumentSeries(w http.ResponseWriter, r *http.Request, id openapi_types.UUID, _ apigen.UpdateDocumentSeriesParams) {
	session, ok := auth.FromContext(r.Context())
	if !ok || !session.HasTenant() {
		httpx.WriteProblem(w, r, "unauthorized", "tenant session required")
		return
	}

	var req apigen.UpdateDocumentSeriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, "malformed_request", "invalid request body")
		return
	}

	var cashRegisterID *pgtype.UUID
	if req.CashRegisterId != nil {
		crid := pgtype.UUID{Bytes: [16]byte(*req.CashRegisterId), Valid: true}
		cashRegisterID = &crid
	}

	seriesID := pgtype.UUID{Bytes: [16]byte(id), Valid: true}
	var updated domain.DocumentSeries
	err := h.pool.InTenantTx(r.Context(), session.Actor, func(tx pgx.Tx) error {
		var txErr error
		updated, txErr = h.store.UpdateDocumentSeries(r.Context(), tx, seriesID, nil, cashRegisterID, req.LastNumber, req.Active, h.client)
		return txErr
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toAPIDocumentSeries(updated))
}
