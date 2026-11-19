package pgsession

import (
	"context"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// BindVersion y Revoke: la guarda del UPDATE de la versión es id, membresía y
// sesión viva; 0 filas significa solo «desligada o revocada».
func TestBindVersionAndRevokeGuards(t *testing.T) {
	d, p := migratedPool(t)
	r := New(p)
	ctx := context.Background()
	version := func(id string) (v *int64, revoked bool) {
		t.Helper()
		err := d.Admin(ctx, t).QueryRow(ctx, `SELECT authz_version, revoked_at IS NOT NULL FROM iam.sessions WHERE id = $1`, id).Scan(&v, &revoked)
		if err != nil {
			t.Fatal(err)
		}
		return v, revoked
	}

	t.Run("binds the version", func(t *testing.T) {
		s := d.SeedSession(t, dbtest.SessionSpec{})
		ok, err := r.BindVersion(ctx, mustUUID(t, s.SessionID), mustUUID(t, s.MembershipID), 7)
		if err != nil || !ok {
			t.Fatalf("BindVersion = %v, %v", ok, err)
		}
		if v, _ := version(s.SessionID); v == nil || *v != 7 {
			t.Fatalf("authz_version = %v, want 7", v)
		}
	})
	t.Run("another membership binds nothing", func(t *testing.T) {
		s := d.SeedSession(t, dbtest.SessionSpec{})
		ok, err := r.BindVersion(ctx, mustUUID(t, s.SessionID), mustUUID(t, "aaaaaaaa-0000-4000-8000-00000000000b"), 7)
		if err != nil || ok {
			t.Fatalf("BindVersion = %v, %v; want false, nil", ok, err)
		}
		if v, _ := version(s.SessionID); v == nil || *v != 1 {
			t.Fatalf("authz_version = %v, want the seeded 1", v)
		}
	})
	t.Run("a detached session stays without version", func(t *testing.T) {
		s := d.SeedSession(t, dbtest.SessionSpec{})
		sid, mid := mustUUID(t, s.SessionID), mustUUID(t, s.MembershipID)
		if ok, err := r.Detach(ctx, sid, mid); err != nil || !ok {
			t.Fatalf("Detach = %v, %v", ok, err)
		}
		ok, err := r.BindVersion(ctx, sid, mid, 9)
		if err != nil || ok {
			t.Fatalf("BindVersion on a detached session = %v, %v; want false, nil", ok, err)
		}
		if v, _ := version(s.SessionID); v != nil {
			t.Fatalf("a detached session got authz_version %d", *v)
		}
	})
	t.Run("a revoked session is left as it is", func(t *testing.T) {
		s := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionRevoked})
		ok, err := r.BindVersion(ctx, mustUUID(t, s.SessionID), mustUUID(t, s.MembershipID), 9)
		if err != nil || ok {
			t.Fatalf("BindVersion on a revoked session = %v, %v", ok, err)
		}
	})
	t.Run("revoke is idempotent and keeps the tenant", func(t *testing.T) {
		s := d.SeedSession(t, dbtest.SessionSpec{})
		for i := 0; i < 2; i++ {
			if err := r.Revoke(ctx, mustUUID(t, s.SessionID)); err != nil {
				t.Fatalf("Revoke %d: %v", i, err)
			}
		}
		if _, revoked := version(s.SessionID); !revoked {
			t.Fatal("the session is not revoked")
		}
		if _, found, err := r.Lookup(ctx, hashOf(s.Token)); err != nil || found {
			t.Fatalf("Lookup of a revoked session = %v, %v", found, err)
		}
	})
}
