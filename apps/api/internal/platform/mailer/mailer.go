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
	"net/mail"
	"strings"
	"unicode/utf8"
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

// Mailer envía un correo. Send respeta la cancelación de ctx. Un error de
// validación (ErrInvalidMessage) no es reintentable; cualquier otro puede serlo.
type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// ErrInvalidMessage envuelve todo error de validación del mensaje.
var ErrInvalidMessage = errors.New("invalid mail message")

// ErrDisabled lo devuelve el adaptador Disabled (MAILER=none).
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

// Validate comprueba lo que todo adaptador exige: un destinatario válido y un
// asunto no vacío. Rechaza los saltos de línea en ambos (inyección de cabeceras).
func Validate(m Message) error {
	if strings.TrimSpace(m.To) == "" {
		return &ValidationError{"To", "is required"}
	}
	if strings.ContainsAny(m.To, "\r\n") {
		return &ValidationError{"To", "must not contain line breaks"}
	}
	// ParseAddress acepta «Ana <a@x>» y listas; exigir que devuelva la misma
	// cadena deja pasar solo una dirección simple.
	addr, err := mail.ParseAddress(m.To)
	if err != nil || addr.Address != m.To {
		return &ValidationError{"To", "must be a single email address"}
	}
	if strings.TrimSpace(m.Subject) == "" {
		return &ValidationError{"Subject", "is required"}
	}
	if strings.ContainsAny(m.Subject, "\r\n") {
		return &ValidationError{"Subject", "must not contain line breaks"}
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

// Send devuelve siempre ErrDisabled.
func (Disabled) Send(context.Context, Message) error { return ErrDisabled }
