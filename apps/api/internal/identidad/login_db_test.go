package identidad_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/bacsystem/wanka/apps/api/internal/identidad/app"
	jobsapi "github.com/bacsystem/wanka/apps/api/internal/platform/jobs"
	"github.com/bacsystem/wanka/apps/api/internal/platform/mailer/stub"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/apigen/identidad"
)

var (
	atRe = regexp.MustCompile(`^wk_at_[A-Za-z0-9_-]{43}$`)
	rtRe = regexp.MustCompile(`^wk_rt_[A-Za-z0-9_-]{43}$`)
	chRe = regexp.MustCompile(`^wk_ch_[A-Za-z0-9_-]{43}$`)
)

func strict(t *testing.T, body []byte, dst any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		t.Fatalf("the body does not fit the generated contract type (%v): %s", err, body)
	}
}

func sha(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

type sessionRow struct {
	accessHash, refreshHash                                   *string
	accessSecs, refreshSecs                                   *float64
	mfaSatisfied                                              bool
	mfaMethod                                                 string
	tenant, membership, refreshFamily, ip, userAgent, revoked *string
}

func (e *env) session(userID string) sessionRow {
	e.t.Helper()
	var s sessionRow
	e.scan(`SELECT encode(access_hash, 'hex'), encode(refresh_hash, 'hex'),
	               extract(epoch FROM access_expires_at - created_at)::float8, extract(epoch FROM refresh_expires_at - created_at)::float8,
	               mfa_satisfied, mfa_method, tenant_id::text, membership_id::text, refresh_family::text, host(ip), user_agent, revoked_at::text
	          FROM iam.sessions WHERE user_id = $1::uuid`, []any{userID},
		&s.accessHash, &s.refreshHash, &s.accessSecs, &s.refreshSecs, &s.mfaSatisfied, &s.mfaMethod, &s.tenant, &s.membership, &s.refreshFamily, &s.ip, &s.userAgent, &s.revoked)
	return s
}

func near(got *float64, want, tol float64) bool {
	return got != nil && *got > want-tol && *got < want+tol
}

func TestLoginAuthenticated(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	e.addMembership(uid, companySpec{name: "Clínica Sur", sites: 2})
	e.setUser(uid, "failed_logins = 2, login_failure_streak = 2, login_window_started_at = now(), login_lockouts = 3")

	res := e.login("  Ana@X.PE ", goodPassword, func(r *http.Request) { r.Header.Set("User-Agent", "WankaBFF/1.0") })
	if res.Code != 200 {
		t.Fatalf("status = %d: %s", res.Code, res.Body)
	}
	var got identidad.LoginAuthenticated
	strict(t, res.Body.Bytes(), &got)
	if got.Status != identidad.LoginAuthenticatedStatus("authenticated") || !atRe.MatchString(got.AccessToken) || !rtRe.MatchString(got.RefreshToken) {
		t.Fatalf("body = %s", res.Body)
	}
	if time.Until(got.ExpiresAt) < 14*time.Minute || time.Until(got.ExpiresAt) > 16*time.Minute {
		t.Errorf("expiresAt = %v, want about 15 min from now", got.ExpiresAt)
	}
	if len(got.Memberships) != 1 || got.Memberships[0].Tenant.Name != "Clínica Sur" || len(got.Memberships[0].Sites) != 2 ||
		len(got.Memberships[0].Sites[0].CashRegisters) != 1 || got.Memberships[0].Sites[0].CashRegisters[0].Code != "C1" ||
		!got.Memberships[0].Selectable || got.Memberships[0].ReadOnly {
		t.Fatalf("memberships = %+v", got.Memberships)
	}

	s := e.session(uid)
	if s.accessHash == nil || *s.accessHash != sha(got.AccessToken) || s.refreshHash == nil || *s.refreshHash != sha(got.RefreshToken) {
		t.Fatal("the session does not store sha256(token) of the issued tokens")
	}
	if !s.mfaSatisfied || s.mfaMethod != "none" || s.tenant != nil || s.membership != nil || s.refreshFamily == nil {
		t.Fatalf("session = %+v, want mfa_satisfied, mfa_method none, no tenant, a refresh family", s)
	}
	if !near(s.accessSecs, 900, 5) || !near(s.refreshSecs, 30*24*3600, 5) {
		t.Fatalf("access ttl = %v s, refresh ttl = %v s, want 900 and 30 days", *s.accessSecs, *s.refreshSecs)
	}
	if s.ip == nil || !strings.HasPrefix(*s.ip, "10.") || s.userAgent == nil || *s.userAgent != "WankaBFF/1.0" {
		t.Fatalf("ip = %v, user agent = %v", s.ip, s.userAgent)
	}
	u := e.user(uid)
	if u.failed != 0 || u.lockouts != 0 || u.windowStarted != nil {
		t.Fatalf("user = %+v: a correct password returns the charge", u)
	}
	// Lo que el login emitió es lo que el kernel acepta como sesión sin empresa.
	rec, found, err := pgsessionLookup(e, got.AccessToken)
	if err != nil || !found || rec != uid {
		t.Fatalf("session lookup: user = %q, found = %v, err = %v", rec, found, err)
	}
}

func TestLoginMembershipStates(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	e.addMembership(uid, companySpec{name: "A activa", status: "activa"})
	e.addMembership(uid, companySpec{name: "B suspendida", status: "suspendida"})
	e.addMembership(uid, companySpec{name: "C cerrada", status: "cerrada"})
	e.addMembership(uid, companySpec{name: "D invitada", membership: "invitado"})
	e.addMembership(uid, companySpec{name: "E suspendido", membership: "suspendido"})
	res := e.login("ana@x.pe", goodPassword)
	var got identidad.LoginAuthenticated
	strict(t, res.Body.Bytes(), &got)
	names := []string{}
	byName := map[string]identidad.MembershipSummary{}
	for _, m := range got.Memberships {
		names = append(names, m.Tenant.Name)
		byName[m.Tenant.Name] = m
	}
	if strings.Join(names, ",") != "A activa,B suspendida,C cerrada" {
		t.Fatalf("memberships = %v, want only the active ones, ordered by name", names)
	}
	if b := byName["B suspendida"]; b.Selectable || b.BlockedReason == nil || *b.BlockedReason != "tenant_suspended" {
		t.Errorf("suspended company = %+v, want it listed but not selectable", b)
	}
	if c := byName["C cerrada"]; !c.ReadOnly || !c.Selectable {
		t.Errorf("closed company = %+v, want readOnly and selectable", c)
	}
}

func TestLoginWithoutAnActiveMembershipIs403AndCreatesNoSession(t *testing.T) {
	for _, membership := range []string{"invitado", "suspendido", ""} {
		t.Run("membership "+membership, func(t *testing.T) {
			e := newEnv(t, envOpts{})
			uid := e.addUser("ana@x.pe", goodPassword)
			if membership != "" {
				e.addMembership(uid, companySpec{membership: membership})
			}
			e.setUser(uid, "failed_logins = 3, login_failure_streak = 3, login_window_started_at = now()")
			res := e.login("ana@x.pe", goodPassword)
			if res.Code != 403 || res.code(t) != "tenant_access_denied" {
				t.Fatalf("status = %d: %s", res.Code, res.Body)
			}
			if e.countSessions(uid) != 0 {
				t.Fatal("a session was created")
			}
			if u := e.user(uid); u.failed != 0 {
				t.Fatalf("failed_logins = %d: the correct password returns the charge", u.failed)
			}
		})
	}
}

// Mismo cuerpo (salvo el requestId) y mismos encabezados para todo caso que revelaría si la cuenta existe.
func TestEveryCaseThatWouldRevealTheAccountGivesTheSameResponse(t *testing.T) {
	e := newEnv(t, envOpts{})
	mk := func(email, pw, set string) {
		uid := e.addUser(email, pw)
		e.addMembership(uid, companySpec{})
		if set != "" {
			e.setUser(uid, set)
		}
	}
	mk("wrong@x.pe", goodPassword, "")
	mk("locked@x.pe", goodPassword, "locked_until = now() + interval '10 minutes'")
	mk("closed@x.pe", goodPassword, "login_failure_streak = 100")
	mk("blocked@x.pe", goodPassword, "status = 'bloqueado'")
	mk("nohash@x.pe", "", "")
	mk("mfaclosed@x.pe", goodPassword, "mfa_lockouts = 3")
	mk("exhausted@x.pe", goodPassword, "failed_logins = 5, login_failure_streak = 5, login_window_started_at = now() - interval '1 minute'")
	cases := []struct{ name, email, pw string }{
		{"wrong password", "wrong@x.pe", "nope"},
		{"unknown email", "nobody@x.pe", goodPassword},
		{"locked, right password", "locked@x.pe", goodPassword},
		{"closed (100 failures in a row), right password", "closed@x.pe", goodPassword},
		{"blocked user, right password", "blocked@x.pe", goodPassword},
		{"no password hash", "nohash@x.pe", goodPassword},
		{"second step closed, right password", "mfaclosed@x.pe", goodPassword},
		{"window exhausted, right password", "exhausted@x.pe", goodPassword},
	}
	var want string
	for i, tc := range cases {
		res := e.login(tc.email, tc.pw)
		if res.Code != 401 || res.code(t) != "invalid_credentials" || res.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatalf("%s: status = %d, body %s", tc.name, res.Code, res.Body)
		}
		if i == 0 {
			want = res.shape()
			continue
		}
		if got := res.shape(); got != want {
			t.Errorf("%s: response differs from the wrong-password one:\n got  %s\n want %s", tc.name, got, want)
		}
	}
	var sessions int
	e.scan(`SELECT count(*) FROM iam.sessions`, nil, &sessions)
	if sessions != 0 {
		t.Fatalf("%d sessions exist", sessions)
	}
}

// Una cuenta que no puede intentar (bloqueada, cerrada, usuario bloqueado, sin contraseña) no suma al contador.
func TestAccountsThatCannotTryAreNotCharged(t *testing.T) {
	for name, set := range map[string]string{
		"locked":       "locked_until = now() + interval '10 minutes'",
		"closed":       "login_failure_streak = 100",
		"blocked user": "status = 'bloqueado'",
		"no hash":      "password_hash = NULL",
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, envOpts{})
			uid := e.addUser("ana@x.pe", goodPassword)
			e.setUser(uid, set)
			for range 3 {
				e.login("ana@x.pe", "nope")
			}
			if u := e.user(uid); u.failed != 0 || u.windowStarted != nil {
				t.Fatalf("user = %+v, want no charge", u)
			}
		})
	}
}

func TestWrongPasswordChargesInAFixedWindow(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	e.addMembership(uid, companySpec{})
	var first time.Time
	for i := 1; i <= 4; i++ {
		if res := e.login("ana@x.pe", "nope"); res.Code != 401 {
			t.Fatalf("attempt %d: status = %d", i, res.Code)
		}
		u := e.user(uid)
		if u.failed != i || u.lockedUntil != nil || u.windowStarted == nil {
			t.Fatalf("after attempt %d: user = %+v, want %d charged and no lock", i, u, i)
		}
		if i == 1 {
			first = *u.windowStarted
		} else if !u.windowStarted.Equal(first) {
			t.Fatalf("the window start moved on attempt %d: it only moves when the previous one expired", i)
		}
	}
}

func TestFifthFailureLocksTheAccountFor15MinutesAndQueuesTheNotice(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	e.addMembership(uid, companySpec{})
	for i := 1; i <= 5; i++ {
		e.login("ana@x.pe", "nope")
	}
	u := e.user(uid)
	if u.failed != 0 || u.lockouts != 1 || u.lockedUntil == nil {
		t.Fatalf("user = %+v, want the counter back to 0, one lockout and a locked_until", u)
	}
	if d := time.Until(*u.lockedUntil); d < 14*time.Minute+30*time.Second || d > 15*time.Minute+30*time.Second {
		t.Fatalf("locked for %v, want 15 min", d)
	}
	// El aviso va por el outbox, en la misma transacción, con solo el usuario y el fin del bloqueo.
	var kind, args string
	var n int
	e.scan(`SELECT count(*), coalesce(max(kind), ''), coalesce(max(args::text), '') FROM ops.river_job WHERE kind = 'identidad.login_lockout_notice'`, nil, &n, &kind, &args)
	if n != 1 {
		t.Fatalf("%d notices queued, want 1", n)
	}
	if strings.Contains(args, "ana@x.pe") || strings.Contains(args, "nope") || !strings.Contains(args, uid) {
		t.Fatalf("job args = %s: only the user id and the end of the lock, no email nor secrets", args)
	}
	// Durante el bloqueo, ni la contraseña correcta entra, y no hay hash que cobrar ni segundo aviso.
	if res := e.login("ana@x.pe", goodPassword); res.Code != 401 {
		t.Fatalf("right password while locked: status = %d", res.Code)
	}
	if e.countSessions(uid) != 0 {
		t.Fatal("a locked account got a session")
	}
	e.scan(`SELECT count(*) FROM ops.river_job WHERE kind = 'identidad.login_lockout_notice'`, nil, &n)
	if after := e.user(uid); after.failed != 0 || after.lockouts != 1 || n != 1 {
		t.Fatalf("after the locked attempt: user = %+v, notices = %d", after, n)
	}
	// Vencido el bloqueo, entra y el contador de bloqueos vuelve a 0.
	e.setUser(uid, "locked_until = now() - interval '1 second'")
	if res := e.login("ana@x.pe", goodPassword); res.Code != 200 {
		t.Fatalf("right password after the lock expired: status = %d: %s", res.Code, res.Body)
	}
	if after := e.user(uid); after.lockouts != 0 || after.failed != 0 {
		t.Fatalf("user = %+v: a correct login clears the lockout counter", after)
	}
}

func TestFourthFailureDoesNotLock(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	e.setUser(uid, "failed_logins = 3, login_failure_streak = 3, login_window_started_at = now()")
	e.login("ana@x.pe", "nope")
	if u := e.user(uid); u.failed != 4 || u.lockedUntil != nil || u.lockouts != 0 {
		t.Fatalf("user = %+v, want 4 charged and no lock", u)
	}
}

// Una contraseña correcta devuelve el cobro: después de ella hacen falta 5 fallos nuevos para bloquear.
func TestCorrectPasswordReturnsTheCharge(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	e.addMembership(uid, companySpec{})
	e.setUser(uid, "failed_logins = 4, login_failure_streak = 4, login_window_started_at = now(), login_lockouts = 3")
	if res := e.login("ana@x.pe", goodPassword); res.Code != 200 {
		t.Fatalf("status = %d: %s", res.Code, res.Body)
	}
	if u := e.user(uid); u.failed != 0 || u.lockouts != 0 || u.windowStarted != nil {
		t.Fatalf("user = %+v", u)
	}
	for range 4 {
		e.login("ana@x.pe", "nope")
	}
	if u := e.user(uid); u.lockedUntil != nil || u.failed != 4 {
		t.Fatalf("user = %+v: the charge was not returned, 4 failures locked the account", u)
	}
}

// Los bordes de la ventana fija de 15 min.
func TestWindowBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name       string
		set        string
		pw         string
		wantStatus int
		wantFailed int
		moved      bool // la ventana empezó de nuevo
	}{
		{"5 charged, window open, right password: no row, closed", "failed_logins = 5, login_failure_streak = 5, login_window_started_at = now() - interval '14 minutes 55 seconds'", goodPassword, 401, 5, false},
		{"5 charged, window open, wrong password: no row, not charged again", "failed_logins = 5, login_failure_streak = 5, login_window_started_at = now() - interval '14 minutes 55 seconds'", "nope", 401, 5, false},
		{"5 charged, window expired, right password: enters", "failed_logins = 5, login_failure_streak = 5, login_window_started_at = now() - interval '15 minutes 5 seconds'", goodPassword, 200, 0, false},
		{"5 charged, window expired, wrong password: a new window with 1", "failed_logins = 5, login_failure_streak = 5, login_window_started_at = now() - interval '15 minutes 5 seconds'", "nope", 401, 1, true},
		{"3 charged, window open, wrong password: 4 and the window does not move", "failed_logins = 3, login_failure_streak = 3, login_window_started_at = now() - interval '14 minutes 55 seconds'", "nope", 401, 4, false},
		{"3 charged, window expired, wrong password: back to 1", "failed_logins = 3, login_failure_streak = 3, login_window_started_at = now() - interval '15 minutes 5 seconds'", "nope", 401, 1, true},
		{"4 charged, window open, wrong password: the 5th, locks", "failed_logins = 4, login_failure_streak = 4, login_window_started_at = now() - interval '14 minutes 55 seconds'", "nope", 401, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, envOpts{})
			uid := e.addUser("ana@x.pe", goodPassword)
			e.addMembership(uid, companySpec{})
			e.setUser(uid, tc.set)
			var before time.Time
			e.scan(`SELECT coalesce(login_window_started_at, now()) FROM iam.users WHERE id = $1::uuid`, []any{uid}, &before)
			res := e.login("ana@x.pe", tc.pw)
			if res.Code != tc.wantStatus {
				t.Fatalf("status = %d: %s", res.Code, res.Body)
			}
			u := e.user(uid)
			if u.failed != tc.wantFailed {
				t.Fatalf("failed_logins = %d, want %d", u.failed, tc.wantFailed)
			}
			if tc.moved && (u.windowStarted == nil || !u.windowStarted.After(before)) {
				t.Fatalf("window started %v, want it moved past %v", u.windowStarted, before)
			}
			if !tc.moved && tc.wantStatus == 401 && u.windowStarted != nil && u.windowStarted.After(before) {
				t.Fatalf("window moved to %v: a fixed window only moves once the previous one expired", u.windowStarted)
			}
		})
	}
}

// Escalera del bloqueo (ADR 0014): listas escritas a mano, no la fórmula del código. Cada caso parte de una racha de
// fallos seguidos, falla una vez y mira qué quedó.
func TestFailureStreakLadder(t *testing.T) {
	for _, tc := range []struct {
		name       string
		before     int           // racha antes del fallo
		wantLocked time.Duration // 0 = no bloquea
		wantCount  int           // login_lockouts esperado
		closed     bool
	}{
		{"3 to 4: nothing", 3, 0, 0, false},
		{"4 to 5: first lockout, 15 min", 4, 15 * time.Minute, 1, false},
		{"9 to 10: second, 30 min", 9, 30 * time.Minute, 2, false},
		{"14 to 15: third, 1 h", 14, time.Hour, 3, false},
		{"19 to 20: fourth, 2 h", 19, 2 * time.Hour, 4, false},
		{"24 to 25: fifth, 4 h", 24, 4 * time.Hour, 5, false},
		{"29 to 30: sixth, 8 h", 29, 8 * time.Hour, 6, false},
		{"34 to 35: seventh, 16 h", 34, 16 * time.Hour, 7, false},
		{"39 to 40: eighth, 24 h (the cap)", 39, 24 * time.Hour, 8, false},
		{"49 to 50: tenth, 24 h, counter at 10", 49, 24 * time.Hour, 10, false},
		{"54 to 55: eleventh, 24 h, counter stays at 10", 54, 24 * time.Hour, 10, false},
		{"94 to 95: nineteenth, 24 h", 94, 24 * time.Hour, 10, false},
		{"97 to 98: nothing", 97, 0, 0, false},
		{"98 to 99: nothing", 98, 0, 0, false},
		{"99 to 100: closed, no lock time", 99, 0, 10, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, envOpts{})
			uid := e.addUser("ana@x.pe", goodPassword)
			e.addMembership(uid, companySpec{})
			e.setUser(uid, "login_failure_streak = "+strconv.Itoa(tc.before))
			if res := e.login("ana@x.pe", "nope"); res.Code != 401 {
				t.Fatalf("status = %d", res.Code)
			}
			u := e.user(uid)
			if u.streak != tc.before+1 || u.failed != map[bool]int{true: 0, false: 1}[tc.wantLocked > 0 || tc.closed] {
				t.Fatalf("user = %+v, want streak %d", u, tc.before+1)
			}
			if tc.wantLocked > 0 {
				if u.lockedUntil == nil || absDur(time.Until(*u.lockedUntil)-tc.wantLocked) > 30*time.Second {
					t.Fatalf("locked_until = %v, want about %v from now", u.lockedUntil, tc.wantLocked)
				}
			} else if u.lockedUntil != nil {
				t.Fatalf("locked_until = %v, want none", u.lockedUntil)
			}
			if u.lockouts != tc.wantCount {
				t.Fatalf("login_lockouts = %d, want %d", u.lockouts, tc.wantCount)
			}
			wantNotices := 0
			if tc.wantLocked > 0 || tc.closed {
				wantNotices = 1
			}
			if got := e.notices(); got != wantNotices {
				t.Fatalf("%d notices queued, want %d", got, wantNotices)
			}
			var hasUntil bool
			if wantNotices == 1 {
				e.scan(`SELECT args ? 'locked_until' FROM ops.river_job WHERE kind = 'identidad.login_lockout_notice'`, nil, &hasUntil)
				if hasUntil == tc.closed {
					t.Fatalf("job args carry locked_until = %v, want %v (only the lockout notice has an end)", hasUntil, !tc.closed)
				}
			}
			if tc.closed {
				var closed bool
				e.scan(`SELECT (args->>'closed')::boolean FROM ops.river_job WHERE kind = 'identidad.login_lockout_notice'`, nil, &closed)
				if !closed {
					t.Fatal("the notice is not the closing one")
				}
			}
		})
	}
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// Con la racha en 100 el ingreso con contraseña está cerrado: todo intento da el 401 genérico sin cobrar nada.
func TestClosedAccountRejectsEveryAttemptWithoutCharging(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	e.addMembership(uid, companySpec{})
	e.setUser(uid, "login_failure_streak = 100, login_lockouts = 10")
	for _, pw := range []string{goodPassword, "nope", goodPassword} {
		if res := e.login("ana@x.pe", pw); res.Code != 401 || res.code(t) != "invalid_credentials" {
			t.Fatalf("password %q: status = %d", pw, res.Code)
		}
	}
	if e.countSessions(uid) != 0 {
		t.Fatal("a closed account got a session")
	}
	if u := e.user(uid); u.streak != 100 || u.failed != 0 || u.windowStarted != nil || e.notices() != 0 {
		t.Fatalf("user = %+v, notices = %d: attempts on a closed account change nothing", u, e.notices())
	}
}

// Una contraseña correcta en cualquier punto de la escalera deja todo en 0.
func TestCorrectPasswordClearsTheWholeLadder(t *testing.T) {
	for _, streak := range []int{1, 4, 5, 34, 39, 94, 99} {
		t.Run("streak "+strconv.Itoa(streak), func(t *testing.T) {
			e := newEnv(t, envOpts{})
			uid := e.addUser("ana@x.pe", goodPassword)
			e.addMembership(uid, companySpec{})
			e.setUser(uid, "login_failure_streak = "+strconv.Itoa(streak)+", login_lockouts = 8, failed_logins = 2, login_window_started_at = now()")
			if res := e.login("ana@x.pe", goodPassword); res.Code != 200 {
				t.Fatalf("status = %d: %s", res.Code, res.Body)
			}
			if u := e.user(uid); u.streak != 0 || u.failed != 0 || u.lockouts != 0 || u.windowStarted != nil || u.lockedUntil != nil {
				t.Fatalf("user = %+v, want everything at 0", u)
			}
		})
	}
}

// La racha no se reinicia ni con la ventana ni con el bloqueo vencido: cuenta fallos con cualquier espaciado.
func TestStreakSurvivesWindowAndLockExpiry(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	e.setUser(uid, "login_failure_streak = 7, failed_logins = 2, login_window_started_at = now() - interval '20 minutes'")
	e.login("ana@x.pe", "nope")
	if u := e.user(uid); u.streak != 8 || u.failed != 1 {
		t.Fatalf("user = %+v, want streak 8 and a new window with 1", u)
	}
}

func TestSecondFactorBranches(t *testing.T) {
	t.Run("confirmed TOTP: mfa_required with a challenge and a half-login session", func(t *testing.T) {
		e := newEnv(t, envOpts{})
		uid := e.addUser("ana@x.pe", goodPassword)
		e.addMembership(uid, companySpec{})
		e.confirmTOTP(uid)
		e.setUser(uid, "failed_logins = 2, login_failure_streak = 2, login_window_started_at = now()")
		res := e.login("ana@x.pe", goodPassword)
		var got identidad.LoginMfaRequired
		strict(t, res.Body.Bytes(), &got)
		if res.Code != 200 || got.Status != identidad.LoginMfaRequiredStatus("mfa_required") || !chRe.MatchString(got.ChallengeToken) {
			t.Fatalf("status = %d: %s", res.Code, res.Body)
		}
		if d := time.Until(got.ExpiresAt); d < 4*time.Minute || d > 6*time.Minute {
			t.Errorf("expiresAt in %v, want about 5 min", d)
		}
		s := e.session(uid)
		if s.accessHash == nil || *s.accessHash != sha(got.ChallengeToken) || s.mfaSatisfied || s.mfaMethod != "none" || s.refreshHash != nil || s.refreshFamily == nil || !near(s.accessSecs, 300, 5) {
			t.Fatalf("session = %+v, want a half login keyed by sha256(challengeToken)", s)
		}
		if strings.Contains(res.Body.String(), "wk_at_") || strings.Contains(res.Body.String(), "wk_rt_") {
			t.Fatal("a half login returned access or refresh tokens")
		}
		// iam.session_lookup ignora la sesión a medio login: el challengeToken no sirve como acceso.
		if _, found, _ := pgsessionLookup(e, got.ChallengeToken); found {
			t.Fatal("the challenge token authenticates requests")
		}
		if u := e.user(uid); u.failed != 0 {
			t.Fatalf("failed_logins = %d", u.failed)
		}
	})
	t.Run("a role that requires 2FA without a factor answers 503 until #25", func(t *testing.T) {
		e := newEnv(t, envOpts{})
		uid := e.addUser("ana@x.pe", goodPassword)
		e.addMembership(uid, companySpec{requiresMFA: true})
		e.setUser(uid, "failed_logins = 2, login_failure_streak = 2, login_window_started_at = now()")
		res := e.login("ana@x.pe", goodPassword)
		if res.Code != 503 || res.code(t) != "dependency_unavailable" || res.Header().Get("Retry-After") != "60" {
			t.Fatalf("status = %d, Retry-After = %q: %s", res.Code, res.Header().Get("Retry-After"), res.Body)
		}
		if e.countSessions(uid) != 0 {
			t.Fatal("somebody who must have 2FA got a session")
		}
		if u := e.user(uid); u.failed != 0 {
			t.Fatalf("failed_logins = %d: the correct password returns the charge", u.failed)
		}
	})
	t.Run("second step locked: 429 with Retry-After until mfa_locked_until, nothing issued", func(t *testing.T) {
		e := newEnv(t, envOpts{})
		uid := e.addUser("ana@x.pe", goodPassword)
		e.addMembership(uid, companySpec{})
		e.confirmTOTP(uid)
		e.setUser(uid, "mfa_locked_until = now() + interval '10 minutes', failed_logins = 3, login_failure_streak = 3, login_window_started_at = now()")
		res := e.login("ana@x.pe", goodPassword)
		var ra int
		_, _ = fmtSscan(res.Header().Get("Retry-After"), &ra)
		if res.Code != 429 || res.code(t) != "rate_limited" || ra < 595 || ra > 601 {
			t.Fatalf("status = %d, Retry-After = %q: %s", res.Code, res.Header().Get("Retry-After"), res.Body)
		}
		if e.countSessions(uid) != 0 {
			t.Fatal("a session was created")
		}
		if u := e.user(uid); u.failed != 0 {
			t.Fatalf("failed_logins = %d: this login does not add to failed_logins", u.failed)
		}
		// Con la contraseña mala sigue siendo el 401 genérico: el 429 solo lo ve quien acertó la contraseña.
		if res := e.login("ana@x.pe", "nope"); res.Code != 401 {
			t.Fatalf("wrong password during the second-step lock: status = %d", res.Code)
		}
	})
	t.Run("second step closed: 401 without adding to failed_logins", func(t *testing.T) {
		e := newEnv(t, envOpts{})
		uid := e.addUser("ana@x.pe", goodPassword)
		e.addMembership(uid, companySpec{})
		e.setUser(uid, "mfa_lockouts = 3, failed_logins = 3, login_failure_streak = 3, login_window_started_at = now()")
		if res := e.login("ana@x.pe", goodPassword); res.Code != 401 {
			t.Fatalf("status = %d", res.Code)
		}
		if u := e.user(uid); u.failed != 0 || u.mfaLockouts != 3 {
			t.Fatalf("user = %+v: the closed second step keeps mfa_lockouts and does not count as a failure", u)
		}
	})
}

func TestIdempotencyKeyIsRejectedBeforeAnythingRuns(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	res := e.login("ana@x.pe", "nope", func(r *http.Request) { r.Header.Set("Idempotency-Key", "0b8f4f5e-3a8e-4c0e-9b7e-1d2f3a4b5c6d") })
	if res.Code != 400 || res.code(t) != "malformed_request" {
		t.Fatalf("status = %d: %s", res.Code, res.Body)
	}
	if u := e.user(uid); u.failed != 0 {
		t.Fatalf("failed_logins = %d: the request ran", u.failed)
	}
}

func TestClientIPInTheSession(t *testing.T) {
	for _, tc := range []struct {
		name   string
		trust  bool
		header string
		want   string // "" = NULL; "10." = prefix del socket
	}{
		{"trusted header", true, "203.0.113.7", "203.0.113.7"},
		{"trusted ipv6", true, "2001:db8::7", "2001:db8::7"},
		{"trusted but invalid", true, "203.0.113.7:80", ""},
		{"not trusted: the header is ignored", false, "203.0.113.7", "10."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, envOpts{trust: tc.trust})
			uid := e.addUser("ana@x.pe", goodPassword)
			e.addMembership(uid, companySpec{})
			res := e.login("ana@x.pe", goodPassword, func(r *http.Request) { r.Header.Set("X-Client-Ip", tc.header) })
			if res.Code != 200 {
				t.Fatalf("status = %d: %s", res.Code, res.Body)
			}
			s := e.session(uid)
			switch {
			case tc.want == "" && s.ip != nil:
				t.Fatalf("ip = %q, want NULL", *s.ip)
			case tc.want == "10." && (s.ip == nil || !strings.HasPrefix(*s.ip, "10.")):
				t.Fatalf("ip = %v, want the socket's", s.ip)
			case tc.want != "" && tc.want != "10." && (s.ip == nil || *s.ip != tc.want):
				t.Fatalf("ip = %v, want %s", s.ip, tc.want)
			}
		})
	}
}

func TestUserAgentIsTruncatedToValidUTF8Within512Bytes(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	e.addMembership(uid, companySpec{})
	ua := strings.Repeat("é", 400) // 800 bytes
	if res := e.login("ana@x.pe", goodPassword, func(r *http.Request) { r.Header.Set("User-Agent", ua) }); res.Code != 200 {
		t.Fatalf("status = %d: %s", res.Code, res.Body)
	}
	var n int
	var valid bool
	e.scan(`SELECT octet_length(user_agent), user_agent = convert_from(convert_to(user_agent, 'UTF8'), 'UTF8') FROM iam.sessions WHERE user_id = $1::uuid`, []any{uid}, &n, &valid)
	if n != 512 || !valid {
		t.Fatalf("user_agent is %d bytes (valid: %v), want 512", n, valid)
	}
}

var _ = context.Background

// Cerrar, restablecer y volver a cerrar: dos avisos de cierre distintos y dos correos (la unicidad del job y la llave de
// idempotencia del correo llevan el instante de cada cierre).
func TestSecondClosingAfterAResetQueuesAndSendsASecondNotice(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	for i := range 2 {
		e.setUser(uid, "login_failure_streak = 99, failed_logins = 0, login_window_started_at = NULL, locked_until = NULL")
		if res := e.login("ana@x.pe", "nope"); res.Code != 401 {
			t.Fatalf("closing %d: status = %d", i+1, res.Code)
		}
		if u := e.user(uid); u.streak != 100 {
			t.Fatalf("closing %d: streak = %d", i+1, u.streak)
		}
		e.setUser(uid, "login_failure_streak = 0, login_lockouts = 0") // el restablecimiento (#26) pone la racha en 0
	}
	rows, err := e.admin.Query(context.Background(), `SELECT args FROM ops.river_job WHERE kind = 'identidad.login_lockout_notice' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var jobs []app.LockoutNoticeArgs
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var a app.LockoutNoticeArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			t.Fatal(err)
		}
		jobs = append(jobs, a)
	}
	if len(jobs) != 2 || !jobs[0].Closed || jobs[0].ClosedAt == nil || jobs[1].ClosedAt == nil || jobs[0].ClosedAt.Equal(*jobs[1].ClosedAt) {
		t.Fatalf("jobs = %+v, want two closing notices with different instants", jobs)
	}
	m := stub.New(slog.New(slog.NewTextHandler(io.Discard, nil)), 10)
	for _, j := range jobs {
		err := app.LockoutNotice(emailDir{"ana@x.pe"}, m)(context.Background(), jobsapi.Job[app.LockoutNoticeArgs]{Args: j})
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := len(m.Sent()); got != 2 {
		t.Fatalf("%d emails sent, want 2: the second closing was deduplicated", got)
	}
}

type emailDir struct{ email string }

func (d emailDir) EmailOf(context.Context, string) (string, error) { return d.email, nil }
