package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Acquire expone a los tests de db_test la conexión cruda del pool, que fuera
// de este paquete no se puede obtener (ver el comentario de acquire).
func (p *Pool) Acquire(ctx context.Context) (*pgxpool.Conn, error) { return p.acquire(ctx) }

// TypedErr expone la clasificación de errores de transacción.
var TypedErr = typedErr
