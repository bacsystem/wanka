package empresas_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bacsystem/wanka/apps/api/internal/empresas"
)

// AC5: FreezeSeries congela la serie e impide asignaciones futuras. El trigger org.guard_series_conflict() impide limpiar el conflicto.
func TestFreezeSeries(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	tenantID := e.seedTenant()
	siteID := e.seedSite(tenantID, "S001", "Sede Central")
	cajaID := e.seedCashRegister(tenantID, siteID, "CAJA-01")
	seriesID := e.seedSeries(tenantID, siteID, cajaID, "01", "F001", 10, true, true, false)

	actor := e.tenantActor(tenantID)

	// 1. Congelar la serie
	err := e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		return empresas.FreezeSeries(ctx, tx, seriesID, "discrepancia en conciliación")
	})
	if err != nil {
		t.Fatalf("FreezeSeries failed: %v", err)
	}

	// 2. Comprobar que llamadas futuras a NextNumber fallan con ErrSeriesConflict
	err = e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		_, err := empresas.NextNumber(ctx, tx, cajaID, "01")
		return err
	})
	if !errors.Is(err, empresas.ErrSeriesConflict) {
		t.Fatalf("NextNumber on frozen series got %v, want ErrSeriesConflict", err)
	}

	// 3. Comprobar que llamadas a NextNumberBySeriesID fallan con ErrSeriesConflict
	err = e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		_, err := empresas.NextNumberBySeriesID(ctx, tx, seriesID)
		return err
	})
	if !errors.Is(err, empresas.ErrSeriesConflict) {
		t.Fatalf("NextNumberBySeriesID on frozen series got %v, want ErrSeriesConflict", err)
	}

	// 4. Intentar limpiar el conflicto mediante SQL directo: debe ser bloqueado por el trigger con SQLSTATE 42501
	err = e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE org.document_series SET provider_conflict_at = NULL WHERE id = $1`, seriesID)
		return err
	})
	if err == nil {
		t.Fatal("expected trigger error when unfreezing series, got nil")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code != "42501" {
			t.Fatalf("trigger error SQLSTATE = %s, want 42501", pgErr.Code)
		}
	} else {
		t.Fatalf("expected pgconn.PgError, got %T: %v", err, err)
	}

	// 5. Congelar una serie ya congelada devuelve ErrSeriesAlreadyFrozen
	err = e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		return empresas.FreezeSeries(ctx, tx, seriesID, "segundo intento")
	})
	if !errors.Is(err, empresas.ErrSeriesAlreadyFrozen) {
		t.Fatalf("FreezeSeries on already frozen series got %v, want ErrSeriesAlreadyFrozen", err)
	}
}
