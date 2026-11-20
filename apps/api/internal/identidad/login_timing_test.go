//go:build !race

package identidad_test

import (
	"sort"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/adapters/argon2"
)

// Informativo (la garantía es TestEveryPlain401OpensTheSameNumberOfTransactions y el piso de tiempo de app.WithFloor).
// Mismo orden de tiempo para correo inexistente, bloqueado, cerrado y sin password_hash: el hash ficticio gasta lo
// mismo que uno real. Mide tiempo, así que no corre con -race (la CI lo corre aparte, sin él).
func TestEnumerationTimingIsOfTheSameOrder(t *testing.T) {
	// ~20 ms por hash: los viajes a la base (~1 ms) no tapan la diferencia entre hashear y no hashear.
	e := newEnv(t, envOpts{hash: argon2.Params{MemoryKiB: 16 * 1024, Iterations: 2, Parallelism: 1}})
	mk := func(email, pw, set string) {
		uid := e.addUser(email, pw)
		e.addMembership(uid, companySpec{})
		if set != "" {
			e.setUser(uid, set)
		}
	}
	mk("wrong@x.pe", goodPassword, "")
	mk("locked@x.pe", goodPassword, "locked_until = now() + interval '10 minutes'")
	mk("closed@x.pe", goodPassword, "login_lockouts = 10")
	mk("blocked@x.pe", goodPassword, "status = 'bloqueado'")
	mk("nohash@x.pe", "", "")
	cases := []struct{ name, email, pw string }{
		{"wrong password", "wrong@x.pe", "nope"},
		{"unknown email", "nobody@x.pe", goodPassword},
		{"locked", "locked@x.pe", goodPassword},
		{"closed", "closed@x.pe", goodPassword},
		{"blocked user", "blocked@x.pe", goodPassword},
		{"no password hash", "nohash@x.pe", goodPassword},
	}
	median := func(email, pw string) time.Duration {
		var ds []time.Duration
		for range 7 {
			// La cuenta de la contraseña mala se bloquearía al 5.º fallo: el contador se restablece fuera de la medición.
			e.exec(`UPDATE iam.users SET failed_logins = 0, login_window_started_at = NULL, locked_until = NULL WHERE email = 'wrong@x.pe'`)
			start := time.Now()
			if res := e.login(email, pw); res.Code != 401 {
				t.Fatalf("%s: status = %d", email, res.Code)
			}
			ds = append(ds, time.Since(start))
		}
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		return ds[len(ds)/2]
	}
	base := time.Duration(0)
	for i, tc := range cases {
		if i == 0 {
			base = median(tc.email, tc.pw)
			continue
		}
		e.login(tc.email, tc.pw)
		got := median(tc.email, tc.pw)
		ratio := float64(got) / float64(base)
		t.Logf("%-18s median %v (%.2fx the wrong-password case, %v)", tc.name, got, ratio, base)
		if ratio < 0.85 || ratio > 1.2 {
			t.Errorf("%s: %v against %v for a wrong password (%.2fx): the response time tells the cases apart", tc.name, got, base, ratio)
		}
	}
}
