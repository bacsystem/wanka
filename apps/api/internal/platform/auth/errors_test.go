package auth_test

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

// statementTimeout es el error que devuelve InTenantTx cuando la base cancela
// una sentencia por statement_timeout.
func statementTimeout() error {
	return fmt.Errorf("running query: %w: %w", db.ErrStatementTimeout,
		&pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout: SELECT secret_column"})
}
