// Package clock da la hora al kernel por una interfaz, para que los tests
// controlen el tiempo sin dormir (ADR 0002).
package clock

import (
	"sync"
	"time"
)

// Clock entrega la hora actual.
type Clock interface {
	Now() time.Time
}

// System es el reloj real.
type System struct{}

// Now devuelve time.Now().
func (System) Now() time.Time { return time.Now() }

// Fake es un reloj manual, seguro para varias goroutines: solo avanza con Advance.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake devuelve un reloj detenido en start.
func NewFake(start time.Time) *Fake { return &Fake{now: start} }

// Now devuelve la hora fijada.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance adelanta el reloj en d.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}
