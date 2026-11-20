package empresas_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/empresas"
	"github.com/bacsystem/wanka/apps/api/internal/platform/audit"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth/pgsession"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

type env struct {
	t     *testing.T
	d     *dbtest.DB
	pool  *db.Pool
	admin *pgx.Conn
	h     http.Handler
	seq   atomic.Int64
}

type fixedResolver struct {
	perms map[permission.Code]struct{}
}

func (f fixedResolver) Resolve(_ context.Context, s auth.Session) (auth.Authz, error) {
	return auth.Authz{
		Version:     s.CurrentAuthzVersion.Int64,
		Permissions: f.perms,
		AllSites:    true,
	}, nil
}

func newEnv(t *testing.T, allowedPerms ...string) *env {
	t.Helper()
	d := dbtest.New(t)
	ctx := context.Background()
	d.Migrate(ctx, t)
	pool, err := db.Open(ctx, d.URL(dbtest.App), db.Options{MaxConns: 35, AcquireTimeout: 30 * time.Second, AppName: "empresas-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	e := &env{
		t:     t,
		d:     d,
		pool:  pool,
		admin: d.Admin(ctx, t),
	}

	permMap := make(map[permission.Code]struct{}, len(allowedPerms))
	for _, p := range allowedPerms {
		permMap[permission.Code(p)] = struct{}{}
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	rt := httpx.NewRouter(httpx.RouterOptions{})
	routes, err := auth.NewRoutes(rt, pgsession.New(pool), entitlements.New(pool, entitlements.Options{Logger: log}),
		fixedResolver{perms: permMap}, audit.NewWriter(pool), auth.Options{TrustClientIP: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := empresas.Mount(routes, pool); err != nil {
		t.Fatal(err)
	}
	e.h = httpx.Chain(rt, log, metrics.Nop{})
	return e
}

func (e *env) subscribe(tenantID string) {
	e.t.Helper()
	_, err := e.admin.Exec(context.Background(), `
		INSERT INTO platform.subscriptions (id, tenant_id, plan_id, status)
		SELECT gen_random_uuid(), $1::uuid, id, 'activa'
		FROM platform.plans WHERE code = 'semilla'
	`, tenantID)
	if err != nil {
		e.t.Fatalf("subscribing tenant: %v", err)
	}
}

func (e *env) tenantActor(tenantID pgtype.UUID) db.Actor {
	var userID pgtype.UUID
	_ = userID.Scan("11111111-1111-1111-1111-111111111111")
	return db.Actor{TenantID: tenantID, UserID: userID, Kind: db.KindUser}
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.admin.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatalf("%s: %v", sql, err)
	}
}

func (e *env) seedTenant() pgtype.UUID {
	e.t.Helper()
	var id pgtype.UUID
	err := e.admin.QueryRow(context.Background(), `
		INSERT INTO org.tenants (id) VALUES (gen_random_uuid()) RETURNING id
	`).Scan(&id)
	if err != nil {
		e.t.Fatalf("seeding tenant: %v", err)
	}
	return id
}

func (e *env) seedSite(tenantID pgtype.UUID, code, name string) pgtype.UUID {
	e.t.Helper()
	var id pgtype.UUID
	err := e.admin.QueryRow(context.Background(), `
		INSERT INTO org.sites (id, tenant_id, code, name)
		VALUES (gen_random_uuid(), $1, $2, $3)
		RETURNING id
	`, tenantID, code, name).Scan(&id)
	if err != nil {
		e.t.Fatalf("seeding site: %v", err)
	}
	return id
}

func (e *env) seedCashRegister(tenantID, siteID pgtype.UUID, code string) pgtype.UUID {
	e.t.Helper()
	var id pgtype.UUID
	err := e.admin.QueryRow(context.Background(), `
		INSERT INTO org.cash_registers (id, tenant_id, site_id, code)
		VALUES (gen_random_uuid(), $1, $2, $3)
		RETURNING id
	`, tenantID, siteID, code).Scan(&id)
	if err != nil {
		e.t.Fatalf("seeding cash register: %v", err)
	}
	return id
}

func (e *env) seedSeries(tenantID, siteID, cashRegisterID pgtype.UUID, docType, series string, lastNum int64, active bool, synced, conflict bool) pgtype.UUID {
	e.t.Helper()
	var syncedAt, conflictAt *time.Time
	now := time.Now().UTC()
	if synced {
		syncedAt = &now
	}
	if conflict {
		conflictAt = &now
	}

	var cr any
	if cashRegisterID.Valid && cashRegisterID.Bytes != [16]byte{} {
		cr = cashRegisterID
	}

	var id pgtype.UUID
	err := e.admin.QueryRow(context.Background(), `
		INSERT INTO org.document_series (
			id, tenant_id, site_id, cash_register_id, doc_type, series, last_number, active, provider_synced_at, provider_conflict_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9
		) RETURNING id
	`, tenantID, siteID, cr, docType, series, lastNum, active, syncedAt, conflictAt).Scan(&id)
	if err != nil {
		e.t.Fatalf("seeding series %s %s: %v", docType, series, err)
	}
	return id
}
