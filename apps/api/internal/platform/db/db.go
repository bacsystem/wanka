// Package db abre el pool de conexiones PostgreSQL de un proceso y corre las
// migraciones goose (ADR 0002, ADR 0001).
//
// Cada proceso se conecta con el rol de su servicio (config.Service); el pool
// nunca crea roles ni es dueño de objetos. Los errores que pueden llevar la
// cadena de conexión se limpian antes de salir de este paquete.
//
// Los datos de tenant se tocan solo dentro de InTenantTx o InTenantReadTx; el
// pool no entrega conexiones crudas (acquire no es exportado). Esto no impide
// todo desvío: go-arch-lint prohíbe importar pgxpool y pgx/stdlib fuera de este
// paquete, y un test (rawconn_test.go) falla si el código de fuera llama a
// pgx.Connect*, pgconn.Connect*, pgxpool.New* o stdlib.Open*, también con alias.
// Siguen siendo posibles, y están prohibidos, guardar el tx.Conn() de fn para
// usarlo después de la transacción y cualquier otra vía que el test no vea; los
// atrapa la revisión de código.
//
// Dentro de fn está prohibido fijar las variables app.* a nivel de sesión
// (set_config con false o SET sin LOCAL): sobreviven al COMMIT en esa conexión.
// No filtran datos de otro tenant porque la siguiente InTenantTx vuelve a fijar
// las cuatro variables y el pool no entrega la conexión a nadie más, pero es un
// uso incorrecto igualmente.
package db

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Errores que la capa HTTP traduce (api/conventions.md §4).
var (
	// ErrOverloaded: no hubo conexión libre dentro del plazo de adquisición:
	// 503 overloaded, Retry-After: 1.
	ErrOverloaded = errors.New("db: no free connection")
	// ErrUnavailable: la base no respondió: 503 dependency_unavailable.
	ErrUnavailable = errors.New("db: database unavailable")
	// ErrCommitUncertain acompaña a ErrUnavailable cuando la conexión se perdió
	// durante el COMMIT: la transacción pudo confirmarse (reintentar solo es
	// seguro con la misma Idempotency-Key o con If-Match).
	ErrCommitUncertain = errors.New("db: commit outcome unknown")
)

const (
	// DefaultAcquireTimeout es cuánto espera una petición una conexión libre
	// antes de recibir ErrOverloaded (capacity.md: 200 ms, nunca encolar sin fin).
	DefaultAcquireTimeout = 200 * time.Millisecond

	pingTimeout = 2 * time.Second
)

// Options configura Open.
type Options struct {
	// MaxConns es el tamaño del pool; mínimo 1.
	MaxConns int32
	// AcquireTimeout acota la adquisición de conexión; cero significa DefaultAcquireTimeout.
	AcquireTimeout time.Duration
	// AppName se ve en pg_stat_activity (application_name).
	AppName string
}

// Pool es el pgxpool de un proceso.
type Pool struct {
	pool           *pgxpool.Pool
	acquireTimeout time.Duration
	scrub          *strings.Replacer
}

// Open construye el pool para rawURL. No conecta: un proceso cuya base está
// caída debe arrancar igual e informarlo por /readyz. Los errores nunca incluyen
// la URL ni su contraseña.
func Open(ctx context.Context, rawURL string, o Options) (*Pool, error) {
	scrub := scrubber(rawURL)
	if o.MaxConns < 1 {
		return nil, errors.New("db: pool size must be at least 1")
	}
	cfg, err := pgxpool.ParseConfig(rawURL)
	if err != nil {
		return nil, scrubbed(scrub, fmt.Errorf("parsing database URL: %w", err))
	}
	cfg.MaxConns = o.MaxConns
	cfg.MinConns = 0
	// Explícitos, no los de la biblioteca: las conexiones se reciclan para
	// tomar una contraseña rotada o una conmutación y detectar las muertas.
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnLifetimeJitter = 3 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	if o.AppName != "" {
		cfg.ConnConfig.RuntimeParams["application_name"] = o.AppName
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, scrubbed(scrub, fmt.Errorf("creating pool: %w", err))
	}
	timeout := o.AcquireTimeout
	if timeout <= 0 {
		timeout = DefaultAcquireTimeout
	}
	return &Pool{pool: pool, acquireTimeout: timeout, scrub: scrub}, nil
}

// acquire devuelve una conexión libre o falla rápido: ErrOverloaded si el pool
// está lleno durante todo el plazo, ErrUnavailable si la base no contesta. Un
// ctx cancelado devuelve su propio error. Quien llama debe hacer Release.
//
// No es exportado a propósito: una conexión cruda permitiría ejecutar SQL fuera
// de InTenantTx (por ejemplo fijar app.tenant_id a nivel de sesión), y
// go-arch-lint solo ve imports, no que un paquete reciba un *pgxpool.Conn.
func (p *Pool) acquire(ctx context.Context) (*pgxpool.Conn, error) {
	return p.acquireWithin(ctx, p.acquireTimeout)
}

func (p *Pool) acquireWithin(ctx context.Context, timeout time.Duration) (*pgxpool.Conn, error) {
	actx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := p.pool.Acquire(actx)
	if err == nil {
		return conn, nil
	}
	switch {
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case errors.Is(err, context.DeadlineExceeded) && p.full():
		return nil, ErrOverloaded
	default:
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, scrubbed(p.scrub, err))
	}
}

// full indica si todas las conexiones del pool están en uso.
func (p *Pool) full() bool {
	s := p.pool.Stat()
	return s.AcquiredConns() >= s.MaxConns()
}

// AcquiredConns es el número de conexiones en uso ahora mismo; sirve para
// comprobar que una petición cancelada devolvió la suya.
func (p *Pool) AcquiredConns() int32 { return p.pool.Stat().AcquiredConns() }

// Ping comprueba que la base responde; es el chequeo de readiness. Con todas
// las conexiones en uso responde ErrOverloaded al instante: /readyz da 503
// overloaded, igual que una petición sin conexión libre (conventions.md §4), y
// no espera 2 s ni culpa a una base sana. Si hay hueco, espera hasta pingTimeout
// por una conexión: un proceso sin tráfico se queda sin conexiones (MaxConnIdleTime)
// y abrir la primera puede tardar más que los 200 ms de una petición; sería un
// falso "no disponible". Se prefirió esto a MinConns = 1, que deja una ventana
// fría hasta 30 s tras reciclar la conexión y conecta en segundo plano al arrancar.
func (p *Pool) Ping(ctx context.Context) error {
	if p.full() {
		return ErrOverloaded
	}
	conn, err := p.acquireWithin(ctx, pingTimeout)
	if err != nil {
		return err
	}
	defer conn.Release()
	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := conn.Conn().Ping(ctx); err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, scrubbed(p.scrub, err))
	}
	return nil
}

// Close cierra todas las conexiones; llamarlo después de detener el servidor HTTP.
func (p *Pool) Close() { p.pool.Close() }

// scrubber reemplaza la URL completa y la contraseña en el texto de un error.
// La contraseña se redacta solo donde es un secreto (":clave@" dentro de una
// URL y "password=clave"), nunca como subcadena suelta: una clave corta como
// "x" mutilaría el resto del mensaje.
func scrubber(rawURL string) *strings.Replacer {
	pairs := []string{}
	if rawURL != "" {
		pairs = append(pairs, rawURL, "<database url>")
	}
	if u, err := url.Parse(rawURL); err == nil && u.User != nil {
		if pw, ok := u.User.Password(); ok && pw != "" {
			for _, form := range []string{pw, url.PathEscape(pw), url.QueryEscape(pw)} {
				pairs = append(pairs, ":"+form+"@", ":[REDACTED]@", "password="+form, "password=[REDACTED]")
			}
		}
	}
	return strings.NewReplacer(pairs...)
}

// scrubbedError conserva el original para errors.Is/As pero imprime un texto limpio.
type scrubbedError struct {
	msg string
	err error
}

func (e *scrubbedError) Error() string { return e.msg }
func (e *scrubbedError) Unwrap() error { return e.err }

func scrubbed(r *strings.Replacer, err error) error {
	return &scrubbedError{msg: r.Replace(err.Error()), err: err}
}
