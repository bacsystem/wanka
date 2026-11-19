package jobs

import (
	"math/rand/v2"
	"time"

	"github.com/riverqueue/river/rivertype"
)

// Backoff por defecto: 10 s, duplicando en cada intento, con tope de 1 h.
const (
	DefaultRetryBase = 10 * time.Second
	DefaultRetryMax  = time.Hour
)

// retryPolicy es la política de reintentos del worker: backoff exponencial con
// jitter. (La de River por defecto crece con attempt^4, que no es exponencial.)
type retryPolicy struct {
	base, max time.Duration
	// jitter devuelve un número en [0, 1); se inyecta en los tests.
	jitter func() float64
	now    func() time.Time
}

func newRetryPolicy(base, max time.Duration) retryPolicy {
	return retryPolicy{base: base, max: max, jitter: rand.Float64, now: time.Now}
}

// NextRetry lo llama River tras un intento fallido; job.Attempt ya cuenta el que falló.
func (p retryPolicy) NextRetry(job *rivertype.JobRow) time.Time {
	return p.now().Add(p.delay(job.Attempt))
}

// delay es base * 2^(attempt-1) con tope max, y jitter «equal»: un valor al azar
// entre la mitad y el total. Así las réplicas no reintentan a la vez los jobs
// que fallaron juntos (una dependencia caída) y la espera nunca baja de la mitad.
func (p retryPolicy) delay(attempt int) time.Duration {
	d := p.max
	// base << n desborda mucho antes de n = 62; con n > 40 ya se pasó de cualquier tope razonable.
	if n := attempt - 1; n >= 0 && n < 40 && p.base<<n > 0 && p.base<<n < p.max {
		d = p.base << n
	}
	return d/2 + time.Duration(p.jitter()*float64(d/2))
}
