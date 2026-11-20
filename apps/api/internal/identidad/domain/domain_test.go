package domain_test

import (
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/domain"
)

// Lista escrita a mano del ADR 0014 (15 min que se duplican hasta 24 h, techo en 100), no la fórmula del código.
func TestLockoutFor(t *testing.T) {
	h := time.Hour
	m := time.Minute
	for _, tc := range []struct {
		streak  int
		none    bool
		closed  bool
		n, cnt  int
		forTime time.Duration
	}{
		{0, true, false, 0, 0, 0}, {1, true, false, 0, 0, 0}, {4, true, false, 0, 0, 0},
		{5, false, false, 1, 1, 15 * m}, {6, true, false, 0, 0, 0}, {9, true, false, 0, 0, 0},
		{10, false, false, 2, 2, 30 * m}, {15, false, false, 3, 3, h}, {20, false, false, 4, 4, 2 * h},
		{25, false, false, 5, 5, 4 * h}, {30, false, false, 6, 6, 8 * h}, {35, false, false, 7, 7, 16 * h},
		{40, false, false, 8, 8, 24 * h}, {45, false, false, 9, 9, 24 * h}, {50, false, false, 10, 10, 24 * h},
		{55, false, false, 11, 10, 24 * h}, {95, false, false, 19, 10, 24 * h},
		{99, true, false, 0, 0, 0}, {100, false, true, 20, 10, 24 * h},
	} {
		got := domain.LockoutFor(tc.streak)
		if got.None != tc.none || got.Closed != tc.closed || (!tc.none && (got.N != tc.n || got.Counted != tc.cnt || got.For != tc.forTime)) {
			t.Errorf("LockoutFor(%d) = %+v, want none=%v closed=%v n=%d counted=%d for=%v", tc.streak, got, tc.none, tc.closed, tc.n, tc.cnt, tc.forTime)
		}
	}
}
