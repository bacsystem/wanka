package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/audit"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

const (
	tenantA = "aaaaaaaa-0000-0000-0000-000000000001"
	tenantB = "bbbbbbbb-0000-0000-0000-000000000002"
	userA   = "eeeeeeee-0000-0000-0000-000000000001"
	userB   = "eeeeeeee-0000-0000-0000-000000000002"
	staff   = "cccccccc-0000-0000-0000-000000000009"
)

func uid(t testing.TB, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatalf("parsing uuid %q: %v", s, err)
	}
	return u
}

func testCtx(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// env es una base migrada con un pool por rol de aplicación.
type env struct {
	d      *dbtest.DB
	app    *db.Pool
	worker *db.Pool
	w      *audit.Writer
}

func newEnv(t testing.TB) *env {
	t.Helper()
	d := dbtest.New(t)
	d.Migrate(context.Background(), t)
	open := func(role string) *db.Pool {
		p, err := db.Open(context.Background(), d.URL(role), db.Options{MaxConns: 10, AppName: "audit-test-" + role})
		if err != nil {
			t.Fatalf("opening the %s pool: %v", role, err)
		}
		t.Cleanup(p.Close)
		return p
	}
	app := open(dbtest.App)
	return &env{d: d, app: app, worker: open(dbtest.Worker), w: audit.NewWriter(app)}
}

func userActor(t testing.TB, tenant, user string) db.Actor {
	return db.Actor{TenantID: uid(t, tenant), UserID: uid(t, user), Kind: db.KindUser}
}

func systemActor(t testing.TB, tenant string) db.Actor {
	return db.Actor{TenantID: uid(t, tenant), Kind: db.KindSystem}
}

// row es un evento leído como superusuario (sin RLS).
type row struct {
	TenantID, ActorType, Action, Result string
	ActorUserID, ImpersonatorID         *string
	EntityType, RequestID, IP           *string
	EntityID                            *string
	Diff                                []byte
}

func events(t testing.TB, ctx context.Context, e *env) []row {
	t.Helper()
	conn := e.d.Admin(ctx, t)
	rows, err := conn.Query(ctx, `SELECT tenant_id::text, actor_type, action, result, actor_user_id::text, impersonator_id::text,
		entity_type, request_id, host(ip), entity_id::text, diff::text FROM audit.events ORDER BY at, action`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []row
	for rows.Next() {
		var r row
		var diff *string
		if err := rows.Scan(&r.TenantID, &r.ActorType, &r.Action, &r.Result, &r.ActorUserID, &r.ImpersonatorID,
			&r.EntityType, &r.RequestID, &r.IP, &r.EntityID, &diff); err != nil {
			t.Fatal(err)
		}
		if diff != nil {
			r.Diff = []byte(*diff)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func count(t testing.TB, ctx context.Context, conn *pgx.Conn, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}
