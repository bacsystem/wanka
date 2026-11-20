package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/app"
	"github.com/bacsystem/wanka/apps/api/internal/platform/jobs"
	"github.com/bacsystem/wanka/apps/api/internal/platform/mailer"
)

type dir struct {
	email string
	err   error
}

func (d dir) EmailOf(context.Context, string) (string, error) { return d.email, d.err }

type sink struct {
	sent []mailer.Message
	err  error
}

func (s *sink) Send(_ context.Context, m mailer.Message) error {
	s.sent = append(s.sent, m)
	return s.err
}

var lockedUntil = time.Date(2026, 11, 19, 21, 30, 0, 0, time.UTC)

var jobArgs = app.LockoutNoticeArgs{UserID: "7f0c0f4e-1111-4222-8333-444455556666", LockedUntil: &lockedUntil}

func isCancel(err error) bool { return err != nil && strings.HasPrefix(err.Error(), "job cancelled") }

func TestLockoutNoticeSendsAMailWithoutLinksOrSecrets(t *testing.T) {
	s := &sink{}
	err := app.LockoutNotice(dir{email: "ana@x.pe"}, s)(context.Background(), jobs.Job[app.LockoutNoticeArgs]{Args: jobArgs})
	if err != nil || len(s.sent) != 1 {
		t.Fatalf("err = %v, sent = %d", err, len(s.sent))
	}
	m := s.sent[0]
	if m.To != "ana@x.pe" {
		t.Errorf("To = %q", m.To)
	}
	if err := mailer.Validate(m); err != nil {
		t.Errorf("the message is not valid for any adapter: %v", err)
	}
	for _, bad := range []string{"http", "://", "www.", "<a", "token", "wk_"} {
		if strings.Contains(strings.ToLower(m.TextBody), bad) || strings.Contains(strings.ToLower(m.Subject), bad) || strings.Contains(strings.ToLower(m.HTMLBody), bad) {
			t.Errorf("the message contains %q: the notice carries no links or secrets", bad)
		}
	}
	// 21:30 UTC son las 16:30 en Lima.
	if !strings.Contains(m.TextBody, "16:30") {
		t.Errorf("body = %q, want the unlock time in Lima (16:30)", m.TextBody)
	}
	if m.IdempotencyKey != "login-lockout:7f0c0f4e-1111-4222-8333-444455556666:"+strconv.FormatInt(lockedUntil.Unix(), 10) {
		t.Errorf("IdempotencyKey = %q: a retry of the same lock must reuse it", m.IdempotencyKey)
	}
}

func TestLockoutNoticeFailureClassification(t *testing.T) {
	transient := errors.New("smtp timeout")
	for name, tc := range map[string]struct {
		dir        dir
		sendErr    error
		wantCancel bool
		wantErr    bool
	}{
		"user is gone: permanent":       {dir{err: app.ErrUserNotFound}, nil, true, true},
		"directory down: retry":         {dir{err: errors.New("db down")}, nil, false, true},
		"mailer disabled: permanent":    {dir{email: "a@x.pe"}, mailer.ErrDisabled, true, true},
		"invalid message: permanent":    {dir{email: "a@x.pe"}, &mailer.ValidationError{Field: "To", Reason: "bad"}, true, true},
		"mailer transient error: retry": {dir{email: "a@x.pe"}, transient, false, true},
		"mailer ok":                     {dir{email: "a@x.pe"}, nil, false, false},
	} {
		t.Run(name, func(t *testing.T) {
			err := app.LockoutNotice(tc.dir, &sink{err: tc.sendErr})(context.Background(), jobs.Job[app.LockoutNoticeArgs]{Args: jobArgs})
			if (err != nil) != tc.wantErr || isCancel(err) != tc.wantCancel {
				t.Fatalf("err = %v, want error=%v cancel=%v", err, tc.wantErr, tc.wantCancel)
			}
		})
	}
}

func TestLockoutNoticeArgsCarryNoPersonalData(t *testing.T) {
	var _ jobs.Args = app.LockoutNoticeArgs{}
	if (app.LockoutNoticeArgs{}).Kind() != "identidad.login_lockout_notice" {
		t.Fatal("the Kind changed: it is stored in every job row")
	}
}

// El aviso de cierre no lleva locked_until (no tiene fin de bloqueo); el de bloqueo sí.
func TestLockoutNoticeArgsJSON(t *testing.T) {
	closed, _ := json.Marshal(app.LockoutNoticeArgs{UserID: "u", Closed: true})
	if strings.Contains(string(closed), "locked_until") || strings.Contains(string(closed), "0001") || !strings.Contains(string(closed), `"closed":true`) {
		t.Fatalf("closing notice args = %s", closed)
	}
	locked, _ := json.Marshal(jobArgs)
	if !strings.Contains(string(locked), `"locked_until":"2026-11-19T21:30:00Z"`) || strings.Contains(string(locked), "closed") {
		t.Fatalf("lockout notice args = %s", locked)
	}
}

func TestClosingNoticeHasItsOwnMailAndIdempotencyKey(t *testing.T) {
	s := &sink{}
	err := app.LockoutNotice(dir{email: "ana@x.pe"}, s)(context.Background(), jobs.Job[app.LockoutNoticeArgs]{Args: app.LockoutNoticeArgs{UserID: "u1", Closed: true}})
	if err != nil || len(s.sent) != 1 || s.sent[0].IdempotencyKey != "login-closed:u1" || !strings.Contains(s.sent[0].TextBody, "100") {
		t.Fatalf("err = %v, sent = %+v", err, s.sent)
	}
}
