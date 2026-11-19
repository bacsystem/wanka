package entitlements

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// El reparto del rollout no puede cambiar entre versiones ni entre réplicas: estos valores son los de hoy.
func TestRolloutBucketGolden(t *testing.T) {
	var tenant pgtype.UUID
	if err := tenant.Scan("0b6f5b3e-8d57-4c3a-9d0e-1c2f3a4b5c6d"); err != nil {
		t.Fatal(err)
	}
	// Valores calculados aparte con sha256(tenant "\x00" flag)[:8] en big endian, módulo 100.
	for key, want := range map[string]int{"nuevo-pos": 37, "beta": 99} {
		if got := rolloutBucket(tenant, key); got != want {
			t.Errorf("rolloutBucket(%s) = %d, want %d", key, got, want)
		}
	}
}

// Si la base devolviera límites que no son números (no debería: lo impide un CHECK), build falla en vez de abrir.
func TestBuildRejectsNonNumericLimits(t *testing.T) {
	sub := "activa"
	got, err := build(pgtype.UUID{}, "activa", &sub, []byte(`{"users":"muchos"}`), []byte(`["ventas"]`), []byte(`[]`), nil)
	if err == nil {
		t.Fatalf("non-numeric limits accepted: %+v", got)
	}
	if len(got.Modules) != 0 {
		t.Error("an error came with modules")
	}
}

func TestLimitOfAnAbsentKeyIsZero(t *testing.T) {
	e := Entitlements{Limits: map[string]int64{LimitUsers: 5}}
	if e.Limit(LimitUsers) != 5 || e.Limit(LimitSites) != 0 || (Entitlements{}).Limit("x") != 0 {
		t.Error("Limit must return the plan's value, and 0 (closed) for an absent key")
	}
}

// Defensa: si un 1.0 llegara a la lectura (jsonb conserva la escala) se normaliza, no da un 500 en todo el plan.
func TestParseLimitsNormalizesTrailingZeros(t *testing.T) {
	got, err := parseLimits([]byte(`{"users": 1.0, "sites": 9223372036854775807.0, "cpe_per_month": 7}`))
	if err != nil {
		t.Fatal(err)
	}
	if got["users"] != 1 || got["sites"] != 9223372036854775807 || got["cpe_per_month"] != 7 {
		t.Errorf("got %v", got)
	}
	for _, bad := range []string{`{"users": 1.5}`, `{"users": "x"}`, `{"users": 9223372036854775808}`, `{"users": 1e2}`} {
		if _, err := parseLimits([]byte(bad)); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}
