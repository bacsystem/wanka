// Package domain son las reglas de identidad que no dependen de la
// infraestructura: la forma del correo y los topes del bloqueo progresivo del
// login (ADR 0004, nota del 2026-11-19).
package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Topes del login (ADR 0014 y su nota del 2026-11-20). Los mismos números están en el SQL de cobro del adaptador de
// PostgreSQL y en los CHECK de iam.users.
const (
	// MaxAttempts son los intentos cobrados por ventana: acota los que están en vuelo y descarta un cobro colgado.
	MaxAttempts = 5
	// Window es la ventana fija de cobro; solo se mueve cuando la anterior venció.
	Window = 15 * time.Minute
	// StreakPerLockout: cada 5 fallos seguidos, con cualquier espaciado, hay un bloqueo.
	StreakPerLockout = 5
	// MaxStreak son los fallos seguidos que cierran el ingreso con contraseña hasta restablecerla o cambiarla.
	MaxStreak = 100
	// BaseLock es el primer bloqueo; cada uno dura el doble que el anterior, hasta MaxLock.
	BaseLock = 15 * time.Minute
	MaxLock  = 24 * time.Hour
	// MaxLockoutsCounted es el tope de iam.users.login_lockouts (CHECK 0..10).
	MaxLockoutsCounted = 10
	// MaxMFALockouts: con 3 bloqueos del segundo paso, el login con la contraseña
	// correcta responde igual que con una mala (conventions.md §2).
	MaxMFALockouts = 3
)

// Lockout es lo que corresponde tras un fallo cobrado con la racha dada.
type Lockout struct {
	// None: la racha no es múltiplo de 5, no pasa nada.
	None bool
	// Closed: la racha llegó a MaxStreak, el ingreso con contraseña queda cerrado.
	Closed bool
	// N es el número del bloqueo (racha / 5) y Counted el valor de login_lockouts (tope 10).
	N, Counted int
	// For es lo que dura el bloqueo: 15 min por 2^(N-1), con tope de 24 h.
	For time.Duration
}

// LockoutFor decide qué pasa tras un fallo cobrado con la racha streak (que ya cuenta ese fallo).
func LockoutFor(streak int) Lockout {
	if streak <= 0 || streak%StreakPerLockout != 0 {
		return Lockout{None: true}
	}
	n := streak / StreakPerLockout
	d := MaxLock
	if n-1 < 7 { // 15 min x 2^6 = 16 h; el siguiente (32 h) ya pasa el tope
		d = min(BaseLock<<(n-1), MaxLock)
	}
	return Lockout{Closed: streak >= MaxStreak, N: n, Counted: min(n, MaxLockoutsCounted), For: d}
}

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
