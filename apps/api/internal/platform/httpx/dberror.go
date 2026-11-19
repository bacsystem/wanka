package httpx

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
)

// WriteDBError responde el error de base de datos que es contrapresión o base
// caída (api/conventions.md §4), siempre con Retry-After: 1 y sin la causa:
//
//   - db.ErrOverloaded (pool sin conexión libre): 503 overloaded;
//   - db.ErrUnavailable (la base no responde o la conexión se perdió): 503
//     dependency_unavailable, con una línea warn y un contador igual que el
//     statement_timeout (fase commit si db.ErrCommitUncertain);
//   - db.ErrStatementTimeout (SQLSTATE 57014 con el ctx vivo): 503
//     dependency_unavailable con el mismo cuerpo que ErrUnavailable, más una
//     línea warn y un contador por fase y ruta (sin SQL ni mensaje de PostgreSQL).
//
// Si el contexto de la petición ya terminó (el cliente se fue o venció el plazo
// del handler) no escribe nada y devuelve true: la respuesta, si la hay, es la
// del middleware de plazo. Devuelve false para cualquier otro error, que
// resuelve quien llama (WriteError lo responde 500 internal).
func WriteDBError(w http.ResponseWriter, r *http.Request, err error) bool {
	ctx := r.Context()
	if ctx.Err() != nil {
		// db no tipa como ErrStatementTimeout la cancelación del ctx de la
		// petición (llega el 57014 crudo o el error del ctx): también es de base.
		var pgErr *pgconn.PgError
		return isDBError(err) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
			errors.As(err, &pgErr) && pgErr.Code == "57014"
	}
	if !isDBError(err) {
		return false
	}
	switch {
	case errors.Is(err, db.ErrOverloaded):
		WriteProblem(w, r, "overloaded", "the server is busy, retry shortly")
	case errors.Is(err, db.ErrStatementTimeout):
		logDBFailure(r, "statement timeout", metrics.StatementTimeouts, phaseFrom(ctx), "57014")
		WriteProblem(w, r, "dependency_unavailable", "a dependency is not responding")
	case errors.Is(err, db.ErrUnavailable):
		// Base caída o conexión perdida. Un COMMIT incierto tiene su propia fase:
		// la transacción pudo confirmarse y soporte tendrá que conciliarla.
		phase := phaseFrom(ctx)
		if errors.Is(err, db.ErrCommitUncertain) {
			phase = PhaseCommit
		}
		var pgErr *pgconn.PgError
		sqlstate := ""
		if errors.As(err, &pgErr) {
			sqlstate = pgErr.Code
		}
		logDBFailure(r, "database unavailable", metrics.DBUnavailable, phase, sqlstate)
		WriteProblem(w, r, "dependency_unavailable", "a dependency is not responding")
	}
	return true
}

// logDBFailure deja una línea warn y suma un contador por fase y ruta. Lleva la
// ruta, la fase, el módulo si lo hay, el sqlstate si lo hay y el tiempo
// transcurrido; nunca el mensaje de PostgreSQL ni el SQL.
func logDBFailure(r *http.Request, msg, counter, phase, sqlstate string) {
	ctx := r.Context()
	d := depsFrom(ctx)
	route := routeOf(r)
	attrs := []any{"route", route, "phase", phase, "elapsed_ms", time.Since(d.start).Milliseconds()}
	if sqlstate != "" {
		attrs = append(attrs, "sqlstate", sqlstate)
	}
	if m := moduleFrom(ctx); m != "" {
		attrs = append(attrs, "module", m)
	}
	d.log.WarnContext(ctx, msg, attrs...)
	d.metrics.Inc(counter, "phase", phase, "route", route)
}

func isDBError(err error) bool {
	return errors.Is(err, db.ErrOverloaded) || errors.Is(err, db.ErrUnavailable) || errors.Is(err, db.ErrStatementTimeout)
}

// routeOf es el patrón de la ruta que atiende r (por ejemplo "GET /v1/items/{id}"),
// nunca la URL: no lleva identificadores ni query.
func routeOf(r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	return "unmatched"
}
