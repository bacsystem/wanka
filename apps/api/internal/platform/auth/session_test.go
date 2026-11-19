package auth_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

func uuid(b byte) pgtype.UUID { return pgtype.UUID{Bytes: [16]byte{15: b}, Valid: true} }

func TestSessionActorComesFromTheSessionOnly(t *testing.T) {
	user := auth.Session{ID: uuid(1), UserID: uuid(2), TenantID: uuid(3), MembershipID: uuid(4)}
	if a := user.Actor(); a.Kind != db.KindUser || a.TenantID != uuid(3) || a.UserID != uuid(2) || a.ImpersonatorID.Valid {
		t.Fatalf("user actor = %+v", a)
	}
	imp := auth.Session{ID: uuid(1), UserID: uuid(2), TenantID: uuid(3), ImpersonatorID: uuid(9)}
	if a := imp.Actor(); a.Kind != db.KindImpersonated || a.ImpersonatorID != uuid(9) {
		t.Fatalf("impersonated actor = %+v", a)
	}
	if !imp.Impersonated() || user.Impersonated() {
		t.Fatal("Impersonated() disagrees with ImpersonatorID")
	}
}

func TestSessionHasTenantIgnoresEmptyAndZeroIDs(t *testing.T) {
	zero := pgtype.UUID{Valid: true}
	for name, tc := range map[string]struct {
		id   pgtype.UUID
		want bool
	}{"unset": {pgtype.UUID{}, false}, "zero": {zero, false}, "set": {uuid(1), true}} {
		if got := (auth.Session{TenantID: tc.id}).HasTenant(); got != tc.want {
			t.Errorf("%s: HasTenant = %v", name, got)
		}
	}
}

func TestSessionWithoutTenantDropsEveryTenantColumn(t *testing.T) {
	v := int64(4)
	s := auth.Session{ID: uuid(1), UserID: uuid(2), TenantID: uuid(3), SiteID: uuid(4), CashRegisterID: uuid(5),
		MembershipID: uuid(6), AuthzVersion: &v, MFAMethod: "totp", MFASatisfied: true}
	u := s.WithoutTenant()
	if u.TenantID.Valid || u.SiteID.Valid || u.CashRegisterID.Valid || u.MembershipID.Valid || u.AuthzVersion != nil {
		t.Fatalf("tenant columns survive: %+v", u)
	}
	if u.ID != s.ID || u.UserID != s.UserID || u.MFAMethod != "totp" || !u.MFASatisfied {
		t.Fatalf("session identity lost: %+v", u)
	}
	if s.TenantID != uuid(3) {
		t.Fatal("WithoutTenant mutated the receiver")
	}
}

func TestSessionContextRoundTrip(t *testing.T) {
	ctx := t.Context()
	if _, ok := auth.SessionFrom(ctx); ok {
		t.Fatal("empty context has a session")
	}
	s := auth.Session{ID: uuid(1)}
	got, ok := auth.SessionFrom(auth.WithSession(ctx, s))
	if !ok || got.ID != s.ID {
		t.Fatalf("got %+v, %v", got, ok)
	}
}

func TestLostMembership(t *testing.T) {
	v := int64(3)
	active := auth.Session{MembershipID: uuid(4), CurrentAuthzVersion: &v, MembershipStatus: auth.MembershipActive}
	deleted := auth.Session{MembershipID: uuid(4)}
	suspended := auth.Session{MembershipID: uuid(4), CurrentAuthzVersion: &v, MembershipStatus: "suspendido"}
	deletedStillActive := auth.Session{MembershipID: uuid(4), MembershipStatus: auth.MembershipActive}
	emptyStatus := auth.Session{MembershipID: uuid(4), CurrentAuthzVersion: &v}
	neverChose := auth.Session{}
	impersonated := auth.Session{ImpersonatorID: uuid(9), MembershipID: uuid(4)}
	for name, tc := range map[string]struct {
		s    auth.Session
		want bool
	}{
		"active": {active, false}, "deleted (no current version)": {deleted, true}, "suspended": {suspended, true},
		"no current version even if the status reads active": {deletedStillActive, true}, "empty status": {emptyStatus, true}, "no membership": {neverChose, false}, "impersonated": {impersonated, false},
	} {
		if got := tc.s.LostMembership(); got != tc.want {
			t.Errorf("%s: LostMembership = %v, want %v", name, got, tc.want)
		}
	}
}
