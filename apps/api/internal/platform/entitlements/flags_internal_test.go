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
