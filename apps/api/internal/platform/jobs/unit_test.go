package jobs

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river/rivertype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

func TestRetryDelayIsExponentialWithJitterAndCap(t *testing.T) {
	const base, max = 10 * time.Second, time.Hour
	at := func(jitter float64) retryPolicy {
		return retryPolicy{base: base, max: max, jitter: func() float64 { return jitter }, now: time.Now}
	}
	cases := []struct {
		attempt int
		full    time.Duration // el valor antes del jitter
	}{
		{1, 10 * time.Second}, {2, 20 * time.Second}, {3, 40 * time.Second}, {4, 80 * time.Second},
		{8, 1280 * time.Second}, {9, 2560 * time.Second}, {10, time.Hour}, {11, time.Hour}, {60, time.Hour}, {1000, time.Hour},
	}
	for _, c := range cases {
		lo, hi := at(0).delay(c.attempt), at(0.999999).delay(c.attempt)
		if lo != c.full/2 {
			t.Errorf("attempt %d: minimum delay %s, want half of %s", c.attempt, lo, c.full)
		}
		if hi < c.full-time.Second || hi >= c.full {
			t.Errorf("attempt %d: maximum delay %s, want just under %s", c.attempt, hi, c.full)
		}
	}
	// Con jitter real, dos intentos iguales no dan siempre lo mismo (si no, las réplicas reintentan a la vez).
	p := newRetryPolicy(base, max)
	seen := map[time.Duration]bool{}
	for range 20 {
		seen[p.delay(5)] = true
	}
	if len(seen) < 10 {
		t.Errorf("only %d distinct delays in 20 draws: no jitter", len(seen))
	}
}

func TestRetryNextRetryUsesTheFailedAttempt(t *testing.T) {
	now := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	p := retryPolicy{base: time.Minute, max: time.Hour, jitter: func() float64 { return 0 }, now: func() time.Time { return now }}
	got := p.NextRetry(&rivertype.JobRow{Attempt: 3})
	if want := now.Add(2 * time.Minute); !got.Equal(want) { // 4 min con la mitad del jitter
		t.Fatalf("next retry %s, want %s", got, want)
	}
}

func TestConfigFromEnv(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

	t.Run("defaults", func(t *testing.T) {
		cfg, err := ConfigFromEnv(env(nil))
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]int{"cpe": 20, "webhooks": 10, "notifications": 10, "reports": 2, "maintenance": 2}
		if len(cfg.Concurrency) != len(want) || cfg.ShutdownTimeout != 30*time.Second {
			t.Fatalf("got %v, %s", cfg.Concurrency, cfg.ShutdownTimeout)
		}
		for q, n := range want {
			if cfg.Concurrency[q] != n {
				t.Errorf("queue %s: %d, want %d", q, cfg.Concurrency[q], n)
			}
		}
	})
	t.Run("overrides", func(t *testing.T) {
		cfg, err := ConfigFromEnv(env(map[string]string{"JOBS_CONCURRENCY_CPE": "5", "JOBS_CONCURRENCY_REPORTS": "1", "JOBS_SHUTDOWN_TIMEOUT": "45s"}))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Concurrency["cpe"] != 5 || cfg.Concurrency["reports"] != 1 || cfg.Concurrency["webhooks"] != 10 || cfg.ShutdownTimeout != 45*time.Second {
			t.Fatalf("got %v, %s", cfg.Concurrency, cfg.ShutdownTimeout)
		}
		if DefaultConcurrency["cpe"] != 20 {
			t.Fatal("ConfigFromEnv changed the package defaults")
		}
	})
	t.Run("bad values name the variable and never echo the value", func(t *testing.T) {
		for k, v := range map[string]string{
			"JOBS_CONCURRENCY_CPE": "lots-123", "JOBS_CONCURRENCY_WEBHOOKS": "-0", "JOBS_CONCURRENCY_REPORTS": "-3",
			"JOBS_CONCURRENCY_NOTIFICATIONS": "100000", "JOBS_SHUTDOWN_TIMEOUT": "soon-456",
		} {
			_, err := ConfigFromEnv(env(map[string]string{k: v}))
			if err == nil || !strings.Contains(err.Error(), k) || strings.Contains(err.Error(), v) {
				t.Errorf("%s=%s: error %v", k, v, err)
			}
		}
		_, err := ConfigFromEnv(env(map[string]string{"JOBS_SHUTDOWN_TIMEOUT": "0s"}))
		if err == nil {
			t.Error("a zero shutdown timeout was accepted")
		}
	})
}

func TestCancelAndSnoozeErrors(t *testing.T) {
	base := db.ErrNoTenant
	c := Cancel(base)
	if !errors.Is(c, base) {
		t.Error("Cancel hides the wrapped error from errors.Is")
	}
	var ce *cancelError
	if !errors.As(c, &ce) {
		t.Error("Cancel is not recognised through errors.As")
	}
	if toRiverError(nil) != nil {
		t.Error("nil must stay nil")
	}
	plain := errors.New("transient")
	if toRiverError(plain) != plain {
		t.Error("a plain error must reach River untouched so it is retried")
	}
	if got := toRiverError(c); got == c || !errors.Is(got, base) {
		t.Error("Cancel must become River's JobCancel and keep the cause")
	}
}
