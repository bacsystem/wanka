// Package mailer es el puerto de envío de correo: lo importan los módulos que
// necesitan mandar un mensaje (invitaciones, recuperación de contraseña, código
// de un solo uso de la 2FA) y nunca el adaptador concreto. El adaptador se elige
// al arrancar el proceso (MAILER=stub|none; el proveedor real llega con
// notificaciones, ola 4).
//
// Los datos del mensaje son personales o secretos: el cuerpo puede llevar un
// código de un solo uso o un enlace con token. Un adaptador nunca los escribe
// en el log. El asunto y el destinatario sí se registran (el destinatario
// enmascarado, ver Mask), así que el asunto no puede llevar ni códigos ni
// tokens: van solo en el cuerpo.
package mailer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Topes de Validate. Las direcciones siguen RFC 5321 (parte local 64, total 254);
// el asunto, el límite de línea de RFC 5322 §2.1.1.
const (
	MaxLocalPartLen = 64
	MaxAddressLen   = 254
	MaxSubjectLen   = 998 // bytes
)

// Message es un correo para un solo destinatario.
type Message struct {
	// To es una dirección simple («ana@example.com»), sin nombre ni lista.
	To string
	// Subject no puede estar vacío ni llevar saltos de línea. No pongas en él
	// códigos ni tokens: los adaptadores lo registran.
	Subject  string
	TextBody string
	HTMLBody string
	// Tags clasifican el mensaje (por ejemplo «mfa_enrollment») para el proveedor;
	// no son secretos ni datos personales.
	Tags []string
	// IdempotencyKey identifica el intento de envío: dos Send con la misma llave
	// entregan un solo correo. Vacía, no deduplica. Quien reenvía a propósito
	// (otro código de 2FA, no el mismo reintento) usa una llave distinta.
	IdempotencyKey string
}

// LogValue deja que slog registre un Message entero sin filtrar nada: solo el
// destinatario enmascarado y el asunto. Nunca el cuerpo, las etiquetas ni la llave.
func (m Message) LogValue() slog.Value {
	return slog.GroupValue(slog.String("recipient", Mask(m.To)), slog.String("subject", m.Subject))
}

// String y GoString dan lo mismo que LogValue a fmt (%v, %+v, %#v, %s) y a los
// errores que formatean el mensaje.
func (m Message) String() string {
	return fmt.Sprintf("Message{recipient:%s subject:%q}", Mask(m.To), m.Subject)
}

// GoString implementa fmt.GoStringer; ver String.
func (m Message) GoString() string { return m.String() }

// Mailer envía un correo. Send respeta la cancelación de ctx. Un error de
// validación (ErrInvalidMessage) y ErrDisabled no son reintentables: ningún
// reintento los arregla. Cualquier otro error puede serlo.
type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// ErrInvalidMessage envuelve todo error de validación del mensaje.
var ErrInvalidMessage = errors.New("invalid mail message")

// ErrDisabled lo devuelve el adaptador Disabled (MAILER=none). No es reintentable:
// seguirá igual mientras no cambie la configuración.
var ErrDisabled = errors.New("mailer is disabled")

// ValidationError dice qué campo es inválido y por qué, sin repetir su valor:
// el mensaje termina en logs y la dirección es un dato personal.
type ValidationError struct {
	Field  string // nombre del campo de Message
	Reason string
}

func (e *ValidationError) Error() string {
	return "invalid mail message: " + e.Field + " " + e.Reason
}

// Unwrap permite errors.Is(err, ErrInvalidMessage).
func (e *ValidationError) Unwrap() error { return ErrInvalidMessage }

// Validate comprueba lo que todo adaptador exige, para que un mensaje que ningún
// proveedor aceptaría falle aquí y no se reintente: un destinatario simple (con los
// topes de MaxLocalPartLen y MaxAddressLen, sin espacios ni caracteres no
// imprimibles) y un asunto no vacío de hasta MaxSubjectLen bytes, sin caracteres de
// control (saltos de línea, NUL, tabulador, C1) ni los separadores U+2028/U+2029
// (inyección de cabeceras).
func Validate(m Message) error {
	if strings.TrimSpace(m.To) == "" {
		return &ValidationError{"To", "is required"}
	}
	if len(m.To) > MaxAddressLen {
		return &ValidationError{"To", "is too long"}
	}
	for _, r := range m.To {
		if !unicode.IsPrint(r) || unicode.IsSpace(r) {
			return &ValidationError{"To", "must not contain spaces, control or non-printable characters"}
		}
	}
	// ParseAddress acepta «Ana <a@x>» y listas; exigir que devuelva la misma
	// cadena deja pasar solo una dirección simple.
	addr, err := mail.ParseAddress(m.To)
	if err != nil || addr.Address != m.To {
		return &ValidationError{"To", "must be a single email address"}
	}
	if at := strings.LastIndexByte(m.To, '@'); at > MaxLocalPartLen {
		return &ValidationError{"To", "has a local part that is too long"}
	}
	if strings.TrimSpace(m.Subject) == "" {
		return &ValidationError{"Subject", "is required"}
	}
	if len(m.Subject) > MaxSubjectLen {
		return &ValidationError{"Subject", "is too long"}
	}
	for _, r := range m.Subject {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return &ValidationError{"Subject", "must not contain control characters or line breaks"}
		}
	}
	return nil
}

// Mask oculta la parte local de una dirección para registrarla: conserva el
// primer carácter si hay al menos tres («a***@example.com») y lo oculta todo si
// no. Lo que no parece una dirección da «***».
func Mask(addr string) string {
	at := strings.LastIndexByte(addr, '@')
	if at < 0 {
		return "***"
	}
	local, domain := addr[:at], addr[at:]
	if utf8.RuneCountInString(local) < 3 {
		return "***" + domain
	}
	r, _ := utf8.DecodeRuneInString(local)
	return string(r) + "***" + domain
}

// Disabled es el adaptador de MAILER=none: rechaza todo envío.
type Disabled struct{}

// Send valida el mensaje (un inválido da ErrInvalidMessage con cualquier
// adaptador) y luego devuelve ErrDisabled.
func (Disabled) Send(_ context.Context, m Message) error {
	if err := Validate(m); err != nil {
		return err
	}
	return ErrDisabled
}
