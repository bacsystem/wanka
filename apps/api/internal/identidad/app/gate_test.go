package app_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/app"
)

func TestGateAdmitsUpToItsSizeAndRejectsTheRest(t *testing.T) {
	g := app.NewGate(2, 0, 5*time.Millisecond)
	r1, err := g.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r2, err := g.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Acquire(context.Background()); !errors.Is(err, app.ErrOverloaded) {
		t.Fatalf("third Acquire = %v, want ErrOverloaded (no queue)", err)
	}
	r1()
	r1() // soltar dos veces no libera el cupo de otro
	if _, err := g.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire after a release = %v", err)
	}
	if _, err := g.Acquire(context.Background()); !errors.Is(err, app.ErrOverloaded) {
		t.Fatalf("Acquire with the gate full again = %v, want ErrOverloaded (a double release must not free two slots)", err)
	}
	r2()
}

func TestGateWaitsOnlyForTheBoundedTime(t *testing.T) {
	g := app.NewGate(1, 4, 40*time.Millisecond)
	release, _ := g.Acquire(context.Background())
	defer release()
	start := time.Now()
	_, err := g.Acquire(context.Background())
	if !errors.Is(err, app.ErrOverloaded) {
		t.Fatalf("Acquire = %v, want ErrOverloaded after the wait", err)
	}
	if d := time.Since(start); d < 30*time.Millisecond || d > time.Second {
		t.Fatalf("waited %v, want about 40ms", d)
	}
}

func TestGateWaiterGetsTheSlotWhenItIsReleased(t *testing.T) {
	g := app.NewGate(1, 1, 5*time.Second)
	release, _ := g.Acquire(context.Background())
	got := make(chan error, 1)
	go func() {
		r, err := g.Acquire(context.Background())
		got <- err
		if r != nil {
			r()
		}
	}()
	time.Sleep(20 * time.Millisecond)
	release()
	select {
	case err := <-got:
		if err != nil {
			t.Fatalf("waiter = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the waiter never got the slot")
	}
}

func TestGateQueueOverflowFailsFast(t *testing.T) {
	g := app.NewGate(1, 1, 2*time.Second)
	release, _ := g.Acquire(context.Background())
	defer release()
	started := make(chan struct{})
	go func() { close(started); _, _ = g.Acquire(context.Background()) }() // ocupa el único lugar de la cola
	<-started
	time.Sleep(30 * time.Millisecond)
	start := time.Now()
	if _, err := g.Acquire(context.Background()); !errors.Is(err, app.ErrOverloaded) {
		t.Fatalf("Acquire = %v, want ErrOverloaded", err)
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("the overflow waited %v: a full queue must reject at once", d)
	}
}

func TestGateHonoursContextCancellation(t *testing.T) {
	g := app.NewGate(1, 1, 5*time.Second)
	release, _ := g.Acquire(context.Background())
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	if _, err := g.Acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire = %v, want context.Canceled", err)
	}
}

func TestGateNeverExceedsItsSize(t *testing.T) {
	const size = 3
	g := app.NewGate(size, 100, 5*time.Second)
	var inside, peak atomic.Int64
	var wg sync.WaitGroup
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := g.Acquire(context.Background())
			if err != nil {
				return
			}
			n := inside.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			inside.Add(-1)
			r()
		}()
	}
	wg.Wait()
	if peak.Load() > size {
		t.Fatalf("peak = %d, want at most %d", peak.Load(), size)
	}
}
