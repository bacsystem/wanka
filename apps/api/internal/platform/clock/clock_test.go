package clock_test

import (
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
)

func TestFakeOnlyMovesWhenAdvanced(t *testing.T) {
	start := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	f := clock.NewFake(start)
	if !f.Now().Equal(start) {
		t.Fatalf("Now = %v, want %v", f.Now(), start)
	}
	f.Advance(1500 * time.Millisecond)
	if want := start.Add(1500 * time.Millisecond); !f.Now().Equal(want) {
		t.Fatalf("Now = %v, want %v", f.Now(), want)
	}
	var _ clock.Clock = clock.System{}
	var _ clock.Clock = f
}
