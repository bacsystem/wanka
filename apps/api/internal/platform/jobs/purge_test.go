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
		_, err := admin.Exec(ctx, `INSERT INTO ops.idempotency_keys (tenant_id, principal_id, key, request_hash, status_code, response, created_at)
			VALUES ($1, gen_random_uuid(), $2::text, sha256(convert_to($2::text, 'UTF8')), 201, '\x7b7d', now() - make_interval(secs => $3))`,
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
	// Horario de claves de idempotencia, y diarios de tokens de usuario y de enrolamientos (#231).
	if len(reg.periodic) != 3 {
		t.Fatalf("%d periodic jobs registered, want 3", len(reg.periodic))
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

// seedIdentityRows inserta, como superusuario (que salta el trigger y fija created_at), n tokens de usuario y n de
// enrolamiento que vencieron hace "expiredAgo". Devuelve nada: se cuentan por token_hash con el prefijo.
func seedIdentityRows(ctx context.Context, t *testing.T, d *dbtest.DB, prefix string, n int, expiredAgo time.Duration) {
	t.Helper()
	admin := d.Admin(ctx, t)
	var user string
	if err := admin.QueryRow(ctx, `INSERT INTO iam.users (id, email) VALUES (gen_random_uuid(), $1) RETURNING id::text`,
		prefix+"@purge.test").Scan(&user); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		h := fmt.Sprintf("%s-%d", prefix, i)
		if _, err := admin.Exec(ctx, `INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, created_at, expires_at)
			VALUES (gen_random_uuid(), $1, 'password_reset', sha256($2::text::bytea), now() - make_interval(secs => $3) - interval '15 minutes',
			        now() - make_interval(secs => $3))`, user, h, expiredAgo.Seconds()); err != nil {
			t.Fatal(err)
		}
		if _, err := admin.Exec(ctx, `INSERT INTO iam.mfa_enrollments (id, user_id, token_hash, email_code_hmac, created_at, expires_at)
			VALUES (gen_random_uuid(), $1, sha256($2::text::bytea), sha256('h'), now() - make_interval(secs => $3) - interval '10 minutes',
			        now() - make_interval(secs => $3))`, user, h, expiredAgo.Seconds()); err != nil {
			t.Fatal(err)
		}
	}
}

func identityCount(ctx context.Context, t *testing.T, d *dbtest.DB, table, prefix string) int {
	t.Helper()
	var n int
	if err := d.Admin(ctx, t).QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE token_hash IN (
		SELECT sha256((p || '-' || g)::bytea) FROM generate_series(0, 20) g, (VALUES ($1::text)) v(p))`, prefix).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// #231: los jobs diarios borran solo lo vencido hace más de 7 días (tokens de usuario) o de 24 h (enrolamientos).
func TestPurgeExpiredIdentityRowsKeepsTheRetentionWindow(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	seedIdentityRows(ctx, t, d, "old", 5, UserTokenRetention+time.Hour)     // fuera de ambas ventanas
	seedIdentityRows(ctx, t, d, "mid", 3, MFAEnrollmentRetention+time.Hour) // fuera solo de la de enrolamientos
	seedIdentityRows(ctx, t, d, "new", 2, MFAEnrollmentRetention-time.Hour) // dentro de ambas
	seedIdentityRows(ctx, t, d, "live", 2, -10*time.Minute)                 // aún vigentes
	worker := openPool(t, d, dbtest.Worker, 3)

	// Lote de 2: 5 filas viejas = 3 transacciones; una sola tanda no las borraría todas.
	if n, err := purgeExpired(ctx, worker, "iam.user_tokens", UserTokenRetention, 2); err != nil || n != 6 {
		t.Fatalf("user tokens purged %d (%v), want 6", n, err)
	}
	if n, err := purgeExpired(ctx, worker, "iam.mfa_enrollments", MFAEnrollmentRetention, 2); err != nil || n != 10 {
		t.Fatalf("enrollments purged %d (%v), want 10 (old + mid + edge + past)", n, err)
	}
	for _, c := range []struct {
		table, prefix string
		want          int
	}{
		{"iam.user_tokens", "old", 0}, {"iam.user_tokens", "mid", 3}, {"iam.user_tokens", "new", 2}, {"iam.user_tokens", "live", 2}, {"iam.user_tokens", "edge", 1}, {"iam.user_tokens", "past", 0},
		{"iam.mfa_enrollments", "old", 0}, {"iam.mfa_enrollments", "mid", 0}, {"iam.mfa_enrollments", "new", 2}, {"iam.mfa_enrollments", "live", 2}, {"iam.mfa_enrollments", "edge", 0}, {"iam.mfa_enrollments", "past", 0},
	} {
		if got := identityCount(ctx, t, d, c.table, c.prefix); got != c.want {
			t.Errorf("%s %q rows left = %d, want %d", c.table, c.prefix, got, c.want)
		}
	}
	// Idempotente, y una tabla fuera de la lista no se purga.
	if n, err := purgeExpired(ctx, worker, "iam.user_tokens", UserTokenRetention, 2); err != nil || n != 0 {
		t.Fatalf("second pass purged %d (%v), want 0", n, err)
	}
	if _, err := purgeExpired(ctx, worker, "iam.users", UserTokenRetention, 2); err == nil {
		t.Fatal("purging iam.users was accepted")
	}
}

// #231 (H1): River no guarda el próximo turno, así que los jobs diarios corren al arrancar el worker. Sin insertar nada
// a mano en ops.river_job: lo prueba el planificador, no solo el handler.
func TestDailyIdentityPurgesRunOnWorkerStart(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	seedIdentityRows(ctx, t, d, "old", 3, UserTokenRetention+time.Hour)
	seedIdentityRows(ctx, t, d, "live", 2, -10*time.Minute)
	pool := openPool(t, d, dbtest.Worker, 5)
	stop := startWorker(t, pool, Config{}, func(r *Registry) {
		if err := RegisterMaintenance(r, pool); err != nil {
			t.Fatal(err)
		}
	})
	admin := d.Admin(ctx, t)
	for _, kind := range []string{KindPurgeUserTokens, KindPurgeMFAEnrollments} {
		eventually(t, 15*time.Second, kind+" to run on start", func() bool {
			rows := jobsOfKind(ctx, t, admin, kind)
			return len(rows) == 1 && rows[0].State == "completed"
		})
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"iam.user_tokens", "iam.mfa_enrollments"} {
		if identityCount(ctx, t, d, table, "old") != 0 || identityCount(ctx, t, d, table, "live") != 2 {
			t.Errorf("%s: the job did not purge exactly the expired rows", table)
		}
	}
}
