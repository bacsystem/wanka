package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/audit"
	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/jobs"
)

// ensurePartitionsEvery es el período del job audit.ensure_partitions. Es
// diario y no mensual a propósito: River reinicia el ciclo de un periódico
// cuando cambia el líder o se reinicia el worker, así que uno de 30 días podría
// no correr nunca. El job es idempotente: 29 de cada 30 días no hace nada.
const ensurePartitionsEvery = 24 * time.Hour

// registerKernelJobs registra los jobs del kernel en el Registry del worker.
func registerKernelJobs(reg *jobs.Registry, pool *db.Pool, clk clock.Clock, log *slog.Logger) error {
	err := jobs.Register(reg, func(ctx context.Context, _ jobs.Job[audit.EnsurePartitionsArgs]) error {
		n, err := audit.EnsurePartitions(ctx, pool, clk.Now(), audit.PartitionsAhead)
		if err != nil {
			return err
		}
		log.Info("audit partitions ensured", "created", n)
		return nil
	})
	if err != nil {
		return err
	}
	if err := reg.AddPeriodic(ensurePartitionsEvery, func() jobs.Args { return audit.EnsurePartitionsArgs{} },
		jobs.WithQueue(jobs.QueueMaintenance), jobs.WithUnique()); err != nil {
		return fmt.Errorf("scheduling %s: %w", audit.EnsurePartitionsKind, err)
	}
	return nil
}
