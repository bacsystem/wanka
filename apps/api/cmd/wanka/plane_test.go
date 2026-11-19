package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

type countingSweeper struct{ n atomic.Int64 }

func (c *countingSweeper) Sweep() int { c.n.Add(1); return 0 }

// Con el intervalo inyectado a 10 ms el bucle barre al menos dos veces, y al
// cancelar el contexto termina antes de 100 ms.
func TestSweepLoopSweepsAndStopsOnCancel(t *testing.T) {
	s := &countingSweeper{}
	ctx, cancel := context.WithCancel(context.Background())
	done := startSweepLoop(ctx, s, 10*time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for s.n.Load() < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("Sweep called %d times in 2 s with a 10 ms interval", s.n.Load())
		}
		time.Sleep(time.Millisecond)
	}
	start := time.Now()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("the sweep loop did not stop after cancel")
	}
	if took := time.Since(start); took >= 100*time.Millisecond {
		t.Fatalf("the sweep loop took %v to stop, want < 100ms", took)
	}
	after := s.n.Load()
	time.Sleep(50 * time.Millisecond)
	if s.n.Load() != after {
		t.Fatal("Sweep kept being called after the loop stopped")
	}
}

// Solo el plano tenant monta las rutas con sesión y el barrido de los limitadores.
func TestOnlyTheTenantPlaneMountsTheSessionChain(t *testing.T) {
	for plane, want := range map[string]bool{"tenant": true, "platform": false, "webhooks": false} {
		t.Run(plane, func(t *testing.T) {
			rt := httpx.NewRouter(httpx.RouterOptions{})
			sw, err := mountPlane(plane, rt, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := sw != nil; got != want {
				t.Fatalf("mountPlane(%s) returned a sweeper = %v, want %v", plane, got, want)
			}
		})
	}
}
