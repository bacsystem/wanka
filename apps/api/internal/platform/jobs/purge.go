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

	// KindPurgeUserTokens y KindPurgeMFAEnrollments son los jobs diarios que borran los tokens de un solo uso y los de
	// enrolamiento vencidos (data-model §5, identidad global de la ola 1, #231).
	KindPurgeUserTokens     = "iam.purge_user_tokens"
	KindPurgeMFAEnrollments = "iam.purge_mfa_enrollments"

	// UserTokenRetention y MFAEnrollmentRetention cuentan desde expires_at: un token consumido vence a lo sumo 7 días
	// después de nacer, así que borrar por expires_at + 7 días cubre «vencidos o consumidos + 7 días», y wanka_worker
	// solo puede leer id y expires_at (nunca el hash ni consumed_at).
	UserTokenRetention     = 7 * 24 * time.Hour
	MFAEnrollmentRetention = 24 * time.Hour

	// IdempotencyRetention es la retención mínima de una clave (data-model §5). Una vencida que el job aún no
	// borró se sigue repitiendo. La política purge_expired de la tabla fija el mismo piso de 24 h.
	IdempotencyRetention = 24 * time.Hour

	purgeEvery      = time.Hour
	purgeDailyEvery = 24 * time.Hour
	// purgeBatch acota cada DELETE: un lote es una transacción corta y no retiene locks mucho tiempo.
	purgeBatch = 1000
)

type purgeIdempotencyArgs struct{}

func (purgeIdempotencyArgs) Kind() string { return KindPurgeIdempotencyKeys }

type purgeUserTokensArgs struct{}

func (purgeUserTokensArgs) Kind() string { return KindPurgeUserTokens }

type purgeMFAEnrollmentsArgs struct{}

func (purgeMFAEnrollmentsArgs) Kind() string { return KindPurgeMFAEnrollments }

// RegisterMaintenance registra los jobs periódicos del kernel: la limpieza horaria de las claves de idempotencia y
// la diaria de los tokens de identidad vencidos. Los registra una vez el comando worker; con N réplicas corre una
// sola vez por ciclo (líder de River).
func RegisterMaintenance(r *Registry, pool *db.Pool) error {
	err := Register(r, func(ctx context.Context, _ Job[purgeIdempotencyArgs]) error {
		_, _, err := purgeIdempotencyKeys(ctx, pool, purgeBatch)
		return err
	})
	if err != nil {
		return err
	}
	if err := r.AddPeriodic(purgeEvery, func() Args { return purgeIdempotencyArgs{} }, WithQueue(QueueMaintenance), WithUnique()); err != nil {
		return err
	}
	err = Register(r, func(ctx context.Context, _ Job[purgeUserTokensArgs]) error {
		_, err := purgeUserTokens(ctx, pool, purgeBatch)
		return err
	})
	if err != nil {
		return err
	}
	if err := r.AddPeriodic(purgeDailyEvery, func() Args { return purgeUserTokensArgs{} }, WithQueue(QueueMaintenance), WithUnique(), WithRunOnStart()); err != nil {
		return err
	}
	err = Register(r, func(ctx context.Context, _ Job[purgeMFAEnrollmentsArgs]) error {
		_, err := purgeMFAEnrollments(ctx, pool, purgeBatch)
		return err
	})
	if err != nil {
		return err
	}
	return r.AddPeriodic(purgeDailyEvery, func() Args { return purgeMFAEnrollmentsArgs{} }, WithQueue(QueueMaintenance), WithUnique(), WithRunOnStart())
}

// purgeUserTokens y purgeMFAEnrollments son el trabajo real de cada job, con su retención fija: el handler registrado
// y los tests llaman a estas funciones, nunca a purgeExpired con una retención propia.
func purgeUserTokens(ctx context.Context, pool *db.Pool, batch int) (int64, error) {
	return purgeExpired(ctx, pool, "iam.user_tokens", UserTokenRetention, batch)
}

func purgeMFAEnrollments(ctx context.Context, pool *db.Pool, batch int) (int64, error) {
	return purgeExpired(ctx, pool, "iam.mfa_enrollments", MFAEnrollmentRetention, batch)
}

// purgeTables son las únicas tablas que purgeExpired acepta: el nombre va en el SQL, así que nunca viene de afuera.
var purgeTables = map[string]bool{"iam.user_tokens": true, "iam.mfa_enrollments": true}

// purgeExpired borra, en lotes de batch filas y una transacción por lote, las filas de table cuyo expires_at pasó hace
// más de retention. Son tablas de identidad global (sin tenant ni RLS): wanka_worker solo tiene DELETE y SELECT de
// (id, expires_at). Devuelve cuántas borró. Si se interrumpe, lo borrado queda borrado y el siguiente ciclo sigue.
func purgeExpired(ctx context.Context, pool *db.Pool, table string, retention time.Duration, batch int) (deleted int64, err error) {
	if !purgeTables[table] {
		return 0, fmt.Errorf("purging %q: table is not purgeable", table)
	}
	for {
		var n int64
		err := pool.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
			tag, err := tx.Exec(ctx, `
				DELETE FROM `+table+` WHERE id IN (
				  SELECT id FROM `+table+` WHERE expires_at < now() - make_interval(secs => $1) ORDER BY expires_at LIMIT $2)`,
				retention.Seconds(), batch)
			n = tag.RowsAffected()
			return err
		})
		if err != nil {
			return deleted, fmt.Errorf("purging %s: %w", table, err)
		}
		deleted += n
		if n < int64(batch) {
			return deleted, nil
		}
	}
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
