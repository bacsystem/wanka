package jobs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// seedKeys inserta n claves del tenant con la antigüedad dada, como superusuario (que fija created_at).
func seedKeys(ctx context.Context, t *testing.T, d *dbtest.DB, tenantID, prefix string, n int, age time.Duration) {
	t.Helper()
	admin := d.Admin(ctx, t)
	for i := 0; i < n; i++ {
		_, err := admin.Exec(ctx, `INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash, status_code, response, created_at)
			VALUES ($1, $2::text, sha256(convert_to($2::text, 'UTF8')), 201, '\x7b7d', now() - make_interval(secs => $3))`,
			tenantID, fmt.Sprintf("%s-%d", prefix, i), age.Seconds())
		if err != nil {
			t.Fatal(err)
		}
	}
}

func keyCount(ctx context.Context, t *testing.T, d *dbtest.DB, where string) int {
	t.Helper()
	var n int
	if err := d.Admin(ctx, t).QueryRow(ctx, `SELECT count(*) FROM ops.idempotency_keys WHERE `+where).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Criterio 9: el job borra en lotes las claves de más de 24 h de todos los tenants y conserva las recientes.
func TestPurgeIdempotencyKeysDeletesExpiredInBatches(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	a, b := d.SeedSession(t, dbtest.SessionSpec{}), d.SeedSession(t, dbtest.SessionSpec{})
	seedKeys(ctx, t, d, a.TenantID, "old-a", 5, 25*time.Hour)
	seedKeys(ctx, t, d, b.TenantID, "old-b", 4, 48*time.Hour)
	seedKeys(ctx, t, d, a.TenantID, "new-a", 3, 23*time.Hour+50*time.Minute)
	seedKeys(ctx, t, d, b.TenantID, "new-b", 2, time.Minute)
	worker := openPool(t, d, dbtest.Worker, 3)

	// Lote de 2: 9 vencidas = 5 transacciones (2+2+2+2+1); una sola tanda no las borraría todas.
	n, batches, err := purgeIdempotencyKeys(ctx, worker, 2)
	if err != nil {
		t.Fatal(err)
	}
	if n != 9 || batches != 5 {
		t.Fatalf("purged %d in %d batches, want 9 in 5 (2+2+2+2+1)", n, batches)
	}
	if got := keyCount(ctx, t, d, `key LIKE 'old-%'`); got != 0 {
		t.Fatalf("%d expired keys left", got)
	}
	if got := keyCount(ctx, t, d, `key LIKE 'new-%'`); got != 5 {
		t.Fatalf("%d recent keys left, want 5: the purge must keep what is under 24 h", got)
	}
	// Idempotente: otra pasada no borra nada.
	if n, batches, err := purgeIdempotencyKeys(ctx, worker, 2); err != nil || n != 0 || batches != 1 {
		t.Fatalf("second pass purged %d in %d batches (%v), want 0 in 1", n, batches, err)
	}
}

// El borde: una clave de justo menos de 24 h se conserva aunque la limpieza corra.
func TestPurgeKeepsAKeyJustUnderRetention(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	a := d.SeedSession(t, dbtest.SessionSpec{})
	seedKeys(ctx, t, d, a.TenantID, "edge", 1, IdempotencyRetention-time.Minute)
	seedKeys(ctx, t, d, a.TenantID, "gone", 1, IdempotencyRetention+time.Minute)
	n, _, err := purgeIdempotencyKeys(ctx, openPool(t, d, dbtest.Worker, 2), 100)
	if err != nil || n != 1 {
		t.Fatalf("purged %d (%v), want 1", n, err)
	}
	if keyCount(ctx, t, d, `key = 'edge-0'`) != 1 || keyCount(ctx, t, d, `key = 'gone-0'`) != 0 {
		t.Fatal("wrong rows survived")
	}
}

// El job periódico está registrado: cola maintenance, cada hora, único; y borra de verdad cuando corre.
func TestRegisterMaintenanceRunsThePurgeJob(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	a := d.SeedSession(t, dbtest.SessionSpec{})
	seedKeys(ctx, t, d, a.TenantID, "old", 3, 26*time.Hour)
	seedKeys(ctx, t, d, a.TenantID, "new", 2, time.Hour)
	pool := openPool(t, d, dbtest.Worker, 5)
	reg := NewRegistry(pool)
	if err := RegisterMaintenance(reg, pool); err != nil {
		t.Fatal(err)
	}
	if len(reg.periodic) != 1 {
		t.Fatalf("%d periodic jobs registered, want 1", len(reg.periodic))
	}
	// Doble registro del mismo kind: error de programación.
	if err := RegisterMaintenance(reg, pool); err == nil {
		t.Fatal("registering the maintenance jobs twice was accepted")
	}

	// Se encola a mano el mismo job (el ciclo horario lo encolaría igual) y un worker real lo ejecuta.
	stop := startWorker(t, pool, Config{}, func(r *Registry) {
		if err := RegisterMaintenance(r, pool); err != nil {
			t.Fatal(err)
		}
	})
	admin := d.Admin(ctx, t)
	if _, err := admin.Exec(ctx, `INSERT INTO ops.river_job (kind, args, queue) VALUES ($1, '{}', 'maintenance')`, KindPurgeIdempotencyKeys); err != nil {
		t.Fatal(err)
	}
	eventually(t, 15*time.Second, "the purge job to complete", func() bool {
		rows := jobsOfKind(ctx, t, admin, KindPurgeIdempotencyKeys)
		return len(rows) == 1 && rows[0].State == "completed"
	})
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if keyCount(ctx, t, d, `key LIKE 'old-%'`) != 0 || keyCount(ctx, t, d, `key LIKE 'new-%'`) != 2 {
		t.Fatal("the job did not purge exactly the expired keys")
	}
}
