package httpx

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
)

// KeyFunc saca de la petición la clave del bucket (la sesión, el tenant...).
// ok=false significa que esta petición no se limita con este limitador.
type KeyFunc func(r *http.Request) (key string, ok bool)

// LimiterConfig configura un Limiter.
type LimiterConfig struct {
	// Name identifica al limitador en el log ("session", "tenant").
	Name string
	// Capacity es el tamaño del bucket (la ráfaga) y Refill el tiempo que tarda
	// en recuperarse una ficha: el ritmo sostenido es una petición por Refill.
	Capacity int
	Refill   time.Duration
	// MaxKeys acota la memoria: con ese número de buckets no llenos, una clave
	// nueva recibe 503 overloaded en vez de expulsar a otra.
	MaxKeys int
	Key     KeyFunc
	Clock   clock.Clock
}

// sweepEvery es cada cuánto puede barrer un limitador que llegó al tope de claves.
const sweepEvery = time.Second

// Limiter es un token bucket por clave, en memoria (api/conventions.md §7).
// Es seguro para varias goroutines. Cada bucket guarda un solo instante, el
// momento en que volverá a estar lleno (el algoritmo GCRA), así que las fichas
// se calculan con aritmética entera de time.Duration, sin redondeos de coma
// flotante. Un bucket lleno equivale a no tenerlo y Sweep lo elimina.
type Limiter struct {
	cfg LimiterConfig

	mu        sync.Mutex
	fullAt    map[string]time.Time
	lastSweep time.Time
}

// NewLimiter valida cfg y devuelve el limitador.
func NewLimiter(cfg LimiterConfig) (*Limiter, error) {
	switch {
	case cfg.Capacity < 1:
		return nil, errors.New("httpx: limiter capacity must be at least 1")
	case cfg.Refill <= 0:
		return nil, errors.New("httpx: limiter refill interval must be positive")
	case cfg.MaxKeys < 1:
		return nil, errors.New("httpx: limiter MaxKeys must be at least 1")
	case cfg.Key == nil:
		return nil, errors.New("httpx: limiter needs a key function")
	case cfg.Clock == nil:
		return nil, errors.New("httpx: limiter needs a clock")
	}
	return &Limiter{cfg: cfg, fullAt: make(map[string]time.Time)}, nil
}

// quotaInfo es lo que se informa en RateLimit-* tras tomar (o negar) una ficha.
type quotaInfo struct {
	limit     int
	remaining int // fichas enteras que quedan, redondeo hacia abajo
	reset     int // segundos hasta el bucket lleno, redondeo hacia arriba
	retry     int // segundos hasta la próxima ficha; solo si se rechazó
}

type outcome int

const (
	granted outcome = iota
	denied
	keysExhausted
)

func ceilSeconds(d time.Duration) int { return int((d + time.Second - 1) / time.Second) }

func (l *Limiter) info(debt time.Duration) quotaInfo {
	used := int((debt + l.cfg.Refill - 1) / l.cfg.Refill) // fichas gastadas, redondeo hacia arriba
	return quotaInfo{limit: l.cfg.Capacity, remaining: max(l.cfg.Capacity-used, 0), reset: ceilSeconds(debt)}
}

// take intenta tomar una ficha de key en now.
func (l *Limiter) take(key string, now time.Time) (quotaInfo, outcome) {
	l.mu.Lock()
	defer l.mu.Unlock()
	full, exists := l.fullAt[key]
	if !exists {
		if len(l.fullAt) >= l.cfg.MaxKeys {
			if now.Sub(l.lastSweep) >= sweepEvery {
				l.sweepLocked(now)
			}
			if len(l.fullAt) >= l.cfg.MaxKeys {
				return quotaInfo{}, keysExhausted
			}
		}
		full = now
	}
	if full.Before(now) {
		full = now
	}
	debt := full.Sub(now)
	if maxDebt := time.Duration(l.cfg.Capacity-1) * l.cfg.Refill; debt > maxDebt {
		q := l.info(debt)
		q.retry = max(ceilSeconds(debt-maxDebt), 1)
		return q, denied
	}
	full = full.Add(l.cfg.Refill)
	l.fullAt[key] = full
	return l.info(full.Sub(now)), granted
}

// refund devuelve la ficha de key que take concedió.
func (l *Limiter) refund(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if full, ok := l.fullAt[key]; ok {
		l.fullAt[key] = full.Add(-l.cfg.Refill)
	}
}

// Sweep elimina los buckets que ya se llenaron y devuelve cuántos. Quien
// construye el limitador puede llamarlo cada cierto tiempo; además se llama
// solo al llegar al tope de claves.
func (l *Limiter) Sweep() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sweepLocked(l.cfg.Clock.Now())
}

func (l *Limiter) sweepLocked(now time.Time) int {
	l.lastSweep = now
	n := 0
	for k, full := range l.fullAt {
		if !full.After(now) {
			delete(l.fullAt, k)
			n++
		}
	}
	return n
}

// Len es el número de buckets en memoria.
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.fullAt)
}

type rateKey struct{}

// rateState lo comparten los limitadores encadenados de una misma petición.
type rateState struct {
	taken []takenToken
	shown *quotaInfo // el del bucket con menos fichas, ya escrito en las cabeceras
}

// takenToken es una ficha concedida por l a key, que se devuelve si un limitador posterior rechaza.
type takenToken struct {
	l   *Limiter
	key string
}

// refundAll devuelve todas las fichas tomadas y dice cuáles eran.
func (s *rateState) refundAll() []takenToken {
	prev := s.taken
	for _, t := range prev {
		t.l.refund(t.key)
	}
	s.taken, s.shown = nil, nil
	return prev
}

// lessTokens dice si a es una cuota con menos fichas que b (a igual número, la que tarda más en llenarse).
func lessTokens(a, b quotaInfo) bool {
	return a.remaining < b.remaining || a.remaining == b.remaining && a.reset > b.reset
}

// peek calcula la cuota de key sin tomar una ficha.
func (l *Limiter) peek(key string) quotaInfo {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.cfg.Clock.Now()
	full := l.fullAt[key]
	if full.Before(now) {
		full = now
	}
	return l.info(full.Sub(now))
}

// setQuota escribe RateLimit-*. Header.Set canonicaliza los nombres
// ("Ratelimit-Limit" en HTTP/1.1); los nombres no distinguen mayúsculas
// (RFC 9110 §5.1) y escribir el mapa con otra grafía rompería Get, Set y Del.
func setQuota(h http.Header, q quotaInfo) {
	h.Set("RateLimit-Limit", strconv.Itoa(q.limit))
	h.Set("RateLimit-Remaining", strconv.Itoa(q.remaining))
	h.Set("RateLimit-Reset", strconv.Itoa(q.reset))
}

// Middleware devuelve el paso de la ruta que aplica el limitador. Se encadena
// con otros (Use(a.Middleware(), b.Middleware())): si uno rechaza, los
// anteriores devuelven su ficha, porque una petición rechazada no consume de
// ningún bucket. Responde 429 rate_limited con Retry-After y, en toda respuesta
// de la ruta, RateLimit-Limit, -Remaining y -Reset del bucket con menos fichas
// (conventions.md §3). Con el tope de claves alcanzado responde 503 overloaded.
func (l *Limiter) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, ok := l.cfg.Key(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			st, _ := r.Context().Value(rateKey{}).(*rateState)
			if st == nil {
				st = &rateState{}
				r = r.WithContext(context.WithValue(r.Context(), rateKey{}, st))
			}
			q, res := l.take(key, l.cfg.Clock.Now())
			switch res {
			case keysExhausted:
				// La cuota informada es la de los buckets ya con la ficha devuelta.
				var best *quotaInfo
				for _, t := range st.refundAll() {
					if p := t.l.peek(t.key); best == nil || lessTokens(p, *best) {
						best = &p
					}
				}
				if best != nil {
					setQuota(w.Header(), *best)
				}
				depsFrom(r.Context()).log.WarnContext(r.Context(), "rate limiter key limit reached",
					"limiter", l.cfg.Name, "max_keys", l.cfg.MaxKeys, "route", routeOf(r))
				WriteProblem(w, r, "overloaded", "the server is busy, retry shortly")
				return
			case denied:
				st.refundAll()
				setQuota(w.Header(), q)
				w.Header().Set("Retry-After", strconv.Itoa(q.retry))
				WriteProblem(w, r, "rate_limited", "too many requests, retry later")
				return
			}
			st.taken = append(st.taken, takenToken{l, key})
			if st.shown == nil || lessTokens(q, *st.shown) {
				st.shown = &q
				setQuota(w.Header(), q)
			}
			next.ServeHTTP(w, r)
		})
	}
}
