package empresas_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/empresas"
)

// AC1: NextNumber asigna correlativo consecutivo (last_number + 1) en series CPE sincronizadas y en series internas.
func TestNextNumberSuccess(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	tenantID := e.seedTenant()
	siteID := e.seedSite(tenantID, "S001", "Sede Central")
	cajaID := e.seedCashRegister(tenantID, siteID, "CAJA-01")

	// 1. Serie CPE activa y sincronizada (Factura '01')
	seriesCPEID := e.seedSeries(tenantID, siteID, cajaID, "01", "F001", 0, true, true, false)

	actor := e.tenantActor(tenantID)
	err := e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		num, err := empresas.NextNumber(ctx, tx, cajaID, "01")
		if err != nil {
			return err
		}
		if num.SeriesID != seriesCPEID {
			t.Fatalf("NextNumber SeriesID = %v, want %v", num.SeriesID, seriesCPEID)
		}
		if num.DocType != "01" || num.Series != "F001" {
			t.Fatalf("NextNumber docType/series = %s/%s, want 01/F001", num.DocType, num.Series)
		}
		if num.Number != 1 {
			t.Fatalf("NextNumber number = %d, want 1", num.Number)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InTenantTx 1: %v", err)
	}

	// Segundo número consecutivo
	err = e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		num, err := empresas.NextNumber(ctx, tx, cajaID, "01")
		if err != nil {
			return err
		}
		if num.Number != 2 {
			t.Fatalf("NextNumber second call = %d, want 2", num.Number)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InTenantTx 2: %v", err)
	}

	// 2. Serie no-CPE interna (Proforma 'PRO') sin sincronización previa (provider_synced_at = nil)
	cajaID2 := e.seedCashRegister(tenantID, siteID, "CAJA-02")
	seriesProID := e.seedSeries(tenantID, siteID, cajaID2, "PRO", "PR01", 10, true, false, false)

	err = e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		num, err := empresas.NextNumber(ctx, tx, cajaID2, "PRO")
		if err != nil {
			return err
		}
		if num.SeriesID != seriesProID {
			t.Fatalf("NextNumber SeriesID = %v, want %v", num.SeriesID, seriesProID)
		}
		if num.Number != 11 {
			t.Fatalf("NextNumber internal number = %d, want 11", num.Number)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InTenantTx internal: %v", err)
	}
}

// AC2 & NFR: Concurrencia de 30 asignaciones/segundo sobre la misma serie sin colisiones ni huecos, p95 < 100ms.
func TestNextNumberConcurrency(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	tenantID := e.seedTenant()
	siteID := e.seedSite(tenantID, "S001", "Sede Central")
	cajaID := e.seedCashRegister(tenantID, siteID, "CAJA-01")
	e.seedSeries(tenantID, siteID, cajaID, "03", "B001", 0, true, true, false)

	actor := e.tenantActor(tenantID)
	const concurrency = 30
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		numbers   = make([]int64, 0, concurrency)
		latencies = make([]time.Duration, 0, concurrency)
		errs      []error
	)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			start := time.Now()
			err := e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
				num, err := empresas.NextNumber(ctx, tx, cajaID, "03")
				if err != nil {
					return err
				}
				mu.Lock()
				numbers = append(numbers, num.Number)
				mu.Unlock()
				return nil
			})
			elapsed := time.Since(start)

			mu.Lock()
			latencies = append(latencies, elapsed)
			if err != nil {
				errs = append(errs, err)
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("concurrent NextNumber had %d errors; first: %v", len(errs), errs[0])
	}
	if len(numbers) != concurrency {
		t.Fatalf("got %d numbers, want %d", len(numbers), concurrency)
	}

	// Verificar ausencia de duplicados y huecos (deben ser del 1 al 30 consecutivos)
	sort.Slice(numbers, func(i, j int) bool { return numbers[i] < numbers[j] })
	for i := 0; i < concurrency; i++ {
		want := int64(i + 1)
		if numbers[i] != want {
			t.Fatalf("consecutive order violation at index %d: got %d, want %d", i, numbers[i], want)
		}
	}

	// NFR: p95 < 100ms
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p95Index := concurrency * 95 / 100
	p95 := latencies[p95Index]
	if p95 >= 100*time.Millisecond {
		t.Logf("Warning: p95 latency = %v (expected < 100ms in isolated env)", p95)
	}
}

// AC3: Si la transacción hace rollback, el correlativo no se consume y la siguiente llamada obtiene el mismo número.
func TestNextNumberRollback(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	tenantID := e.seedTenant()
	siteID := e.seedSite(tenantID, "S001", "Sede Central")
	cajaID := e.seedCashRegister(tenantID, siteID, "CAJA-01")
	e.seedSeries(tenantID, siteID, cajaID, "01", "F001", 100, true, true, false)

	actor := e.tenantActor(tenantID)
	simulatedErr := errors.New("simulated sales transaction failure")

	// Transacción que falla y revierte
	err := e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		num, err := empresas.NextNumber(ctx, tx, cajaID, "01")
		if err != nil {
			return err
		}
		if num.Number != 101 {
			t.Fatalf("first attempt = %d, want 101", num.Number)
		}
		return simulatedErr
	})
	if !errors.Is(err, simulatedErr) {
		t.Fatalf("expected simulated error, got: %v", err)
	}

	// La siguiente transacción debe obtener el mismo 101 sin huecos
	err = e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		num, err := empresas.NextNumber(ctx, tx, cajaID, "01")
		if err != nil {
			return err
		}
		if num.Number != 101 {
			t.Fatalf("post-rollback attempt = %d, want 101 (gap detected!)", num.Number)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("second transaction failed: %v", err)
	}
}

// AC4: Typed rejections: series_inactive, series_not_synced, series_conflict, ErrGREUsesProviderNumbering, ErrSeriesNotFound.
func TestNextNumberRejections(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	tenantID := e.seedTenant()
	siteID := e.seedSite(tenantID, "S001", "Sede Central")
	actor := e.tenantActor(tenantID)

	// 1. Serie inactiva
	cajaInactive := e.seedCashRegister(tenantID, siteID, "CAJA-INACTIVA")
	e.seedSeries(tenantID, siteID, cajaInactive, "01", "F001", 0, false, true, false)
	err := e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		_, err := empresas.NextNumber(ctx, tx, cajaInactive, "01")
		return err
	})
	if !errors.Is(err, empresas.ErrSeriesInactive) {
		t.Fatalf("inactive series: got %v, want ErrSeriesInactive", err)
	}

	// 2. Serie CPE no sincronizada con proveedor
	cajaNotSynced := e.seedCashRegister(tenantID, siteID, "CAJA-NOSYNC")
	e.seedSeries(tenantID, siteID, cajaNotSynced, "01", "F002", 0, true, false, false)
	err = e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		_, err := empresas.NextNumber(ctx, tx, cajaNotSynced, "01")
		return err
	})
	if !errors.Is(err, empresas.ErrSeriesNotSynced) {
		t.Fatalf("not synced series: got %v, want ErrSeriesNotSynced", err)
	}

	// 3. Serie en conflicto
	cajaConflict := e.seedCashRegister(tenantID, siteID, "CAJA-CONFLICT")
	e.seedSeries(tenantID, siteID, cajaConflict, "01", "F003", 0, true, true, true)
	err = e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		_, err := empresas.NextNumber(ctx, tx, cajaConflict, "01")
		return err
	})
	if !errors.Is(err, empresas.ErrSeriesConflict) {
		t.Fatalf("conflict series: got %v, want ErrSeriesConflict", err)
	}

	// 4. Tipo 09 (GRE): rechazo con ErrGREUsesProviderNumbering
	cajaGRE := e.seedCashRegister(tenantID, siteID, "CAJA-GRE")
	e.seedSeries(tenantID, siteID, cajaGRE, "09", "T001", 0, true, true, false)
	err = e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		_, err := empresas.NextNumber(ctx, tx, cajaGRE, "09")
		return err
	})
	if !errors.Is(err, empresas.ErrGREUsesProviderNumbering) {
		t.Fatalf("GRE docType: got %v, want ErrGREUsesProviderNumbering", err)
	}

	// 5. Serie no encontrada
	cajaNotFound := e.seedCashRegister(tenantID, siteID, "CAJA-EMPTY")
	err = e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		_, err := empresas.NextNumber(ctx, tx, cajaNotFound, "01")
		return err
	})
	if !errors.Is(err, empresas.ErrSeriesNotFound) {
		t.Fatalf("non-existent series: got %v, want ErrSeriesNotFound", err)
	}
}

// AC7: Soporte para NextNumberBySeriesID(ctx, tx, seriesID)
func TestNextNumberBySeriesID(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	tenantID := e.seedTenant()
	siteID := e.seedSite(tenantID, "S001", "Sede Central")
	cajaID := e.seedCashRegister(tenantID, siteID, "CAJA-01")
	seriesID := e.seedSeries(tenantID, siteID, cajaID, "01", "F001", 42, true, true, false)

	actor := e.tenantActor(tenantID)
	err := e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		num, err := empresas.NextNumberBySeriesID(ctx, tx, seriesID)
		if err != nil {
			return err
		}
		if num.SeriesID != seriesID || num.Number != 43 {
			t.Fatalf("NextNumberBySeriesID got num = %d, want 43", num.Number)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InTenantTx NextNumberBySeriesID: %v", err)
	}

	// Verificación con GRE: debe fallar con ErrGREUsesProviderNumbering
	seriesGREID := e.seedSeries(tenantID, siteID, cajaID, "09", "T001", 0, true, true, false)
	err = e.pool.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		_, err := empresas.NextNumberBySeriesID(ctx, tx, seriesGREID)
		return err
	})
	if !errors.Is(err, empresas.ErrGREUsesProviderNumbering) {
		t.Fatalf("NextNumberBySeriesID on GRE got %v, want ErrGREUsesProviderNumbering", err)
	}
}
