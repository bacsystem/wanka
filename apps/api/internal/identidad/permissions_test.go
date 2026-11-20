package identidad

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

// Estas pruebas están en el paquete (y no en identidad_test) para contar las
// consultas del resolver a través del puerto readRunner.

type env struct {
	d     *dbtest.DB
	admin *pgx.Conn
	pool  *db.Pool
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	d.Migrate(ctx, t)
	pool, err := db.Open(ctx, d.URL(dbtest.App), db.Options{MaxConns: 4, AppName: "identidad-resolver-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return &env{d: d, admin: d.Admin(ctx, t), pool: pool}
}

func (e *env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.admin.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func one[T any](t *testing.T, e *env, sql string, args ...any) T {
	t.Helper()
	var v T
	if err := e.admin.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

func (e *env) tenant(t *testing.T) pgtype.UUID {
	return one[pgtype.UUID](t, e, `INSERT INTO org.tenants (id) VALUES (gen_random_uuid()) RETURNING id`)
}

func (e *env) user(t *testing.T) pgtype.UUID {
	return one[pgtype.UUID](t, e, `INSERT INTO iam.users (id, email) SELECT i, i::text || '@seed.test' FROM gen_random_uuid() i RETURNING id`)
}

// customRole crea un rol propio con los permisos dados.
func (e *env) customRole(t *testing.T, tenant pgtype.UUID, name string, mfa bool, perms ...string) pgtype.UUID {
	t.Helper()
	id := one[pgtype.UUID](t, e, `INSERT INTO iam.roles (id, tenant_id, name, requires_mfa) VALUES (gen_random_uuid(), $1, $2, $3) RETURNING id`, tenant, name, mfa)
	for _, p := range perms {
		e.exec(t, `INSERT INTO iam.role_permissions (tenant_id, role_id, permission_code) VALUES ($1, $2, $3)`, tenant, id, p)
	}
	return id
}

// baseRole siembra los roles base de la empresa (como la fachada, en una transacción de tenant) y devuelve el de la plantilla.
func (e *env) baseRole(t *testing.T, tenant pgtype.UUID, template string) pgtype.UUID {
	t.Helper()
	err := e.pool.InTenantTx(context.Background(), db.Actor{TenantID: tenant, Kind: db.KindSystem}, func(tx pgx.Tx) error {
		return SeedTenantRoles(context.Background(), tx, tenant)
	})
	if err != nil {
		t.Fatal(err)
	}
	return one[pgtype.UUID](t, e, `SELECT id FROM iam.roles WHERE tenant_id = $1 AND template_code = $2`, tenant, template)
}

func (e *env) membership(t *testing.T, tenant, user, role pgtype.UUID, status string) pgtype.UUID {
	t.Helper()
	return one[pgtype.UUID](t, e, `INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status, all_sites) VALUES (gen_random_uuid(), $1, $2, $3, $4, true) RETURNING id`, tenant, user, role, status)
}

func session(tenant, user, membership pgtype.UUID) auth.Session {
	return auth.Session{
		Actor:        db.Actor{TenantID: tenant, UserID: user, Kind: db.KindUser},
		MembershipID: membership,
	}
}

func codesOf(az auth.Authz) []string {
	var out []string
	for c := range az.Permissions {
		out = append(out, string(c))
	}
	slices.Sort(out)
	return out
}

func TestResolveCustomRoleReadsItsOwnPermissionsMFAAndSites(t *testing.T) {
	e := newEnv(t)
	ten, usr := e.tenant(t), e.user(t)
	role := e.customRole(t, ten, "Supervisor", true, "ventas.pos:read", "ventas.pos:create")
	mem := e.membership(t, ten, usr, role, "activo")
	site := one[pgtype.UUID](t, e, `INSERT INTO org.sites (id, tenant_id, code, name) VALUES (gen_random_uuid(), $1, 'S1', 'Sede 1') RETURNING id`, ten)
	e.exec(t, `UPDATE iam.memberships SET all_sites = false WHERE id = $1`, mem)
	e.exec(t, `INSERT INTO iam.membership_sites (tenant_id, membership_id, site_id) VALUES ($1, $2, $3)`, ten, mem, site)
	e.exec(t, `UPDATE iam.memberships SET authz_version = 7 WHERE id = $1`, mem)

	az, err := NewPermissionResolver(e.pool).Resolve(context.Background(), session(ten, usr, mem))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := codesOf(az), []string{"ventas.pos:create", "ventas.pos:read"}; !slices.Equal(got, want) {
		t.Errorf("permissions = %v, want %v", got, want)
	}
	if az.Version != 7 || !az.RequiresMFA || az.AllSites || len(az.Sites) != 1 || az.Sites[0] != site {
		t.Errorf("authz = %+v, want version 7, MFA, not all sites, one site %v", az, site)
	}
}

func TestResolveSystemRoleReadsTheTemplate(t *testing.T) {
	e := newEnv(t)
	ten, usr := e.tenant(t), e.user(t)
	role := e.baseRole(t, ten, "cajero")
	mem := e.membership(t, ten, usr, role, "activo")
	az, err := NewPermissionResolver(e.pool).Resolve(context.Background(), session(ten, usr, mem))
	if err != nil {
		t.Fatal(err)
	}
	want := one[[]string](t, e, `SELECT array_agg(permission_code ORDER BY permission_code) FROM iam.role_template_permissions WHERE template_code = 'cajero'`)
	if got := codesOf(az); !slices.Equal(got, want) || len(want) != 21 {
		t.Errorf("cajero permissions = %d, want the 21 of its template", len(got))
	}
	if az.RequiresMFA || !az.AllSites {
		t.Errorf("cajero authz = %+v, want optional 2FA and all sites", az)
	}
	// El dueño exige 2FA.
	owner := e.baseRole(t, ten, "dueno")
	usr2 := e.user(t)
	mem2 := e.membership(t, ten, usr2, owner, "activo")
	az2, err := NewPermissionResolver(e.pool).Resolve(context.Background(), session(ten, usr2, mem2))
	if err != nil || !az2.RequiresMFA || len(az2.Permissions) != len(permission.All()) {
		t.Errorf("dueno: %d permissions, MFA %v, err %v; want %d with MFA", len(az2.Permissions), az2.RequiresMFA, err, len(permission.All()))
	}
}

// Un rol propio sin permisos devuelve el conjunto vacío (no nil): niega todo, sin error.
func TestResolveRoleWithoutPermissionsDeniesEverything(t *testing.T) {
	e := newEnv(t)
	ten, usr := e.tenant(t), e.user(t)
	mem := e.membership(t, ten, usr, e.customRole(t, ten, "Vacío", false), "activo")
	az, err := NewPermissionResolver(e.pool).Resolve(context.Background(), session(ten, usr, mem))
	if err != nil {
		t.Fatal(err)
	}
	if az.Permissions == nil || len(az.Permissions) != 0 {
		t.Errorf("permissions = %#v, want an empty non-nil set", az.Permissions)
	}
}

// Membresía ausente, suspendida, invitada, de otro usuario o de otro tenant: ErrMembershipLost, que el paso
// responde como 403 tenant_access_denied sin revocar la sesión.
func TestResolveLostMembershipVariants(t *testing.T) {
	e := newEnv(t)
	tenA, tenB := e.tenant(t), e.tenant(t)
	usr, other := e.user(t), e.user(t)
	roleA := e.customRole(t, tenA, "R", false, "ventas.pos:read")
	roleB := e.customRole(t, tenB, "R", false, "ventas.pos:read")
	active := e.membership(t, tenA, usr, roleA, "activo")
	suspended := e.membership(t, tenA, e.user(t), roleA, "suspendido")
	suspendedUsr := one[pgtype.UUID](t, e, `SELECT user_id FROM iam.memberships WHERE id = $1`, suspended)
	invited := e.membership(t, tenA, e.user(t), roleA, "invitado")
	invitedUsr := one[pgtype.UUID](t, e, `SELECT user_id FROM iam.memberships WHERE id = $1`, invited)
	inB := e.membership(t, tenB, usr, roleB, "activo")
	missing := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}

	cases := []struct {
		name string
		s    auth.Session
	}{
		{"missing membership", session(tenA, usr, missing)},
		{"suspended", session(tenA, suspendedUsr, suspended)},
		{"invited", session(tenA, invitedUsr, invited)},
		{"membership of another user", session(tenA, other, active)},
		{"membership of another tenant (actor in A, membership in B)", session(tenA, usr, inB)},
		{"actor of another tenant (actor in B, membership in A)", session(tenB, usr, active)},
		{"empty membership id", session(tenA, usr, pgtype.UUID{})},
		{"empty user id", session(tenA, pgtype.UUID{}, active)},
		{"empty tenant id", session(pgtype.UUID{}, usr, active)},
		{"impersonated actor", auth.Session{MembershipID: active, Actor: db.Actor{TenantID: tenA, UserID: usr, Kind: db.KindImpersonated, ImpersonatorID: other}}},
	}
	r := NewPermissionResolver(e.pool)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			az, err := r.Resolve(context.Background(), c.s)
			if !errors.Is(err, auth.ErrMembershipLost) {
				t.Fatalf("err = %v, want auth.ErrMembershipLost", err)
			}
			if len(az.Permissions) != 0 {
				t.Fatalf("a lost membership came with permissions: %v", az.Permissions)
			}
		})
	}
	// El control positivo: la misma membresía activa sí resuelve.
	if _, err := r.Resolve(context.Background(), session(tenA, usr, active)); err != nil {
		t.Fatalf("control: %v", err)
	}
}

// Nunca devuelve permisos de un rol de otra empresa: dos empresas con un rol del mismo nombre y permisos distintos.
func TestResolveDoesNotMixTenants(t *testing.T) {
	e := newEnv(t)
	tenA, tenB := e.tenant(t), e.tenant(t)
	usr := e.user(t) // el mismo usuario en las dos empresas
	roleA := e.customRole(t, tenA, "Caja", false, "ventas.pos:read")
	roleB := e.customRole(t, tenB, "Caja", false, "inventario.stock:read", "inventario.stock:update")
	memA, memB := e.membership(t, tenA, usr, roleA, "activo"), e.membership(t, tenB, usr, roleB, "activo")
	r := NewPermissionResolver(e.pool)
	a, err := r.Resolve(context.Background(), session(tenA, usr, memA))
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Resolve(context.Background(), session(tenB, usr, memB))
	if err != nil {
		t.Fatal(err)
	}
	if got := codesOf(a); !slices.Equal(got, []string{"ventas.pos:read"}) {
		t.Errorf("tenant A permissions = %v", got)
	}
	if got := codesOf(b); !slices.Equal(got, []string{"inventario.stock:read", "inventario.stock:update"}) {
		t.Errorf("tenant B permissions = %v", got)
	}
}

// Un rol propio con permisos sembrados directamente no hereda los de una plantilla del mismo nombre.
func TestResolveCustomRoleNamedLikeATemplateKeepsItsOwnPermissions(t *testing.T) {
	e := newEnv(t)
	ten, usr := e.tenant(t), e.user(t)
	mem := e.membership(t, ten, usr, e.customRole(t, ten, "Dueño", false, "ventas.pos:read"), "activo")
	az, err := NewPermissionResolver(e.pool).Resolve(context.Background(), session(ten, usr, mem))
	if err != nil || !slices.Equal(codesOf(az), []string{"ventas.pos:read"}) {
		t.Fatalf("permissions = %v (%v), want only ventas.pos:read", codesOf(az), err)
	}
}

// Rol system cuya plantilla falta: niega todo y deja un log error con el código de plantilla. La FK impide
// este estado, así que se arma como superusuario sin los triggers de integridad.
func TestResolveSystemRoleWithoutTemplateDeniesAndLogs(t *testing.T) {
	e := newEnv(t)
	ten, usr := e.tenant(t), e.user(t)
	role := e.baseRole(t, ten, "almacenero")
	mem := e.membership(t, ten, usr, role, "activo")
	for _, tbl := range []string{"iam.roles", "iam.role_templates", "iam.role_template_permissions"} {
		e.exec(t, `ALTER TABLE `+tbl+` DISABLE TRIGGER ALL`)
	}
	// Solo la fila de la plantilla: sus permisos quedan huérfanos y el resolver no debe usarlos.
	e.exec(t, `DELETE FROM iam.role_templates WHERE code = 'almacenero'`)

	var buf syncBuffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	az, err := NewPermissionResolver(e.pool).Resolve(context.Background(), session(ten, usr, mem))
	if err != nil {
		t.Fatal(err)
	}
	if az.Permissions == nil || len(az.Permissions) != 0 {
		t.Errorf("permissions = %v, want an empty set", az.Permissions)
	}
	if log := buf.String(); !strings.Contains(log, "level=ERROR") || !strings.Contains(log, "template_code=almacenero") {
		t.Errorf("log = %q, want an error with template_code=almacenero", log)
	}
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuffer) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

// Base caída o consulta cancelada: error que no es membresía perdida ni resultado vacío, nunca un acceso.
func TestResolveReportsTheDatabaseFailure(t *testing.T) {
	e := newEnv(t)
	ten, usr := e.tenant(t), e.user(t)
	mem := e.membership(t, ten, usr, e.customRole(t, ten, "R", false, "ventas.pos:read"), "activo")
	r := NewPermissionResolver(e.pool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	az, err := r.Resolve(ctx, session(ten, usr, mem))
	if err == nil || errors.Is(err, auth.ErrMembershipLost) || len(az.Permissions) != 0 {
		t.Fatalf("cancelled: %v, %v; want a non-nil error and no permissions", az, err)
	}

	short, cancel2 := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel2()
	time.Sleep(time.Millisecond)
	if _, err := r.Resolve(short, session(ten, usr, mem)); err == nil || errors.Is(err, auth.ErrMembershipLost) {
		t.Fatalf("deadline: %v, want a non-nil error", err)
	}

	e.pool.Close() // base «caída» para el pool
	if _, err := r.Resolve(context.Background(), session(ten, usr, mem)); err == nil || errors.Is(err, auth.ErrMembershipLost) {
		t.Fatalf("closed pool: %v, want a non-nil error", err)
	}
}

// countingRunner envuelve el pool y cuenta las transacciones y las sentencias de la aplicación.
type countingRunner struct {
	pool       *db.Pool
	txs, stmts int
	lastActor  db.Actor
}

type countingTx struct {
	pgx.Tx
	n *int
}

func (c countingTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	*c.n++
	return c.Tx.Exec(ctx, sql, args...)
}
func (c countingTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	*c.n++
	return c.Tx.Query(ctx, sql, args...)
}
func (c countingTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	*c.n++
	return c.Tx.QueryRow(ctx, sql, args...)
}

func (r *countingRunner) InTenantReadTx(ctx context.Context, a db.Actor, fn func(pgx.Tx) error) error {
	r.txs++
	r.lastActor = a
	return r.pool.InTenantReadTx(ctx, a, func(tx pgx.Tx) error { return fn(countingTx{Tx: tx, n: &r.stmts}) })
}

// Criterio 1: una consulta, en una transacción de tenant de solo lectura, con el Actor de la sesión; los dos caminos
// (rol propio y rol base).
func TestResolveRunsOneQueryInOneReadOnlyTransactionWithTheSessionActor(t *testing.T) {
	e := newEnv(t)
	ten, usr := e.tenant(t), e.user(t)
	custom := e.membership(t, ten, usr, e.customRole(t, ten, "R", false, "ventas.pos:read"), "activo")
	usr2 := e.user(t)
	base := e.membership(t, ten, usr2, e.baseRole(t, ten, "cajero"), "activo")
	for name, s := range map[string]auth.Session{"custom": session(ten, usr, custom), "system": session(ten, usr2, base)} {
		t.Run(name, func(t *testing.T) {
			cr := &countingRunner{pool: e.pool}
			if _, err := (permissionResolver{tx: cr}).Resolve(context.Background(), s); err != nil {
				t.Fatal(err)
			}
			if cr.txs != 1 || cr.stmts != 1 {
				t.Errorf("transactions = %d, statements = %d; want 1 and 1", cr.txs, cr.stmts)
			}
			if cr.lastActor != s.Actor {
				t.Errorf("actor = %+v, want the session's %+v", cr.lastActor, s.Actor)
			}
		})
	}
	// Solo lectura: una escritura dentro de la misma transacción la rechaza la base (25006).
	err := e.pool.InTenantReadTx(context.Background(), db.Actor{TenantID: ten, Kind: db.KindSystem}, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE iam.memberships SET authz_version = 99`)
		return err
	})
	if err == nil {
		t.Fatal("InTenantReadTx accepted a write")
	}
}
