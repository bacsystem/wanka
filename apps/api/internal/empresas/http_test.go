package empresas_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

type seriesResponse struct {
	ID             string  `json:"id"`
	SiteID         string  `json:"siteId"`
	CashRegisterID *string `json:"cashRegisterId"`
	DocType        string  `json:"docType"`
	Series         string  `json:"series"`
	LastNumber     int64   `json:"lastNumber"`
	Active         bool    `json:"active"`
	ProviderStatus string  `json:"providerStatus"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// AC8: Endpoint GET /v1/document-series/{id} devuelve el estado de la serie, último número, local, caja, tipo, activo y providerStatus.
func TestGetDocumentSeriesHTTP(t *testing.T) {
	e := newEnv(t, string(permission.EmpresasSeriesRead))
	session := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(session.TenantID)

	var tenantID pgtype.UUID
	if err := tenantID.Scan(session.TenantID); err != nil {
		t.Fatal(err)
	}
	siteID := e.seedSite(tenantID, "S001", "Sede Central")
	cajaID := e.seedCashRegister(tenantID, siteID, "CAJA-01")

	seriesID := e.seedSeries(tenantID, siteID, cajaID, "01", "F001", 42, true, true, false)

	req := httptest.NewRequest("GET", "/v1/document-series/"+seriesID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+session.Token)
	w := httptest.NewRecorder()

	e.h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /v1/document-series/{id} status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp seriesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response JSON: %v", err)
	}

	if resp.ID != seriesID.String() {
		t.Errorf("resp.ID = %s, want %s", resp.ID, seriesID.String())
	}
	if resp.SiteID != siteID.String() {
		t.Errorf("resp.SiteID = %s, want %s", resp.SiteID, siteID.String())
	}
	if resp.CashRegisterID == nil || *resp.CashRegisterID != cajaID.String() {
		t.Errorf("resp.CashRegisterID = %v, want %s", resp.CashRegisterID, cajaID.String())
	}
	if resp.DocType != "01" || resp.Series != "F001" {
		t.Errorf("resp docType/series = %s/%s, want 01/F001", resp.DocType, resp.Series)
	}
	if resp.LastNumber != 42 {
		t.Errorf("resp.LastNumber = %d, want 42", resp.LastNumber)
	}
	if !resp.Active {
		t.Errorf("resp.Active = false, want true")
	}
	if resp.ProviderStatus != "synced" {
		t.Errorf("resp.ProviderStatus = %s, want synced", resp.ProviderStatus)
	}
	if resp.CreatedAt == "" || resp.UpdatedAt == "" {
		t.Errorf("resp timestamps should not be empty")
	}
}

// AC8: Verificación de providerStatus enum (synced, pending, conflict, not_applicable)
func TestGetDocumentSeriesProviderStatus(t *testing.T) {
	e := newEnv(t, string(permission.EmpresasSeriesRead))
	session := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(session.TenantID)

	var tenantID pgtype.UUID
	_ = tenantID.Scan(session.TenantID)
	siteID := e.seedSite(tenantID, "S001", "Sede Central")

	cases := []struct {
		name       string
		docType    string
		series     string
		synced     bool
		conflict   bool
		wantStatus string
	}{
		{"CPE sincronizada", "01", "F001", true, false, "synced"},
		{"CPE pendiente", "03", "B001", false, false, "pending"},
		{"CPE en conflicto", "07", "FC01", true, true, "conflict"},
		{"Interna no aplicable", "PRO", "PR01", false, false, "not_applicable"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sID := e.seedSeries(tenantID, siteID, pgtype.UUID{}, tc.docType, tc.series, 0, true, tc.synced, tc.conflict)

			req := httptest.NewRequest("GET", "/v1/document-series/"+sID.String(), nil)
			req.Header.Set("Authorization", "Bearer "+session.Token)
			w := httptest.NewRecorder()

			e.h.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
			}

			var resp seriesResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decoding JSON: %v", err)
			}
			if resp.ProviderStatus != tc.wantStatus {
				t.Errorf("providerStatus = %s, want %s", resp.ProviderStatus, tc.wantStatus)
			}
		})
	}
}

// AC9: Intentar acceder a una serie de otro tenant vía API devuelve not_found (aislamiento RLS).
func TestGetDocumentSeriesTenantIsolation(t *testing.T) {
	e := newEnv(t, string(permission.EmpresasSeriesRead))

	// Tenant A
	sessionA := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(sessionA.TenantID)
	// Tenant B
	sessionB := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(sessionB.TenantID)

	var tenantBID pgtype.UUID
	_ = tenantBID.Scan(sessionB.TenantID)
	siteBID := e.seedSite(tenantBID, "S002", "Sede B")
	seriesBID := e.seedSeries(tenantBID, siteBID, pgtype.UUID{}, "01", "F001", 10, true, true, false)

	// El Tenant A intenta consultar la serie que pertenece al Tenant B
	req := httptest.NewRequest("GET", "/v1/document-series/"+seriesBID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+sessionA.Token)
	w := httptest.NewRecorder()

	e.h.ServeHTTP(w, req)

	// RLS debe aislar los datos: el Tenant A recibe 404 (no existe en su tenant)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant access status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}

// Reversiones y rechazos en el endpoint HTTP: malformed_request (400), not_found (404), unauthorized (401), forbidden (403).
func TestGetDocumentSeriesRejections(t *testing.T) {
	e := newEnv(t, string(permission.EmpresasSeriesRead))
	session := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(session.TenantID)

	// 1. UUID mal formado -> 400
	req := httptest.NewRequest("GET", "/v1/document-series/not-a-valid-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+session.Token)
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("malformed UUID status = %d, want 400; body: %s", w.Code, w.Body.String())
	}

	// 2. UUID inexistente -> 404
	req = httptest.NewRequest("GET", "/v1/document-series/00000000-0000-0000-0000-000000000000", nil)
	req.Header.Set("Authorization", "Bearer "+session.Token)
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("non-existent series status = %d, want 404; body: %s", w.Code, w.Body.String())
	}

	// 3. Petición no autenticada -> 401
	req = httptest.NewRequest("GET", "/v1/document-series/11111111-1111-1111-1111-111111111111", nil)
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated status = %d, want 401; body: %s", w.Code, w.Body.String())
	}

	// 4. Sesión sin permiso empresas.series:read -> 403
	envNoPerm := newEnv(t) // sin permisos
	sessionNoPerm := envNoPerm.d.SeedSession(t, dbtest.SessionSpec{})
	envNoPerm.subscribe(sessionNoPerm.TenantID)
	req = httptest.NewRequest("GET", "/v1/document-series/11111111-1111-1111-1111-111111111111", nil)
	req.Header.Set("Authorization", "Bearer "+sessionNoPerm.Token)
	w = httptest.NewRecorder()
	envNoPerm.h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("forbidden status = %d, want 403; body: %s", w.Code, w.Body.String())
	}
}
