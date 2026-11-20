// Package app es el caso de uso del login: el orden de ADR 0004 (semáforo, cobro
// del intento, hash, resultado) sobre los puertos Store y Hasher.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/domain"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
)

// Errores del login. El HTTP de cada uno lo decide el adaptador.
var (
	// ErrInvalidCredentials cubre todo caso que revelaría si la cuenta existe: contraseña mala, correo
	// inexistente, cuenta bloqueada o cerrada, usuario bloqueado, sin contraseña y segundo paso cerrado.
	ErrInvalidCredentials = errors.New("identidad: invalid credentials")
	// ErrTenantAccessDenied: contraseña correcta y ninguna membresía activa. No se crea sesión.
	ErrTenantAccessDenied = errors.New("identidad: the user has no active membership")
	// ErrEnrollmentUnavailable: el usuario debe enrolar el segundo factor y esa rama es de #25.
	// Cierra: nadie que deba tener 2FA entra sin ella.
	ErrEnrollmentUnavailable = errors.New("identidad: mandatory 2FA enrollment is not available yet")
)

// MFALockedError: contraseña correcta con el segundo paso bloqueado (conventions.md §2).
type MFALockedError struct{ RetryAfter time.Duration }

func (e *MFALockedError) Error() string { return "identidad: the second step is locked" }

// Account es lo que devuelve el cobro del intento.
type Account struct {
	ID           string
	PasswordHash string
	// FailedLogins son los intentos cobrados en la ventana, incluido este. Con 5 y contraseña mala hay que bloquear.
	FailedLogins int
	// FailureStreak son los fallos seguidos con este cobro incluido (0 a 100); solo una contraseña correcta, el
	// restablecimiento o el cambio de contraseña lo ponen en 0.
	FailureStreak int
}

// UserState es el estado de iam.users leído con el candado de la fila. Los tiempos los calcula la base
// con su reloj, no el proceso.
type UserState struct {
	Active       bool
	PasswordHash string
	FailedLogins int
	// FailureStreak: la racha de fallos seguidos; con MaxStreak el ingreso con contraseña está cerrado.
	FailureStreak int
	// Locked: locked_until está en el futuro.
	Locked bool
	// MFALockouts son los bloqueos del segundo paso; MFALockedFor es lo que falta del bloqueo vigente (0 si no hay).
	MFALockouts  int
	MFALockedFor time.Duration
}

// Membership es una membresía activa del usuario (MembershipSummary del contrato).
type Membership struct {
	ID            string
	TenantID      string
	TenantName    string
	Industry      string
	TenantStatus  string
	RoleID        string
	RoleName      string
	RequiresMFA   bool
	Selectable    bool
	BlockedReason string
	ReadOnly      bool
	Sites         []Site
}

// Site es una sede activa dentro del alcance de la membresía.
type Site struct {
	ID            string
	Name          string
	CashRegisters []CashRegister
}

// CashRegister es una caja activa de una sede.
type CashRegister struct{ ID, Code string }

// NewSession es la sesión sin empresa que crea el login.
type NewSession struct {
	AccessHash   auth.TokenHash
	RefreshHash  *auth.TokenHash // nil en la sesión a medio login (challengeToken)
	MFASatisfied bool
	AccessTTL    time.Duration
	RefreshTTL   time.Duration // 0 si no hay refresh
	IP           netip.Addr
	UserAgent    string
}

// UserTx son las operaciones del login dentro de la transacción de un usuario, que corre con
// iam.users bloqueada FOR UPDATE.
type UserTx interface {
	// Lock lee y bloquea la fila del usuario.
	Lock(ctx context.Context) (UserState, error)
	// ClearLoginCounters pone failed_logins y login_lockouts en 0: devuelve el cobro.
	ClearLoginCounters(ctx context.Context) error
	// LockAccount aplica el bloqueo (o el cierre) que corresponde a la racha streak (domain.LockoutFor) y encola el
	// aviso en la misma transacción. No hace nada y devuelve false si la fila ya no lo permite.
	LockAccount(ctx context.Context, streak int) (bool, error)
	ActiveMemberships(ctx context.Context) ([]Membership, error)
	// HasConfirmedTOTP dice si el usuario tiene el segundo factor activo.
	HasConfirmedTOTP(ctx context.Context) (bool, error)
	// CreateSession inserta la sesión y devuelve su vencimiento de acceso, con el reloj de la base.
	CreateSession(ctx context.Context, s NewSession) (accessExpiresAt time.Time, err error)
}

// Store es el puerto de persistencia del login.
type Store interface {
	// ChargeAttempt cobra un intento del correo ya normalizado en una transacción corta, atómica y sin
	// red. found es falso si no hay cuenta que cobrar: el correo no existe, la cuenta está bloqueada
	// o cerrada, el usuario está bloqueado o no tiene contraseña.
	ChargeAttempt(ctx context.Context, email string) (acct Account, found bool, err error)
	// InUserTx corre fn en una transacción con el usuario fijado: COMMIT si fn devuelve nil.
	InUserTx(ctx context.Context, userID string, fn func(UserTx) error) error
}

// Hasher es el puerto del hash de contraseñas.
type Hasher interface {
	// Verify devuelve un error si encoded no se puede leer; nunca "coincide" en ese caso.
	Verify(password, encoded string) (bool, error)
	// Dummy gasta el mismo tiempo que Verify.
	Dummy(password string)
}

// Tiempos de la sesión (conventions.md §2).
const (
	AccessTTL    = 15 * time.Minute
	RefreshTTL   = 30 * 24 * time.Hour
	ChallengeTTL = 5 * time.Minute
)

// Outcome es la rama del 200.
type Outcome uint8

// Ramas del 200 del login.
const (
	Authenticated Outcome = iota + 1
	MFARequired
)

// Result es un login que terminó en 200.
type Result struct {
	Outcome        Outcome
	AccessToken    string
	RefreshToken   string
	ChallengeToken string
	ExpiresAt      time.Time
	Memberships    []Membership
}

// Request es la petición del login ya validada. Email llega normalizado.
type Request struct {
	Email     string
	Password  string
	IP        netip.Addr
	UserAgent string
}

// Service es el caso de uso del login.
type Service struct {
	store  Store
	hasher Hasher
	gate   *Gate
	log    *slog.Logger
	floor  time.Duration
}

// Option ajusta el Service.
type Option func(*Service)

// WithFloor fija el piso de tiempo de todo 401 invalid_credentials: no responde antes de que pasen d desde que
// empezó la petición. Tapa lo poco que difieren las ramas (el segundo paso cerrado, el 5.º fallo que bloquea) del
// camino ficticio. Cero lo apaga.
func WithFloor(d time.Duration) Option { return func(s *Service) { s.floor = d } }

// NewService arma el caso de uso.
func NewService(store Store, hasher Hasher, gate *Gate, log *slog.Logger, opts ...Option) *Service {
	s := &Service{store: store, hasher: hasher, gate: gate, log: log}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Login sigue el orden de ADR 0004: (1) semáforo del hash; sin cupo, ErrOverloaded y no se cobró nada.
// (2) cobro del intento en una transacción corta. (3) hash, ficticio si no hubo cuenta que cobrar; nunca
// dentro de una transacción. (4) resultado, en una sola transacción con la fila del usuario bloqueada.
func (s *Service) Login(ctx context.Context, req Request) (Result, error) {
	start := time.Now()
	res, err := s.login(ctx, req)
	if errors.Is(err, ErrInvalidCredentials) && s.floor > 0 {
		// Piso común: el 401 no sale antes, sea cual sea la rama que lo produjo.
		t := time.NewTimer(s.floor - time.Since(start))
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
		}
	}
	return res, err
}

func (s *Service) login(ctx context.Context, req Request) (Result, error) {
	release, err := s.gate.Acquire(ctx)
	if err != nil {
		return Result{}, err
	}
	defer release()

	acct, found, err := s.store.ChargeAttempt(ctx, req.Email)
	if err != nil {
		return Result{}, fmt.Errorf("charging the login attempt: %w", err)
	}
	ok := false
	if found {
		ok, err = s.hasher.Verify(req.Password, acct.PasswordHash)
		if err != nil {
			// Un hash guardado ilegible cierra: se trata como contraseña mala. Ni el hash ni el correo van al log.
			s.log.ErrorContext(ctx, "stored password hash is unreadable", "user_id", acct.ID)
			s.hasher.Dummy(req.Password) // sin esto, este usuario respondería sin hashear
			ok = false
		}
	} else {
		s.hasher.Dummy(req.Password)
	}
	release()

	switch {
	case !found:
		return Result{}, ErrInvalidCredentials
	case !ok:
		// Solo el 5.º fallo de la ventana abre una segunda transacción: la contraseña mala hace en la base lo
		// mismo que el correo inexistente (una transacción), y el tiempo no distingue la cuenta.
		if !domain.LockoutFor(acct.FailureStreak).None {
			if err := s.registerFailure(ctx, acct); err != nil {
				return Result{}, err
			}
		}
		return Result{}, ErrInvalidCredentials
	}
	return s.complete(ctx, acct, req)
}

// registerFailure aplica el 5.º fallo de la ventana: bloqueo de 15 min y aviso.
func (s *Service) registerFailure(ctx context.Context, acct Account) error {
	err := s.store.InUserTx(ctx, acct.ID, func(tx UserTx) error {
		st, err := tx.Lock(ctx)
		if err != nil {
			return err
		}
		if !st.Active || st.Locked {
			return nil
		}
		_, err = tx.LockAccount(ctx, acct.FailureStreak)
		return err
	})
	if err != nil {
		return fmt.Errorf("registering the failed login: %w", err)
	}
	return nil
}

// complete es el resultado de una contraseña correcta.
func (s *Service) complete(ctx context.Context, acct Account, req Request) (Result, error) {
	// Los tokens se emiten antes de la transacción: el CSPRNG no espera a la base.
	access, err := auth.NewAccessToken()
	if err != nil {
		return Result{}, err
	}
	refresh, err := auth.NewRefreshToken()
	if err != nil {
		return Result{}, err
	}
	challenge, err := auth.NewChallengeToken()
	if err != nil {
		return Result{}, err
	}
	var (
		res      Result
		decision error // la decisión se confirma (COMMIT) aunque sea un rechazo: el cobro ya se devolvió
	)
	err = s.store.InUserTx(ctx, acct.ID, func(tx UserTx) error {
		res, decision = Result{}, nil
		st, err := tx.Lock(ctx)
		if err != nil {
			return err
		}
		// Entre el cobro y aquí la cuenta pudo bloquearse, cerrarse o cambiar de contraseña: cierra, sin tocar nada.
		// Cerrada mientras tanto: la racha llegó al tope por otro intento, no por este cobro.
		closedMeanwhile := st.FailureStreak >= domain.MaxStreak && acct.FailureStreak < domain.MaxStreak
		if !st.Active || st.Locked || closedMeanwhile || st.PasswordHash != acct.PasswordHash {
			decision = ErrInvalidCredentials
			return nil
		}
		// Contraseña correcta: el cobro se devuelve, también cuando después el segundo paso está cerrado o bloqueado.
		if err := tx.ClearLoginCounters(ctx); err != nil {
			return err
		}
		switch {
		case st.MFALockouts >= domain.MaxMFALockouts:
			decision = ErrInvalidCredentials
			return nil
		case st.MFALockedFor > 0:
			decision = &MFALockedError{RetryAfter: st.MFALockedFor}
			return nil
		}
		mems, err := tx.ActiveMemberships(ctx)
		if err != nil {
			return err
		}
		if len(mems) == 0 {
			decision = ErrTenantAccessDenied
			return nil
		}
		totp, err := tx.HasConfirmedTOTP(ctx)
		if err != nil {
			return err
		}
		switch {
		case totp:
			exp, err := tx.CreateSession(ctx, NewSession{AccessHash: challenge.Hash, AccessTTL: ChallengeTTL, IP: req.IP, UserAgent: req.UserAgent})
			if err != nil {
				return err
			}
			res = Result{Outcome: MFARequired, ChallengeToken: challenge.Value, ExpiresAt: exp}
		case mustEnroll(mems):
			decision = ErrEnrollmentUnavailable
		default:
			rh := refresh.Hash
			exp, err := tx.CreateSession(ctx, NewSession{AccessHash: access.Hash, RefreshHash: &rh, MFASatisfied: true,
				AccessTTL: AccessTTL, RefreshTTL: RefreshTTL, IP: req.IP, UserAgent: req.UserAgent})
			if err != nil {
				return err
			}
			res = Result{Outcome: Authenticated, AccessToken: access.Value, RefreshToken: refresh.Value, ExpiresAt: exp, Memberships: mems}
		}
		return nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("completing the login: %w", err)
	}
	if decision != nil {
		return Result{}, decision
	}
	return res, nil
}

// mustEnroll: algún rol de las membresías activas exige 2FA y el usuario aún no la tiene.
func mustEnroll(ms []Membership) bool {
	for _, m := range ms {
		if m.RequiresMFA {
			return true
		}
	}
	return false
}
