package identidad_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/identidad"
	"github.com/bacsystem/wanka/apps/api/internal/identidad/adapters/argon2"
	"github.com/bacsystem/wanka/apps/api/internal/platform/audit"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth/pgsession"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
)

// fastParams son baratos: los tests de comportamiento no necesitan el costo de producción.
var fastParams = argon2.Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1}

const goodPassword = "correct horse battery staple"

type envOpts struct {
	trust    bool
	hash     argon2.Params
	gateSize int
	gateWait time.Duration
	// emailCap e ipCap fijan las cuotas; sin ellos son holgadas para que no estorben.
	emailCap, ipCap int
}

type env struct {
	t     *testing.T
	d     *dbtest.DB
	pool  *db.Pool
	admin *pgx.Conn
	h     http.Handler
	logs  *bytes.Buffer
	seq   atomic.Int64
	hash  *argon2.Hasher
}

func newEnv(t *testing.T, o envOpts) *env {
	t.Helper()
	if o.hash == (argon2.Params{}) {
		o.hash = fastParams
	}
	if o.emailCap == 0 {
		o.emailCap = 100000
	}
	if o.ipCap == 0 {
		o.ipCap = 100000
	}
	d := dbtest.New(t)
	ctx := context.Background()
	d.Migrate(ctx, t)
	pool, err := db.Open(ctx, d.URL(dbtest.App), db.Options{MaxConns: 20, AppName: "identidad-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	hasher, err := argon2.New(o.hash)
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, d: d, pool: pool, admin: d.Admin(ctx, t), logs: &bytes.Buffer{}, hash: hasher}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	rt := httpx.NewRouter(httpx.RouterOptions{})
	routes, err := auth.NewRoutes(rt, pgsession.New(pool), entitlements.New(pool, entitlements.Options{Logger: log}),
		auth.DenyAllPermissions{}, audit.NewWriter(pool), auth.Options{TrustClientIP: o.trust})
	if err != nil {
		t.Fatal(err)
	}
	err = identidad.Mount(routes, pool, identidad.Options{
		Hash: o.hash, GateSize: o.gateSize, GateWait: o.gateWait, Log: log,
		EmailLimiter: httpx.LimiterConfig{Capacity: o.emailCap, Refill: 12 * time.Second, MaxKeys: 1000},
		IPLimiter:    httpx.LimiterConfig{Capacity: o.ipCap, Refill: 2 * time.Second, MaxKeys: 1000},
	})
	if err != nil {
		t.Fatal(err)
	}
	e.h = httpx.Chain(rt, slog.New(slog.NewTextHandler(e.logs, nil)), metrics.Nop{})
	return e
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.admin.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatalf("%s: %v", sql, err)
	}
}

func (e *env) scan(sql string, args []any, dst ...any) {
	e.t.Helper()
	if err := e.admin.QueryRow(context.Background(), sql, args...).Scan(dst...); err != nil {
		e.t.Fatalf("%s: %v", sql, err)
	}
}

// addUser crea un usuario activo con la contraseña pw (vacía: sin password_hash) y devuelve su id.
func (e *env) addUser(email, pw string) string {
	e.t.Helper()
	var hash any
	if pw != "" {
		h, err := e.hash.Hash(pw)
		if err != nil {
			e.t.Fatal(err)
		}
		hash = h
	}
	var id string
	e.scan(`INSERT INTO iam.users (id, email, password_hash) VALUES (gen_random_uuid(), $1, $2) RETURNING id::text`, []any{email, hash}, &id)
	return id
}

// setUser aplica asignaciones SQL de prueba a la fila del usuario, por ejemplo "failed_logins = 4".
func (e *env) setUser(id, assignments string) {
	e.t.Helper()
	e.exec(`UPDATE iam.users SET `+assignments+` WHERE id = $1::uuid`, id)
}

type userRow struct {
	failed, lockouts, mfaLockouts, streak int
	windowStarted, lockedUntil            *time.Time
	status                                string
}

func (e *env) user(id string) userRow {
	e.t.Helper()
	var r userRow
	e.scan(`SELECT failed_logins, login_lockouts, login_failure_streak, mfa_lockouts, login_window_started_at, locked_until, status FROM iam.users WHERE id = $1::uuid`,
		[]any{id}, &r.failed, &r.lockouts, &r.streak, &r.mfaLockouts, &r.windowStarted, &r.lockedUntil, &r.status)
	return r
}

type companySpec struct {
	status      string // trial, activa, suspendida, cerrada
	name        string
	requiresMFA bool
	membership  string // activo (por defecto), invitado, suspendido
	sites       int    // sedes activas (cada una con una caja)
}

// addMembership crea una empresa con un rol y una membresía del usuario, y devuelve el id de la membresía.
func (e *env) addMembership(userID string, c companySpec) string {
	e.t.Helper()
	if c.status == "" {
		c.status = "activa"
	}
	if c.membership == "" {
		c.membership = "activo"
	}
	if c.name == "" {
		c.name = "Empresa " + fmt.Sprint(e.seq.Add(1))
	}
	var tenant, role, membership string
	e.scan(`INSERT INTO org.tenants (id, status, closed_at, trade_name, industry) VALUES (gen_random_uuid(), $1, CASE WHEN $1 = 'cerrada' THEN now() END, $2, 'retail')
	        RETURNING id::text`, []any{c.status, c.name}, &tenant)
	e.scan(`INSERT INTO iam.roles (id, tenant_id, name, requires_mfa) VALUES (gen_random_uuid(), $1::uuid, 'Rol', $2) RETURNING id::text`, []any{tenant, c.requiresMFA}, &role)
	e.scan(`INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status, all_sites) VALUES (gen_random_uuid(), $1::uuid, $2::uuid, $3::uuid, $4, true) RETURNING id::text`,
		[]any{tenant, userID, role, c.membership}, &membership)
	for i := range c.sites {
		e.exec(`WITH s AS (INSERT INTO org.sites (id, tenant_id, code, name) VALUES (gen_random_uuid(), $1::uuid, $2, $3) RETURNING id)
		        INSERT INTO org.cash_registers (id, tenant_id, site_id, code) SELECT gen_random_uuid(), $1::uuid, s.id, $4 FROM s`,
			tenant, fmt.Sprintf("S%d", i+1), fmt.Sprintf("Sede %d", i+1), fmt.Sprintf("C%d", i+1))
	}
	return membership
}

func (e *env) confirmTOTP(userID string) {
	e.t.Helper()
	e.exec(`INSERT INTO iam.user_totp (user_id, secret_enc, confirmed_at) VALUES ($1::uuid, '\x01', now())`, userID)
}

func (e *env) countSessions(userID string) int {
	e.t.Helper()
	var n int
	e.scan(`SELECT count(*) FROM iam.sessions WHERE user_id = $1::uuid`, []any{userID}, &n)
	return n
}

type response struct {
	*httptest.ResponseRecorder
}

func (r response) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &m); err != nil {
		t.Fatalf("body is not JSON (%v): %s", err, r.Body)
	}
	return m
}

func (r response) code(t *testing.T) string {
	t.Helper()
	c, _ := r.json(t)["code"].(string)
	return c
}

var requestID = regexp.MustCompile(`"requestId":"[^"]*"`)

// shape es lo que un cliente puede observar de una respuesta de error sin el requestId, que es único por petición.
func (r response) shape() string {
	h := r.Header()
	return fmt.Sprintf("%d|%s|%s|%s|%s", r.Code, h.Get("Content-Type"), h.Get("WWW-Authenticate"), h.Get("Retry-After"),
		requestID.ReplaceAllString(r.Body.String(), `"requestId":"-"`))
}

// login hace POST /v1/auth/login desde una IP de socket propia de cada llamada (para no compartir cuota).
func (e *env) login(email, pw string, mut ...func(*http.Request)) response {
	e.t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": pw})
	return e.post(string(body), mut...)
}

func (e *env) post(body string, mut ...func(*http.Request)) response {
	e.t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	n := e.seq.Add(1)
	r.RemoteAddr = fmt.Sprintf("10.%d.%d.%d:4000", n>>16&255, n>>8&255, n&255)
	for _, m := range mut {
		m(r)
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return response{w}
}

// pgsessionLookup busca un token de acceso como lo hace el kernel y devuelve el id del usuario de la sesión.
func pgsessionLookup(e *env, token string) (string, bool, error) {
	h := sha256.Sum256([]byte(token))
	rec, found, err := pgsession.New(e.pool).Lookup(context.Background(), h)
	if err != nil || !found {
		return "", found, err
	}
	return rec.UserID.String(), true, nil
}

func fmtSscan(s string, dst *int) (int, error) { return fmt.Sscan(s, dst) }
