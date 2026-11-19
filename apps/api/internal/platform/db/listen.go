package db

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
)

// Espera entre reintentos de la conexión de Listen: crece al doble hasta el tope, con jitter.
const (
	listenRetryMin = 200 * time.Millisecond
	listenRetryMax = 10 * time.Second
)

// ListenHooks son los avisos opcionales de Listen.
type ListenHooks struct {
	// OnConnect se llama cada vez que el LISTEN queda activo, también tras una
	// reconexión. Lo que ocurrió mientras no había conexión no se notificó: quien
	// cachea debe descartar todo lo que tenga.
	OnConnect func()
	// OnError recibe el error que cortó la conexión, ya sin la cadena de conexión.
	OnError func(error)
}

// Listen escucha channel (LISTEN) en una conexión propia, fuera del pool: no le
// quita un lugar a las peticiones. Llama a onNotify con el payload de cada
// NOTIFY, en la goroutine de Listen y de uno en uno. Bloquea hasta que ctx se
// cancela (devuelve nil); quien lo llama es el dueño de esa goroutine y la
// termina cancelando ctx. Si la conexión se pierde reintenta con backoff
// exponencial y jitter, sin tope de intentos.
//
// Un NOTIFY solo llega a quien está escuchando en ese momento: es una
// optimización de la caché, no su garantía (la garantía es el TTL).
func (p *Pool) Listen(ctx context.Context, channel string, onNotify func(payload string), hooks ListenHooks) error {
	wait := listenRetryMin
	for ctx.Err() == nil {
		connected, err := p.listenOnce(ctx, channel, onNotify, hooks.OnConnect)
		if ctx.Err() != nil {
			return nil
		}
		if connected {
			wait = listenRetryMin // una conexión que sí funcionó reinicia el backoff
		}
		if hooks.OnError != nil {
			hooks.OnError(err)
		}
		// Jitter completo: entre wait/2 y wait, para que las réplicas no reconecten a la vez.
		d := wait/2 + rand.N(wait/2+1)
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
		}
		wait = min(wait*2, listenRetryMax)
	}
	return nil
}

// listenOnce abre la conexión, escucha hasta que falla o ctx termina y la cierra.
// connected dice si el LISTEN llegó a quedar activo.
func (p *Pool) listenOnce(ctx context.Context, channel string, onNotify func(string), onConnect func()) (connected bool, err error) {
	cfg := p.pool.Config().ConnConfig.Copy()
	cfg.RuntimeParams["application_name"] += "-listen"
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return false, fmt.Errorf("%w: connecting listener: %w", ErrUnavailable, scrubbed(p.scrub, err))
	}
	defer func() {
		// El contexto del llamador ya puede estar cancelado: cerrar no depende de él.
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()
		_ = conn.Close(cctx)
	}()
	if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{channel}.Sanitize()); err != nil {
		return false, fmt.Errorf("listening on %s: %w", channel, scrubbed(p.scrub, err))
	}
	if onConnect != nil {
		onConnect()
	}
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return true, fmt.Errorf("waiting for notification on %s: %w", channel, scrubbed(p.scrub, err))
		}
		onNotify(n.Payload)
	}
}
