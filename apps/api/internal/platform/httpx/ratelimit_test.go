package httpx_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

var t0 = time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

// headerKey toma la clave de una cabecera de test: sin sesión ni tenant reales.
func headerKey(name string) httpx.KeyFunc {
	return func(r *http.Request) (string, bool) {
		v := r.Header.Get(name)
		return v, v != ""
	}
}

func newLimiter(t *testing.T, clk *clock.Fake, c httpx.LimiterConfig) *httpx.Limiter {
	t.Helper()
	c.Clock = clk
	l, err := httpx.NewLimiter(c)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// limited arma Chain con GET /r (200, o lo que devuelva status) detrás de los limitadores.
func limited(t *testing.T, status int, ls ...*httpx.Limiter) (http.Handler, *logBuf) {
	t.Helper()
	var mws []httpx.Middleware
	for _, l := range ls {
		mws = append(mws, l.Middleware())
	}
	h, buf, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /r", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = io.WriteString(w, "ok")
		}, httpx.Use(mws...))
		rt.HandleFunc("GET /free", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	})
	return h, buf
}

func hit(h http.Handler, hdr ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/r", nil)
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	return do(h, req)
}

func intHeader(t *testing.T, rec *httptest.ResponseRecorder, name string) int {
	t.Helper()
	v := rec.Header().Get(name)
	n, err := strconv.Atoi(v)
	if err != nil {
		t.Fatalf("%s = %q, want an integer", name, v)
	}
	return n
}

func quota(t *testing.T, rec *httptest.ResponseRecorder) [3]int {
	t.Helper()
	return [3]int{intHeader(t, rec, "RateLimit-Limit"), intHeader(t, rec, "RateLimit-Remaining"), intHeader(t, rec, "RateLimit-Reset")}
}

func TestLimiterRejectsWhenTheKeyRunsOutOfTokens(t *testing.T) {
	clk := clock.NewFake(t0)
	l := newLimiter(t, clk, httpx.LimiterConfig{Name: "session", Capacity: 3, Refill: time.Second, MaxKeys: 10, Key: headerKey("X-Test-Key")})
	h, _ := limited(t, 200, l)

	for i, want := range [][3]int{{3, 2, 1}, {3, 1, 2}, {3, 0, 3}} {
		rec := hit(h, "X-Test-Key", "a")
		if rec.Code != 200 {
			t.Fatalf("request %d: %d", i, rec.Code)
		}
		if got := quota(t, rec); got != want {
			t.Fatalf("request %d: RateLimit-* = %v, want %v", i, got, want)
		}
		if rec.Header().Get("Retry-After") != "" {
			t.Fatalf("request %d: Retry-After on a success", i)
		}
	}
	rec := hit(h, "X-Test-Key", "a")
	mustProblem(t, rec, http.StatusTooManyRequests, "rate_limited")
	if rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q, want 1", rec.Header().Get("Retry-After"))
	}
	if got := quota(t, rec); got != [3]int{3, 0, 3} {
		t.Fatalf("429 RateLimit-* = %v", got)
	}
	// Solo los tres campos separados del borrador IETF hasta -06.
	for _, banned := range []string{"RateLimit", "RateLimit-Policy"} {
		if _, ok := rec.Header()[banned]; ok {
			t.Fatalf("unexpected header %s", banned)
		}
	}
	// Otra clave no se ve afectada.
	if rec := hit(h, "X-Test-Key", "b"); rec.Code != 200 {
		t.Fatalf("other key: %d", rec.Code)
	}
	// Tras la recarga de una ficha vuelve a pasar.
	clk.Advance(time.Second)
	if rec := hit(h, "X-Test-Key", "a"); rec.Code != 200 {
		t.Fatalf("after refill: %d", rec.Code)
	}
}

func TestLimiterRetryAfterIsSecondsToTheNextTokenRoundedUp(t *testing.T) {
	clk := clock.NewFake(t0)
	l := newLimiter(t, clk, httpx.LimiterConfig{Name: "s", Capacity: 1, Refill: 2 * time.Second, MaxKeys: 10, Key: headerKey("K")})
	h, _ := limited(t, 200, l)
	if rec := hit(h, "K", "a"); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	for _, step := range []struct {
		advance time.Duration
		retry   string
	}{
		{0, "2"},                       // 2 s hasta la ficha
		{500 * time.Millisecond, "2"},  // 1,5 s -> 2
		{1001 * time.Millisecond, "1"}, // 0,499 s -> 1
		{498 * time.Millisecond, "1"},  // 1 ms -> mínimo 1
	} {
		clk.Advance(step.advance)
		rec := hit(h, "K", "a")
		mustProblem(t, rec, http.StatusTooManyRequests, "rate_limited")
		if got := rec.Header().Get("Retry-After"); got != step.retry {
			t.Fatalf("after +%v: Retry-After = %q, want %q", step.advance, got, step.retry)
		}
	}
	clk.Advance(time.Millisecond)
	if rec := hit(h, "K", "a"); rec.Code != 200 {
		t.Fatalf("token should be back: %d", rec.Code)
	}
}

func TestLimiterRemainingRoundsDownAndResetRoundsUp(t *testing.T) {
	clk := clock.NewFake(t0)
	l := newLimiter(t, clk, httpx.LimiterConfig{Name: "s", Capacity: 5, Refill: 10 * time.Second, MaxKeys: 10, Key: headerKey("K")})
	h, _ := limited(t, 200, l)
	if got := quota(t, hit(h, "K", "a")); got != [3]int{5, 4, 10} {
		t.Fatalf("first: %v", got)
	}
	clk.Advance(5 * time.Second) // 4,5 fichas
	// Esta petición deja 3,5 fichas: Remaining 3 y 35 s -> faltan 15 s para llenarse.
	if got := quota(t, hit(h, "K", "a")); got != [3]int{5, 3, 15} {
		t.Fatalf("second: %v", got)
	}
}

func TestLimiterHeadersOnAnyStatusAndOnlyOnLimitedRoutes(t *testing.T) {
	clk := clock.NewFake(t0)
	for _, status := range []int{200, 404, 422, 500} {
		l := newLimiter(t, clk, httpx.LimiterConfig{Name: "s", Capacity: 3, Refill: time.Second, MaxKeys: 10, Key: headerKey("K")})
		h, _ := limited(t, status, l)
		rec := hit(h, "K", "a")
		if rec.Code != status || rec.Header().Get("RateLimit-Limit") != "3" {
			t.Fatalf("status %d: got %d headers %v", status, rec.Code, rec.Header())
		}
	}
	l := newLimiter(t, clk, httpx.LimiterConfig{Name: "s", Capacity: 3, Refill: time.Second, MaxKeys: 10, Key: headerKey("K")})
	h, _ := limited(t, 200, l)
	rec := do(h, httptest.NewRequest(http.MethodGet, "/free", nil))
	for k := range rec.Header() {
		if strings.HasPrefix(k, "Ratelimit") {
			t.Fatalf("route without limiter sends %s", k)
		}
	}
	// Sin clave (ok=false) no se limita ni se informa.
	rec = hit(h)
	if rec.Code != 200 || rec.Header().Get("RateLimit-Limit") != "" {
		t.Fatalf("no key: %d %v", rec.Code, rec.Header())
	}
}

// Dos limitadores encadenados (en #174, sesión y tenant): una petición rechazada
// no consume fichas de ningún bucket (conventions.md §7).
func TestChainedLimitersRefundWhenALaterOneRejects(t *testing.T) {
	clk := clock.NewFake(t0)
	session := newLimiter(t, clk, httpx.LimiterConfig{Name: "session", Capacity: 3, Refill: time.Hour, MaxKeys: 10, Key: headerKey("X-Session")})
	tenant := newLimiter(t, clk, httpx.LimiterConfig{Name: "tenant", Capacity: 1, Refill: time.Hour, MaxKeys: 10, Key: headerKey("X-Tenant")})
	h, _ := limited(t, 200, session, tenant)

	steps := []struct {
		tenant string
		want   int
		quota  [3]int // el bucket con menos fichas (en 429, el que rechaza)
	}{
		{"T1", 200, [3]int{1, 0, 3600}}, // sesión 2 fichas, tenant 0: se informa el del tenant
		{"T1", 429, [3]int{1, 0, 3600}}, // rechaza el tenant; la sesión recupera su ficha
		{"T2", 200, [3]int{1, 0, 3600}}, // sesión 1 ficha tras A y C
		{"T3", 200, [3]int{3, 0, 10800}},
		{"T4", 429, [3]int{3, 0, 10800}}, // ahora sí rechaza la sesión
	}
	for i, s := range steps {
		rec := hit(h, "X-Session", "S1", "X-Tenant", s.tenant)
		if rec.Code != s.want {
			t.Fatalf("step %d (%s): status %d, want %d", i, s.tenant, rec.Code, s.want)
		}
		if s.want == 429 {
			mustProblem(t, rec, 429, "rate_limited")
		}
		if got := quota(t, rec); got != s.quota {
			t.Fatalf("step %d: RateLimit-* = %v, want %v", i, got, s.quota)
		}
	}
	// El tercer paso dejó la sesión con 1 ficha de 3 solo si la rechazada no consumió.
	if rec := hit(h, "X-Session", "S2", "X-Tenant", "T1"); rec.Code != 429 {
		t.Fatalf("T1 stays exhausted for any session: %d", rec.Code)
	}
}

func TestChainedLimitersReportTheBucketWithFewerTokens(t *testing.T) {
	clk := clock.NewFake(t0)
	session := newLimiter(t, clk, httpx.LimiterConfig{Name: "session", Capacity: 10, Refill: time.Second, MaxKeys: 10, Key: headerKey("X-Session")})
	tenant := newLimiter(t, clk, httpx.LimiterConfig{Name: "tenant", Capacity: 100, Refill: time.Second, MaxKeys: 10, Key: headerKey("X-Tenant")})
	h, _ := limited(t, 200, session, tenant)
	rec := hit(h, "X-Session", "S", "X-Tenant", "T")
	if got := quota(t, rec); got != [3]int{10, 9, 1} {
		t.Fatalf("session has fewer tokens: %v", got)
	}
	// Con el orden inverso se informa lo mismo.
	h, _ = limited(t, 200, tenant, session)
	rec = hit(h, "X-Session", "S", "X-Tenant", "T")
	if got := quota(t, rec); got != [3]int{10, 8, 2} {
		t.Fatalf("reverse order: %v", got)
	}
}

func TestLimiterSweepDropsOnlyFullBuckets(t *testing.T) {
	clk := clock.NewFake(t0)
	l := newLimiter(t, clk, httpx.LimiterConfig{Name: "s", Capacity: 2, Refill: time.Second, MaxKeys: 10, Key: headerKey("K")})
	h, _ := limited(t, 200, l)
	hit(h, "K", "a")
	hit(h, "K", "a")
	hit(h, "K", "b")
	if l.Len() != 2 || l.Sweep() != 0 {
		t.Fatalf("Len = %d: a non-full bucket must stay", l.Len())
	}
	clk.Advance(time.Second) // b recupera su ficha y está lleno; a tiene 1 de 2
	if n := l.Sweep(); n != 1 || l.Len() != 1 {
		t.Fatalf("Sweep removed %d, Len = %d, want 1 and 1", n, l.Len())
	}
	clk.Advance(time.Second)
	if n := l.Sweep(); n != 1 || l.Len() != 0 {
		t.Fatalf("Sweep removed %d, Len = %d, want 1 and 0", n, l.Len())
	}
	// Un bucket lleno equivale a no tenerlo: la clave vuelve con toda su capacidad.
	if got := quota(t, hit(h, "K", "a")); got != [3]int{2, 1, 1} {
		t.Fatalf("fresh bucket: %v", got)
	}
}

func TestLimiterKeyCapIs503AndNeverEvictsAbusers(t *testing.T) {
	clk := clock.NewFake(t0)
	l := newLimiter(t, clk, httpx.LimiterConfig{Name: "session", Capacity: 1, Refill: time.Hour, MaxKeys: 2, Key: headerKey("K")})
	h, buf := limited(t, 200, l)
	hit(h, "K", "a")
	hit(h, "K", "b")

	rec := hit(h, "K", "c")
	mustProblem(t, rec, http.StatusServiceUnavailable, "overloaded")
	if rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	if w := buf.lines(t, "rate limiter key limit reached"); len(w) != 1 || w[0]["level"] != "WARN" || w[0]["limiter"] != "session" || w[0]["max_keys"] != float64(2) {
		t.Fatalf("warn = %v", w)
	}
	// "a" sigue limitado: no se le expulsó para hacer sitio.
	mustProblem(t, hit(h, "K", "a"), http.StatusTooManyRequests, "rate_limited")
	if l.Len() != 2 {
		t.Fatalf("Len = %d", l.Len())
	}
	// La clave nueva rechazada no dejó rastro.
	clk.Advance(2 * time.Hour)
	if rec := hit(h, "K", "c"); rec.Code != 200 {
		t.Fatalf("once the old buckets are full again a new key fits: %d", rec.Code)
	}
}

// Al tope, barrer es O(n): se hace como mucho una vez por segundo para que un
// ataque con claves nuevas no convierta cada petición en un recorrido completo.
func TestLimiterKeyCapSweepsAtMostOncePerSecond(t *testing.T) {
	clk := clock.NewFake(t0)
	l := newLimiter(t, clk, httpx.LimiterConfig{Name: "s", Capacity: 2, Refill: 100 * time.Millisecond, MaxKeys: 2, Key: headerKey("K")})
	h, _ := limited(t, 200, l)
	hit(h, "K", "a")
	hit(h, "K", "b")
	mustProblem(t, hit(h, "K", "c"), 503, "overloaded") // barre (nada que quitar) en t0
	clk.Advance(150 * time.Millisecond)                 // a y b ya están llenos
	mustProblem(t, hit(h, "K", "c"), 503, "overloaded") // no se vuelve a barrer
	clk.Advance(time.Second)
	if rec := hit(h, "K", "c"); rec.Code != 200 {
		t.Fatalf("after the throttle the sweep frees the room: %d", rec.Code)
	}
}

func TestLimiterNeverGrantsMoreThanCapacityUnderConcurrency(t *testing.T) {
	const capacity, workers = 10, 100
	clk := clock.NewFake(t0)
	l := newLimiter(t, clk, httpx.LimiterConfig{Name: "s", Capacity: capacity, Refill: time.Second, MaxKeys: 10, Key: headerKey("K")})
	h, _ := limited(t, 200, l)

	run := func(advanceOnce bool) (granted int32) {
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if advanceOnce && i == workers/2 {
					clk.Advance(time.Second)
				}
				if hit(h, "K", "same").Code == 200 {
					atomic.AddInt32(&granted, 1)
				}
			}()
		}
		close(start)
		wg.Wait()
		return granted
	}
	if g := run(false); g != capacity {
		t.Fatalf("granted %d with a frozen clock, want exactly %d", g, capacity)
	}
	// Con una recarga de un intervalo en medio: a lo sumo una ficha más.
	clk.Advance(time.Hour)
	l.Sweep()
	if g := run(true); g > capacity+1 || g < capacity {
		t.Fatalf("granted %d, want %d or %d", g, capacity, capacity+1)
	}
}

func TestNewLimiterValidatesItsConfig(t *testing.T) {
	ok := httpx.LimiterConfig{Name: "s", Capacity: 1, Refill: time.Second, MaxKeys: 1, Key: headerKey("K"), Clock: clock.NewFake(t0)}
	if _, err := httpx.NewLimiter(ok); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*httpx.LimiterConfig){
		"capacity": func(c *httpx.LimiterConfig) { c.Capacity = 0 },
		"refill":   func(c *httpx.LimiterConfig) { c.Refill = 0 },
		"max keys": func(c *httpx.LimiterConfig) { c.MaxKeys = 0 },
		"key func": func(c *httpx.LimiterConfig) { c.Key = nil },
		"clock":    func(c *httpx.LimiterConfig) { c.Clock = nil },
	} {
		c := ok
		mutate(&c)
		if _, err := httpx.NewLimiter(c); err == nil {
			t.Errorf("%s: invalid config accepted", name)
		}
	}
}

// Con el tope de claves del segundo limitador, el primero devuelve su ficha y
// el 503 no informa RateLimit-*.
func TestChainedLimitersRefundWhenALaterOneHitsItsKeyCap(t *testing.T) {
	clk := clock.NewFake(t0)
	session := newLimiter(t, clk, httpx.LimiterConfig{Name: "session", Capacity: 4, Refill: time.Hour, MaxKeys: 10, Key: headerKey("X-Session")})
	tenant := newLimiter(t, clk, httpx.LimiterConfig{Name: "tenant", Capacity: 10, Refill: time.Hour, MaxKeys: 1, Key: headerKey("X-Tenant")})
	h, _ := limited(t, 200, session, tenant)

	if rec := hit(h, "X-Session", "S", "X-Tenant", "T1"); rec.Code != 200 {
		t.Fatalf("first request: %d", rec.Code)
	}
	rec := hit(h, "X-Session", "S", "X-Tenant", "T2") // el tenant no tiene sitio para T2
	mustProblem(t, rec, http.StatusServiceUnavailable, "overloaded")
	// No hay un bucket que informar: el 503 no lleva RateLimit-* (conventions.md §3).
	for k := range rec.Header() {
		if strings.HasPrefix(strings.ToLower(k), "ratelimit") {
			t.Fatalf("the key-cap 503 carries %s", k)
		}
	}
	// La sesión gastó 1 ficha (la primera); la del 503 se devolvió: quedan 3,
	// así que tres más pasan y la cuarta ya no.
	for i := 0; i < 3; i++ {
		if rec := hit(h, "X-Session", "S", "X-Tenant", "T1"); rec.Code != 200 {
			t.Fatalf("request %d after the 503: %d", i, rec.Code)
		}
	}
	mustProblem(t, hit(h, "X-Session", "S", "X-Tenant", "T1"), http.StatusTooManyRequests, "rate_limited")
}
