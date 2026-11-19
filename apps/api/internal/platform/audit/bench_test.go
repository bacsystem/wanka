package audit_test

import (
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/platform/audit"
)

// BenchmarkTxWithRecord y BenchmarkTxWithoutRecord miden el costo de Record sobre el camino de escritura (criterio no
// funcional: < 2 ms): la diferencia entre una InTenantTx que hace un cambio y la misma con el evento.
//
//	go test -run '^$' -bench 'BenchmarkTx' -benchtime 500x ./internal/platform/audit/
func BenchmarkTxWithoutRecord(b *testing.B) { benchTx(b, false) }
func BenchmarkTxWithRecord(b *testing.B)    { benchTx(b, true) }

func benchTx(b *testing.B, withRecord bool) {
	e := newEnv(b)
	ctx := testCtx(b)
	if _, err := e.d.Admin(ctx, b).Exec(ctx, `CREATE TABLE public.change (id int, note text); GRANT ALL ON public.change TO wanka_app`); err != nil {
		b.Fatal(err)
	}
	actor := userActor(b, tenantA, userA)
	evt := audit.Event{Action: "ventas.documento:update", EntityType: "sales.document",
		EntityID: uid(b, "99999999-0000-0000-0000-000000000001"), Diff: map[string]any{"status": map[string]string{"from": "borrador", "to": "emitido"}}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := e.app.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO public.change VALUES ($1, 'x')`, i); err != nil {
				return err
			}
			if withRecord {
				return e.w.Record(ctx, tx, evt)
			}
			return nil
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
