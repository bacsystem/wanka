package identidad_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/adapters/postgres"
	"github.com/bacsystem/wanka/apps/api/internal/identidad/app"
	"github.com/bacsystem/wanka/apps/api/internal/platform/jobs"
)

// slowHasher cuenta los hashes reales y los alarga para que las peticiones se solapen de verdad. Si hay un
// admin, comprueba en cada hash que el rol de la app no tenga ninguna transacción abierta.
type slowHasher struct {
	inner    app.Hasher
	env      *env
	delay    time.Duration
	verifies atomic.Int64
	dummies  atomic.Int64
	open     atomic.Int64 // veces que se vio una transacción abierta durante un hash
	block    chan struct{}
	entered  chan struct{}
	mu       sync.Mutex
	trace    []bool        // en orden de llegada al hash: true si era la contraseña correcta
	holdBad  chan struct{} // si no es nil, los hashes de contraseñas malas esperan aquí
}

func (h *slowHasher) check() {
	if h.env != nil {
		var n int
		_ = h.env.admin.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND usename = 'wanka_app' AND state LIKE 'idle in transaction%'`).Scan(&n)
		if n > 0 {
			h.open.Add(1)
		}
	}
	if h.entered != nil {
		select {
		case h.entered <- struct{}{}:
		default:
		}
	}
	if h.block != nil {
		<-h.block
	}
	time.Sleep(h.delay)
}

func (h *slowHasher) Verify(pw, enc string) (bool, error) {
	h.verifies.Add(1)
	h.mu.Lock()
	h.trace = append(h.trace, pw == goodPassword)
	h.mu.Unlock()
	if h.holdBad != nil && pw != goodPassword {
		<-h.holdBad
	}
	h.check()
	return h.inner.Verify(pw, enc)
}

func (h *slowHasher) Dummy(pw string) { h.dummies.Add(1); h.check(); h.inner.Dummy(pw) }

func (e *env) service(t *testing.T, h *slowHasher, gate *app.Gate) *app.Service {
	t.Helper()
	enq, err := jobs.NewEnqueuer(e.pool)
	if err != nil {
		t.Fatal(err)
	}
	h.inner = e.hash
	return app.NewService(postgres.New(e.pool, enq), h, gate, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (e *env) notices() int {
	var n int
	e.scan(`SELECT count(*) FROM ops.river_job WHERE kind = 'identidad.login_lockout_notice'`, nil, &n)
	return n
}

func req(email, pw string) app.Request {
	return app.Request{Email: email, Password: pw, IP: netip.MustParseAddr("203.0.113.1"), UserAgent: "test"}
}

// N inicios de sesión simultáneos con contraseña mala no regalan N intentos: el cobro es un solo UPDATE y solo
// los 5 primeros llegan a verificar. Del 6.º en adelante no hay fila que cobrar y se tratan como bloqueados.
func TestConcurrentFailuresDoNotGiftAttempts(t *testing.T) {
	for _, tc := range []struct {
		name         string
		charged      int
		n            int
		wantVerifies int64
	}{
		{"fresh account, 16 at once", 0, 16, 5},
		{"4 already charged, 12 at once", 4, 12, 1},
		{"5 charged in the window, 12 at once", 5, 12, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, envOpts{})
			uid := e.addUser("ana@x.pe", goodPassword)
			if tc.charged > 0 {
				e.setUser(uid, "failed_logins = "+string(rune('0'+tc.charged))+", login_failure_streak = "+string(rune('0'+tc.charged))+", login_window_started_at = now()")
			}
			h := &slowHasher{delay: 30 * time.Millisecond}
			svc := e.service(t, h, app.NewGate(64, 128, 30*time.Second))
			var wg sync.WaitGroup
			errs := make(chan error, tc.n)
			for range tc.n {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := svc.Login(context.Background(), req("ana@x.pe", "nope"))
					errs <- err
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				if !errors.Is(err, app.ErrInvalidCredentials) {
					t.Fatalf("Login = %v, want ErrInvalidCredentials", err)
				}
			}
			if got := h.verifies.Load(); got != tc.wantVerifies {
				t.Fatalf("%d passwords were verified, want %d: concurrent requests got extra attempts", got, tc.wantVerifies)
			}
			if got := h.dummies.Load(); got != int64(tc.n)-tc.wantVerifies {
				t.Fatalf("%d dummy hashes, want %d", got, int64(tc.n)-tc.wantVerifies)
			}
			u := e.user(uid)
			wantLock := tc.charged+tc.n >= 5 && tc.charged < 5
			if wantLock && (u.lockouts != 1 || u.failed != 0 || u.lockedUntil == nil || e.notices() != 1) {
				t.Fatalf("user = %+v, notices = %d: want exactly one lockout and one notice", u, e.notices())
			}
		})
	}
}

// Una contraseña correcta entre muchas malas, todas a la vez. Un ingreso correcto devuelve el cobro por diseño (ADR 0004:
// failed_logins y la racha vuelven a 0), así que el presupuesto de 5 se reinicia: como mucho 4 malas cobradas antes de su
// devolución (la correcta ocupa el quinto cobro de la ventana) y 5 después, sea cual sea el orden. Sin ingreso correcto, 5.
func TestConcurrentMixWithTheRightPasswordKeepsTheBudgetPerReset(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	e.addMembership(uid, companySpec{})
	h := &slowHasher{delay: 20 * time.Millisecond}
	svc := e.service(t, h, app.NewGate(64, 128, 30*time.Second))
	var wg sync.WaitGroup
	var ok atomic.Int64
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pw := "nope"
			if i == 7 {
				pw = goodPassword
			}
			if _, err := svc.Login(context.Background(), req("ana@x.pe", pw)); err == nil {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	want := int64(5)
	if ok.Load() == 1 {
		want = 10 // 4 malas antes de la devolución, la correcta y 5 malas después
	}
	if got := h.verifies.Load(); got > want || ok.Load() > 1 {
		t.Fatalf("%d passwords were verified (want at most %d) and %d logins succeeded; order (true = the correct one): %v", got, want, ok.Load(), h.trace)
	}
}

// El reinicio, sin carreras: 4 malas cobradas y en vuelo, la correcta entra y devuelve el cobro, y las malas siguientes
// vuelven a tener 5 intentos. Es el 10 que vio la CI (4 + 1 + 5), explicado: no hay un cobro de más, el ingreso legítimo
// reinicia el presupuesto. Ni el tiempo ni el orden del planificador intervienen.
func TestACorrectLoginRestartsTheBudgetOfTheBadOnes(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	e.addMembership(uid, companySpec{})
	h := &slowHasher{holdBad: make(chan struct{})}
	svc := e.service(t, h, app.NewGate(64, 128, 60*time.Second))
	waitTrace := func(n int) {
		t.Helper()
		deadline := time.Now().Add(60 * time.Second)
		for {
			h.mu.Lock()
			got := len(h.trace)
			h.mu.Unlock()
			if got >= n {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("only %d hashes started, want %d", got, n)
			}
			time.Sleep(time.Millisecond)
		}
	}
	var wg sync.WaitGroup
	bad := func(n int) {
		for range n {
			wg.Add(1)
			go func() { defer wg.Done(); _, _ = svc.Login(context.Background(), req("ana@x.pe", "nope")) }()
		}
	}
	bad(4)
	waitTrace(4) // 4 cobradas, detenidas en el hash
	if _, err := svc.Login(context.Background(), req("ana@x.pe", goodPassword)); err != nil {
		t.Fatalf("the correct password: %v", err)
	}
	if u := e.user(uid); u.failed != 0 || u.streak != 0 {
		t.Fatalf("user = %+v: the correct login returns the charge, the 4 in flight included", u)
	}
	bad(12)
	waitTrace(4 + 1 + 5) // 5 más cobran; las otras 7 ya no consiguen fila y hashean un ficticio
	close(h.holdBad)
	wg.Wait()
	if h.verifies.Load() != 10 || h.dummies.Load() != 7 {
		t.Fatalf("verified %d and dummied %d, want 10 (4 + the correct one + 5) and 7", h.verifies.Load(), h.dummies.Load())
	}
}

// Un 503 del semáforo no cobra el intento (ADR 0004: el semáforo va antes del cobro).
func TestOverloadedRejectionIsNotChargedInTheDatabase(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	h := &slowHasher{block: make(chan struct{}), entered: make(chan struct{}, 1)}
	svc := e.service(t, h, app.NewGate(1, 0, 5*time.Millisecond))
	done := make(chan error, 1)
	go func() { _, err := svc.Login(context.Background(), req("ana@x.pe", "nope")); done <- err }()
	select {
	case <-h.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first login never reached the hasher")
	}
	for range 5 {
		if _, err := svc.Login(context.Background(), req("ana@x.pe", "nope")); !errors.Is(err, app.ErrOverloaded) {
			t.Fatalf("Login = %v, want ErrOverloaded", err)
		}
	}
	// Tampoco una cuenta que no existe esquiva el semáforo: recibe el mismo 503.
	if _, err := svc.Login(context.Background(), req("nobody@x.pe", "nope")); !errors.Is(err, app.ErrOverloaded) {
		t.Fatalf("Login for an unknown email = %v, want ErrOverloaded", err)
	}
	close(h.block)
	if err := <-done; !errors.Is(err, app.ErrInvalidCredentials) {
		t.Fatalf("first Login = %v", err)
	}
	if u := e.user(uid); u.failed != 1 {
		t.Fatalf("failed_logins = %d, want 1: the 503s must not be charged", u.failed)
	}
}

// ADR 0004: el hash nunca corre dentro de una transacción (ni el real ni el ficticio), medido en la base real.
func TestHashNeverRunsWithATransactionOpenInPostgres(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	e.addMembership(uid, companySpec{})
	h := &slowHasher{env: e}
	svc := e.service(t, h, app.NewGate(4, 4, time.Second))
	for _, tc := range []struct{ email, pw string }{{"ana@x.pe", goodPassword}, {"ana@x.pe", "nope"}, {"nobody@x.pe", "x"}} {
		_, _ = svc.Login(context.Background(), req(tc.email, tc.pw))
	}
	if h.verifies.Load()+h.dummies.Load() != 3 {
		t.Fatalf("hashes = %d, want 3", h.verifies.Load()+h.dummies.Load())
	}
	if h.open.Load() != 0 {
		t.Fatalf("a transaction was open during %d hashes", h.open.Load())
	}
}
