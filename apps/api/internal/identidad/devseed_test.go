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
