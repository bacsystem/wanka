// Package stub es el adaptador de prueba del puerto mailer: no envía nada.
// Guarda los mensajes en una bandeja en memoria (Sent) para que los tests de
// identidad comprueben «se envió la invitación», y registra solo el destinatario
// enmascarado y el asunto: nunca el cuerpo, los enlaces, las etiquetas ni la
// llave de idempotencia. Los módulos importan mailer, no este paquete; solo
// el arranque del proceso y los tests lo construyen.
package stub

import (
	"context"
	"log/slog"
	"slices"
	"sync"

	"github.com/bacsystem/wanka/apps/api/internal/platform/mailer"
)

// DefaultMaxSent acota la bandeja: el stub puede correr en producción hasta que
// exista un adaptador real y no debe crecer sin límite.
const DefaultMaxSent = 1000

// Stub implementa mailer.Mailer. Es seguro para varias goroutines.
type Stub struct {
	log *slog.Logger
	max int

	mu   sync.Mutex
	sent []mailer.Message
	keys map[string]struct{} // llaves de los mensajes que siguen en la bandeja
}

// New devuelve un Stub que registra en log y guarda hasta max mensajes
// (max < 1 usa DefaultMaxSent); al llenarse descarta el más antiguo.
func New(log *slog.Logger, max int) *Stub {
	if max < 1 {
		max = DefaultMaxSent
	}
	return &Stub{log: log, max: max, keys: make(map[string]struct{})}
}

// La deduplicación por IdempotencyKey solo vale mientras el mensaje siga en la
// bandeja: al salir de ella (más de max envíos después) su llave se olvida y un
// reenvío con esa llave se registra otra vez. Es un stub, no un registro durable.
//
// Send valida el mensaje, lo guarda y lo registra. Con una IdempotencyKey ya
// vista no guarda ni registra otro envío y devuelve nil.
func (s *Stub) Send(ctx context.Context, m mailer.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := mailer.Validate(m); err != nil {
		return err
	}
	m.Tags = slices.Clone(m.Tags) // el llamador sigue siendo dueño de su slice

	s.mu.Lock()
	if m.IdempotencyKey != "" {
		if _, dup := s.keys[m.IdempotencyKey]; dup {
			s.mu.Unlock()
			s.log.DebugContext(ctx, "mail already sent (stub), skipped")
			return nil
		}
		s.keys[m.IdempotencyKey] = struct{}{}
	}
	s.sent = append(s.sent, m)
	if len(s.sent) > s.max {
		delete(s.keys, s.sent[0].IdempotencyKey)
		s.sent = s.sent[1:]
	}
	s.mu.Unlock()

	// Solo estos dos campos salen del mensaje hacia el log.
	s.log.InfoContext(ctx, "mail sent (stub)", "recipient", mailer.Mask(m.To), "subject", m.Subject)
	return nil
}

// Sent devuelve una copia de la bandeja, del más antiguo al más reciente.
func (s *Stub) Sent() []mailer.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]mailer.Message, len(s.sent))
	for i, m := range s.sent {
		m.Tags = slices.Clone(m.Tags)
		out[i] = m
	}
	return out
}
