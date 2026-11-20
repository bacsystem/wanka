package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/jobs"
	"github.com/bacsystem/wanka/apps/api/internal/platform/mailer"
)

// LockoutNoticeKind es el Kind estable del job que avisa del bloqueo al titular.
const LockoutNoticeKind = "identidad.login_lockout_notice"

// LockoutNoticeArgs son los argumentos del aviso: solo el usuario y el fin del bloqueo, sin correo ni
// secretos (la tabla de jobs se ve desde la consola). El job es único por los dos campos.
type LockoutNoticeArgs struct {
	UserID      string    `json:"user_id"`
	LockedUntil time.Time `json:"locked_until"`
	// Closed: el aviso es el del cierre del ingreso con contraseña (100 fallos seguidos), sin fin de bloqueo.
	Closed bool `json:"closed,omitempty"`
}

// Kind es el nombre estable del job.
func (LockoutNoticeArgs) Kind() string { return LockoutNoticeKind }

// EmailDirectory da el correo de un usuario al worker, que no lo recibe en los argumentos del job.
type EmailDirectory interface {
	// EmailOf devuelve ErrUserNotFound si el usuario ya no existe.
	EmailOf(ctx context.Context, userID string) (string, error)
}

// ErrUserNotFound: el usuario del job no existe; reintentar no lo arregla.
var ErrUserNotFound = errors.New("identidad: user not found")

// lima es la zona en que se muestra la hora del fin del bloqueo (Perú no tiene horario de verano).
var lima = time.FixedZone("America/Lima", -5*3600)

// LockoutNotice es el handler del job: manda el aviso del bloqueo por el puerto Mailer. El aviso no lleva
// enlaces ni secretos (conventions.md §2), así que puede viajar por el outbox.
func LockoutNotice(dir EmailDirectory, m mailer.Mailer) func(ctx context.Context, job jobs.Job[LockoutNoticeArgs]) error {
	return func(ctx context.Context, job jobs.Job[LockoutNoticeArgs]) error {
		to, err := dir.EmailOf(ctx, job.Args.UserID)
		if errors.Is(err, ErrUserNotFound) {
			return jobs.Cancel(err)
		}
		if err != nil {
			return fmt.Errorf("reading the recipient: %w", err)
		}
		msg := mailer.Message{
			To:      to,
			Subject: "Bloqueamos temporalmente el ingreso a tu cuenta de Wanka",
			TextBody: "Hubo varios intentos fallidos de iniciar sesión en tu cuenta. Por seguridad, bloqueamos el ingreso hasta el " +
				job.Args.LockedUntil.In(lima).Format("02/01/2006 15:04") + " (hora de Lima).\n\nSi no fuiste tú, alguien tiene tu contraseña: restablécela desde la pantalla de ingreso de Wanka.\n\n" +
				"Wanka nunca te pedirá tu contraseña por teléfono, WhatsApp ni chat.\n",
			Tags:           []string{"login_lockout"},
			IdempotencyKey: fmt.Sprintf("login-lockout:%s:%d", job.Args.UserID, job.Args.LockedUntil.Unix()),
		}
		if job.Args.Closed {
			msg.Subject = "Cerramos el ingreso con contraseña a tu cuenta de Wanka"
			msg.TextBody = "Hubo 100 intentos fallidos seguidos de iniciar sesión en tu cuenta. Por seguridad, cerramos el ingreso con contraseña.\n\n" +
				"Alguien tiene tu contraseña. Para volver a entrar, restablécela desde la pantalla de ingreso de Wanka.\n\n" +
				"Wanka nunca te pedirá tu contraseña por teléfono, WhatsApp ni chat.\n"
			msg.Tags = []string{"login_closed"}
			msg.IdempotencyKey = "login-closed:" + job.Args.UserID
		}
		switch err := m.Send(ctx, msg); {
		case err == nil:
			return nil
		case errors.Is(err, mailer.ErrInvalidMessage), errors.Is(err, mailer.ErrDisabled):
			return jobs.Cancel(err) // ningún reintento lo arregla
		default:
			return fmt.Errorf("sending the lockout notice: %w", err)
		}
	}
}
