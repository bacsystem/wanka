package stub_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
	"github.com/bacsystem/wanka/apps/api/internal/platform/mailer"
	"github.com/bacsystem/wanka/apps/api/internal/platform/mailer/stub"
)

// El módulo que consume el puerto solo ve la interfaz.
var _ mailer.Mailer = (*stub.Stub)(nil)

func newStub(max int) (*stub.Stub, *bytes.Buffer) {
	var buf bytes.Buffer
	return stub.New(logging.New(&buf, slog.LevelDebug), max), &buf
}

func msg() mailer.Message {
	return mailer.Message{
		To:             "ana.perez@example.com",
		Subject:        "Tu código de verificación",
		TextBody:       "Tu código es 482913",
		HTMLBody:       `<a href="https://wanka.test/reset?token=wk_rt_SECRETO">entrar</a>`,
		Tags:           []string{"mfa_enrollment"},
		IdempotencyKey: "mfa:u1:1",
	}
}

func TestSendRecordsAndLogsMaskedRecipientAndSubject(t *testing.T) {
	s, buf := newStub(10)
	if err := s.Send(context.Background(), msg()); err != nil {
		t.Fatal(err)
	}
	sent := s.Sent()
	if len(sent) != 1 || sent[0].To != "ana.perez@example.com" || sent[0].TextBody != "Tu código es 482913" {
		t.Fatalf("inbox = %+v", sent)
	}
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log is not JSON: %v: %q", err, buf.String())
	}
	if line["recipient"] != "a***@example.com" || line["subject"] != "Tu código de verificación" {
		t.Fatalf("log = %v", line)
	}
}

// Invariante: ningún envío deja en el log código, token, enlace ni la dirección completa.
func TestLogNeverContainsBodyTokenOrFullAddress(t *testing.T) {
	s, buf := newStub(10)
	m := msg()
	if err := s.Send(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	// También el segundo envío (duplicado) y uno inválido.
	_ = s.Send(context.Background(), m)
	bad := m
	bad.IdempotencyKey = "otro"
	bad.Subject = ""
	_ = s.Send(context.Background(), bad)

	out := buf.String()
	for _, secret := range []string{"482913", "wk_rt_SECRETO", "reset?token", "ana.perez", "mfa:u1:1", "mfa_enrollment"} {
		if strings.Contains(out, secret) {
			t.Errorf("log leaks %q:\n%s", secret, out)
		}
	}
}

func TestInvalidMessageIsRejectedAndNotRecorded(t *testing.T) {
	s, buf := newStub(10)
	for _, edit := range []func(*mailer.Message){
		func(m *mailer.Message) { m.To = "" },
		func(m *mailer.Message) { m.To = "no-es-correo" },
		func(m *mailer.Message) { m.Subject = "" },
	} {
		m := msg()
		edit(&m)
		err := s.Send(context.Background(), m)
		if !errors.Is(err, mailer.ErrInvalidMessage) {
			t.Fatalf("want ErrInvalidMessage, got %v", err)
		}
	}
	if len(s.Sent()) != 0 {
		t.Fatal("invalid messages must not be recorded")
	}
	if strings.Contains(buf.String(), "mail sent") {
		t.Fatalf("invalid message logged as sent: %s", buf.String())
	}
}

// Un mensaje inválido no consume su IdempotencyKey: el reintento corregido sí sale.
func TestInvalidMessageDoesNotConsumeIdempotencyKey(t *testing.T) {
	s, _ := newStub(10)
	bad := msg()
	bad.Subject = ""
	_ = s.Send(context.Background(), bad)
	if err := s.Send(context.Background(), msg()); err != nil {
		t.Fatal(err)
	}
	if n := len(s.Sent()); n != 1 {
		t.Fatalf("sent = %d, want 1", n)
	}
}

func TestSameIdempotencyKeyRecordsOnce(t *testing.T) {
	s, _ := newStub(10)
	for i := 0; i < 2; i++ {
		if err := s.Send(context.Background(), msg()); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(s.Sent()); n != 1 {
		t.Fatalf("sent = %d, want 1", n)
	}
	other := msg()
	other.IdempotencyKey = "mfa:u1:2" // un reenvío del código es otro intento
	if err := s.Send(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	if n := len(s.Sent()); n != 2 {
		t.Fatalf("sent = %d, want 2", n)
	}
}

func TestEmptyIdempotencyKeyNeverDeduplicates(t *testing.T) {
	s, _ := newStub(10)
	m := msg()
	m.IdempotencyKey = ""
	_ = s.Send(context.Background(), m)
	_ = s.Send(context.Background(), m)
	if n := len(s.Sent()); n != 2 {
		t.Fatalf("sent = %d, want 2", n)
	}
}

func TestSentReturnsACopy(t *testing.T) {
	s, _ := newStub(10)
	m := msg()
	_ = s.Send(context.Background(), m)
	m.Tags[0] = "mutado" // el llamador sigue dueño de su Message
	got := s.Sent()
	if got[0].Tags[0] != "mfa_enrollment" {
		t.Fatal("stub kept an alias of the caller's Tags")
	}
	got[0].Tags[0] = "otro"
	got[0].To = "x@example.com"
	if again := s.Sent(); again[0].Tags[0] != "mfa_enrollment" || again[0].To != "ana.perez@example.com" {
		t.Fatal("Sent exposes internal state")
	}
}

// El stub puede correr en producción hasta la ola 4: la bandeja no crece sin límite.
func TestInboxIsBounded(t *testing.T) {
	s, _ := newStub(3)
	for i := 0; i < 5; i++ {
		m := msg()
		m.IdempotencyKey = fmt.Sprintf("k%d", i)
		m.Subject = fmt.Sprintf("n%d", i)
		if err := s.Send(context.Background(), m); err != nil {
			t.Fatal(err)
		}
	}
	got := s.Sent()
	if len(got) != 3 || got[0].Subject != "n2" || got[2].Subject != "n4" {
		t.Fatalf("inbox = %+v", got)
	}
	// La llave expulsada se olvida (la memoria también está acotada).
	m := msg()
	m.IdempotencyKey = "k0"
	m.Subject = "otra vez"
	_ = s.Send(context.Background(), m)
	got = s.Sent()
	if len(got) != 3 || got[2].Subject != "otra vez" {
		t.Fatalf("an evicted key must be forgotten: %+v", got)
	}
}

func TestCancelledContextSendsNothing(t *testing.T) {
	s, _ := newStub(10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Send(ctx, msg()); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if len(s.Sent()) != 0 {
		t.Fatal("recorded after cancel")
	}
}

// Varias goroutinas envían y leen a la vez; -race detecta un acceso sin candado.
func TestConcurrentSendAndSent(t *testing.T) {
	s, _ := newStub(20)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := msg()
			m.IdempotencyKey = fmt.Sprintf("k%d", i)
			_ = s.Send(context.Background(), m)
			_ = s.Sent()
		}()
	}
	wg.Wait()
	if n := len(s.Sent()); n != 20 {
		t.Fatalf("sent = %d, want 20", n)
	}
}

func TestConcurrentSameKeyRecordsOnce(t *testing.T) {
	s, _ := newStub(100)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Send(context.Background(), msg())
			_ = s.Sent()
		}()
	}
	wg.Wait()
	if n := len(s.Sent()); n != 1 {
		t.Fatalf("sent = %d, want 1", n)
	}
}
