package app_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/app"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

// fakeStore es un Store en memoria. inTx dice si hay una transacción abierta: el hash no puede correr con ella.
type fakeStore struct {
	mu           sync.Mutex
	acct         app.Account
	found        bool
	state        app.UserState
	mems         []app.Membership
	totp         bool
	charged      int
	txs          int // transacciones abiertas: el cobro y cada InUserTx
	cleared      int
	locked       int
	lockedStreak int
	sessions     []app.NewSession
	inTx         atomic.Bool
}

func (f *fakeStore) ChargeAttempt(context.Context, string) (app.Account, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.charged++
	f.txs++
	return f.acct, f.found, nil
}

func (f *fakeStore) InUserTx(_ context.Context, _ string, fn func(app.UserTx) error) error {
	f.txs++
	f.inTx.Store(true)
	defer f.inTx.Store(false)
	return fn(f)
}

func (f *fakeStore) Lock(context.Context) (app.UserState, error) { return f.state, nil }
func (f *fakeStore) ClearLoginCounters(context.Context) error    { f.cleared++; return nil }
func (f *fakeStore) LockAccount(_ context.Context, streak int) (bool, error) {
	f.locked++
	f.lockedStreak = streak
	return true, nil
}
func (f *fakeStore) ActiveMemberships(context.Context) ([]app.Membership, error) {
	return f.mems, nil
}
func (f *fakeStore) HasConfirmedTOTP(context.Context) (bool, error) { return f.totp, nil }
func (f *fakeStore) CreateSession(_ context.Context, s app.NewSession) (time.Time, error) {
	f.sessions = append(f.sessions, s)
	return time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC).Add(s.AccessTTL), nil
}

// fakeHasher cuenta y registra si corrió dentro de una transacción.
type fakeHasher struct {
	store    *fakeStore
	ok       bool
	err      error
	verifies atomic.Int64
	dummies  atomic.Int64
	inTx     atomic.Bool
	block    chan struct{} // si no es nil, Verify y Dummy esperan aquí
	entered  chan struct{}
}

func (h *fakeHasher) note() {
	if h.store.inTx.Load() {
		h.inTx.Store(true)
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
}
func (h *fakeHasher) Verify(string, string) (bool, error) {
	h.verifies.Add(1)
	h.note()
	return h.ok, h.err
}
func (h *fakeHasher) Dummy(string) { h.dummies.Add(1); h.note() }

var req = app.Request{Email: "ana@x.pe", Password: "pw", IP: netip.MustParseAddr("203.0.113.9"), UserAgent: "ua"}

func goodStore() *fakeStore {
	return &fakeStore{
		acct: app.Account{ID: "u1", PasswordHash: "h"}, found: true,
		state: app.UserState{Active: true, PasswordHash: "h"},
		mems:  []app.Membership{{ID: "m1", TenantID: "t1", TenantStatus: "activa", Selectable: true}},
	}
}

func newSvc(st *fakeStore, h *fakeHasher, gate *app.Gate) *app.Service {
	if gate == nil {
		gate = app.NewGate(4, 4, 50*time.Millisecond)
	}
	return app.NewService(st, h, gate, discard)
}

func TestLoginAuthenticated(t *testing.T) {
	st := goodStore()
	res, err := newSvc(st, &fakeHasher{store: st, ok: true}, nil).Login(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != app.Authenticated || res.AccessToken == "" || res.RefreshToken == "" || len(res.Memberships) != 1 {
		t.Fatalf("result = %+v", res)
	}
	if st.cleared != 1 {
		t.Fatalf("cleared = %d, want the charge returned once", st.cleared)
	}
	s := st.sessions[0]
	if !s.MFASatisfied || s.RefreshHash == nil || s.AccessTTL != 15*time.Minute || s.RefreshTTL != 30*24*time.Hour {
		t.Fatalf("session = %+v, want mfa satisfied, refresh and 15 min / 30 days", s)
	}
	// Lo que se guarda es el sha256 del token completo entregado (conventions.md §2), nunca el token.
	if s.AccessHash != sha256.Sum256([]byte(res.AccessToken)) || *s.RefreshHash != sha256.Sum256([]byte(res.RefreshToken)) || s.AccessHash == *s.RefreshHash {
		t.Fatal("the session hashes are not the sha256 of the issued tokens")
	}
}

func TestLoginMFARequiredIssuesAChallengeNotAnAccessToken(t *testing.T) {
	st := goodStore()
	st.totp = true
	res, err := newSvc(st, &fakeHasher{store: st, ok: true}, nil).Login(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != app.MFARequired || res.ChallengeToken == "" || res.AccessToken != "" || res.RefreshToken != "" || len(res.Memberships) != 0 {
		t.Fatalf("result = %+v", res)
	}
	s := st.sessions[0]
	if s.AccessHash != sha256.Sum256([]byte(res.ChallengeToken)) {
		t.Fatal("the half-login session stores a hash that is not the challenge token's")
	}
	if s.MFASatisfied || s.RefreshHash != nil || s.AccessTTL != 5*time.Minute {
		t.Fatalf("session = %+v, want a half login of 5 min without refresh", s)
	}
	if want := res.ExpiresAt; !want.Equal(time.Date(2026, 11, 19, 12, 5, 0, 0, time.UTC)) {
		t.Fatalf("expiresAt = %v", want)
	}
}

// Contraseña correcta, pero cada caso termina sin sesión y con el cobro devuelto.
func TestLoginRejectionsAfterAGoodPasswordGiveTheChargeBack(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*fakeStore)
		want func(error) bool
	}{
		{"second step closed", func(s *fakeStore) { s.state.MFALockouts = 3 }, func(e error) bool { return errors.Is(e, app.ErrInvalidCredentials) }},
		{"second step locked", func(s *fakeStore) { s.state.MFALockedFor = 90 * time.Second }, func(e error) bool {
			var l *app.MFALockedError
			return errors.As(e, &l) && l.RetryAfter == 90*time.Second
		}},
		{"no active membership", func(s *fakeStore) { s.mems = nil }, func(e error) bool { return errors.Is(e, app.ErrTenantAccessDenied) }},
		{"role requires 2FA and none enrolled", func(s *fakeStore) { s.mems[0].RequiresMFA = true }, func(e error) bool { return errors.Is(e, app.ErrEnrollmentUnavailable) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := goodStore()
			tc.edit(st)
			_, err := newSvc(st, &fakeHasher{store: st, ok: true}, nil).Login(context.Background(), req)
			if !tc.want(err) {
				t.Fatalf("err = %v", err)
			}
			if len(st.sessions) != 0 {
				t.Fatal("a session was created")
			}
			if st.cleared != 1 {
				t.Fatalf("cleared = %d, want 1: the correct password returns the charge", st.cleared)
			}
		})
	}
}

// Con TOTP confirmado y un rol que exige 2FA gana el reto: el usuario ya tiene el factor.
func TestLoginTOTPWinsOverMandatoryEnrollment(t *testing.T) {
	st := goodStore()
	st.totp, st.mems[0].RequiresMFA = true, true
	res, err := newSvc(st, &fakeHasher{store: st, ok: true}, nil).Login(context.Background(), req)
	if err != nil || res.Outcome != app.MFARequired {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
}

func TestLoginStateChangedBetweenChargeAndResultClosesWithoutTouchingIt(t *testing.T) {
	for name, edit := range map[string]func(*app.UserState){
		"blocked user":         func(s *app.UserState) { s.Active = false },
		"locked meanwhile":     func(s *app.UserState) { s.Locked = true },
		"closed meanwhile":     func(s *app.UserState) { s.FailureStreak = 100 },
		"password changed":     func(s *app.UserState) { s.PasswordHash = "other" },
		"password removed":     func(s *app.UserState) { s.PasswordHash = ""; s.Active = false },
		"hash differs by case": func(s *app.UserState) { s.PasswordHash = "H" },
	} {
		t.Run(name, func(t *testing.T) {
			st := goodStore()
			edit(&st.state)
			_, err := newSvc(st, &fakeHasher{store: st, ok: true}, nil).Login(context.Background(), req)
			if !errors.Is(err, app.ErrInvalidCredentials) || len(st.sessions) != 0 {
				t.Fatalf("err = %v, sessions = %d", err, len(st.sessions))
			}
			if st.cleared != 0 {
				t.Fatal("a closed account had its counters cleared")
			}
		})
	}
}

func TestLoginUnknownAccountHashesADummyAndNeverVerifies(t *testing.T) {
	st := &fakeStore{found: false}
	h := &fakeHasher{store: st}
	_, err := newSvc(st, h, nil).Login(context.Background(), req)
	if !errors.Is(err, app.ErrInvalidCredentials) {
		t.Fatalf("err = %v", err)
	}
	if h.dummies.Load() != 1 || h.verifies.Load() != 0 {
		t.Fatalf("dummies = %d, verifies = %d, want 1 and 0", h.dummies.Load(), h.verifies.Load())
	}
}

func TestLoginWrongPasswordLocksOnlyOnEveryFifthStreakFailure(t *testing.T) {
	for _, tc := range []struct {
		name       string
		streak     int // racha con este cobro incluido
		edit       func(*app.UserState)
		wantLocked int
	}{
		{"1st failure", 1, nil, 0},
		{"4th failure", 4, nil, 0},
		{"5th failure", 5, nil, 1},
		{"6th failure", 6, nil, 0},
		{"10th failure", 10, nil, 1},
		{"99th failure", 99, nil, 0},
		{"100th failure: closes", 100, nil, 1},
		{"5th failure but already locked", 5, func(s *app.UserState) { s.Locked = true }, 0},
		{"5th failure of a blocked user", 5, func(s *app.UserState) { s.Active = false }, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := goodStore()
			st.acct.FailureStreak = tc.streak
			if tc.edit != nil {
				tc.edit(&st.state)
			}
			_, err := newSvc(st, &fakeHasher{store: st, ok: false}, nil).Login(context.Background(), req)
			if !errors.Is(err, app.ErrInvalidCredentials) {
				t.Fatalf("err = %v", err)
			}
			if st.locked != tc.wantLocked || st.cleared != 0 || (tc.wantLocked == 1 && st.lockedStreak != tc.streak) {
				t.Fatalf("locked = %d (streak %d), cleared = %d, want %d", st.locked, st.lockedStreak, st.cleared, tc.wantLocked)
			}
		})
	}
}

func TestLoginUnreadableStoredHashIsAWrongPassword(t *testing.T) {
	st := goodStore()
	h := &fakeHasher{store: st, ok: true, err: errors.New("malformed")}
	_, err := newSvc(st, h, nil).Login(context.Background(), req)
	if !errors.Is(err, app.ErrInvalidCredentials) || len(st.sessions) != 0 {
		t.Fatalf("err = %v, sessions = %d", err, len(st.sessions))
	}
	// Un hash ilegible no puede responder sin hashear: eso lo distinguiría por tiempo.
	if h.dummies.Load() != 1 {
		t.Fatalf("dummies = %d, want 1", h.dummies.Load())
	}
}

// El tiempo no revela la cuenta: todo 401 que no es el 5.º fallo usa el mismo número de transacciones que el correo
// inexistente (el cobro). Determinista; no depende del reloj del runner.
func TestEveryPlain401OpensTheSameNumberOfTransactions(t *testing.T) {
	for _, tc := range []struct {
		name string
		st   func() *fakeStore
		ok   bool
		txs  int
	}{
		{"unknown email, locked, closed, blocked or no hash (no account to charge)", func() *fakeStore { return &fakeStore{} }, false, 1},
		{"wrong password, 1st failure", func() *fakeStore { st := goodStore(); st.acct.FailureStreak = 1; return st }, false, 1},
		{"wrong password, 4th failure", func() *fakeStore { st := goodStore(); st.acct.FailureStreak = 4; return st }, false, 1},
		{"unreadable stored hash", func() *fakeStore { st := goodStore(); st.acct.FailureStreak = 2; return st }, true, 1},
		{"wrong password, 99th failure", func() *fakeStore { st := goodStore(); st.acct.FailureStreak = 99; return st }, false, 1},
		{"wrong password, 100th failure: closes", func() *fakeStore { st := goodStore(); st.acct.FailureStreak = 100; return st }, false, 2},
		{"wrong password, 5th failure: the one that locks", func() *fakeStore { st := goodStore(); st.acct.FailureStreak = 5; return st }, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := tc.st()
			h := &fakeHasher{store: st, ok: false}
			if tc.ok { // el hash guardado no se puede leer
				h.err = errors.New("malformed")
			}
			_, err := newSvc(st, h, nil).Login(context.Background(), req)
			if !errors.Is(err, app.ErrInvalidCredentials) {
				t.Fatalf("err = %v", err)
			}
			if st.txs != tc.txs {
				t.Fatalf("%d transactions, want %d", st.txs, tc.txs)
			}
		})
	}
}

// Piso de tiempo común: todo 401 tarda al menos el piso, y lo que no es 401 no se retrasa.
func TestFloorDelaysEvery401AndNothingElse(t *testing.T) {
	const floor = 80 * time.Millisecond
	svc := func(st *fakeStore, ok bool) *app.Service {
		return app.NewService(st, &fakeHasher{store: st, ok: ok}, app.NewGate(4, 4, time.Second), discard, app.WithFloor(floor))
	}
	closed := goodStore()
	closed.state.MFALockouts = 3
	for name, st := range map[string]*app.Service{
		"unknown": svc(&fakeStore{}, false), "wrong password": svc(goodStore(), false), "second step closed": svc(closed, true),
	} {
		start := time.Now()
		_, err := st.Login(context.Background(), req)
		if !errors.Is(err, app.ErrInvalidCredentials) || time.Since(start) < floor {
			t.Errorf("%s: err = %v after %v, want ErrInvalidCredentials not before %v", name, err, time.Since(start), floor)
		}
	}
	start := time.Now()
	if _, err := svc(goodStore(), true).Login(context.Background(), req); err != nil || time.Since(start) >= floor {
		t.Errorf("a successful login: err = %v after %v, want no delay", err, time.Since(start))
	}
}

// ADR 0004: el hash nunca corre dentro de una transacción (ni el real ni el ficticio).
func TestLoginNeverHashesInsideATransaction(t *testing.T) {
	for _, tc := range []struct {
		name  string
		found bool
		ok    bool
	}{{"verify, good", true, true}, {"verify, bad", true, false}, {"dummy", false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			st := goodStore()
			st.found = tc.found
			h := &fakeHasher{store: st, ok: tc.ok}
			_, _ = newSvc(st, h, nil).Login(context.Background(), req)
			if h.verifies.Load()+h.dummies.Load() != 1 {
				t.Fatal("the hasher did not run exactly once")
			}
			if h.inTx.Load() {
				t.Fatal("the password hash ran inside a transaction")
			}
		})
	}
}

// Un 503 del semáforo no cobra el intento: la cuenta no pierde un intento por la carga del servidor.
func TestLoginOverloadedDoesNotChargeTheAttempt(t *testing.T) {
	st := goodStore()
	h := &fakeHasher{store: st, ok: true, block: make(chan struct{}), entered: make(chan struct{}, 1)}
	svc := newSvc(st, h, app.NewGate(1, 0, 5*time.Millisecond))
	done := make(chan error, 1)
	go func() { _, err := svc.Login(context.Background(), req); done <- err }()
	select {
	case <-h.entered: // la primera petición tiene el cupo y ya cobró
	case <-time.After(2 * time.Second):
		t.Fatal("the first login never reached the hasher")
	}
	_, err := svc.Login(context.Background(), req)
	if !errors.Is(err, app.ErrOverloaded) {
		t.Fatalf("second Login = %v, want ErrOverloaded", err)
	}
	if st.charged != 1 {
		t.Fatalf("charged = %d, want 1: the rejected request must not be charged", st.charged)
	}
	close(h.block)
	if err := <-done; err != nil {
		t.Fatalf("first Login = %v", err)
	}
}

// El hash ficticio también toma el semáforo: si no, con carga solo las cuentas que existen recibirían 503.
func TestLoginDummyHashAlsoTakesTheGate(t *testing.T) {
	st := goodStore()
	st.found = false
	h := &fakeHasher{store: st, block: make(chan struct{}), entered: make(chan struct{}, 1)}
	svc := newSvc(st, h, app.NewGate(1, 0, 5*time.Millisecond))
	done := make(chan error, 1)
	go func() { _, err := svc.Login(context.Background(), req); done <- err }()
	select {
	case <-h.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the login for an unknown account never hashed a dummy")
	}
	if _, err := svc.Login(context.Background(), req); !errors.Is(err, app.ErrOverloaded) {
		t.Fatalf("Login for an unknown account with the gate full = %v, want ErrOverloaded, the same as for a real account", err)
	}
	close(h.block)
	<-done
}

// El cupo se suelta antes de la transacción del resultado: una base lenta no retiene el semáforo del hash.
func TestLoginReleasesTheGateBeforeTheResultTransaction(t *testing.T) {
	st := goodStore()
	gate := app.NewGate(1, 0, time.Millisecond)
	h := &fakeHasher{store: st, ok: true}
	svc := newSvc(st, h, gate)
	probe := &gateProbe{gate: gate}
	st2 := &probingStore{fakeStore: st, probe: probe}
	svc = app.NewService(st2, h, gate, discard)
	if _, err := svc.Login(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if !probe.acquired {
		t.Fatal("the gate was still held while the result transaction ran")
	}
}

type gateProbe struct {
	gate     *app.Gate
	acquired bool
}

type probingStore struct {
	*fakeStore
	probe *gateProbe
}

func (p *probingStore) InUserTx(ctx context.Context, id string, fn func(app.UserTx) error) error {
	if r, err := p.probe.gate.Acquire(ctx); err == nil {
		p.probe.acquired = true
		r()
	}
	return p.fakeStore.InUserTx(ctx, id, fn)
}
