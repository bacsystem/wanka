package app

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

// ErrOverloaded: no hay cupo para otro hash dentro de la espera acotada. El login
// responde 503 overloaded y no se cobró el intento.
var ErrOverloaded = errors.New("identidad: no capacity for another password hash")

// Gate es el semáforo del hash (ADR 0004): como mucho size hashes a la vez, y la
// espera está acotada en número (maxWaiting en cola) y en tiempo (wait). Lleno, en
// vez de encolar sin fin, rechaza. Lo comparten el hash real y el ficticio: si
// solo el real lo tomara, con carga solo las cuentas que existen recibirían 503.
type Gate struct {
	slots      chan struct{}
	waiting    atomic.Int64
	maxWaiting int64
	wait       time.Duration
}

// NewGate crea el semáforo. size y maxWaiting menores que 1 y wait no positivo se corrigen al mínimo.
func NewGate(size, maxWaiting int, wait time.Duration) *Gate {
	size = max(size, 1)
	if wait <= 0 {
		wait = time.Millisecond
	}
	return &Gate{slots: make(chan struct{}, size), maxWaiting: int64(max(maxWaiting, 0)), wait: wait}
}

// Acquire toma un cupo y devuelve la función que lo suelta (llamarla dos veces es inofensivo). Devuelve
// ErrOverloaded si la cola está llena o la espera vence, y el error del contexto si este termina antes.
func (g *Gate) Acquire(ctx context.Context) (release func(), err error) {
	select {
	case g.slots <- struct{}{}:
		return g.releaser(), nil
	default:
	}
	if g.waiting.Add(1) > g.maxWaiting {
		g.waiting.Add(-1)
		return nil, ErrOverloaded
	}
	defer g.waiting.Add(-1)
	t := time.NewTimer(g.wait)
	defer t.Stop()
	select {
	case g.slots <- struct{}{}:
		return g.releaser(), nil
	case <-t.C:
		return nil, ErrOverloaded
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (g *Gate) releaser() func() {
	var done atomic.Bool
	return func() {
		if done.CompareAndSwap(false, true) {
			<-g.slots
		}
	}
}
