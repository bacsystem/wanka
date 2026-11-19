package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

// Trabajos de mantenimiento del kernel, en la cola maintenance.
const (
	// KindPurgeIdempotencyKeys es el job horario que borra ops.idempotency_keys vencidas.
	KindPurgeIdempotencyKeys = "ops.purge_idempotency_keys"

	// IdempotencyRetention es la retención mínima de una clave (data-model §5). Una vencida que el job aún no
	// borró se sigue repitiendo. La política purge_expired de la tabla fija el mismo piso de 24 h.
	IdempotencyRetention = 24 * time.Hour

	purgeEvery = time.Hour
	// purgeBatch acota cada DELETE: un lote es una transacción corta y no retiene locks mucho tiempo.
	purgeBatch = 1000
)

type purgeIdempotencyArgs struct{}

func (purgeIdempotencyArgs) Kind() string { return KindPurgeIdempotencyKeys }

// RegisterMaintenance registra los jobs periódicos del kernel: hoy, la limpieza horaria de las claves de
// idempotencia. Los registra una vez el comando worker; con N réplicas corre una sola vez por ciclo (líder de River).
func RegisterMaintenance(r *Registry, pool *db.Pool) error {
	err := Register(r, func(ctx context.Context, _ Job[purgeIdempotencyArgs]) error {
		_, _, err := purgeIdempotencyKeys(ctx, pool, purgeBatch)
		return err
	})
	if err != nil {
		return err
	}
	return r.AddPeriodic(purgeEvery, func() Args { return purgeIdempotencyArgs{} }, WithQueue(QueueMaintenance), WithUnique())
}

// purgeIdempotencyKeys borra, en lotes de batch filas y una transacción por lote, las claves de más de
// IdempotencyRetention de todos los tenants. Devuelve cuántas borró y en cuántos lotes. Corre como wanka_worker
// sin tenant: la política purge_expired de la tabla le deja ver y borrar solo filas vencidas, y el GRANT solo le
// deja leer created_at, así que el lote se corta por esa columna (con empates puede pasarse de batch; no importa).
// Si se interrumpe, lo borrado queda borrado y el siguiente ciclo sigue: es idempotente.
func purgeIdempotencyKeys(ctx context.Context, pool *db.Pool, batch int) (deleted int64, batches int, err error) {
	for {
		var n int64
		err := pool.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
			tag, err := tx.Exec(ctx, `
				WITH cutoff AS (SELECT now() - make_interval(secs => $1) AS at)
				DELETE FROM ops.idempotency_keys
				WHERE created_at < (SELECT at FROM cutoff)
				  AND created_at <= coalesce((SELECT created_at FROM ops.idempotency_keys
				                              WHERE created_at < (SELECT at FROM cutoff)
				                              ORDER BY created_at OFFSET $2 - 1 LIMIT 1), 'infinity')`,
				IdempotencyRetention.Seconds(), batch)
			n = tag.RowsAffected()
			return err
		})
		if err != nil {
			return deleted, batches, fmt.Errorf("purging idempotency keys: %w", err)
		}
		deleted += n
		batches++
		if n < int64(batch) {
			return deleted, batches, nil
		}
	}
}
