package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Acquire expone a los tests de db_test la conexión cruda del pool, que fuera
// de este paquete no se puede obtener (ver el comentario de acquire).
func (p *Pool) Acquire(ctx context.Context) (*pgxpool.Conn, error) { return p.acquire(ctx) }

// TypedErr expone la clasificación de errores de transacción.
var TypedErr = typedErr

// OpenTraced abre un Pool cuyas conexiones reportan cada sentencia al tracer,
// para comprobar qué SQL manda una transacción y cuántas idas y vueltas cuesta.
func OpenTraced(ctx context.Context, rawURL string, tr pgx.QueryTracer) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(rawURL)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.Tracer = tr
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Pool{pool: pool, acquireTimeout: DefaultAcquireTimeout, scrub: scrubber(rawURL)}, nil
}
