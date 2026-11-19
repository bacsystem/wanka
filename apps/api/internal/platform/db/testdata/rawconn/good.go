package good

// Fixture de rawconn_test.go: usos permitidos fuera de db, no se detectan.
import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func fine(tx pgx.Tx, err error) bool {
	var pgErr *pgconn.PgError
	_ = tx
	return pgErr != nil
}
