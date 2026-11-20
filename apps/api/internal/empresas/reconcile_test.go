package empresas_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/empresas"
)

// AC6: Reconciliación diaria (ReconcileSeries).
// - Si Khipu reporta un correlativo mayor al local para 01/03/07/08, congelar la serie.
// - Para 09 (GRE), no congelar por número (ADR 0013 §5), solo alertar.
// - Si el anexo no coincide con el local asignado, congelar la serie (incluida 09).
func TestReconcileSeries(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	tenantID := e.seedTenant()
	siteID := e.seedSite(tenantID, "0000", "Sede Principal")
	cajaID := e.seedCashRegister(tenantID, siteID, "CAJA-01")

	// 1. Serie Factura '01' con correlativo local 10
	e.seedSeries(tenantID, siteID, cajaID, "01", "F001", 10, true, true, false)

	// 2. Serie GRE '09' con correlativo local 5
	e.seedSeries(tenantID, siteID, cajaID, "09", "T001", 5, true, true, false)

	// 3. Serie Boleta '03' con correlativo local 20 en anexo '0000'
	e.seedSeries(tenantID, siteID, cajaID, "03", "B001", 20, true, true, false)

	// 4. Serie GRE '09' en anexo '0000' en caja 02 para probar discrepancia de anexo
	cajaID2 := e.seedCashRegister(tenantID, siteID, "CAJA-02")
	e.seedSeries(tenantID, siteID, cajaID2, "09", "T002", 15, true, true, false)

	actor := e.tenantActor(tenantID)

	remoteData := []empresas.RemoteSeries{
		// Caso A: Factura F001 reporta 15 (> local 10) en el mismo anexo '0000' -> debe congelar
		{DocType: "01", Series: "F001", UltimoNumero: 15, Anexo: "0000"},
		// Caso B: GRE T001 reporta 8 (> local 5) en el mismo anexo '0000' -> NO congela, solo alerta (ADR 0013 §5)
		{DocType: "09", Series: "T001", UltimoNumero: 8, Anexo: "0000"},
		// Caso C: Boleta B001 reporta anexo '0001' (distinto a '0000') -> debe congelar por discrepancia de anexo
		{DocType: "03", Series: "B001", UltimoNumero: 20, Anexo: "0001"},
		// Caso D: GRE T002 reporta anexo '9999' (distinto a '0000') -> debe congelar por anexo aunque sea GRE
		{DocType: "09", Series: "T002", UltimoNumero: 15, Anexo: "9999"},
	}

	var report empresas.ReconcileReport
	err := e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		var err error
		report, err = empresas.ReconcileSeries(ctx, tx, remoteData)
		return err
	})
	if err != nil {
		t.Fatalf("ReconcileSeries failed: %v", err)
	}

	if report.Processed != 4 {
		t.Fatalf("Processed = %d, want 4", report.Processed)
	}
	// Deben congelarse 3 series: F001 (número), B001 (anexo), T002 (anexo)
	if report.FrozenCount != 3 {
		t.Fatalf("FrozenCount = %d, want 3", report.FrozenCount)
	}
	// Debe haber 1 alerta para T001
	if len(report.Alerts) != 1 {
		t.Fatalf("Alerts count = %d, want 1; alerts: %v", len(report.Alerts), report.Alerts)
	}

	// Comprobaciones de estado post-reconciliación:
	err = e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		// F001 debe estar congelada
		_, err := empresas.NextNumber(ctx, tx, cajaID, "01")
		if !errors.Is(err, empresas.ErrSeriesConflict) {
			t.Fatalf("F001 should be frozen, got %v", err)
		}

		// T001 NO debe estar congelada (sólo alertó)
		var conflictAt *string
		err = tx.QueryRow(ctx, `SELECT provider_conflict_at::text FROM org.document_series WHERE doc_type = '09' AND series = 'T001'`).Scan(&conflictAt)
		if err != nil {
			return err
		}
		if conflictAt != nil {
			t.Fatalf("T001 should not be frozen by number disparity, got conflict_at: %v", *conflictAt)
		}

		// B001 debe estar congelada por anexo
		_, err = empresas.NextNumber(ctx, tx, cajaID, "03")
		if !errors.Is(err, empresas.ErrSeriesConflict) {
			t.Fatalf("B001 should be frozen by annex mismatch, got %v", err)
		}

		// T002 debe estar congelada por anexo
		err = tx.QueryRow(ctx, `SELECT provider_conflict_at::text FROM org.document_series WHERE doc_type = '09' AND series = 'T002'`).Scan(&conflictAt)
		if err != nil {
			return err
		}
		if conflictAt == nil {
			t.Fatal("T002 should be frozen by annex mismatch, but conflict_at is nil")
		}

		return nil
	})
	if err != nil {
		t.Fatalf("verification transaction failed: %v", err)
	}
}
