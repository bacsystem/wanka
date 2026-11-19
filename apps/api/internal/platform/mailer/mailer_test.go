package mailer_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/mailer"
)

func valid() mailer.Message {
	return mailer.Message{To: "ana@example.com", Subject: "Hola", TextBody: "cuerpo"}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*mailer.Message)
		field string // "" = válido
	}{
		{"valid", func(*mailer.Message) {}, ""},
		{"no recipient", func(m *mailer.Message) { m.To = "" }, "To"},
		{"blank recipient", func(m *mailer.Message) { m.To = "   " }, "To"},
		{"no at sign", func(m *mailer.Message) { m.To = "ana.example.com" }, "To"},
		{"no domain", func(m *mailer.Message) { m.To = "ana@" }, "To"},
		{"display name", func(m *mailer.Message) { m.To = "Ana <ana@example.com>" }, "To"},
		{"two recipients", func(m *mailer.Message) { m.To = "a@example.com, b@example.com" }, "To"},
		{"recipient with newline", func(m *mailer.Message) { m.To = "a@example.com\r\nBcc: x@example.com" }, "To"},
		{"recipient with NBSP", func(m *mailer.Message) { m.To = "ana@exam\u00a0ple.com" }, "To"},
		{"recipient with ideographic space", func(m *mailer.Message) { m.To = "ana@example.com\u3000" }, "To"},
		{"recipient with NUL", func(m *mailer.Message) { m.To = "a\x00na@example.com" }, "To"},
		{"recipient with zero-width char", func(m *mailer.Message) { m.To = "an\u200ba@example.com" }, "To"},
		{"local part 65 chars", func(m *mailer.Message) { m.To = strings.Repeat("a", 65) + "@example.com" }, "To"},
		{"local part 64 chars", func(m *mailer.Message) { m.To = strings.Repeat("a", 64) + "@example.com" }, ""},
		{"address over 254", func(m *mailer.Message) {
			m.To = "a@" + strings.Repeat("b", 60) + "." + strings.Repeat("c", 60) + "." + strings.Repeat("d", 60) + "." + strings.Repeat("e", 60) + "." + strings.Repeat("f", 60) + ".com"
		}, "To"},
		{"subject with NUL", func(m *mailer.Message) { m.Subject = "Ho\x00la" }, "Subject"},
		{"subject with vertical tab", func(m *mailer.Message) { m.Subject = "Ho\vla" }, "Subject"},
		{"subject with C1 control", func(m *mailer.Message) { m.Subject = "Ho\u0085la" }, "Subject"},
		{"subject with line separator", func(m *mailer.Message) { m.Subject = "Ho\u2028la" }, "Subject"},
		{"subject with paragraph separator", func(m *mailer.Message) { m.Subject = "Ho\u2029la" }, "Subject"},
		{"subject with tab", func(m *mailer.Message) { m.Subject = "Ho\tla" }, "Subject"},
		{"subject 998 bytes", func(m *mailer.Message) { m.Subject = strings.Repeat("a", 998) }, ""},
		{"subject 999 bytes", func(m *mailer.Message) { m.Subject = strings.Repeat("a", 999) }, "Subject"},
		{"subject with accents", func(m *mailer.Message) { m.Subject = "Código de verificación ñ" }, ""},
		{"empty subject", func(m *mailer.Message) { m.Subject = "" }, "Subject"},
		{"blank subject", func(m *mailer.Message) { m.Subject = " \t " }, "Subject"},
		{"subject with newline", func(m *mailer.Message) { m.Subject = "Hola\r\nBcc: x@example.com" }, "Subject"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := valid()
			tc.edit(&m)
			err := mailer.Validate(m)
			if tc.field == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var ve *mailer.ValidationError
			if !errors.As(err, &ve) || !errors.Is(err, mailer.ErrInvalidMessage) {
				t.Fatalf("want ValidationError wrapping ErrInvalidMessage, got %v", err)
			}
			if ve.Field != tc.field {
				t.Fatalf("field = %q, want %q", ve.Field, tc.field)
			}
		})
	}
}

// El error de validación no puede repetir la dirección ni el asunto: terminan en logs.
func TestValidationErrorDoesNotEchoValues(t *testing.T) {
	m := valid()
	m.To = "secreto@example.com\n"
	err := mailer.Validate(m)
	if err == nil || strings.Contains(err.Error(), "secreto") {
		t.Fatalf("error must not echo the address: %v", err)
	}
}

func TestMask(t *testing.T) {
	cases := map[string]string{
		"ana.perez@example.com": "a***@example.com",
		"ab@example.com":        "***@example.com",
		"a@example.com":         "***@example.com",
		"":                      "***",
		"sin-arroba":            "***",
		"@example.com":          "***@example.com",
	}
	for in, want := range cases {
		if got := mailer.Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, want %q", in, got, want)
		}
	}
	// Nunca devuelve la parte local completa.
	if got := mailer.Mask("ana.perez@example.com"); strings.Contains(got, "perez") {
		t.Fatalf("leaks local part: %s", got)
	}
}

func TestDisabledRefusesToSend(t *testing.T) {
	err := mailer.Disabled{}.Send(context.Background(), valid())
	if !errors.Is(err, mailer.ErrDisabled) {
		t.Fatalf("want ErrDisabled, got %v", err)
	}
}

// Disabled valida primero: un mensaje inválido da ErrInvalidMessage con cualquier
// adaptador, y ErrDisabled solo llega con un mensaje bueno.
func TestDisabledValidatesBeforeRefusing(t *testing.T) {
	bad := valid()
	bad.To = ""
	err := mailer.Disabled{}.Send(context.Background(), bad)
	if !errors.Is(err, mailer.ErrInvalidMessage) || errors.Is(err, mailer.ErrDisabled) {
		t.Fatalf("want ErrInvalidMessage only, got %v", err)
	}
}

func fullMessage() mailer.Message {
	return mailer.Message{
		To: "ana.perez@example.com", Subject: "Tu código", TextBody: "Tu código es 482913",
		HTMLBody: "<a href=https://x/reset?token=wk_rt_SECRETO>", Tags: []string{"mfa_enrollment"}, IdempotencyKey: "mfa:u1:1",
	}
}

func assertNoLeak(t *testing.T, out string) {
	t.Helper()
	for _, secret := range []string{"482913", "wk_rt_SECRETO", "ana.perez", "mfa:u1:1", "mfa_enrollment"} {
		if strings.Contains(out, secret) {
			t.Errorf("output leaks %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, "a***@example.com") || !strings.Contains(out, "Tu código") {
		t.Errorf("output lost the masked recipient or subject: %s", out)
	}
}

// Un Message registrado entero por descuido (log o fmt) no vuelca datos personales.
func TestMessageLoggedWholeLeaksNothing(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Error("send failed", "message", fullMessage())
	assertNoLeak(t, buf.String())

	m := fullMessage()
	for _, out := range []string{
		fmt.Sprintf("%v", m), fmt.Sprintf("%+v", m), fmt.Sprintf("%#v", m), fmt.Sprintf("%s", m),
		fmt.Sprintf("%v", &m), fmt.Errorf("send %v: %w", m, errors.New("x")).Error(),
	} {
		assertNoLeak(t, out)
	}
}
