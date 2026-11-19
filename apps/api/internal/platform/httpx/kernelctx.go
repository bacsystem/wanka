package httpx

import (
	"context"
	"log/slog"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
)

// Fases de una petición para el log y el contador de statement_timeout
// (conventions.md §4). Sin fijar, la fase es PhaseHandler.
const (
	PhaseHandler       = "handler"
	PhaseSessionLookup = "session_lookup"
	PhaseEntitlements  = "entitlements"
	PhaseAuthz         = "authz"
	PhaseExport        = "export"
	// PhaseCommit la fija WriteDBError al perderse la conexión durante el COMMIT.
	PhaseCommit = "commit"
)

type ctxKey int

const (
	depsKey ctxKey = iota
	phaseKey
	moduleKey
)

// deps es lo que Chain deja en el contexto para que WriteProblem, WriteDBError
// y los middlewares registren sin recibir un logger en cada llamada.
type deps struct {
	log     *slog.Logger
	metrics metrics.Recorder
	start   time.Time // llegada de la petición
}

func withDeps(ctx context.Context, d deps) context.Context { return context.WithValue(ctx, depsKey, d) }

// depsFrom devuelve lo fijado por Chain; fuera de Chain, el logger por defecto
// y métricas nulas, para no perder el log ni caer.
func depsFrom(ctx context.Context) deps {
	if d, ok := ctx.Value(depsKey).(deps); ok {
		return d
	}
	return deps{log: slog.Default(), metrics: metrics.Nop{}, start: time.Now()}
}

// WithPhase marca la fase de la petición (PhaseSessionLookup, ...) para el log
// de un statement_timeout.
func WithPhase(ctx context.Context, phase string) context.Context {
	return context.WithValue(ctx, phaseKey, phase)
}

// WithModule marca el módulo que atiende la petición, para el mismo log.
func WithModule(ctx context.Context, module string) context.Context {
	return context.WithValue(ctx, moduleKey, module)
}

func phaseFrom(ctx context.Context) string {
	if p, ok := ctx.Value(phaseKey).(string); ok && p != "" {
		return p
	}
	return PhaseHandler
}

func moduleFrom(ctx context.Context) string {
	m, _ := ctx.Value(moduleKey).(string)
	return m
}
