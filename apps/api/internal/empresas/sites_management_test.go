package empresas_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	apigen "github.com/bacsystem/wanka/apps/api/internal/apigen/empresas"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

// AC1: Dueño crea local con código de anexo SUNAT (4 dígitos, único en la empresa; opcional si no emite).
func TestSitesManagement_Create(t *testing.T) {
	e := newEnv(t,
		string(permission.EmpresasSitesCreate),
		string(permission.EmpresasSitesRead),
	)
	session := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(session.TenantID)

	// 1. Crear local con anexo SUNAT válido (4 dígitos)
	body, _ := json.Marshal(apigen.CreateSiteRequest{
		Code:       "SEDE-01",
		Name:       "Sede Principal",
		Address:    ptr("Av. Principal 123"),
		SunatAnnex: ptr("0001"),
	})
	req := httptest.NewRequest("POST", "/v1/sites", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("POST /v1/sites status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var created apigen.Site
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Code != "SEDE-01" || created.Name != "Sede Principal" || created.SunatAnnex == nil || *created.SunatAnnex != "0001" {
		t.Errorf("created site mismatch: %+v", created)
	}

	// 2. Crear local sin anexo SUNAT (opcional, p. ej. solo almacén)
	bodyNoAnnex, _ := json.Marshal(apigen.CreateSiteRequest{
		Code:    "ALM-01",
		Name:    "Almacén Central",
		Address: ptr("Calle Depósito 456"),
	})
	req = httptest.NewRequest("POST", "/v1/sites", bytes.NewReader(bodyNoAnnex))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /v1/sites (no annex) status = %d, want 201; body: %s", w.Code, w.Body.String())
	}

	// 3. Rechazar anexo inválido (no 4 dígitos) con 422
	bodyBadAnnex, _ := json.Marshal(apigen.CreateSiteRequest{
		Code:       "SEDE-02",
		Name:       "Sede 2",
		SunatAnnex: ptr("123"), // 3 dígitos
	})
	req = httptest.NewRequest("POST", "/v1/sites", bytes.NewReader(bodyBadAnnex))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("POST /v1/sites (bad annex) status = %d, want 422; body: %s", w.Code, w.Body.String())
	}

	// 4. Rechazar anexo duplicado en la empresa con 409
	bodyDupAnnex, _ := json.Marshal(apigen.CreateSiteRequest{
		Code:       "SEDE-03",
		Name:       "Sede 3",
		SunatAnnex: ptr("0001"), // ya usado en SEDE-01
	})
	req = httptest.NewRequest("POST", "/v1/sites", bytes.NewReader(bodyDupAnnex))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("POST /v1/sites (duplicate annex) status = %d, want 409; body: %s", w.Code, w.Body.String())
	}

	// 5. Rechazar código duplicado en la empresa con 409
	bodyDupCode, _ := json.Marshal(apigen.CreateSiteRequest{
		Code:       "SEDE-01", // ya usado
		Name:       "Otra sede",
		SunatAnnex: ptr("0002"),
	})
	req = httptest.NewRequest("POST", "/v1/sites", bytes.NewReader(bodyDupCode))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("POST /v1/sites (duplicate code) status = %d, want 409; body: %s", w.Code, w.Body.String())
	}
}

// AC6: Dado un local con ventas cuando se desactiva entonces no admite nuevas ventas, sus cajas y series quedan inactivas en Wanka.
func TestSitesManagement_DeactivateCascade(t *testing.T) {
	e := newEnv(t,
		string(permission.EmpresasSitesCreate),
		string(permission.EmpresasSitesUpdate),
		string(permission.EmpresasSitesRead),
		string(permission.EmpresasSeriesCreate),
		string(permission.EmpresasSeriesRead),
	)
	session := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(session.TenantID)

	var tenantID pgtype.UUID
	_ = tenantID.Scan(session.TenantID)

	// Crear local con anexo
	siteID := e.seedSite(tenantID, "0001", "Sede Miraflores")
	e.exec(`UPDATE org.sites SET sunat_annex = '0001' WHERE id = $1`, siteID)

	// Crear caja en el local
	cajaID := e.seedCashRegister(tenantID, siteID, "CAJA-01")

	// Crear series en el local
	series1 := e.seedSeries(tenantID, siteID, cajaID, "01", "F001", 10, true, true, false)
	series2 := e.seedSeries(tenantID, siteID, cajaID, "03", "B001", 5, true, true, false)

	// Desactivar el local vía PATCH /v1/sites/{id}
	body, _ := json.Marshal(apigen.UpdateSiteRequest{
		Active: ptr(false),
	})
	req := httptest.NewRequest("PATCH", "/v1/sites/"+siteID.String(), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PATCH /v1/sites/{id} status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	// Verificar que el local quedó inactivo
	var siteActive bool
	e.admin.QueryRow(t.Context(), `SELECT active FROM org.sites WHERE id = $1`, siteID).Scan(&siteActive)
	if siteActive {
		t.Errorf("site should be inactive")
	}

	// AC6: Verificar que sus cajas quedaron inactivas
	var cajaActive bool
	e.admin.QueryRow(t.Context(), `SELECT active FROM org.cash_registers WHERE id = $1`, cajaID).Scan(&cajaActive)
	if cajaActive {
		t.Errorf("cash register should be cascaded to inactive")
	}

	// AC6: Verificar que sus series quedaron inactivas
	var s1Active, s2Active bool
	e.admin.QueryRow(t.Context(), `SELECT active FROM org.document_series WHERE id = $1`, series1).Scan(&s1Active)
	e.admin.QueryRow(t.Context(), `SELECT active FROM org.document_series WHERE id = $1`, series2).Scan(&s2Active)
	if s1Active || s2Active {
		t.Errorf("series should be cascaded to inactive: s1=%v, s2=%v", s1Active, s2Active)
	}
}

// AC10: Dado un local con alguna serie ya registrada en Khipu (o 0000) cuando se intenta cambiar su anexo entonces se rechaza (409 invalid_state_transition).
func TestSitesManagement_AnnexImmutableWithSyncedSeries(t *testing.T) {
	e := newEnv(t,
		string(permission.EmpresasSitesUpdate),
		string(permission.EmpresasSitesRead),
	)
	session := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(session.TenantID)

	var tenantID pgtype.UUID
	_ = tenantID.Scan(session.TenantID)

	// 1. Local con serie sincronizada
	siteID := e.seedSite(tenantID, "S001", "Sede con serie")
	e.exec(`UPDATE org.sites SET sunat_annex = '0001' WHERE id = $1`, siteID)
	cajaID := e.seedCashRegister(tenantID, siteID, "CAJA-01")
	_ = e.seedSeries(tenantID, siteID, cajaID, "01", "F001", 0, true, true, false) // provider_synced_at = now()

	// Intentar cambiar anexo de 0001 a 0002 -> 409
	body, _ := json.Marshal(apigen.UpdateSiteRequest{
		SunatAnnex: ptr("0002"),
	})
	req := httptest.NewRequest("PATCH", "/v1/sites/"+siteID.String(), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("PATCH /v1/sites/{id} (annex with synced series) status = %d, want 409; body: %s", w.Code, w.Body.String())
	}

	// 2. Local sin series sincronizadas -> cambiar anexo debe permitirse
	siteFreeID := e.seedSite(tenantID, "S002", "Sede libre")
	e.exec(`UPDATE org.sites SET sunat_annex = '0003' WHERE id = $1`, siteFreeID)

	bodyFree, _ := json.Marshal(apigen.UpdateSiteRequest{
		SunatAnnex: ptr("0004"),
	})
	req = httptest.NewRequest("PATCH", "/v1/sites/"+siteFreeID.String(), bytes.NewReader(bodyFree))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PATCH /v1/sites/{id} (free site annex change) status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

// AC14: Aislamiento multi-tenant (Tenant A no puede leer ni modificar local de Tenant B -> 404).
func TestSitesManagement_TenantIsolation(t *testing.T) {
	e := newEnv(t,
		string(permission.EmpresasSitesRead),
		string(permission.EmpresasSitesUpdate),
	)

	// Tenant A
	sessionA := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(sessionA.TenantID)
	// Tenant B
	sessionB := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(sessionB.TenantID)

	var tenantBID pgtype.UUID
	_ = tenantBID.Scan(sessionB.TenantID)
	siteBID := e.seedSite(tenantBID, "S-B", "Sede Tenant B")

	// Tenant A pide local de Tenant B
	req := httptest.NewRequest("GET", "/v1/sites/"+siteBID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+sessionA.Token)
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("cross-tenant GET /v1/sites/{id} status = %d, want 404; body: %s", w.Code, w.Body.String())
	}

	// Tenant A intenta modificar local de Tenant B
	body, _ := json.Marshal(apigen.UpdateSiteRequest{Name: ptr("Hack")})
	req = httptest.NewRequest("PATCH", "/v1/sites/"+siteBID.String(), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+sessionA.Token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("cross-tenant PATCH /v1/sites/{id} status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}

func ptr[T any](v T) *T {
	return &v
}
