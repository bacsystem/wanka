package db_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// La sobrecarga de InTenantTx es la diferencia entre estos tres: BenchmarkTxBaseline
// hace BEGIN + la consulta + COMMIT sin variables (lo que costaría cualquier
// transacción), y BenchmarkInTenantTx añade el set_config. Para capacity.md:
//
//	go test -run '^$' -bench 'Tx' -count 10 ./internal/platform/db
func benchPool(b *testing.B) *db.Pool {
	b.Helper()
	d := dbtest.New(b)
	d.Migrate(context.Background(), b)
	p, err := db.Open(context.Background(), d.URL(dbtest.App), db.Options{MaxConns: 1})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(p.Close)
	return p
}

func BenchmarkAutocommitQuery(b *testing.B) {
	p := benchPool(b)
	ctx := context.Background()
	for b.Loop() {
		conn, err := p.Acquire(ctx)
		if err != nil {
			b.Fatal(err)
		}
		var n int
		err = conn.QueryRow(ctx, `SELECT 1`).Scan(&n)
		conn.Release()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTxBaseline(b *testing.B) {
	p := benchPool(b)
	ctx := context.Background()
	for b.Loop() {
		conn, err := p.Acquire(ctx)
		if err != nil {
			b.Fatal(err)
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			b.Fatal(err)
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT 1`).Scan(&n); err != nil {
			b.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			b.Fatal(err)
		}
		conn.Release()
	}
}

func BenchmarkInTenantTx(b *testing.B) {
	p := benchPool(b)
	ctx := context.Background()
	a := actor(b, tenantA, db.KindUser)
	for b.Loop() {
		err := p.InTenantTx(ctx, a, func(tx pgx.Tx) error {
			var n int
			return tx.QueryRow(ctx, `SELECT 1`).Scan(&n)
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
