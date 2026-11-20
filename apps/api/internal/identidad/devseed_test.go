package identidad_test

import (
	"context"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/identidad"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

const devPassword = "Demo-Wanka-2026-local"

// La semilla local crea la empresa, un usuario por rol base con membresía activa y un local con su caja; correrla
// dos veces no duplica nada, y cada usuario inicia sesión con la contraseña de prueba por el login real.
func TestDevSeedIsIdempotentAndUsersCanLogIn(t *testing.T) {
	e := newEnv(t, envOpts{})
	ctx := context.Background()
	su, err := db.Open(ctx, e.d.SuperuserURL(), db.Options{MaxConns: 2, AppName: "devseed-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(su.Close)

	for run := 1; run <= 2; run++ {
		users, err := identidad.DevSeed(ctx, su, fastParams, devPassword)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if len(users) != len(identidad.DevRoles) {
			t.Fatalf("run %d: %d users", run, len(users))
		}
	}
	count := func(sql string) (n int) {
		t.Helper()
		e.scan(sql, nil, &n)
		return n
	}
	for sql, want := range map[string]int{
		`SELECT count(*) FROM org.tenants`:                               1,
		`SELECT count(*) FROM iam.users WHERE email LIKE '%@demo.local'`: 6,
		`SELECT count(*) FROM iam.roles WHERE system`:                    6,
		`SELECT count(*) FROM iam.memberships WHERE status = 'activo'`:   6,
		`SELECT count(*) FROM org.sites`:                                 1,
		`SELECT count(*) FROM org.cash_registers`:                        1,
		`SELECT count(DISTINCT role_id) FROM iam.memberships`:            6,
	} {
		if got := count(sql); got != want {
			t.Errorf("%s = %d, want %d", sql, got, want)
		}
	}
	// Los roles que exigen 2FA (dueno, administrador, contador) responden 503 hasta que #25 permita enrolarla; el resto da 200.
	mfaRoles := map[string]bool{"dueno": true, "administrador": true, "contador": true}
	for _, r := range identidad.DevRoles {
		want := 200
		if mfaRoles[r] {
			want = 503
		}
		if resp := e.login(r+"@demo.local", devPassword); resp.Code != want {
			t.Errorf("login %s: status %d, want %d, body %s", r, resp.Code, want, resp.Body.String())
		}
	}
	if resp := e.login("cajero@demo.local", "otra-clave-cualquiera"); resp.Code == 200 {
		t.Error("a wrong password must not log in")
	}
}

// Resembrar deja a cada usuario de prueba en estado limpio: sin racha de fallos, sin bloqueo y con la empresa y las
// membresías activas, aunque pruebas anteriores las hayan cerrado.
func TestDevSeedRestoresCleanState(t *testing.T) {
	e := newEnv(t, envOpts{})
	ctx := context.Background()
	su, err := db.Open(ctx, e.d.SuperuserURL(), db.Options{MaxConns: 2, AppName: "devseed-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(su.Close)
	seed := func() {
		t.Helper()
		if _, err := identidad.DevSeed(ctx, su, fastParams, devPassword); err != nil {
			t.Fatal(err)
		}
	}
	seed()
	var uid string
	e.scan(`SELECT id::text FROM iam.users WHERE email = 'cajero@demo.local'`, nil, &uid)

	// Acceso por contraseña cerrado (racha 100) y bloqueado: la semilla lo repone.
	e.setUser(uid, "login_failure_streak = 100, failed_logins = 5, login_lockouts = 3, locked_until = now() + interval '1 hour', login_window_started_at = now()")
	e.exec(`UPDATE org.tenants SET status = 'suspendida'`)
	e.exec(`UPDATE iam.memberships SET status = 'suspendido'`)
	seed()
	if resp := e.login("cajero@demo.local", devPassword); resp.Code != 200 {
		t.Fatalf("after reseeding a closed account: status %d, body %s", resp.Code, resp.Body.String())
	}

	// Racha 4: tras resembrar, un fallo no bloquea y la contraseña correcta sigue entrando.
	e.setUser(uid, "login_failure_streak = 4, failed_logins = 4, login_window_started_at = now()")
	seed()
	if resp := e.login("cajero@demo.local", "otra-clave-cualquiera"); resp.Code != 401 {
		t.Fatalf("wrong password: status %d", resp.Code)
	}
	if resp := e.login("cajero@demo.local", devPassword); resp.Code != 200 {
		t.Fatalf("one failure after reseeding locked the account: status %d, body %s", resp.Code, resp.Body.String())
	}
	var status string
	e.scan(`SELECT status FROM org.tenants`, nil, &status)
	if status != "activa" {
		t.Fatalf("tenant status = %s", status)
	}
}
