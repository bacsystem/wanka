package mailer_test

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
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

// Criterio 1: un módulo importa el puerto, nunca el adaptador. El puerto tampoco
// importa al stub (el grafo iría al revés).
func TestPortDoesNotImportAdapters(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pkgs {
		for name, f := range p.Files {
			for _, imp := range f.Imports {
				if strings.Contains(imp.Path.Value, "mailerstub") {
					t.Errorf("%s imports %s", name, imp.Path.Value)
				}
			}
		}
	}
}
