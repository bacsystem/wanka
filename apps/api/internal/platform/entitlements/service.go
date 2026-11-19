package entitlements

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

const (
	// DefaultTTL es la vigencia de una entrada de la caché (ADR 0005).
	DefaultTTL = 30 * time.Second
	// DefaultMaxEntries acota la caché: una entrada por empresa con sesión activa.
	DefaultMaxEntries = 10_000
	// Channel es el canal de LISTEN/NOTIFY que invalida la caché. El payload es
	// el UUID de la empresa afectada, o vacío si el cambio alcanza a todas.
	Channel = "entitlements_changed"
)

// errLoadAborted lo ve quien espera una lectura cuyo líder entró en pánico.
var errLoadAborted = errors.New("entitlements: load aborted")

// DB es lo que el servicio necesita de la base; lo cumple *db.Pool.
type DB interface {
	InTenantReadTx(ctx context.Context, a db.Actor, fn func(pgx.Tx) error) error
	Listen(ctx context.Context, channel string, onNotify func(payload string), hooks db.ListenHooks) error
}

// Options ajusta un Service. El valor cero usa los valores por defecto.
type Options struct {
	Clock      clock.Clock
	Logger     *slog.Logger
	TTL        time.Duration
	MaxEntries int
}

type entry struct {
	ent     Entitlements
	expires time.Time
}

// call es una lectura en vuelo: las peticiones que llegan con la caché fría para
// la misma empresa esperan a la primera en vez de abrir una consulta cada una.
type call struct {
	done chan struct{}
	ent  Entitlements
	err  error
}

// Service lee los entitlements de una empresa y los guarda en memoria por
// tenant. Es seguro para uso concurrente.
type Service struct {
	db         DB
	clock      clock.Clock
	log        *slog.Logger
	ttl        time.Duration
	maxEntries int

	mu       sync.Mutex
	cache    map[[16]byte]entry
	inflight map[[16]byte]*call
	// epoch sube con cada invalidación. Una lectura que empezó antes de una
	// invalidación no se guarda: pudo leer los datos viejos.
	epoch uint64
}

// New arma el servicio sobre d. No abre goroutines: la invalidación por NOTIFY
// corre en Run, que arranca quien construye el servicio.
func New(d DB, o Options) *Service {
	s := &Service{
		db:         d,
		clock:      o.Clock,
		log:        o.Logger,
		ttl:        o.TTL,
		maxEntries: o.MaxEntries,
		cache:      map[[16]byte]entry{},
		inflight:   map[[16]byte]*call{},
	}
	if s.clock == nil {
		s.clock = clock.System{}
	}
	if s.log == nil {
		s.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if s.ttl <= 0 {
		s.ttl = DefaultTTL
	}
	if s.maxEntries <= 0 {
		s.maxEntries = DefaultMaxEntries
	}
	return s
}

// For devuelve los entitlements de la empresa: de la caché si la entrada tiene
// menos de TTL y nadie la invalidó, y si no con una sola consulta a la base.
//
// Un error de la base sale tipado (db.ErrOverloaded, db.ErrUnavailable,
// db.ErrStatementTimeout) y nunca se guarda: la siguiente petición vuelve a
// intentarlo. Una empresa sin suscripción no es un error: devuelve StatusNoPlan
// y ningún módulo. Un tenant vacío es db.ErrNoTenant.
func (s *Service) For(ctx context.Context, tenant pgtype.UUID) (Entitlements, error) {
	if err := ctx.Err(); err != nil {
		return Entitlements{}, err
	}
	if !tenant.Valid || tenant.Bytes == [16]byte{} {
		return Entitlements{}, db.ErrNoTenant
	}
	key := tenant.Bytes

	s.mu.Lock()
	if e, ok := s.cache[key]; ok {
		if s.clock.Now().Before(e.expires) {
			s.mu.Unlock()
			return e.ent.clone(), nil
		}
		delete(s.cache, key)
	}
	if c, ok := s.inflight[key]; ok {
		s.mu.Unlock()
		select {
		case <-c.done:
			if c.err != nil {
				return Entitlements{}, c.err
			}
			return c.ent.clone(), nil
		case <-ctx.Done():
			return Entitlements{}, ctx.Err()
		}
	}
	c := &call{done: make(chan struct{}), err: errLoadAborted}
	s.inflight[key] = c
	epoch := s.epoch
	s.mu.Unlock()

	// La lectura corre sin la cancelación de este llamador: otros pueden estar
	// esperándola. La acotan el statement_timeout del rol y el plazo de adquisición.
	ent, err := s.lead(context.WithoutCancel(ctx), tenant, key, c, epoch)
	if err != nil {
		return Entitlements{}, err
	}
	return ent.clone(), nil
}

// lead hace la lectura de c y publica su resultado, también si entra en pánico.
func (s *Service) lead(ctx context.Context, tenant pgtype.UUID, key [16]byte, c *call, epoch uint64) (Entitlements, error) {
	defer func() {
		s.mu.Lock()
		delete(s.inflight, key)
		if c.err == nil && s.epoch == epoch {
			s.store(key, entry{ent: c.ent, expires: s.clock.Now().Add(s.ttl)})
		}
		s.mu.Unlock()
		close(c.done)
	}()
	ent, err := load(ctx, s.db, tenant, s.clock.Now())
	c.ent, c.err = ent, err
	return ent, err
}

// store guarda e respetando el tope de entradas. Llamar con s.mu tomado.
func (s *Service) store(key [16]byte, e entry) {
	if _, ok := s.cache[key]; !ok && len(s.cache) >= s.maxEntries {
		now := s.clock.Now()
		for k, old := range s.cache {
			if !now.Before(old.expires) {
				delete(s.cache, k)
			}
		}
		for k := range s.cache { // sigue lleno: sale una entrada cualquiera
			if len(s.cache) < s.maxEntries {
				break
			}
			delete(s.cache, k)
		}
	}
	s.cache[key] = e
}

// Invalidate descarta la entrada de la empresa. Es lo que hace un NOTIFY con su
// UUID; sirve también a un proceso que cambia entitlements y no quiere esperar.
func (s *Service) Invalidate(tenant pgtype.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.epoch++
	delete(s.cache, tenant.Bytes)
}

// InvalidateAll descarta toda la caché.
func (s *Service) InvalidateAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.epoch++
	clear(s.cache)
}

// onNotify aplica un NOTIFY entitlements_changed. Un payload vacío o que no es
// un UUID invalida todo: ante la duda, se vuelve a leer.
func (s *Service) onNotify(payload string) {
	var u pgtype.UUID
	if payload == "" || u.Scan(payload) != nil {
		s.InvalidateAll()
		return
	}
	s.Invalidate(u)
}

// Run escucha entitlements_changed e invalida la caché hasta que ctx se
// cancela. Lo arranca el proceso que construye el servicio, en una goroutine
// suya que termina cancelando ctx. Al (re)conectar descarta toda la caché,
// porque no se vieron los avisos de mientras tanto. Sin Run la caché sigue
// siendo correcta, solo tarda hasta el TTL en enterarse de un cambio.
func (s *Service) Run(ctx context.Context) error {
	return s.db.Listen(ctx, Channel, s.onNotify, db.ListenHooks{
		OnConnect: s.InvalidateAll,
		OnError: func(err error) {
			s.log.Warn("entitlements listener lost its connection, retrying", "error", err)
		},
	})
}
