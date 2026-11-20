// Package domain son las reglas de identidad que no dependen de la
// infraestructura: la forma del correo y los topes del bloqueo progresivo del
// login (ADR 0004, nota del 2026-11-19).
package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Topes del login (api/conventions.md §2 y ADR 0004). Los mismos números están en
// el SQL de cobro del adaptador de PostgreSQL y en los CHECK de iam.users.
const (
	// MaxAttempts son los intentos cobrados por ventana: el 5.º fallo bloquea la cuenta.
	MaxAttempts = 5
	// Window es la ventana fija de cobro; solo se mueve cuando la anterior venció.
	Window = 15 * time.Minute
	// LockDuration es lo que dura el bloqueo tras el 5.º fallo.
	LockDuration = 15 * time.Minute
	// MaxLockouts es el techo absoluto: con 10 bloqueos el ingreso con contraseña
	// queda cerrado hasta restablecerla o cambiarla.
	MaxLockouts = 10
	// MaxMFALockouts: con 3 bloqueos del segundo paso, el login con la contraseña
	// correcta responde igual que con una mala (conventions.md §2).
	MaxMFALockouts = 3
)

// Largos del contrato (LoginRequest), en caracteres.
const (
	MinEmailLen    = 3
	MaxEmailLen    = 254
	MinPasswordLen = 1
	MaxPasswordLen = 128
)

// NormalizeEmail deja el correo como lo guarda iam.users.email: sin espacios al
// borde y en minúsculas. El CHECK de la base solo exige las minúsculas, así que
// esta función es la única que impide que 'a@x.pe ' y 'a@x.pe' sean dos usuarios.
func NormalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// EmailLenOK dice si un correo ya normalizado cumple el largo del contrato.
func EmailLenOK(email string) bool {
	n := utf8.RuneCountInString(email)
	return n >= MinEmailLen && n <= MaxEmailLen
}

// EmailHasNUL dice si el correo trae un NUL: PostgreSQL no lo admite en un text y la consulta fallaría con un 500
// en vez de un 422.
func EmailHasNUL(email string) bool { return strings.ContainsRune(email, 0) }

// PasswordLenOK dice si la contraseña cumple el largo del contrato.
func PasswordLenOK(password string) bool {
	n := utf8.RuneCountInString(password)
	return n >= MinPasswordLen && n <= MaxPasswordLen
}
