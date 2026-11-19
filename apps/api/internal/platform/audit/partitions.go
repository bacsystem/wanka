package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

// PartitionsAhead es cuántos meses después del actual deben tener partición: la
// migración crea el actual y los 3 siguientes, y el job lo mantiene así.
const PartitionsAhead = 3

// EnsurePartitionsKind es el Kind del job periódico (cola maintenance) que crea
// las particiones futuras de audit.events.
const EnsurePartitionsKind = "audit.ensure_partitions"

// EnsurePartitionsArgs son los argumentos del job: ninguno, no es de un tenant.
type EnsurePartitionsArgs struct{}

// Kind es el nombre estable del job.
func (EnsurePartitionsArgs) Kind() string { return EnsurePartitionsKind }

// noTenantRunner es lo que EnsurePartitions necesita del pool; *db.Pool lo cumple.
type noTenantRunner interface {
	InNoTenantTx(ctx context.Context, s db.NoTenantScope, fn func(pgx.Tx) error) error
}

// EnsurePartitions crea, si faltan, las particiones de audit.events del mes de
// now (en UTC) y de los ahead meses siguientes, y devuelve cuántas creó. Es
// idempotente: repetirla, o correrla desde dos workers a la vez, no falla ni
// duplica. Llama a audit.ensure_partitions (SECURITY DEFINER) con el rol
// wanka_worker: el worker no es dueño de la tabla y no puede hacer el DDL solo.
func EnsurePartitions(ctx context.Context, pool noTenantRunner, now time.Time, ahead int) (int, error) {
	var created int
	err := pool.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT audit.ensure_partitions($1::date, $2::int)`,
			now.UTC().Format("2006-01-02"), ahead).Scan(&created)
	})
	if err != nil {
		return 0, fmt.Errorf("ensuring audit.events partitions: %w", err)
	}
	return created, nil
}
