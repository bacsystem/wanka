package bad

// Fixture de rawconn_test.go: no se compila (vive en testdata). Cada línea
// marcada con "want" debe ser detectada aunque el paquete tenga alias.
import (
	pg "github.com/jackc/pgx/v5"
	pgc "github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	sd "github.com/jackc/pgx/v5/stdlib"
)

func bad() {
	_, _ = pg.Connect(nil, "")             // want
	_, _ = pg.ConnectConfig(nil, nil)      // want
	_, _ = pgc.Connect(nil, "")            // want
	_, _ = pgc.ConnectConfig(nil, nil)     // want
	_, _ = pgxpool.New(nil, "")            // want
	_, _ = pgxpool.NewWithConfig(nil, nil) // want
	_ = sd.OpenDB                          // want
	_ = sd.OpenDBFromPool                  // want
}
