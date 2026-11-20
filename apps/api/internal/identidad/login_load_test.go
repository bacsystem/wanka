//go:build !race

package identidad_test

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/adapters/argon2"
)

// Prueba de carga corta del login (#24, R1: 20/s sujeto a medición). No corre en la suite: se activa con
// WANKA_LOAD=1 y se mide con -v y sin -race, con los núcleos de la réplica que se quiere medir:
//
//	WANKA_LOAD=1 GOMAXPROCS=2 WANKA_TEST_ADMIN_URL=... go test ./internal/identidad -run TestLoginLoad -v -count=1
//
// WANKA_LOAD_M (KiB), WANKA_LOAD_T y WANKA_LOAD_P eligen los parámetros de argon2id (por defecto los de producción).
// Una cuenta por usuario virtual (si no, el bloqueo y la cuota por correo medirían otra cosa). Va en proceso, contra
// el router y una base real, así que cuenta el hash, el semáforo y los viajes a la base; no cuenta la red.
func TestLoginLoad(t *testing.T) {
	if os.Getenv("WANKA_LOAD") == "" {
		t.Skip("set WANKA_LOAD=1 to run the load test")
	}
	envInt := func(name string, def int) int {
		if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
			return v
		}
		return def
	}
	p := argon2.Params{MemoryKiB: uint32(envInt("WANKA_LOAD_M", int(argon2.Default.MemoryKiB))),
		Iterations: uint32(envInt("WANKA_LOAD_T", int(argon2.Default.Iterations))), Parallelism: uint8(envInt("WANKA_LOAD_P", int(argon2.Default.Parallelism)))}
	e := newEnv(t, envOpts{hash: p}) // gate de producción: GOMAXPROCS cupos, 2 x núcleos en cola y 300 ms
	const accounts = 400
	for i := range accounts {
		uid := e.addUser(fmt.Sprintf("load%d@x.pe", i), goodPassword)
		e.addMembership(uid, companySpec{})
	}
	t.Logf("argon2id m=%d KiB t=%d p=%d, GOMAXPROCS=%d, %d accounts", p.MemoryKiB, p.Iterations, p.Parallelism, runtime.GOMAXPROCS(0), accounts)

	// Un hash suelto, sin cola: lo que cuesta el trabajo de una petición.
	h, _ := argon2.New(p)
	enc, _ := h.Hash(goodPassword)
	var one []time.Duration
	for range 10 {
		s := time.Now()
		_, _ = h.Verify(goodPassword, enc)
		one = append(one, time.Since(s))
	}
	sort.Slice(one, func(i, j int) bool { return one[i] < one[j] })
	t.Logf("one hash alone: median %v", one[len(one)/2])

	// Etapa en bucle cerrado: C usuarios virtuales piden sin pausa durante dur. Da el máximo sostenido.
	closed := func(c int, dur time.Duration) {
		var mu sync.Mutex
		var rs []sample
		var next int
		var wg sync.WaitGroup
		stop := time.Now().Add(dur)
		for range c {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for time.Now().Before(stop) {
					mu.Lock()
					i := next % accounts
					next++
					mu.Unlock()
					s := time.Now()
					res := e.login(fmt.Sprintf("load%d@x.pe", i), goodPassword)
					d := time.Since(s)
					mu.Lock()
					rs = append(rs, sample{d, res.Code})
					mu.Unlock()
					if res.Code == http.StatusServiceUnavailable {
						time.Sleep(100 * time.Millisecond) // un cliente real espera el Retry-After; sin pausa el 503 inmediato giraría en vacío
					}
				}
			}()
		}
		wg.Wait()
		report(t, fmt.Sprintf("closed loop, %2d virtual users", c), rs, dur)
	}
	// Etapa en bucle abierto: llegan rate peticiones por segundo durante dur, sin esperar a las anteriores.
	open := func(rate int, dur time.Duration) {
		var mu sync.Mutex
		var rs []sample
		var wg sync.WaitGroup
		tick := time.NewTicker(time.Second / time.Duration(rate))
		defer tick.Stop()
		stop := time.Now().Add(dur)
		n := 0
		for time.Now().Before(stop) {
			<-tick.C
			i := n % accounts
			n++
			wg.Add(1)
			go func() {
				defer wg.Done()
				s := time.Now()
				res := e.login(fmt.Sprintf("load%d@x.pe", i), goodPassword)
				d := time.Since(s)
				mu.Lock()
				rs = append(rs, sample{d, res.Code})
				mu.Unlock()
			}()
		}
		wg.Wait()
		report(t, fmt.Sprintf("open loop, %3d req/s arriving", rate), rs, dur)
	}
	for _, c := range []int{2, 4, 6} {
		closed(c, 6*time.Second)
	}
	for _, r := range []int{10, 20, 25, 30, 40} {
		open(r, 8*time.Second)
	}
}

type sample struct {
	d      time.Duration
	status int
}

func report(t *testing.T, label string, rs []sample, dur time.Duration) {
	var ok, busy, other int
	var lat []time.Duration
	for _, r := range rs {
		switch r.status {
		case http.StatusOK:
			ok++
			lat = append(lat, r.d)
		case http.StatusServiceUnavailable:
			busy++
		default:
			other++
		}
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	q := func(p float64) time.Duration {
		if len(lat) == 0 {
			return 0
		}
		return lat[min(int(float64(len(lat))*p), len(lat)-1)]
	}
	t.Logf("%-32s %6.1f ok/s   p50 %-8v p95 %-8v p99 %-8v   503: %d (%.0f%%)   other: %d",
		label, float64(ok)/dur.Seconds(), q(.5).Round(time.Millisecond), q(.95).Round(time.Millisecond), q(.99).Round(time.Millisecond),
		busy, 100*float64(busy)/float64(max(len(rs), 1)), other)
}
