package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/apigen/identidad"
	"github.com/bacsystem/wanka/apps/api/internal/identidad/adapters/httpapi"
	"github.com/bacsystem/wanka/apps/api/internal/identidad/app"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
)

type fakeUC struct {
	mu   sync.Mutex
	reqs []app.Request
	res  app.Result
	err  error
}

func (f *fakeUC) Login(_ context.Context, r app.Request) (app.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, r)
	return f.res, f.err
}

func (f *fakeUC) calls() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.reqs) }

type env struct {
	uc  *fakeUC
	clk *clock.Fake
	h   interface {
		ServeHTTP(http.ResponseWriter, *http.Request)
	}
	logs *bytes.Buffer
}

type envOpts struct {
	trust    bool
	emailCap int
	ipCap    int
}

func newEnv(t *testing.T, o envOpts) *env {
	t.Helper()
	e := &env{uc: &fakeUC{}, clk: clock.NewFake(time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)), logs: &bytes.Buffer{}}
	opts := httpapi.Options{ClientIP: func(r *http.Request) netip.Addr { return auth.ClientIP(r, o.trust) }, Clock: e.clk}
	if o.emailCap > 0 {
		opts.EmailLimiter = httpx.LimiterConfig{Capacity: o.emailCap, Refill: 12 * time.Second, MaxKeys: 100}
	}
	if o.ipCap > 0 {
		opts.IPLimiter = httpx.LimiterConfig{Capacity: o.ipCap, Refill: 2 * time.Second, MaxKeys: 100}
	}
	l, err := httpapi.NewLogin(e.uc, opts)
	if err != nil {
		t.Fatal(err)
	}
	e.h = httpx.Chain(l, slog.New(slog.NewTextHandler(e.logs, nil)), metrics.Nop{})
	return e
}

type resp struct {
	*httptest.ResponseRecorder
}

func (e *env) post(body string, mut ...func(*http.Request)) resp {
	r := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "198.51.100.1:5555"
	for _, m := range mut {
		if m != nil {
			m(r)
		}
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return resp{w}
}

func (r resp) problem(t *testing.T) (code string, errs []struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}) {
	t.Helper()
	var p struct {
		Code   string `json:"code"`
		Status int    `json:"status"`
		Errors []struct {
			Field string `json:"field"`
			Code  string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil || p.Status != r.Code {
		t.Fatalf("not a problem body (%v): %s", err, r.Body.String())
	}
	return p.Code, p.Errors
}

func login(email, pw string) string {
	b, _ := json.Marshal(map[string]string{"email": email, "password": pw})
	return string(b)
}

func TestMalformedRequestsAre400AndNeverReachTheUseCase(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		mut  func(*http.Request)
	}{
		"empty body":         {"", nil},
		"not JSON":           {"{nope", nil},
		"array":              {`[]`, nil},
		"email is a number":  {`{"email":1,"password":"x"}`, nil},
		"password is a bool": {`{"email":"a@x.pe","password":true}`, nil},
		"trailing data":      {login("a@x.pe", "x") + `{}`, nil},
		"wrong content type": {login("a@x.pe", "x"), func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }},
		"no content type":    {login("a@x.pe", "x"), func(r *http.Request) { r.Header.Del("Content-Type") }},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, envOpts{})
			res := e.post(tc.body, tc.mut)
			if code, _ := res.problem(t); res.Code != 400 || code != "malformed_request" {
				t.Fatalf("status = %d, code = %q: %s", res.Code, code, res.Body)
			}
			if e.uc.calls() != 0 {
				t.Fatal("the use case ran")
			}
		})
	}
}

func TestSchemaViolationsAre422WithTheOffendingFields(t *testing.T) {
	long := func(n int) string { return strings.Repeat("a", n) }
	for name, tc := range map[string]struct {
		body   string
		fields map[string]string
	}{
		"missing both":           {`{}`, map[string]string{"email": "required", "password": "required"}},
		"null email":             {`{"email":null,"password":"x"}`, map[string]string{"email": "required"}},
		"missing password":       {`{"email":"a@x.pe"}`, map[string]string{"password": "required"}},
		"email of 2":             {login("ab", "x"), map[string]string{"email": "invalid_length"}},
		"email of 255":           {login(long(255), "x"), map[string]string{"email": "invalid_length"}},
		"2 after trimming":       {login("   ab   ", "x"), map[string]string{"email": "invalid_length"}},
		"empty password":         {login("a@x.pe", ""), map[string]string{"password": "invalid_length"}},
		"password of 129":        {login("a@x.pe", long(129)), map[string]string{"password": "invalid_length"}},
		"NUL in the email":       {`{"email":"a\u0000b@x.pe","password":"x"}`, map[string]string{"email": "invalid_format"}},
		"unknown field":          {`{"email":"a@x.pe","password":"x","remember":true}`, map[string]string{"remember": "unknown_field"}},
		"both wrong":             {login("", ""), map[string]string{"email": "invalid_length", "password": "invalid_length"}},
		"128 runes of 2 bytes":   {login("a@x.pe", strings.Repeat("ñ", 129)), map[string]string{"password": "invalid_length"}},
		"unknown and bad length": {`{"email":"a","password":"x","x":1}`, map[string]string{"email": "invalid_length", "x": "unknown_field"}},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, envOpts{})
			res := e.post(tc.body)
			code, errs := res.problem(t)
			if res.Code != 422 || code != "validation_failed" {
				t.Fatalf("status = %d, code = %q: %s", res.Code, code, res.Body)
			}
			got := map[string]string{}
			for _, f := range errs {
				got[f.Field] = f.Code
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.fields) {
				t.Fatalf("fields = %v, want %v", got, tc.fields)
			}
			if e.uc.calls() != 0 {
				t.Fatal("the use case ran")
			}
		})
	}
}

// Los bordes que el contrato admite pasan: 3 y 254 caracteres de correo, 1 y 128 de contraseña (en caracteres, no en bytes).
func TestLengthBoundsThatPass(t *testing.T) {
	for name, body := range map[string]string{
		"email of 3":       login("a@b", "x"),
		"email of 254":     login(strings.Repeat("a", 254), "x"),
		"password of 128":  login("a@x.pe", strings.Repeat("a", 128)),
		"128 runes, 256 B": login("a@x.pe", strings.Repeat("ñ", 128)),
		"password of 1":    login("a@x.pe", "x"),
		"254 + spaces":     login("  "+strings.Repeat("a", 254)+"  ", "x"),
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, envOpts{})
			e.uc.err = app.ErrInvalidCredentials
			if res := e.post(body); res.Code != 401 || e.uc.calls() != 1 {
				t.Fatalf("status = %d, calls = %d: %s", res.Code, e.uc.calls(), res.Body)
			}
		})
	}
}

// Nota de la QA del PR #244: el CHECK de la base no rechaza espacios al borde, así que la normalización es de Go.
func TestEmailIsTrimmedAndLowercasedBeforeTheUseCase(t *testing.T) {
	for in, want := range map[string]string{
		"ana@x.pe":         "ana@x.pe",
		"ANA@X.PE":         "ana@x.pe",
		"  Ana@X.pe":       "ana@x.pe",
		"ana@x.pe  ":       "ana@x.pe",
		"\t Ana@X.PE \r\n": "ana@x.pe",
		"ÁNA@X.PE":         "ána@x.pe",
	} {
		e := newEnv(t, envOpts{})
		e.uc.err = app.ErrInvalidCredentials
		e.post(login(in, "pw"))
		if e.uc.calls() != 1 || e.uc.reqs[0].Email != want {
			t.Errorf("email %q reached the use case as %+v, want %q", in, e.uc.reqs, want)
		}
	}
}

func TestPasswordIsPassedAsIsNotTrimmed(t *testing.T) {
	e := newEnv(t, envOpts{})
	e.uc.err = app.ErrInvalidCredentials
	e.post(login("a@x.pe", "  Pw with spaces  "))
	if got := e.uc.reqs[0].Password; got != "  Pw with spaces  " {
		t.Fatalf("password = %q", got)
	}
}

// Las cuotas de conventions.md §7: 5/min por IP+email y 30/min por IP.
func TestQuotaPerIPAndEmail(t *testing.T) {
	e := newEnv(t, envOpts{})
	e.uc.err = app.ErrInvalidCredentials
	for i := 1; i <= 5; i++ {
		if res := e.post(login("a@x.pe", "x")); res.Code != 401 {
			t.Fatalf("attempt %d: status = %d", i, res.Code)
		}
	}
	res := e.post(login("a@x.pe", "x"))
	if code, _ := res.problem(t); res.Code != 429 || code != "rate_limited" {
		t.Fatalf("6th attempt: status = %d: %s", res.Code, res.Body)
	}
	if ra := res.Header().Get("Retry-After"); ra != "12" {
		t.Errorf("Retry-After = %q, want the 12 s of the next token", ra)
	}
	if res.Header().Get("RateLimit-Limit") == "" {
		t.Error("the 429 has no RateLimit-* headers")
	}
	if e.uc.calls() != 5 {
		t.Fatalf("the use case ran %d times, want 5", e.uc.calls())
	}
	// Otro correo desde la misma IP sigue pasando.
	if res := e.post(login("b@x.pe", "x")); res.Code != 401 {
		t.Fatalf("another email: status = %d", res.Code)
	}
	// El mismo correo en otro formato comparte el bucket: la normalización va antes de la cuota.
	if res := e.post(login("  A@X.PE ", "x")); res.Code != 429 {
		t.Fatalf("the same email with spaces and capitals: status = %d, want 429", res.Code)
	}
	// Pasan 12 s y se recupera una ficha.
	e.clk.Advance(12 * time.Second)
	if res := e.post(login("a@x.pe", "x")); res.Code != 401 {
		t.Fatalf("after the refill: status = %d", res.Code)
	}
}

func TestQuotaPerIPOnly(t *testing.T) {
	e := newEnv(t, envOpts{})
	e.uc.err = app.ErrInvalidCredentials
	for i := 1; i <= 30; i++ {
		if res := e.post(login(fmt.Sprintf("u%d@x.pe", i), "x")); res.Code != 401 {
			t.Fatalf("attempt %d: status = %d", i, res.Code)
		}
	}
	res := e.post(login("u31@x.pe", "x"))
	if res.Code != 429 || res.Header().Get("Retry-After") != "2" {
		t.Fatalf("31st attempt: status = %d, Retry-After = %q", res.Code, res.Header().Get("Retry-After"))
	}
}

// Una petición rechazada no consume fichas de ningún bucket: lo que rechaza el bucket IP+email devuelve la ficha de la IP.
func TestRejectedRequestsDoNotSpendTheIPToken(t *testing.T) {
	e := newEnv(t, envOpts{})
	e.uc.err = app.ErrInvalidCredentials
	for range 5 {
		e.post(login("a@x.pe", "x"))
	}
	for range 20 { // rechazadas por IP+email
		if res := e.post(login("a@x.pe", "x")); res.Code != 429 {
			t.Fatalf("status = %d", res.Code)
		}
	}
	// A la IP le quedan 25 fichas de 30: 25 correos distintos pasan, el 26.º no.
	for i := 1; i <= 25; i++ {
		if res := e.post(login(fmt.Sprintf("n%d@x.pe", i), "x")); res.Code != 401 {
			t.Fatalf("distinct email %d: status = %d (the rejected requests spent IP tokens)", i, res.Code)
		}
	}
	if res := e.post(login("n26@x.pe", "x")); res.Code != 429 {
		t.Fatalf("26th distinct email: status = %d, want 429", res.Code)
	}
}

// El esquema va antes que las cuotas: lo malformado no gasta fichas.
func TestInvalidRequestsDoNotSpendQuota(t *testing.T) {
	e := newEnv(t, envOpts{emailCap: 1, ipCap: 1})
	e.uc.err = app.ErrInvalidCredentials
	for range 10 {
		e.post(`{}`)
		e.post(`nope`)
	}
	if res := e.post(login("a@x.pe", "x")); res.Code != 401 {
		t.Fatalf("status = %d: malformed requests spent the quota", res.Code)
	}
}

// API_TRUST_CLIENT_IP: la cuota por IP solo confía en X-Client-Ip si está activada.
func TestQuotaIPComesFromXClientIPOnlyWhenTrusted(t *testing.T) {
	withIP := func(ip string) func(*http.Request) {
		return func(r *http.Request) { r.Header.Set("X-Client-Ip", ip) }
	}
	t.Run("trusted: each X-Client-Ip has its own bucket", func(t *testing.T) {
		e := newEnv(t, envOpts{trust: true, ipCap: 2})
		e.uc.err = app.ErrInvalidCredentials
		for i := 1; i <= 2; i++ {
			if res := e.post(login(fmt.Sprintf("a%d@x.pe", i), "x"), withIP("203.0.113.1")); res.Code != 401 {
				t.Fatalf("status = %d", res.Code)
			}
		}
		if res := e.post(login("a3@x.pe", "x"), withIP("203.0.113.1")); res.Code != 429 {
			t.Fatalf("third from the same IP: status = %d, want 429", res.Code)
		}
		if res := e.post(login("a4@x.pe", "x"), withIP("203.0.113.2")); res.Code != 401 {
			t.Fatalf("another IP: status = %d, want its own bucket", res.Code)
		}
		if got := e.uc.reqs[len(e.uc.reqs)-1].IP; got != netip.MustParseAddr("203.0.113.2") {
			t.Fatalf("the use case got IP %v", got)
		}
	})
	t.Run("not trusted: the header is ignored and the socket IP is used", func(t *testing.T) {
		e := newEnv(t, envOpts{trust: false, ipCap: 2})
		e.uc.err = app.ErrInvalidCredentials
		for i := 1; i <= 2; i++ {
			if res := e.post(login(fmt.Sprintf("a%d@x.pe", i), "x"), withIP(fmt.Sprintf("203.0.113.%d", i))); res.Code != 401 {
				t.Fatalf("status = %d", res.Code)
			}
		}
		if res := e.post(login("a3@x.pe", "x"), withIP("203.0.113.99")); res.Code != 429 {
			t.Fatalf("a spoofed X-Client-Ip got a fresh bucket: status = %d", res.Code)
		}
		if got := e.uc.reqs[0].IP; got != netip.MustParseAddr("198.51.100.1") {
			t.Fatalf("the use case got IP %v, want the socket's", got)
		}
	})
	t.Run("trusted: a missing, repeated or invalid header shares one unknown bucket", func(t *testing.T) {
		e := newEnv(t, envOpts{trust: true, ipCap: 3})
		e.uc.err = app.ErrInvalidCredentials
		two := func(r *http.Request) {
			r.Header.Add("X-Client-Ip", "203.0.113.1")
			r.Header.Add("X-Client-Ip", "203.0.113.2")
		}
		muts := []func(*http.Request){nil, withIP("not-an-ip"), two}
		for i, m := range muts {
			if res := e.post(login(fmt.Sprintf("a%d@x.pe", i), "x"), m); res.Code != 401 {
				t.Fatalf("case %d: status = %d", i, res.Code)
			}
		}
		if res := e.post(login("z@x.pe", "x"), withIP("203.0.113.1:80")); res.Code != 429 {
			t.Fatalf("an address with a port got its own bucket: status = %d", res.Code)
		}
		if got := e.uc.reqs[0].IP; got.IsValid() {
			t.Fatalf("an unknown IP reached the use case as %v", got)
		}
	})
}

func TestErrorMapping(t *testing.T) {
	for name, tc := range map[string]struct {
		err        error
		status     int
		code       string
		retryAfter string
		bearer     bool
	}{
		"invalid credentials": {app.ErrInvalidCredentials, 401, "invalid_credentials", "", true},
		"wrapped":             {fmt.Errorf("x: %w", app.ErrInvalidCredentials), 401, "invalid_credentials", "", true},
		"no tenant":           {app.ErrTenantAccessDenied, 403, "tenant_access_denied", "", false},
		"overloaded":          {app.ErrOverloaded, 503, "overloaded", "1", false},
		"enrollment":          {app.ErrEnrollmentUnavailable, 503, "dependency_unavailable", "60", false},
		"mfa locked":          {&app.MFALockedError{RetryAfter: 90 * time.Second}, 429, "rate_limited", "90", false},
		"mfa locked, 1.2 s":   {&app.MFALockedError{RetryAfter: 1200 * time.Millisecond}, 429, "rate_limited", "2", false},
		"mfa locked, 0":       {&app.MFALockedError{}, 429, "rate_limited", "1", false},
		"unexpected":          {errors.New("boom"), 500, "internal", "", false},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, envOpts{})
			e.uc.err = tc.err
			res := e.post(login("a@x.pe", "x"))
			if code, _ := res.problem(t); res.Code != tc.status || code != tc.code {
				t.Fatalf("status = %d, code = %q: %s", res.Code, code, res.Body)
			}
			if got := res.Header().Get("Retry-After"); got != tc.retryAfter {
				t.Errorf("Retry-After = %q, want %q", got, tc.retryAfter)
			}
			if got := res.Header().Get("WWW-Authenticate") == "Bearer"; got != tc.bearer {
				t.Errorf("WWW-Authenticate present = %v, want %v", got, tc.bearer)
			}
		})
	}
}

var (
	atRe = regexp.MustCompile(`^wk_at_[A-Za-z0-9_-]{43}$`)
	rtRe = regexp.MustCompile(`^wk_rt_[A-Za-z0-9_-]{43}$`)
	chRe = regexp.MustCompile(`^wk_ch_[A-Za-z0-9_-]{43}$`)
)

func decodeStrict(t *testing.T, body []byte, dst any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		t.Fatalf("the body does not fit the generated contract type (%v): %s", err, body)
	}
}

// El cuerpo del 200 cabe en los tipos que genera openapi/paths/identidad.yaml, sin campos de más, y cumple sus patrones.
func TestAuthenticatedBodyMatchesTheContract(t *testing.T) {
	e := newEnv(t, envOpts{})
	tok := func(p string) string { return p + strings.Repeat("A", 43) }
	e.uc.res = app.Result{Outcome: app.Authenticated, AccessToken: tok("wk_at_"), RefreshToken: tok("wk_rt_"),
		ExpiresAt: time.Date(2026, 11, 19, 12, 15, 0, 0, time.UTC),
		Memberships: []app.Membership{
			{ID: "11111111-1111-1111-1111-111111111111", TenantID: "22222222-2222-2222-2222-222222222222", TenantName: "Clínica Sur", Industry: "veterinaria",
				TenantStatus: "activa", RoleID: "33333333-3333-3333-3333-333333333333", RoleName: "Dueño", RequiresMFA: true, Selectable: true,
				Sites: []app.Site{{ID: "44444444-4444-4444-4444-444444444444", Name: "Principal",
					CashRegisters: []app.CashRegister{{ID: "55555555-5555-5555-5555-555555555555", Code: "C1"}}}}},
			{ID: "66666666-6666-6666-6666-666666666666", TenantID: "77777777-7777-7777-7777-777777777777", TenantStatus: "suspendida",
				RoleID: "88888888-8888-8888-8888-888888888888", RoleName: "Cajero", BlockedReason: "tenant_suspended"},
		}}
	res := e.post(login("a@x.pe", "x"))
	if res.Code != 200 {
		t.Fatalf("status = %d: %s", res.Code, res.Body)
	}
	if ct := res.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := res.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q: a response with tokens must not be cached", cc)
	}
	var got identidad.LoginAuthenticated
	decodeStrict(t, res.Body.Bytes(), &got)
	if got.Status != identidad.LoginAuthenticatedStatus("authenticated") || !atRe.MatchString(got.AccessToken) || !rtRe.MatchString(got.RefreshToken) {
		t.Fatalf("body = %+v", got)
	}
	if got.ExpiresAt.IsZero() || len(got.Memberships) != 2 {
		t.Fatalf("body = %+v", got)
	}
	// expiresAt es RFC 3339 con zona.
	var raw map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &raw)
	if _, err := time.Parse(time.RFC3339, raw["expiresAt"].(string)); err != nil {
		t.Errorf("expiresAt = %v: %v", raw["expiresAt"], err)
	}
	m0, m1 := got.Memberships[0], got.Memberships[1]
	if !m0.Selectable || m0.BlockedReason != nil || m0.Tenant.Name != "Clínica Sur" || !m0.Role.RequiresMfa || len(m0.Sites) != 1 || len(m0.Sites[0].CashRegisters) != 1 {
		t.Errorf("membership 0 = %+v", m0)
	}
	if m1.Selectable || m1.BlockedReason == nil || *m1.BlockedReason != "tenant_suspended" || m1.Sites == nil {
		t.Errorf("membership 1 = %+v: a suspended company is not selectable and still lists no sites as []", m1)
	}
	if !strings.Contains(res.Body.String(), `"sites":[]`) || strings.Contains(res.Body.String(), `"blockedReason":""`) {
		t.Errorf("body = %s", res.Body)
	}
}

func TestMFARequiredBodyMatchesTheContract(t *testing.T) {
	e := newEnv(t, envOpts{})
	e.uc.res = app.Result{Outcome: app.MFARequired, ChallengeToken: "wk_ch_" + strings.Repeat("B", 43), ExpiresAt: time.Date(2026, 11, 19, 12, 5, 0, 0, time.UTC)}
	res := e.post(login("a@x.pe", "x"))
	var got identidad.LoginMfaRequired
	decodeStrict(t, res.Body.Bytes(), &got)
	if res.Code != 200 || got.Status != identidad.LoginMfaRequiredStatus("mfa_required") || !chRe.MatchString(got.ChallengeToken) || got.ExpiresAt.IsZero() {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body)
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Error("Cache-Control is not no-store")
	}
}

// Mismo cuerpo para todo caso que revelaría si la cuenta existe (salvo el requestId, único por petición).
func TestInvalidCredentialsBodyIsTheSameWhateverTheCause(t *testing.T) {
	norm := func(b []byte) string {
		return regexp.MustCompile(`"requestId":"[^"]*"`).ReplaceAllString(string(b), `"requestId":"-"`)
	}
	var first string
	for i, err := range []error{app.ErrInvalidCredentials, fmt.Errorf("closed: %w", app.ErrInvalidCredentials), fmt.Errorf("other: %w", app.ErrInvalidCredentials)} {
		e := newEnv(t, envOpts{})
		e.uc.err = err
		res := e.post(login("a@x.pe", "x"))
		if i == 0 {
			first = norm(res.Body.Bytes())
			continue
		}
		if got := norm(res.Body.Bytes()); got != first {
			t.Fatalf("body %d = %s, want %s", i, got, first)
		}
	}
}

// La contraseña (y el correo) nunca van a los logs, en ninguna rama.
func TestPasswordNeverReachesTheLogs(t *testing.T) {
	const secret = "S3cretPassw0rd!-zq"
	for name, tc := range map[string]struct {
		err error
		res app.Result
	}{
		"ok":          {nil, app.Result{Outcome: app.Authenticated, AccessToken: "wk_at_" + strings.Repeat("A", 43), RefreshToken: "wk_rt_" + strings.Repeat("A", 43)}},
		"bad":         {app.ErrInvalidCredentials, app.Result{}},
		"unexpected":  {errors.New("db exploded"), app.Result{}},
		"overloaded":  {app.ErrOverloaded, app.Result{}},
		"enrollment":  {app.ErrEnrollmentUnavailable, app.Result{}},
		"mfa lockout": {&app.MFALockedError{RetryAfter: time.Minute}, app.Result{}},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, envOpts{})
			e.uc.err, e.uc.res = tc.err, tc.res
			e.post(login("ana@x.pe", secret))
			e.post(`{"email":"ana@x.pe","password":` + `"` + secret + `",}`) // malformada
			if strings.Contains(e.logs.String(), secret) {
				t.Fatalf("the password is in the logs:\n%s", e.logs)
			}
			if strings.Contains(e.logs.String(), "ana@x.pe") {
				t.Fatalf("the email is in the logs:\n%s", e.logs)
			}
		})
	}
}
