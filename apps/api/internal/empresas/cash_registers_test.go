package empresas_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	apigen "github.com/bacsystem/wanka/apps/api/internal/apigen/empresas"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

// CRUD y validaciones de cajas registradoras (#33).
func TestCashRegistersManagement(t *testing.T) {
	e := newEnv(t,
		string(permission.EmpresasSitesRead),
		string(permission.EmpresasSitesUpdate),
	)
	session := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(session.TenantID)

	var tenantID pgtype.UUID
	_ = tenantID.Scan(session.TenantID)

	siteID := e.seedSite(tenantID, "S001", "Sede 1")
	inactiveSiteID := e.seedSite(tenantID, "S002", "Sede Inactiva")
	e.exec(`UPDATE org.sites SET active = false WHERE id = $1`, inactiveSiteID)

	// 1. Crear caja exitosamente en local activo
	body, _ := json.Marshal(apigen.CreateCashRegisterRequest{
		SiteId: openapi_types.UUID(siteID.Bytes),
		Code:   "CAJA-01",
	})
	req := httptest.NewRequest("POST", "/v1/cash-registers", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("POST /v1/cash-registers status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var created apigen.CashRegister
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Code != "CAJA-01" || !created.Active {
		t.Errorf("created cash register mismatch: %+v", created)
	}

	// 2. Rechazar creación en local inactivo con 422
	bodyInactive, _ := json.Marshal(apigen.CreateCashRegisterRequest{
		SiteId: openapi_types.UUID(inactiveSiteID.Bytes),
		Code:   "CAJA-02",
	})
	req = httptest.NewRequest("POST", "/v1/cash-registers", bytes.NewReader(bodyInactive))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("POST /v1/cash-registers (inactive site) status = %d, want 422; body: %s", w.Code, w.Body.String())
	}

	// 3. Rechazar código de caja duplicado en la empresa con 409
	bodyDup, _ := json.Marshal(apigen.CreateCashRegisterRequest{
		SiteId: openapi_types.UUID(siteID.Bytes),
		Code:   "CAJA-01", // ya existe
	})
	req = httptest.NewRequest("POST", "/v1/cash-registers", bytes.NewReader(bodyDup))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("POST /v1/cash-registers (duplicate code) status = %d, want 409; body: %s", w.Code, w.Body.String())
	}

	// 4. Listar cajas (GET /v1/cash-registers)
	req = httptest.NewRequest("GET", "/v1/cash-registers?siteId="+siteID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+session.Token)
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /v1/cash-registers status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var list []apigen.CashRegister
	_ = json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 1 {
		t.Errorf("expected 1 cash register, got %d", len(list))
	}

	// 5. Desactivar caja (PATCH /v1/cash-registers/{id}) -> inactiva sus series
	crID := pgtype.UUID{Bytes: [16]byte(created.Id), Valid: true}
	seriesID := e.seedSeries(tenantID, siteID, crID, "01", "F001", 0, true, false, false)

	bodyUpdate, _ := json.Marshal(apigen.UpdateCashRegisterRequest{
		Active: ptr(false),
	})
	req = httptest.NewRequest("PATCH", "/v1/cash-registers/"+crID.String(), bytes.NewReader(bodyUpdate))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PATCH /v1/cash-registers/{id} status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	// Verificar que la serie de la caja quedó inactiva
	var sActive bool
	e.admin.QueryRow(t.Context(), `SELECT active FROM org.document_series WHERE id = $1`, seriesID).Scan(&sActive)
	if sActive {
		t.Errorf("series should be deactivated when cash register is deactivated")
	}
}

// Aislamiento multi-tenant de cajas (404 al intentar acceder a caja de otra empresa).
func TestCashRegisters_TenantIsolation(t *testing.T) {
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
	cajaBID := e.seedCashRegister(tenantBID, siteBID, "CAJA-B")

	// Tenant A pide caja de Tenant B
	req := httptest.NewRequest("GET", "/v1/cash-registers/"+cajaBID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+sessionA.Token)
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("cross-tenant GET /v1/cash-registers/{id} status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}
