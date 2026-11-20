package db_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// connectAs abre una conexión con el rol real (nunca superusuario) y la cierra con el test.
func connectAs(t *testing.T, d *dbtest.DB, role string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), d.URL(role))
	if err != nil {
		t.Fatalf("connecting as %s: %v", role, err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

func migrated(t *testing.T) *dbtest.DB {
	t.Helper()
	d := dbtest.New(t)
	d.Migrate(context.Background(), t)
	return d
}

func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// wantState exige que err sea un error de PostgreSQL con ese SQLSTATE.
func wantState(t *testing.T, err error, code, what string) {
	t.Helper()
	if got := sqlState(err); got != code {
		t.Errorf("%s: SQLSTATE = %q (err=%v), want %s", what, got, err, code)
	}
}

// lookupRow es lo que devuelve iam.session_lookup.
type lookupRow struct {
	SessionID                                            string
	UserID, TenantID, SiteID, CashRegisterID, Membership *string
	SessionVersion, CurrentVersion                       *int64
	MFASatisfied                                         bool
	MFAMethod                                            string
	Impersonator                                         *string
	MembershipStatus                                     *string
}

const lookupSQL = `SELECT session_id::text, user_id::text, tenant_id::text, site_id::text, cash_register_id::text,
	membership_id::text, session_authz_version, current_authz_version, mfa_satisfied, mfa_method,
	impersonator_id::text, membership_status FROM iam.session_lookup($1)`

func lookup(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, token string) ([]lookupRow, error) {
	hash := sha256.Sum256([]byte(token))
	rows, err := q.Query(ctx, lookupSQL, hash[:])
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []lookupRow
	for rows.Next() {
		var r lookupRow
		if err := rows.Scan(&r.SessionID, &r.UserID, &r.TenantID, &r.SiteID, &r.CashRegisterID, &r.Membership,
			&r.SessionVersion, &r.CurrentVersion, &r.MFASatisfied, &r.MFAMethod, &r.Impersonator, &r.MembershipStatus); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func currentTenantSetting(t *testing.T, tx pgx.Tx) string {
	t.Helper()
	var s string
	if err := tx.QueryRow(context.Background(), `SELECT coalesce(current_setting('app.tenant_id', true), '')`).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// A1: sesión ligada, versión 1 en la sesión y 2 en la membresía.
func TestSessionLookupBoundSessionReturnsBothVersionsAndRestoresTheTenant(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	s := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound, SessionVersion: 1, MembershipVersion: 2})
	app := connectAs(t, d, dbtest.App)

	tx, err := app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // la lectura no deja nada que deshacer
	rows, err := lookup(ctx, tx, s.Token)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.SessionID != s.SessionID || deref(r.UserID) != s.UserID || deref(r.TenantID) != s.TenantID || deref(r.Membership) != s.MembershipID {
		t.Errorf("ids = %+v, want %+v", r, s)
	}
	if deref(r.SiteID) != s.SiteID || deref(r.CashRegisterID) != s.CashRegisterID {
		t.Errorf("site/cash register = %v/%v, want %s/%s", deref(r.SiteID), deref(r.CashRegisterID), s.SiteID, s.CashRegisterID)
	}
	if r.SessionVersion == nil || *r.SessionVersion != 1 || r.CurrentVersion == nil || *r.CurrentVersion != 2 {
		t.Errorf("versions = %v/%v, want 1/2", r.SessionVersion, r.CurrentVersion)
	}
	if deref(r.MembershipStatus) != "activo" || !r.MFASatisfied {
		t.Errorf("status = %q mfa = %v", deref(r.MembershipStatus), r.MFASatisfied)
	}
	// Misma transacción: set_config(..., true) dura hasta su fin, así que solo aquí se ve si la función restauró.
	if got := currentTenantSetting(t, tx); got != "" {
		t.Errorf("app.tenant_id after the lookup = %q, want empty", got)
	}
}

// Invariante: el valor que tenía el llamador al entrar es el que queda al salir.
func TestSessionLookupKeepsTheCallersOwnTenant(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	s := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	const caller = "cccccccc-0000-0000-0000-00000000000c"
	app := connectAs(t, d, dbtest.App)
	tx, err := app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // la lectura no deja nada que deshacer
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, caller); err != nil {
		t.Fatal(err)
	}
	rows, err := lookup(ctx, tx, s.Token)
	// La versión actual solo se lee con el tenant de la sesión: si vuelve, la función lo fijó y lo restauró.
	if err != nil || len(rows) != 1 || rows[0].CurrentVersion == nil {
		t.Fatalf("lookup = %v, %v", rows, err)
	}
	if got := currentTenantSetting(t, tx); got != caller {
		t.Errorf("app.tenant_id after the lookup = %q, want the caller's %q", got, caller)
	}
}

func TestSessionLookupFindsNothingForEveryNonAuthenticatingSession(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	app := connectAs(t, d, dbtest.App)
	cases := []struct {
		name  string
		token func() string
	}{
		{"unknown token", func() string { return "wk_at_" + "x" }},
		{"expired", func() string { return d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionExpired}).Token }},
		{"revoked", func() string { return d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionRevoked}).Token }},
		{"half login (mfa not satisfied)", func() string { return d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionHalfLogin}).Token }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows, err := lookup(ctx, app, c.token())
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 0 {
				t.Errorf("rows = %d, want 0", len(rows))
			}
		})
	}
	// Control: la misma siembra sin el defecto sí autentica.
	if rows, err := lookup(ctx, app, d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound}).Token); err != nil || len(rows) != 1 {
		t.Fatalf("control lookup = %d rows, %v", len(rows), err)
	}
}

// A8 y la membresía suspendida.
func TestSessionLookupWithADeletedOrSuspendedMembership(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	app := connectAs(t, d, dbtest.App)

	t.Run("deleted membership returns the row without the current version", func(t *testing.T) {
		s := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound, SessionVersion: 1, MembershipVersion: 2})
		// Como superusuario: iam.guard_membership_deletes solo deja borrar a un rol de aplicación una membresía invitada.
		// No debe fallar ni cascadear sobre la sesión: iam.sessions no tiene FK a la membresía.
		if _, err := d.Admin(ctx, t).Exec(ctx, `DELETE FROM iam.memberships WHERE id = $1`, s.MembershipID); err != nil {
			t.Fatalf("deleting the membership: %v", err)
		}
		rows, err := lookup(ctx, app, s.Token)
		if err != nil || len(rows) != 1 {
			t.Fatalf("lookup = %d rows, %v", len(rows), err)
		}
		r := rows[0]
		if r.CurrentVersion != nil || r.MembershipStatus != nil || r.Membership == nil || r.SessionVersion == nil || *r.SessionVersion != 1 {
			t.Errorf("row = %+v, want session version 1 and no current version or status", r)
		}
	})

	t.Run("suspended membership", func(t *testing.T) {
		s := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound, MembershipStatus: "suspendido"})
		rows, err := lookup(ctx, app, s.Token)
		if err != nil || len(rows) != 1 {
			t.Fatalf("lookup = %d rows, %v", len(rows), err)
		}
		if got := deref(rows[0].MembershipStatus); got != "suspendido" {
			t.Errorf("membership_status = %q", got)
		}
	})

	t.Run("impersonated sessions revoked or expired are not found", func(t *testing.T) {
		admin := d.Admin(ctx, t)
		for name, set := range map[string]string{"revoked": `revoked_at = now()`, "expired": `access_expires_at = now() - interval '1 second'`} {
			s := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionImpersonated})
			if rows, err := lookup(ctx, app, s.Token); err != nil || len(rows) != 1 {
				t.Fatalf("%s control: %d rows, %v", name, len(rows), err)
			}
			if _, err := admin.Exec(ctx, `UPDATE iam.sessions SET `+set+` WHERE id = $1`, s.SessionID); err != nil {
				t.Fatal(err)
			}
			if rows, err := lookup(ctx, app, s.Token); err != nil || len(rows) != 0 {
				t.Errorf("%s impersonated session: %d rows, %v; want 0", name, len(rows), err)
			}
		}
	})

	t.Run("unbound and impersonated sessions have no membership data", func(t *testing.T) {
		for name, kind := range map[string]dbtest.SessionKind{"no tenant": dbtest.SessionNoTenant, "impersonated": dbtest.SessionImpersonated} {
			s := d.SeedSession(t, dbtest.SessionSpec{Kind: kind})
			rows, err := lookup(ctx, app, s.Token)
			if err != nil || len(rows) != 1 {
				t.Fatalf("%s: lookup = %d rows, %v", name, len(rows), err)
			}
			r := rows[0]
			if r.CurrentVersion != nil || r.MembershipStatus != nil || r.Membership != nil || r.SessionVersion != nil {
				t.Errorf("%s: row = %+v", name, r)
			}
			if kind == dbtest.SessionImpersonated && (r.MFAMethod != "platform" || r.Impersonator == nil || r.TenantID == nil) {
				t.Errorf("impersonated row = %+v", r)
			}
			if kind == dbtest.SessionNoTenant && r.TenantID != nil {
				t.Errorf("no-tenant session has tenant %v", *r.TenantID)
			}
		}
	})
}

// A2: sin tenant fijado, las tablas de tenant no se ven.
func TestTenantTablesAreInvisibleToTheAppWithoutATenant(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	admin := d.Admin(ctx, t)
	app := connectAs(t, d, dbtest.App)
	for _, table := range []string{"iam.memberships", "iam.roles", "org.tenants"} {
		if n := queryOne[int](t, admin, `SELECT count(*) FROM `+table); n == 0 {
			t.Fatalf("%s is empty: the seed did not insert", table)
		}
		if n := queryOne[int](t, app, `SELECT count(*) FROM `+table); n != 0 {
			t.Errorf("wanka_app sees %d rows of %s without a tenant, want 0", n, table)
		}
	}
}

// Con el tenant fijado ve solo el suyo (la política no es un "todo cerrado").
func TestTenantTablesShowOnlyTheCurrentTenant(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	a := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	app := connectAs(t, d, dbtest.App)
	tx, err := app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // solo lectura
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, a.TenantID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"iam.memberships", "iam.roles", "org.tenants"} {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil || n != 1 {
			t.Errorf("%s with tenant A = %d, %v; want 1", table, n, err)
		}
	}
}

// A9: el UPDATE de desligar del ADR 0011.
func TestUnbindingASessionKeepsItAliveAndPartialUnbindsFail(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	app := connectAs(t, d, dbtest.App)
	const unbind = `UPDATE iam.sessions SET tenant_id = NULL, membership_id = NULL, site_id = NULL, cash_register_id = NULL, authz_version = NULL
		WHERE id = $1 AND membership_id = $2 AND impersonator_id IS NULL AND revoked_at IS NULL`

	t.Run("full unbind", func(t *testing.T) {
		s := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
		tag, err := app.Exec(ctx, unbind, s.SessionID, s.MembershipID)
		if err != nil || tag.RowsAffected() != 1 {
			t.Fatalf("unbind = %v, %v", tag, err)
		}
		if tag, err := app.Exec(ctx, unbind, s.SessionID, s.MembershipID); err != nil || tag.RowsAffected() != 0 {
			t.Errorf("second unbind = %v, %v; want 0 rows", tag, err)
		}
		var nulls, revoked bool
		err = app.QueryRow(ctx, `SELECT tenant_id IS NULL AND membership_id IS NULL AND site_id IS NULL AND cash_register_id IS NULL AND authz_version IS NULL,
			revoked_at IS NOT NULL FROM iam.sessions WHERE id = $1`, s.SessionID).Scan(&nulls, &revoked)
		if err != nil || !nulls || revoked {
			t.Errorf("after unbind: nulls=%v revoked=%v err=%v", nulls, revoked, err)
		}
		rows, err := lookup(ctx, app, s.Token)
		if err != nil || len(rows) != 1 || rows[0].TenantID != nil || rows[0].CurrentVersion != nil || !rows[0].MFASatisfied {
			t.Errorf("lookup after unbind = %+v, %v; want a live session without tenant", rows, err)
		}
	})

	partial := []struct{ name, set string }{
		{"only membership_id", `membership_id = NULL`},
		{"only tenant_id", `tenant_id = NULL`},
		{"authz_version kept with no membership", `tenant_id = NULL, membership_id = NULL, site_id = NULL, cash_register_id = NULL`},
	}
	for _, c := range partial {
		t.Run(c.name, func(t *testing.T) {
			s := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
			_, err := app.Exec(ctx, `UPDATE iam.sessions SET `+c.set+` WHERE id = $1`, s.SessionID)
			wantState(t, err, "23514", c.name)
		})
	}
}

// A7 y los demás CHECK de iam.sessions. Las filas impersonadas se insertan como superusuario: wanka_app no puede
// (el trigger iam.guard_session_writes lo para antes de los CHECK), y los CHECK valen igual para quien sí.
func TestSessionChecks(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	b := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	app := connectAs(t, d, dbtest.App)
	admin := d.Admin(ctx, t)
	const ins = `INSERT INTO iam.sessions (id, user_id, tenant_id, membership_id, access_hash, access_expires_at, authz_version, mfa_satisfied, mfa_method, impersonator_id)
		VALUES (gen_random_uuid(), $1, $2, $3, sha256(gen_random_uuid()::text::bytea), now() + interval '1 hour', $4, true, $5, $6)`
	u, tn, m := b.UserID, b.TenantID, b.MembershipID
	staff := b.RoleID // cualquier uuid: el staff no tiene FK hasta la ola 6
	cases := []struct {
		name         string
		user         *string
		tenant, memb *string
		version      *int64
		method       string
		imp          *string
		want         string
	}{
		{"platform without impersonator (A7)", &u, nil, nil, nil, "platform", nil, "23514"},
		{"impersonator with another method (A7)", nil, nil, nil, nil, "totp", &staff, "23514"},
		{"impersonated session with a user_id", &u, &tn, nil, nil, "platform", &staff, "23514"},
		{"normal session without user_id", nil, nil, nil, nil, "none", nil, "23514"},
		{"membership without tenant", &u, nil, &m, nil, "none", nil, "23514"},
		{"tenant without membership or impersonator", &u, &tn, nil, nil, "none", nil, "23514"},
		{"version without membership", &u, nil, nil, i64(1), "none", nil, "23514"},
		{"version on an impersonated session", nil, &tn, nil, i64(1), "platform", &staff, "23514"},
		{"unknown mfa method", &u, nil, nil, nil, "sms", nil, "23514"},
		{"bound session", &u, &tn, &m, i64(1), "none", nil, ""},
		{"impersonated session", nil, &tn, nil, nil, "platform", &staff, ""},
		{"unbound session", &u, nil, nil, nil, "totp", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conn := app
			if c.imp != nil {
				conn = admin
			}
			_, err := conn.Exec(ctx, ins, c.user, c.tenant, c.memb, c.version, c.method, c.imp)
			if c.want == "" {
				if err != nil {
					t.Errorf("valid row rejected: %v", err)
				}
				return
			}
			wantState(t, err, c.want, c.name)
		})
	}
	t.Run("impersonated sessions have no refresh and live at most 60 minutes", func(t *testing.T) {
		const impIns = `INSERT INTO iam.sessions (id, tenant_id, access_hash, access_expires_at, refresh_hash, mfa_satisfied, mfa_method, impersonator_id, created_at)
			VALUES (gen_random_uuid(), $1, sha256(gen_random_uuid()::text::bytea), '2026-01-01 01:00:00+00'::timestamptz + $2::interval,
			        $3, true, 'platform', $4, '2026-01-01 01:00:00+00')`
		_, err := admin.Exec(ctx, impIns, tn, "60 minutes", []byte("0123456789abcdef0123456789abcdef"), staff)
		wantState(t, err, "23514", "impersonated with refresh_hash")
		_, err = admin.Exec(ctx, impIns, tn, "61 minutes", nil, staff)
		wantState(t, err, "23514", "impersonated living 61 minutes")
		if _, err = admin.Exec(ctx, impIns, tn, "60 minutes", nil, staff); err != nil {
			t.Errorf("impersonated session living exactly 60 minutes rejected: %v", err)
		}
		// Una sesión normal sí lleva refresh y vive más.
		_, err = app.Exec(ctx, `INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, refresh_hash, mfa_satisfied, mfa_method)
			VALUES (gen_random_uuid(), $1, sha256('n1'), now() + interval '2 hours', sha256('n2'), true, 'none')`, u)
		if err != nil {
			t.Errorf("normal session with refresh rejected: %v", err)
		}
	})
	t.Run("hash length", func(t *testing.T) {
		_, err := app.Exec(ctx, `INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, mfa_satisfied, mfa_method)
			VALUES (gen_random_uuid(), $1, '\x01', now(), true, 'none')`, u)
		wantState(t, err, "23514", "access_hash of 1 byte")
		_, err = app.Exec(ctx, `INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, refresh_hash, mfa_satisfied, mfa_method)
			VALUES (gen_random_uuid(), $1, sha256('r'), now(), '\x01', true, 'none')`, u)
		wantState(t, err, "23514", "refresh_hash of 1 byte")
	})
	t.Run("user must exist", func(t *testing.T) {
		_, err := app.Exec(ctx, `INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, mfa_satisfied, mfa_method)
			VALUES (gen_random_uuid(), gen_random_uuid(), sha256('u'), now(), true, 'none')`)
		wantState(t, err, "23503", "unknown user")
	})
	t.Run("access_hash is unique", func(t *testing.T) {
		_, err := app.Exec(ctx, `INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, mfa_satisfied, mfa_method)
			VALUES (gen_random_uuid(), $1, sha256($2::bytea), now(), true, 'none')`, u, []byte(b.Token))
		wantState(t, err, "23505", "duplicate access_hash")
	})
}

func TestCoreTablesConstraints(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	admin := d.Admin(ctx, t)
	exec := func(sql string, args ...any) error { _, err := admin.Exec(ctx, sql, args...); return err }

	if err := exec(`INSERT INTO org.tenants (id) VALUES ('a0000000-0000-0000-0000-000000000001')`); err != nil {
		t.Fatal(err)
	}
	if got := queryOne[string](t, admin, `SELECT status FROM org.tenants`); got != "trial" {
		t.Errorf("default tenant status = %q, want trial", got)
	}
	wantState(t, exec(`INSERT INTO org.tenants (id, status) VALUES (gen_random_uuid(), 'cerrada')`), "23514", "cerrada without closed_at")
	wantState(t, exec(`INSERT INTO org.tenants (id, status) VALUES (gen_random_uuid(), 'x')`), "23514", "tenant status")
	if err := exec(`INSERT INTO org.tenants (id, status, closed_at) VALUES (gen_random_uuid(), 'cerrada', now())`); err != nil {
		t.Errorf("closed tenant with closed_at rejected: %v", err)
	}
	wantState(t, exec(`INSERT INTO iam.users (id, email, status) VALUES (gen_random_uuid(), 'x@test.pe', 'x')`), "23514", "user status")
	if err := exec(`INSERT INTO iam.users (id, email) VALUES ('b0000000-0000-0000-0000-000000000001', 'b1@test.pe')`); err != nil {
		t.Fatal(err)
	}
	if got := queryOne[string](t, admin, `SELECT status FROM iam.users`); got != "activo" {
		t.Errorf("default user status = %q, want activo", got)
	}

	const t1, t2, usr = "a0000000-0000-0000-0000-000000000001", "a0000000-0000-0000-0000-000000000002", "b0000000-0000-0000-0000-000000000001"
	if err := exec(`INSERT INTO org.tenants (id) VALUES ($1)`, t2); err != nil {
		t.Fatal(err)
	}
	if err := exec(`INSERT INTO iam.roles (id, tenant_id, name) VALUES ('c0000000-0000-0000-0000-000000000001', $1, 'Cajero')`, t1); err != nil {
		t.Fatal(err)
	}
	wantState(t, exec(`INSERT INTO iam.roles (id, tenant_id, name) VALUES (gen_random_uuid(), $1, 'Cajero')`, t1), "23505", "role name unique per tenant")
	wantState(t, exec(`INSERT INTO iam.roles (id, tenant_id, name) VALUES (gen_random_uuid(), gen_random_uuid(), 'X')`), "23503", "role of an unknown tenant")
	const mem = `INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status, authz_version) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)`
	const role = "c0000000-0000-0000-0000-000000000001"
	wantState(t, exec(mem, t2, usr, role, "activo", 1), "23503", "role of another tenant (composite FK)")
	wantState(t, exec(mem, t1, usr, role, "otro", 1), "23514", "membership status")
	wantState(t, exec(mem, t1, usr, role, "activo", 0), "23514", "authz_version > 0")
	if err := exec(mem, t1, usr, role, "invitado", 1); err != nil {
		t.Fatalf("valid membership rejected: %v", err)
	}
	wantState(t, exec(mem, t1, usr, role, "activo", 1), "23505", "one membership per user and tenant")
	if got := queryOne[int](t, admin, `SELECT authz_version FROM iam.memberships`); got != 1 {
		t.Errorf("authz_version = %d", got)
	}

	// created_at / updated_at en toda tabla mutable (data-model §1.6); las sesiones solo llevan created_at.
	for _, table := range []string{"org.tenants", "iam.users", "iam.roles", "iam.memberships"} {
		for _, col := range []string{"created_at", "updated_at"} {
			ok := queryOne[bool](t, admin, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = split_part($1,'.',1)
				AND table_name = split_part($1,'.',2) AND column_name = $2 AND data_type = 'timestamp with time zone' AND is_nullable = 'NO' AND column_default LIKE 'now()%')`, table, col)
			if !ok {
				t.Errorf("%s.%s must be timestamptz NOT NULL DEFAULT now()", table, col)
			}
		}
	}
}

// Lo propio de iam.sessions y de session_lookup. Las reglas generales de catálogo de ADR 0001 (RLS, políticas,
// únicos, FK, funciones definer, roles) viven en catalog_test.go y corren sobre todas las tablas.
func TestIAMSessionCatalog(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	admin := d.Admin(ctx, t)

	t.Run("iam.sessions and iam.users have no RLS", func(t *testing.T) {
		for _, tbl := range []string{"iam.sessions", "iam.users"} {
			if queryOne[bool](t, admin, `SELECT relrowsecurity OR relforcerowsecurity FROM pg_class WHERE oid = $1::regclass`, tbl) {
				t.Errorf("%s must not have RLS", tbl)
			}
		}
	})

	t.Run("iam.sessions has no foreign key to membership, tenant, site or cash register", func(t *testing.T) {
		rows, err := admin.Query(ctx, `SELECT confrelid::regclass::text FROM pg_constraint WHERE conrelid = 'iam.sessions'::regclass AND contype = 'f'`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var refs []string
		for rows.Next() {
			var r string
			if err := rows.Scan(&r); err != nil {
				t.Fatal(err)
			}
			refs = append(refs, r)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if len(refs) != 1 || refs[0] != "iam.users" {
			t.Errorf("iam.sessions references %v, want only iam.users", refs)
		}
	})

	t.Run("sessions are indexed by user_id and created_at", func(t *testing.T) {
		for _, col := range []string{"user_id", "created_at"} {
			ok := queryOne[bool](t, admin, `SELECT EXISTS (SELECT 1 FROM pg_index i WHERE i.indrelid = 'iam.sessions'::regclass
				AND i.indkey[0] = (SELECT attnum FROM pg_attribute WHERE attrelid = 'iam.sessions'::regclass AND attname = $1))`, col)
			if !ok {
				t.Errorf("iam.sessions has no index on %s", col)
			}
		}
	})

	t.Run("session_lookup: plpgsql STABLE SECURITY DEFINER, EXECUTE only for wanka_app", func(t *testing.T) {
		const fn = `'iam.session_lookup(bytea)'::regprocedure`
		// Dueño, search_path y EXECUTE de PUBLIC los revisa la regla general de funciones definer (catalog_test.go).
		if !queryOne[bool](t, admin, `SELECT prosecdef AND prolang = (SELECT oid FROM pg_language WHERE lanname = 'plpgsql') AND provolatile = 's' FROM pg_proc WHERE oid = `+fn) {
			t.Error("session_lookup must be plpgsql STABLE SECURITY DEFINER")
		}
		// El dueño siempre ejecuta su función; del resto, solo wanka_app.
		for role, want := range map[string]bool{dbtest.App: true, dbtest.Worker: false, dbtest.Webhooks: false, dbtest.Platform: false} {
			if got := queryOne[bool](t, admin, `SELECT has_function_privilege($1, `+fn+`, 'EXECUTE')`, role); got != want {
				t.Errorf("EXECUTE for %s = %v, want %v", role, got, want)
			}
		}
	})

	t.Run("guard trigger: invoker's rights, pinned search_path, qualified names", func(t *testing.T) {
		const fn = `'iam.guard_session_writes()'::regprocedure`
		if queryOne[bool](t, admin, `SELECT prosecdef FROM pg_proc WHERE oid = `+fn) {
			t.Error("the guard trigger must not be SECURITY DEFINER")
		}
		if got := queryOne[string](t, admin, `SELECT proconfig::text FROM pg_proc WHERE oid = `+fn); got != `{"search_path=pg_catalog, pg_temp"}` {
			t.Errorf("guard proconfig = %s", got)
		}
		// Con el search_path fijo, todo objeto va cualificado: el cuerpo no usa to_jsonb, pg_roles ni now() sueltos.
		src := queryOne[string](t, admin, `SELECT prosrc FROM pg_proc WHERE oid = `+fn)
		for _, bare := range []string{" to_jsonb(", "(to_jsonb(", " pg_roles ", " now()"} {
			if strings.Contains(src, bare) {
				t.Errorf("guard body has the unqualified %q", bare)
			}
		}
		if n := queryOne[int](t, admin, `SELECT count(*) FROM pg_trigger WHERE tgrelid = 'iam.sessions'::regclass AND tgname = 'sessions_guard_writes' AND tgenabled = 'O'`); n != 1 {
			t.Error("trigger sessions_guard_writes is missing or disabled")
		}
	})

	t.Run("the creation grant on the schema is revoked again", func(t *testing.T) {
		if queryOne[bool](t, admin, `SELECT has_schema_privilege('wanka_definer', 'iam', 'CREATE')`) {
			t.Error("wanka_definer still has CREATE on schema iam")
		}
		if got := queryOne[string](t, admin, `SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname = 'iam'`); got != "wanka_migrator" {
			t.Errorf("schema iam owner = %s", got)
		}
	})
}

// Matriz de GRANT del ADR 0001 para estas cinco tablas: exacta, sin privilegios de más.
func TestTableGrantsMatchTheADR0001Matrix(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	admin := d.Admin(ctx, t)
	want := map[string]string{
		"wanka_app|iam.users":           "INSERT,SELECT,UPDATE",
		"wanka_app|iam.sessions":        "INSERT,SELECT",
		"wanka_app|iam.roles":           "DELETE,INSERT,SELECT,UPDATE",
		"wanka_app|iam.memberships":     "DELETE,INSERT,SELECT,UPDATE",
		"wanka_app|org.tenants":         "SELECT",
		"wanka_worker|iam.sessions":     "DELETE",
		"wanka_worker|iam.memberships":  "SELECT",
		"wanka_worker|iam.roles":        "SELECT",
		"wanka_worker|org.tenants":      "SELECT",
		"wanka_definer|iam.sessions":    "SELECT",
		"wanka_definer|iam.memberships": "SELECT",
		// Catálogo de permisos (#172): solo lectura para api, worker y plataforma.
		"wanka_app|iam.permissions":      "SELECT",
		"wanka_worker|iam.permissions":   "SELECT",
		"wanka_platform|iam.permissions": "SELECT",
		// Identidad de la ola 1, F2 (#232, ADR 0001 nota del 2026-11-19). Las plantillas son un catálogo global: SELECT para
		// wanka_app y wanka_definer, y nada para worker, webhooks ni plataforma. Sedes y cajas, solo SELECT de tabla (el
		// INSERT y el UPDATE son por columna).
		"wanka_app|iam.role_permissions":              "DELETE,INSERT,SELECT",
		"wanka_app|iam.membership_sites":              "DELETE,INSERT,SELECT",
		"wanka_app|iam.role_templates":                "SELECT",
		"wanka_app|iam.role_template_permissions":     "SELECT",
		"wanka_definer|iam.role_templates":            "SELECT",
		"wanka_definer|iam.role_template_permissions": "SELECT",
		"wanka_app|org.sites":                         "SELECT",
		"wanka_app|org.cash_registers":                "SELECT",
		"wanka_worker|org.sites":                      "SELECT",
		"wanka_worker|org.cash_registers":             "SELECT",
		"wanka_app|org.tenant_settings":               "SELECT",
		"wanka_worker|org.tenant_settings":            "SELECT",
		"wanka_app|org.document_series":               "SELECT",
		"wanka_worker|org.document_series":            "SELECT",
		"wanka_app|iam.list_my_memberships":           "SELECT",
		// Identidad global de la ola 1 (#231): el UPDATE y el SELECT por columna se prueban en iam_identity_test.go.
		"wanka_app|iam.user_totp":          "INSERT,SELECT,UPDATE",
		"wanka_app|iam.mfa_enrollments":    "INSERT,SELECT",
		"wanka_app|iam.user_tokens":        "INSERT,SELECT",
		"wanka_worker|iam.mfa_enrollments": "DELETE",
		"wanka_worker|iam.user_tokens":     "DELETE",
	}
	rows, err := admin.Query(ctx, `
		SELECT grantee, table_schema || '.' || table_name, string_agg(privilege_type, ',' ORDER BY privilege_type)
		  FROM information_schema.role_table_grants
		 WHERE table_schema IN ('iam', 'org') AND grantee <> 'wanka_migrator'
		 GROUP BY 1, 2`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var grantee, table, privs string
		if err := rows.Scan(&grantee, &table, &privs); err != nil {
			t.Fatal(err)
		}
		got[grantee+"|"+table] = privs
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("grants %s = %q, want %q", k, got[k], w)
		}
	}
	for k, g := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected grant %s = %s (webhooks and platform get nothing here)", k, g)
		}
	}
	// wanka_app: UPDATE de iam.sessions solo por columna, sin id, user_id, impersonator_id, refresh_family ni created_at.
	appUpd := queryOne[string](t, admin, `SELECT string_agg(column_name, ',' ORDER BY column_name) FROM information_schema.column_privileges
		WHERE grantee = 'wanka_app' AND table_schema = 'iam' AND table_name = 'sessions' AND privilege_type = 'UPDATE'`)
	for _, col := range []string{"created_at", "id", "user_id", "impersonator_id", "refresh_family"} {
		if queryOne[bool](t, admin, `SELECT has_column_privilege('wanka_app', 'iam.sessions', $1, 'UPDATE')`, col) {
			t.Errorf("wanka_app can UPDATE iam.sessions.%s", col)
		}
	}
	wantUpd := "access_expires_at,access_hash,authz_version,cash_register_id,ip,membership_id,mfa_method,mfa_satisfied,prev_refresh_hash,refresh_expires_at,refresh_hash,revoked_at,rotated_at,site_id,tenant_id,user_agent"
	if appUpd != wantUpd {
		t.Errorf("wanka_app column UPDATE on iam.sessions = %q, want %q", appUpd, wantUpd)
	}
	// wanka_worker: SELECT por columna sobre iam.sessions, sin los hashes (el WHERE del DELETE lo exige).
	cols := queryOne[string](t, admin, `SELECT string_agg(column_name, ',' ORDER BY column_name) FROM information_schema.column_privileges
		WHERE grantee = 'wanka_worker' AND table_schema = 'iam' AND table_name = 'sessions' AND privilege_type = 'SELECT'`)
	if want := "access_expires_at,created_at,id,refresh_expires_at,revoked_at"; cols != want {
		t.Errorf("wanka_worker column SELECT on iam.sessions = %q, want %q", cols, want)
	}
	for _, role := range []string{"wanka_app", "wanka_worker", "wanka_definer"} {
		for _, schema := range []string{"iam", "org"} {
			if !queryOne[bool](t, admin, `SELECT has_schema_privilege($1, $2, 'USAGE')`, role, schema) {
				t.Errorf("%s lacks USAGE on %s", role, schema)
			}
		}
	}
	// wanka_platform lee iam.permissions (#172): USAGE en iam, y nada en org. wanka_webhooks no tiene ninguno.
	if !queryOne[bool](t, admin, `SELECT has_schema_privilege('wanka_platform', 'iam', 'USAGE')`) {
		t.Error("wanka_platform lacks USAGE on iam (needed to read iam.permissions)")
	}
	for _, role := range []string{"wanka_webhooks", "wanka_platform"} {
		for _, schema := range []string{"iam", "org"} {
			if role == "wanka_platform" && schema == "iam" {
				continue
			}
			if queryOne[bool](t, admin, `SELECT has_schema_privilege($1, $2, 'USAGE')`, role, schema) {
				t.Errorf("%s must not have USAGE on %s", role, schema)
			}
		}
	}
}

// Criterio no funcional: la búsqueda lee por índice único dos veces, nunca recorre la tabla. Se mide la función misma,
// llamada como wanka_app: auto_explain (cargado por el superusuario de la sesión) manda al cliente el plan de cada sentencia
// anidada, con RLS activa y con el rol que de verdad las ejecuta (wanka_definer).
func TestSessionLookupUsesUniqueIndexesWith100kSessions(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	s := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	admin := d.Admin(ctx, t)
	// Membresías de sobra para que el planificador no prefiera recorrer una tabla de una fila.
	if _, err := admin.Exec(ctx, `
		WITH u AS (INSERT INTO iam.users (id, email) SELECT i, i::text || '@seed.test' FROM (SELECT gen_random_uuid() AS i FROM generate_series(1, 20000)) g RETURNING id)
		INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status) SELECT gen_random_uuid(), $1, u.id, $2, 'activo' FROM u`,
		s.TenantID, s.RoleID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, mfa_satisfied, mfa_method)
		SELECT gen_random_uuid(), $1, sha256(g::text::bytea), now() + interval '1 hour', true, 'none' FROM generate_series(1, 100000) g`, s.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `ANALYZE iam.sessions; ANALYZE iam.memberships`); err != nil {
		t.Fatal(err)
	}

	cfg, err := pgx.ParseConfig(d.SuperuserURL())
	if err != nil {
		t.Fatal(err)
	}
	var plans []string
	cfg.OnNotice = func(_ *pgconn.PgConn, n *pgconn.Notice) { plans = append(plans, n.Message) }
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	for _, stmt := range []string{`LOAD 'auto_explain'`, `SET auto_explain.log_min_duration = 0`, `SET auto_explain.log_nested_statements = on`,
		`SET client_min_messages = log`, `SET ROLE wanka_app`} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	rows, err := lookup(ctx, conn, s.Token)
	if err != nil || len(rows) != 1 || rows[0].CurrentVersion == nil {
		t.Fatalf("lookup as wanka_app = %v, %v", rows, err)
	}
	all := strings.Join(plans, "\n")
	t.Logf("auto_explain of iam.session_lookup as wanka_app with 100 000 sessions:\n%s", all)
	if strings.Contains(all, "Seq Scan") {
		t.Errorf("session_lookup does a Seq Scan:\n%s", all)
	}
	// Las dos lecturas son por índice único: access_hash y la PK de memberships (user_id se compara en plpgsql).
	for _, idx := range []string{`Index Scan using sessions_access_hash_key on sessions`, `Index Scan using memberships_pkey on memberships`} {
		if !regexp.MustCompile(idx).MatchString(all) {
			t.Errorf("plan lacks %q:\n%s", idx, all)
		}
	}
}

func TestSeedSessionStoresOnlyTheHashOfAWellFormedToken(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	s := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	if !regexp.MustCompile(`^wk_at_[A-Za-z0-9_-]{43}$`).MatchString(s.Token) {
		t.Fatalf("token = %q, want wk_at_ + 43 base64url characters", s.Token)
	}
	admin := d.Admin(ctx, t)
	want := sha256.Sum256([]byte(s.Token))
	var got []byte
	if err := admin.QueryRow(ctx, `SELECT access_hash FROM iam.sessions WHERE id = $1`, s.SessionID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want[:]) {
		t.Error("access_hash is not sha256 of the full token")
	}
	other := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	if other.Token == s.Token || other.TenantID == s.TenantID {
		t.Error("two seeds must not share token or tenant")
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func i64(v int64) *int64 { return &v }

func TestWorkerCanPurgeSessionsByAgeButNeverReadTheHashes(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	old := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	fresh := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	admin := d.Admin(ctx, t)
	if _, err := admin.Exec(ctx, `UPDATE iam.sessions SET created_at = now() - interval '200 days' WHERE id = $1`, old.SessionID); err != nil {
		t.Fatal(err)
	}
	w := connectAs(t, d, dbtest.Worker)
	for _, col := range []string{"access_hash", "refresh_hash", "prev_refresh_hash", "user_id", "tenant_id"} {
		_, err := w.Exec(ctx, `SELECT `+col+` FROM iam.sessions`)
		wantState(t, err, "42501", "SELECT "+col)
	}
	tag, err := w.Exec(ctx, `DELETE FROM iam.sessions WHERE created_at < now() - interval '120 days'`)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("purge = %v, %v; want 1 row", tag, err)
	}
	if n := queryOne[int](t, admin, `SELECT count(*) FROM iam.sessions WHERE id IN ($1, $2)`, old.SessionID, fresh.SessionID); n != 1 {
		t.Errorf("sessions left = %d, want only the fresh one", n)
	}
	_, err = w.Exec(ctx, `UPDATE iam.sessions SET revoked_at = now()`)
	wantState(t, err, "42501", "worker UPDATE")
}

// H13: sondeo de la escritura de wanka_app sobre iam.sessions (GRANT por columna y trigger de la impersonada).
func TestAppCannotStretchOrRewriteImpersonatedSessions(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	imp := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionImpersonated})
	b := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	app := connectAs(t, d, dbtest.App)
	upd := func(id, set string) error {
		_, err := app.Exec(ctx, `UPDATE iam.sessions SET `+set+` WHERE id = $1`, id)
		return err
	}

	// (a) columnas fuera del GRANT, en una sesión normal y en la impersonada.
	for _, id := range []string{b.SessionID, imp.SessionID} {
		for _, set := range []string{`created_at = created_at + interval '30 days'`, `impersonator_id = NULL`, `user_id = NULL`,
			`refresh_family = gen_random_uuid()`, `id = gen_random_uuid()`} {
			wantState(t, upd(id, set), "42501", "(a) UPDATE "+set)
		}
	}
	// (b) sobre una impersonada, columnas del GRANT, pero el trigger las para.
	wantState(t, upd(imp.SessionID, `access_expires_at = access_expires_at + interval '30 days'`), "42501", "(b) stretch")
	wantState(t, upd(imp.SessionID, `tenant_id = '`+b.TenantID+`'`), "42501", "(b) change tenant")
	wantState(t, upd(imp.SessionID, `revoked_at = now(), access_expires_at = access_expires_at + interval '30 days'`), "42501", "(b) revoke and stretch")
	// (c) INSERT de una impersonada.
	_, err := app.Exec(ctx, `INSERT INTO iam.sessions (id, tenant_id, access_hash, access_expires_at, mfa_satisfied, mfa_method, impersonator_id)
		VALUES (gen_random_uuid(), $1, sha256('imp-insert'), now() + interval '30 days', true, 'platform', gen_random_uuid())`, b.TenantID)
	wantState(t, err, "42501", "(c) INSERT of an impersonated session")
	// (d) revocar sí.
	if err := upd(imp.SessionID, `revoked_at = now()`); err != nil {
		t.Errorf("(d) revoking an impersonated session: %v", err)
	}
	wantState(t, upd(imp.SessionID, `revoked_at = NULL`), "42501", "un-revoking")

	// La rotación del refresh de una sesión normal funciona.
	err = upd(b.SessionID, `access_hash = sha256('new-a'), refresh_hash = sha256('new-r'), prev_refresh_hash = sha256('old-r'), rotated_at = now(),
		access_expires_at = now() + interval '15 minutes', refresh_expires_at = now() + interval '30 days'`)
	if err != nil {
		t.Errorf("refresh rotation: %v", err)
	}
}

// H19: refresh_family se fija en el INSERT y la app no la cambia; el flujo de login (insert a medio login, verify) funciona.
func TestRefreshFamilyIsSetOnInsertAndNeverChanged(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	b := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	app := connectAs(t, d, dbtest.App)
	if fam := queryOne[*string](t, d.Admin(ctx, t), `SELECT refresh_family::text FROM iam.sessions WHERE id = $1`, b.SessionID); fam == nil {
		t.Error("SeedSession did not set refresh_family")
	}
	// Login a medio camino: con refresh_family y sin refresh_hash todavía.
	var id string
	err := app.QueryRow(ctx, `INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, refresh_family, mfa_satisfied, mfa_method)
		VALUES (gen_random_uuid(), $1, sha256('half'), now() + interval '10 minutes', gen_random_uuid(), false, 'totp') RETURNING id::text`, b.UserID).Scan(&id)
	if err != nil {
		t.Fatalf("half-login INSERT: %v", err)
	}
	// verify: completa la sesión con la 2FA y emite los tokens.
	_, err = app.Exec(ctx, `UPDATE iam.sessions SET mfa_satisfied = true, mfa_method = 'totp', access_hash = sha256('v-a'), refresh_hash = sha256('v-r'),
		access_expires_at = now() + interval '15 minutes', refresh_expires_at = now() + interval '30 days' WHERE id = $1`, id)
	if err != nil {
		t.Errorf("verify UPDATE: %v", err)
	}
	_, err = app.Exec(ctx, `UPDATE iam.sessions SET refresh_family = gen_random_uuid() WHERE id = $1`, id)
	wantState(t, err, "42501", "changing refresh_family")
}

// H1: una sesión del usuario A con el tenant y la membresía de B no ve la membresía de B como vigente.
func TestSessionLookupIgnoresAMembershipOfAnotherUser(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	a := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound, MembershipVersion: 2})
	b := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound, MembershipVersion: 7})
	app := connectAs(t, d, dbtest.App)
	if rows, err := lookup(ctx, app, a.Token); err != nil || len(rows) != 1 || rows[0].CurrentVersion == nil || *rows[0].CurrentVersion != 2 {
		t.Fatalf("control: %v, %v", rows, err)
	}
	// wanka_app puede escribir tenant_id y membership_id de su sesión: se cruza con la membresía de B.
	if _, err := app.Exec(ctx, `UPDATE iam.sessions SET tenant_id = $2, membership_id = $3 WHERE id = $1`, a.SessionID, b.TenantID, b.MembershipID); err != nil {
		t.Fatal(err)
	}
	rows, err := lookup(ctx, app, a.Token)
	if err != nil || len(rows) != 1 {
		t.Fatalf("crossed lookup = %v, %v", rows, err)
	}
	if r := rows[0]; r.CurrentVersion != nil || r.MembershipStatus != nil || r.Membership == nil {
		t.Errorf("crossed session sees %v/%v, want the membership absent (it is unbound by ADR 0011, never current)", r.CurrentVersion, deref(r.MembershipStatus))
	}
}

// Revocación irreversible, re-revocar una impersonada y created_at del servidor (seguimientos de seguridad de #200).
func TestRevocationIsIrreversibleAndCreatedAtComesFromTheServer(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	b := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	imp := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionImpersonated})
	app := connectAs(t, d, dbtest.App)
	upd := func(id, set string) error {
		_, err := app.Exec(ctx, `UPDATE iam.sessions SET `+set+` WHERE id = $1`, id)
		return err
	}
	for name, id := range map[string]string{"normal": b.SessionID, "impersonated": imp.SessionID} {
		if err := upd(id, `revoked_at = now()`); err != nil {
			t.Fatalf("%s: revoking: %v", name, err)
		}
		wantState(t, upd(id, `revoked_at = NULL`), "42501", name+": un-revoking")
		// Revocar otra vez no falla ni mueve el instante (la transacción anterior ya terminó: now() cambió).
		var before, after time.Time
		if err := app.QueryRow(ctx, `SELECT revoked_at FROM iam.sessions WHERE id = $1`, id).Scan(&before); err != nil {
			t.Fatal(err)
		}
		if err := upd(id, `revoked_at = now() + interval '1 day'`); err != nil {
			t.Errorf("%s: revoking again: %v", name, err)
		}
		if err := app.QueryRow(ctx, `SELECT revoked_at FROM iam.sessions WHERE id = $1`, id).Scan(&after); err != nil || !after.Equal(before) {
			t.Errorf("%s: revoked_at moved from %v to %v (err %v)", name, before, after, err)
		}
	}
	// UPDATE masivo por usuario: revoca las vivas aunque alguna ya estuviera revocada, y no mueve el instante de las viejas.
	live := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionBound})
	admin := d.Admin(ctx, t)
	if _, err := admin.Exec(ctx, `UPDATE iam.sessions SET user_id = $1 WHERE id = $2`, b.UserID, live.SessionID); err != nil {
		t.Fatal(err)
	}
	var bBefore time.Time
	if err := app.QueryRow(ctx, `SELECT revoked_at FROM iam.sessions WHERE id = $1`, b.SessionID).Scan(&bBefore); err != nil {
		t.Fatal(err)
	}
	tag, err := app.Exec(ctx, `UPDATE iam.sessions SET revoked_at = now() WHERE user_id = $1`, b.UserID)
	if err != nil || tag.RowsAffected() != 2 {
		t.Fatalf("bulk revoke = %v, %v; want 2 rows with no error", tag, err)
	}
	var liveRevoked bool
	var bAt time.Time
	if err := app.QueryRow(ctx, `SELECT (SELECT revoked_at IS NOT NULL FROM iam.sessions WHERE id = $1),
		(SELECT revoked_at FROM iam.sessions WHERE id = $2)`, live.SessionID, b.SessionID).Scan(&liveRevoked, &bAt); err != nil {
		t.Fatal(err)
	}
	if !liveRevoked || !bAt.Equal(bBefore) {
		t.Errorf("bulk revoke: live revoked = %v, already-revoked instant %v moved to %v", liveRevoked, bBefore, bAt)
	}

	var created, now time.Time
	err = app.QueryRow(ctx, `INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, mfa_satisfied, mfa_method, created_at)
		VALUES (gen_random_uuid(), $1, sha256('created'), now(), true, 'none', now() - interval '400 days') RETURNING created_at, now()`, b.UserID).Scan(&created, &now)
	if err != nil {
		t.Fatal(err)
	}
	if !created.Equal(now) {
		t.Errorf("created_at = %v, want the server's now() = %v", created, now)
	}
}

// H10: una impersonada con membership_id y sin user_id (la escribe un superusuario o wanka_definer) ve la membresía ausente:
// NULL no coincide con ningún usuario (por eso la comparación es IS DISTINCT FROM y no <>).
func TestSessionLookupImpersonatedWithAMembershipSeesItAbsent(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	imp := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionImpersonated, MembershipVersion: 3})
	admin := d.Admin(ctx, t)
	if _, err := admin.Exec(ctx, `UPDATE iam.sessions SET membership_id = $2 WHERE id = $1`, imp.SessionID, imp.MembershipID); err != nil {
		t.Fatal(err)
	}
	rows, err := lookup(ctx, connectAs(t, d, dbtest.App), imp.Token)
	if err != nil || len(rows) != 1 {
		t.Fatalf("lookup = %v, %v", rows, err)
	}
	if r := rows[0]; r.UserID != nil || r.Membership == nil || r.CurrentVersion != nil || r.MembershipStatus != nil {
		t.Errorf("row = %+v, want user_id NULL, membership_id set and the membership absent", r)
	}
}

// publicExecuteAllowed lista las funciones de iam que PUBLIC puede ejecutar a propósito (firma como la
// imprime regprocedure). Está vacía: una función nueva de iam sale de CREATE FUNCTION con EXECUTE para PUBLIC y
// wanka_platform, que desde #172 tiene USAGE en iam, podría llamarla. Cada función debe hacer REVOKE ... FROM PUBLIC
// y GRANT al rol que la usa; agregar una excepción aquí es una decisión de arquitectura.
var publicExecuteAllowed = map[string]bool{}

// Ninguna función de iam (salvo triggers, que solo dispara el motor) conserva el EXECUTE por defecto de PUBLIC.
func TestNoIAMFunctionIsExecutableByPublic(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	rows, err := d.Admin(ctx, t).Query(ctx, `
		SELECT p.oid::regprocedure::text
		  FROM pg_proc p
		 WHERE p.pronamespace = 'iam'::regnamespace
		   AND p.prorettype <> 'trigger'::regtype
		   AND has_function_privilege('public', p.oid, 'EXECUTE')
		 ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var fn string
		if err := rows.Scan(&fn); err != nil {
			t.Fatal(err)
		}
		if !publicExecuteAllowed[fn] {
			t.Errorf("PUBLIC can execute %s: REVOKE ALL ON FUNCTION ... FROM PUBLIC and grant it to the role that uses it", fn)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if n != len(publicExecuteAllowed) {
		t.Errorf("%d functions of iam are executable by PUBLIC, the allowlist has %d", n, len(publicExecuteAllowed))
	}
	// La consulta no está vacía por error: iam tiene funciones que no son triggers (session_lookup).
	if c := queryOne[int](t, d.Admin(ctx, t), `SELECT count(*) FROM pg_proc WHERE pronamespace = 'iam'::regnamespace AND prorettype <> 'trigger'::regtype`); c == 0 {
		t.Error("no non-trigger function found in iam: the query checks nothing")
	}
}
