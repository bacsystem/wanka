package empresas_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	apigen "github.com/bacsystem/wanka/apps/api/internal/apigen/empresas"
	"github.com/bacsystem/wanka/apps/api/internal/empresas"
	"github.com/bacsystem/wanka/apps/api/internal/empresas/adapters/postgres"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

type fakeProviderClient struct {
	remoteSeries []empresas.RemoteSeries
}

func (f *fakeProviderClient) GetSeries(_ context.Context, _ pgtype.UUID) ([]empresas.RemoteSeries, error) {
	return f.remoteSeries, nil
}

// AC2 & AC3: Creación de series, validación de formato por tipo y requisito de anexo SUNAT para CPE.
func TestSeriesManagement_CreateAndFormatValidation(t *testing.T) {
	e := newEnv(t,
		string(permission.EmpresasSeriesCreate),
		string(permission.EmpresasSeriesRead),
	)
	session := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(session.TenantID)

	var tenantID pgtype.UUID
	_ = tenantID.Scan(session.TenantID)

	// Local con anexo SUNAT
	siteID := e.seedSite(tenantID, "S001", "Sede Emisora")
	e.exec(`UPDATE org.sites SET sunat_annex = '0001' WHERE id = $1`, siteID)

	// Local sin anexo SUNAT
	siteNoAnnexID := e.seedSite(tenantID, "S002", "Almacén Sin Anexo")

	// 1. Series CPE válidas en local con anexo
	validCases := []struct {
		docType apigen.CreateDocumentSeriesRequestDocType
		series  string
	}{
		{"01", "F001"},
		{"03", "B001"},
		{"07", "FC01"},
		{"07", "BC01"},
		{"08", "FD01"},
		{"08", "BD01"},
		{"09", "T001"},
		{"PRO", "PR01"},
		{"OC", "OC2026"},
		{"Z", "Z01"},
	}

	for _, tc := range validCases {
		t.Run("valid_"+string(tc.docType)+"_"+tc.series, func(t *testing.T) {
			body, _ := json.Marshal(apigen.CreateDocumentSeriesRequest{
				SiteId:  openapi_types.UUID(siteID.Bytes),
				DocType: tc.docType,
				Series:  tc.series,
			})
			req := httptest.NewRequest("POST", "/v1/document-series", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+session.Token)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			e.h.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Fatalf("POST /v1/document-series (%s %s) status = %d, want 201; body: %s", tc.docType, tc.series, w.Code, w.Body.String())
			}
		})
	}

	// 2. AC3: Series con formato inválido para su tipo son rechazadas con 422
	invalidCases := []struct {
		name    string
		docType apigen.CreateDocumentSeriesRequestDocType
		series  string
	}{
		{"Factura con B", "01", "B001"},
		{"Boleta con F", "03", "F001"},
		{"GRE sin T", "09", "F001"},
		{"Factura con longitud corta", "01", "F1"},
		{"Serie interna con minúsculas o símbolos", "PRO", "pro-1"},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(apigen.CreateDocumentSeriesRequest{
				SiteId:  openapi_types.UUID(siteID.Bytes),
				DocType: tc.docType,
				Series:  tc.series,
			})
			req := httptest.NewRequest("POST", "/v1/document-series", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+session.Token)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			e.h.ServeHTTP(w, req)

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("POST /v1/document-series (%s %s) status = %d, want 422; body: %s", tc.docType, tc.series, w.Code, w.Body.String())
			}
		})
	}

	// 3. AC2: Serie CPE en local sin anexo SUNAT -> rechazado con 422
	bodyNoAnnex, _ := json.Marshal(apigen.CreateDocumentSeriesRequest{
		SiteId:  openapi_types.UUID(siteNoAnnexID.Bytes),
		DocType: "01",
		Series:  "F002",
	})
	req := httptest.NewRequest("POST", "/v1/document-series", bytes.NewReader(bodyNoAnnex))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("CPE in site without annex status = %d, want 422; body: %s", w.Code, w.Body.String())
	}

	// 4. Serie interna en local sin anexo SUNAT -> permitido
	bodyInternal, _ := json.Marshal(apigen.CreateDocumentSeriesRequest{
		SiteId:  openapi_types.UUID(siteNoAnnexID.Bytes),
		DocType: "PRO",
		Series:  "PR02",
	})
	req = httptest.NewRequest("POST", "/v1/document-series", bytes.NewReader(bodyInternal))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("internal series in site without annex status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

// AC4: Serie duplicada en la empresa (mismo tipo y código) se rechaza con 409 duplicate.
func TestSeriesManagement_Duplicate(t *testing.T) {
	e := newEnv(t,
		string(permission.EmpresasSeriesCreate),
		string(permission.EmpresasSeriesRead),
	)
	session := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(session.TenantID)

	var tenantID pgtype.UUID
	_ = tenantID.Scan(session.TenantID)

	siteID := e.seedSite(tenantID, "S001", "Sede 1")
	e.exec(`UPDATE org.sites SET sunat_annex = '0001' WHERE id = $1`, siteID)

	// Crear F001 inicial
	body, _ := json.Marshal(apigen.CreateDocumentSeriesRequest{
		SiteId:  openapi_types.UUID(siteID.Bytes),
		DocType: "01",
		Series:  "F001",
	})
	req := httptest.NewRequest("POST", "/v1/document-series", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first creation status = %d, want 201", w.Code)
	}

	// Intentar crear F001 nuevamente para tipo 01 -> 409
	reqDup := httptest.NewRequest("POST", "/v1/document-series", bytes.NewReader(body))
	reqDup.Header.Set("Authorization", "Bearer "+session.Token)
	reqDup.Header.Set("Content-Type", "application/json")
	wDup := httptest.NewRecorder()
	e.h.ServeHTTP(wDup, reqDup)
	if wDup.Code != http.StatusConflict {
		t.Fatalf("duplicate series status = %d, want 409; body: %s", wDup.Code, wDup.Body.String())
	}
}

// AC7: Dado una serie cuando se desactiva entonces deja de numerar.
func TestSeriesManagement_Deactivate(t *testing.T) {
	e := newEnv(t,
		string(permission.EmpresasSeriesUpdate),
		string(permission.EmpresasSeriesRead),
	)
	session := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(session.TenantID)

	var tenantID pgtype.UUID
	_ = tenantID.Scan(session.TenantID)

	siteID := e.seedSite(tenantID, "S001", "Sede 1")
	seriesID := e.seedSeries(tenantID, siteID, pgtype.UUID{}, "01", "F001", 10, true, true, false)

	// Desactivar serie vía PATCH
	body, _ := json.Marshal(apigen.UpdateDocumentSeriesRequest{
		Active: ptr(false),
	})
	req := httptest.NewRequest("PATCH", "/v1/document-series/"+seriesID.String(), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PATCH /v1/document-series/{id} status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var updated apigen.DocumentSeries
	_ = json.NewDecoder(w.Body).Decode(&updated)
	if updated.Active {
		t.Errorf("series should be inactive")
	}
}

// AC8: Dado una serie inactiva cuando se reactiva y Khipu tiene un último número mayor o en otro anexo,
// entonces la reactivación se rechaza y la serie queda en conflicto en ese mismo momento permanentemente.
func TestSeriesManagement_ReactivateConflict(t *testing.T) {
	fakeClient := &fakeProviderClient{
		remoteSeries: []empresas.RemoteSeries{
			// En Khipu el correlativo remoto está en 50 (> local 20)
			{DocType: "01", Series: "F001", UltimoNumero: 50, Anexo: "0001"},
		},
	}

	e := newEnv(t,
		string(permission.EmpresasSeriesUpdate),
		string(permission.EmpresasSeriesRead),
	)
	session := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(session.TenantID)

	var tenantID pgtype.UUID
	_ = tenantID.Scan(session.TenantID)

	siteID := e.seedSite(tenantID, "S001", "Sede 1")
	e.exec(`UPDATE org.sites SET sunat_annex = '0001' WHERE id = $1`, siteID)

	// Serie inactiva, pero ya sincronizada previamente con local last_number = 20
	seriesID := e.seedSeries(tenantID, siteID, pgtype.UUID{}, "01", "F001", 20, false, true, false)

	// Intentar reactivar pasando por la verificación de Khipu
	actor := e.tenantActor(tenantID)
	store := postgres.NewStore(e.pool)
	err := e.pool.InTenantTx(t.Context(), actor, func(tx pgx.Tx) error {
		_, err := store.UpdateDocumentSeries(t.Context(), tx, seriesID, nil, nil, nil, ptr(true), fakeClient)
		return err
	})

	if err == nil {
		t.Fatalf("expected error on reactivation with disparity, got nil")
	}

	// AC8: Verificar que la serie quedó en conflicto en la base de datos
	var conflictAt *string
	e.admin.QueryRow(t.Context(), `SELECT provider_conflict_at::text FROM org.document_series WHERE id = $1`, seriesID).Scan(&conflictAt)
	if conflictAt == nil {
		t.Fatalf("series should be immediately frozen with provider_conflict_at")
	}
}

// AC9: Dado una serie ya registrada en Khipu cuando se intenta cambiar su local entonces se rechaza;
// la caja de una serie sí puede cambiar dentro del mismo local, pero no a otro local.
func TestSeriesManagement_EditConstraints(t *testing.T) {
	e := newEnv(t,
		string(permission.EmpresasSeriesUpdate),
		string(permission.EmpresasSeriesRead),
	)
	session := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(session.TenantID)

	var tenantID pgtype.UUID
	_ = tenantID.Scan(session.TenantID)

	site1ID := e.seedSite(tenantID, "S001", "Sede 1")
	site2ID := e.seedSite(tenantID, "S002", "Sede 2")
	caja1 := e.seedCashRegister(tenantID, site1ID, "CAJA-01")
	caja2 := e.seedCashRegister(tenantID, site1ID, "CAJA-02")
	cajaSite2 := e.seedCashRegister(tenantID, site2ID, "CAJA-S2")

	// Serie ya sincronizada en local 1, caja 1
	seriesID := e.seedSeries(tenantID, site1ID, caja1, "01", "F001", 10, true, true, false)

	// 1. Cambiar de caja dentro del mismo local (caja1 -> caja2) -> Permitido
	actor := e.tenantActor(tenantID)
	err := e.pool.InTenantTx(t.Context(), actor, func(tx pgx.Tx) error {
		updated, err := empresas.UpdateDocumentSeries(t.Context(), tx, seriesID, nil, &caja2, nil, nil, nil)
		if err != nil {
			return err
		}
		if updated.CashRegisterID != caja2 {
			t.Errorf("cash register was not updated")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("changing cash register within same site failed: %v", err)
	}

	// 2. Intentar cambiar de local una serie ya sincronizada -> 409
	err = e.pool.InTenantTx(t.Context(), actor, func(tx pgx.Tx) error {
		_, err := empresas.UpdateDocumentSeries(t.Context(), tx, seriesID, &site2ID, nil, nil, nil, nil)
		return err
	})
	if err == nil {
		t.Fatalf("changing site of synced series should be rejected")
	}

	// 3. Intentar asignar una caja que pertenece a otro local -> Rechazado con mismatch
	err = e.pool.InTenantTx(t.Context(), actor, func(tx pgx.Tx) error {
		_, err := empresas.UpdateDocumentSeries(t.Context(), tx, seriesID, nil, &cajaSite2, nil, nil, nil)
		return err
	})
	if err == nil {
		t.Fatalf("assigning cash register from different site should be rejected")
	}
}

// AC11: Dado que el último número de una serie se edita mientras no está sincronizada -> permitido;
// una vez sincronizada con Khipu -> rechazado con 409.
func TestSeriesManagement_EditLastNumber(t *testing.T) {
	e := newEnv(t,
		string(permission.EmpresasSeriesUpdate),
		string(permission.EmpresasSeriesRead),
	)
	session := e.d.SeedSession(t, dbtest.SessionSpec{})
	e.subscribe(session.TenantID)

	var tenantID pgtype.UUID
	_ = tenantID.Scan(session.TenantID)

	siteID := e.seedSite(tenantID, "S001", "Sede 1")

	// 1. Serie pendiente (no sincronizada aún)
	pendingSeriesID := e.seedSeries(tenantID, siteID, pgtype.UUID{}, "01", "F001", 0, true, false, false)

	body, _ := json.Marshal(apigen.UpdateDocumentSeriesRequest{
		LastNumber: ptr(int64(100)),
	})
	req := httptest.NewRequest("PATCH", "/v1/document-series/"+pendingSeriesID.String(), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("editing last number of pending series status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	// 2. Serie sincronizada con Khipu -> intentar editar lastNumber da 409
	syncedSeriesID := e.seedSeries(tenantID, siteID, pgtype.UUID{}, "03", "B001", 50, true, true, false)

	bodySynced, _ := json.Marshal(apigen.UpdateDocumentSeriesRequest{
		LastNumber: ptr(int64(200)),
	})
	reqSynced := httptest.NewRequest("PATCH", "/v1/document-series/"+syncedSeriesID.String(), bytes.NewReader(bodySynced))
	reqSynced.Header.Set("Authorization", "Bearer "+session.Token)
	reqSynced.Header.Set("Content-Type", "application/json")
	wSynced := httptest.NewRecorder()
	e.h.ServeHTTP(wSynced, reqSynced)

	if wSynced.Code != http.StatusConflict {
		t.Fatalf("editing last number of synced series status = %d, want 409; body: %s", wSynced.Code, wSynced.Body.String())
	}
}

// AC13: Permisos de acceso (403 si el usuario carece del permiso de lectura, creación o actualización).
func TestSeriesManagement_Permissions(t *testing.T) {
	// Entorno sin permisos
	envNoPerm := newEnv(t)
	session := envNoPerm.d.SeedSession(t, dbtest.SessionSpec{})
	envNoPerm.subscribe(session.TenantID)

	// 1. POST /v1/sites sin empresas.sites:create -> 403
	body, _ := json.Marshal(apigen.CreateSiteRequest{Code: "S001", Name: "Sede"})
	req := httptest.NewRequest("POST", "/v1/sites", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	envNoPerm.h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("POST /v1/sites without permission status = %d, want 403", w.Code)
	}

	// 2. POST /v1/document-series sin empresas.series:create -> 403
	bodySeries, _ := json.Marshal(apigen.CreateDocumentSeriesRequest{
		DocType: "01",
		Series:  "F001",
	})
	req = httptest.NewRequest("POST", "/v1/document-series", bytes.NewReader(bodySeries))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	envNoPerm.h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("POST /v1/document-series without permission status = %d, want 403", w.Code)
	}
}
