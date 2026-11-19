package entitlements_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
)

// env es una base migrada con sus roles reales. Las semillas se escriben como
// superusuario (no pasa por RLS); las lecturas, con el rol wanka_app.
type env struct {
	t     testing.TB
	d     *dbtest.DB
	admin *pgx.Conn
}

func newEnv(t testing.TB) *env {
	t.Helper()
	d := dbtest.New(t)
	d.Migrate(context.Background(), t)
	return &env{t: t, d: d, admin: d.Admin(context.Background(), t)}
}

// exec corre SQL como superusuario y falla el test si no sale bien.
func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.admin.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatalf("seeding %q: %v", sql, err)
	}
}

// uuid pide un UUID nuevo a la base.
func (e *env) uuid() pgtype.UUID {
	e.t.Helper()
	var u pgtype.UUID
	if err := e.admin.QueryRow(context.Background(), `SELECT gen_random_uuid()`).Scan(&u); err != nil {
		e.t.Fatal(err)
	}
	return u
}

// tenant crea una empresa con el estado dado.
func (e *env) tenant(status string) pgtype.UUID {
	e.t.Helper()
	id := e.uuid()
	e.exec(`INSERT INTO org.tenants (id, status, closed_at) VALUES ($1, $2::text, CASE WHEN $2::text = 'cerrada' THEN now() END)`, id, status)
	return id
}

// plan crea un plan con esos módulos; limits (JSON) se aplica sobre los tres límites por defecto, que el CHECK exige completos, y devuelve su id.
func (e *env) plan(code, limits string, modules ...string) pgtype.UUID {
	e.t.Helper()
	var id pgtype.UUID
	if err := e.admin.QueryRow(context.Background(),
		`INSERT INTO platform.plans (code, name, limits) VALUES ($1, $1, '{"users": 1000, "sites": 100, "cpe_per_month": 1000}'::jsonb || $2::jsonb) RETURNING id`, code, limits).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	for _, m := range modules {
		e.exec(`INSERT INTO platform.plan_modules (plan_id, module_code) VALUES ($1, $2)`, id, m)
	}
	return id
}

// subscribe suscribe a la empresa al plan.
func (e *env) subscribe(tenant, plan pgtype.UUID, status string) {
	e.t.Helper()
	e.exec(`INSERT INTO platform.subscriptions (id, tenant_id, plan_id, status) VALUES (gen_random_uuid(), $1, $2, $3)`, tenant, plan, status)
}

// pool abre un pool con el rol de la API; se cierra con el test.
func (e *env) pool(maxConns int32) *db.Pool {
	e.t.Helper()
	p, err := db.Open(context.Background(), e.d.URL(dbtest.App), db.Options{MaxConns: maxConns, AppName: "entitlements-test"})
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(p.Close)
	return p
}

// service arma un Service sobre un pool nuevo.
func (e *env) service(o entitlements.Options) *entitlements.Service {
	e.t.Helper()
	return entitlements.New(e.pool(4), o)
}

func ctx(t testing.TB) context.Context {
	t.Helper()
	c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return c
}

func mustFor(t testing.TB, s *entitlements.Service, tenant pgtype.UUID) entitlements.Entitlements {
	t.Helper()
	got, err := s.For(ctx(t), tenant)
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	return got
}

// countingDB envuelve un entitlements.DB y cuenta las lecturas.
type countingDB struct {
	entitlements.DB
	reads  atomic.Int64
	before func() // opcional: corre antes de cada lectura
}

func (c *countingDB) InTenantReadTx(ctx context.Context, a db.Actor, fn func(pgx.Tx) error) error {
	c.reads.Add(1)
	if c.before != nil {
		c.before()
	}
	return c.DB.InTenantReadTx(ctx, a, fn)
}

func newFake() *clock.Fake { return clock.NewFake(time.Date(2026, 11, 19, 15, 0, 0, 0, time.UTC)) }

// uuidOf convierte el texto de un UUID.
func (e *env) uuidOf(s string) pgtype.UUID {
	e.t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		e.t.Fatal(err)
	}
	return u
}
