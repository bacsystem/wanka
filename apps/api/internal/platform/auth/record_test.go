package auth

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

func id(b byte) pgtype.UUID { return pgtype.UUID{Bytes: [16]byte{15: b}, Valid: true} }

func i8(n int64) pgtype.Int8 { return pgtype.Int8{Int64: n, Valid: true} }

func active() SessionRecord {
	return SessionRecord{SessionID: id(1), UserID: id(2), TenantID: id(3), SiteID: id(5), CashRegisterID: id(6), MembershipID: id(4),
		SessionAuthzVersion: i8(1), CurrentAuthzVersion: i8(1), MembershipStatus: pgtype.Text{String: MembershipActive, Valid: true},
		MFASatisfied: true, MFAMethod: "totp"}
}

func TestRecordCoherent(t *testing.T) {
	mod := func(f func(*SessionRecord)) SessionRecord { r := active(); f(&r); return r }
	cases := map[string]struct {
		r    SessionRecord
		want bool
	}{
		"bound":                 {active(), true},
		"never chose a company": {mod(func(r *SessionRecord) { r.TenantID, r.MembershipID = pgtype.UUID{}, pgtype.UUID{} }), true},
		"impersonated": {mod(func(r *SessionRecord) {
			r.MembershipID, r.UserID, r.ImpersonatorID = pgtype.UUID{}, pgtype.UUID{}, id(9)
		}), true},
		"impersonated with a user id": {mod(func(r *SessionRecord) { r.MembershipID, r.ImpersonatorID = pgtype.UUID{}, id(9) }), false},
		"impersonated with a zero user": {mod(func(r *SessionRecord) {
			r.MembershipID, r.UserID, r.ImpersonatorID = pgtype.UUID{}, pgtype.UUID{Valid: true}, id(9)
		}), true},
		"tenant without membership": {mod(func(r *SessionRecord) { r.MembershipID = pgtype.UUID{} }), false},
		"membership without tenant": {mod(func(r *SessionRecord) { r.TenantID = pgtype.UUID{} }), false},
		"impersonator without tenant": {mod(func(r *SessionRecord) {
			r.TenantID, r.MembershipID, r.ImpersonatorID = pgtype.UUID{}, pgtype.UUID{}, id(9)
		}), false},
		"empty user":                      {mod(func(r *SessionRecord) { r.UserID = pgtype.UUID{} }), false},
		"zero user":                       {mod(func(r *SessionRecord) { r.UserID = pgtype.UUID{Valid: true} }), false},
		"empty session id":                {mod(func(r *SessionRecord) { r.SessionID = pgtype.UUID{} }), false},
		"zero tenant counts as no tenant": {mod(func(r *SessionRecord) { r.TenantID = pgtype.UUID{Valid: true} }), false},
	}
	for name, tc := range cases {
		if got := tc.r.coherent(); got != tc.want {
			t.Errorf("%s: coherent = %v, want %v", name, got, tc.want)
		}
	}
}

func TestRecordLostMembership(t *testing.T) {
	mod := func(f func(*SessionRecord)) SessionRecord { r := active(); f(&r); return r }
	cases := map[string]struct {
		r    SessionRecord
		want bool
	}{
		"active":                               {active(), false},
		"deleted":                              {mod(func(r *SessionRecord) { r.CurrentAuthzVersion, r.MembershipStatus = pgtype.Int8{}, pgtype.Text{} }), true},
		"no current version, status active":    {mod(func(r *SessionRecord) { r.CurrentAuthzVersion = pgtype.Int8{} }), true},
		"suspended":                            {mod(func(r *SessionRecord) { r.MembershipStatus.String = "suspendido" }), true},
		"null status":                          {mod(func(r *SessionRecord) { r.MembershipStatus = pgtype.Text{} }), true},
		"empty status":                         {mod(func(r *SessionRecord) { r.MembershipStatus = pgtype.Text{Valid: true} }), true},
		"never chose a company":                {mod(func(r *SessionRecord) { r.MembershipID = pgtype.UUID{} }), false},
		"impersonated":                         {mod(func(r *SessionRecord) { r.ImpersonatorID, r.MembershipID = id(9), pgtype.UUID{} }), false},
		"impersonated with a stale membership": {mod(func(r *SessionRecord) { r.ImpersonatorID = id(9); r.CurrentAuthzVersion = pgtype.Int8{} }), false},
	}
	for name, tc := range cases {
		if got := tc.r.lostMembership(); got != tc.want {
			t.Errorf("%s: lostMembership = %v, want %v", name, got, tc.want)
		}
	}
}

func TestRecordSession(t *testing.T) {
	s := active().session(false)
	if s.ID != id(1) || s.Actor != (db.Actor{TenantID: id(3), UserID: id(2), Kind: db.KindUser}) || s.SiteID != id(5) ||
		s.CashRegisterID != id(6) || s.MembershipID != id(4) || s.SessionAuthzVersion != i8(1) || s.CurrentAuthzVersion != i8(1) || s.MFAMethod != "totp" {
		t.Fatalf("session = %+v", s)
	}
	imp := active()
	imp.MembershipID, imp.UserID, imp.ImpersonatorID = pgtype.UUID{}, pgtype.UUID{}, id(9)
	if a := imp.session(false).Actor; a != (db.Actor{TenantID: id(3), UserID: id(9), Kind: db.KindImpersonated, ImpersonatorID: id(9)}) {
		t.Fatalf("impersonated actor = %+v", a)
	}
	d := active().session(true)
	if d.HasTenant() || d.Actor.TenantID.Valid || d.SiteID.Valid || d.CashRegisterID.Valid || d.MembershipID.Valid || d.SessionAuthzVersion.Valid || d.CurrentAuthzVersion.Valid {
		t.Fatalf("detached session keeps tenant columns: %+v", d)
	}
	if d.ID != id(1) || d.Actor.UserID != id(2) || d.Actor.Kind != db.KindUser || d.MFAMethod != "totp" {
		t.Fatalf("detached session lost its identity: %+v", d)
	}
}
