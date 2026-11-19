package main

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/config"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

type countingSweeper struct {
	n      atomic.Int64
	active atomic.Int64
	work   time.Duration
}

func (c *countingSweeper) Sweep() int {
	c.active.Add(1)
	defer c.active.Add(-1)
	c.n.Add(1)
	time.Sleep(c.work)
	return 0
}

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
			sw, err := mountPlane(plane, rt, nil, auth.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if got := sw != nil; got != want {
				t.Fatalf("mountPlane(%s) returned a sweeper = %v, want %v", plane, got, want)
			}
		})
	}
}

// El cableado de producción: barrido cada 10 s y los valores de conventions.md §7
// (auth.Options vacío toma los de auth.Default*).
func TestDefaultWiringIsTheContract(t *testing.T) {
	if defaultWiring.sweepInterval != 10*time.Second {
		t.Fatalf("sweep interval = %v, want 10s", defaultWiring.sweepInterval)
	}
	if !reflect.DeepEqual(defaultWiring.auth, auth.Options{}) {
		t.Fatalf("auth options = %+v, want the defaults of conventions.md §7 (empty Options)", defaultWiring.auth)
	}
	if defaultWiring.wrapSweeper != nil {
		t.Fatal("production wiring must not wrap the sweeper")
	}
}

// serve arranca el bucle de barrido del plano tenant con el intervalo del
// cableado, lo para al cancelar el contexto del servidor y espera a que salga.
func TestServeRunsTheSweepLoopAndWaitsForItOnShutdown(t *testing.T) {
	sw := &countingSweeper{work: 30 * time.Millisecond}
	w := wiring{sweepInterval: 10 * time.Millisecond, wrapSweeper: func(sweeper) sweeper { return sw }}
	cfg := config.Config{DatabaseURL: "postgres://wanka_app:x@127.0.0.1:1/none", DBPoolMax: 1, HTTPAddr: "127.0.0.1:0"}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- serve(ctx, log, cfg, config.API, "tenant", w) }()

	deadline := time.Now().Add(3 * time.Second)
	for sw.n.Load() < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("serve called Sweep %d times in 3 s with a 10 ms interval", sw.n.Load())
		}
		time.Sleep(time.Millisecond)
	}
	// Se cancela con un Sweep en curso (dura 30 ms): serve no vuelve hasta que termina.
	for sw.active.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("serve = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after cancel")
	}
	if sw.active.Load() != 0 {
		t.Fatal("serve returned while the sweep loop was still running")
	}
}

func TestServeDoesNotSweepOtherPlanes(t *testing.T) {
	for plane, svc := range map[string]config.Service{"platform": config.Platform, "webhooks": config.Webhooks} {
		t.Run(plane, func(t *testing.T) {
			var wrapped atomic.Bool
			w := wiring{sweepInterval: 10 * time.Millisecond, wrapSweeper: func(s sweeper) sweeper { wrapped.Store(true); return s }}
			cfg := config.Config{DatabaseURL: "postgres://x:x@127.0.0.1:1/none", DBPoolMax: 1, HTTPAddr: "127.0.0.1:0"}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			if err := serve(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, svc, plane, w); err != nil {
				t.Fatal(err)
			}
			if wrapped.Load() {
				t.Fatal("a sweeper exists on a plane that must not mount the session chain")
			}
		})
	}
}
