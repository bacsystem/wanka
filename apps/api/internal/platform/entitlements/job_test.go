package entitlements_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
)

// denial extrae el *Denial de un error de CheckJob.
func denial(t *testing.T, err error) *entitlements.Denial {
	t.Helper()
	var d *entitlements.Denial
	if !errors.As(err, &d) {
		t.Fatalf("err = %v, want a *entitlements.Denial", err)
	}
	return d
}

// Criterio 10: un job de un tenant no corre su lógica con el módulo
// deshabilitado ni con la empresa cerrada; la misma comprobación que el middleware.
func TestJobsUseTheSameCheckAsTheMiddleware(t *testing.T) {
	e := newEnv(t)
	s := e.service(entitlements.Options{Clock: clock.NewFake(newFake().Now())})

	t.Run("module enabled runs", func(t *testing.T) {
		if err := s.CheckJob(ctx(t), e.seeded("ventas"), entitlements.Job{Module: "ventas"}); err != nil {
			t.Fatalf("CheckJob = %v", err)
		}
	})
	t.Run("module disabled does not run", func(t *testing.T) {
		d := denial(t, s.CheckJob(ctx(t), e.seeded("ventas"), entitlements.Job{Module: "rubro.veterinaria"}))
		if d.Code != entitlements.CodeModuleNotEnabled {
			t.Fatalf("code = %s", d.Code)
		}
	})
	t.Run("closed company does not run, not even a job that finishes issued documents", func(t *testing.T) {
		tenant := e.tenant("cerrada")
		e.subscribe(tenant, e.plan("p-closed", `{}`, "ventas"), "activa")
		for _, j := range []entitlements.Job{{Module: "ventas"}, {Module: entitlements.NoModule, FinishesIssued: true}} {
			if d := denial(t, s.CheckJob(ctx(t), tenant, j)); d.Code != entitlements.CodeReadOnly {
				t.Fatalf("job %+v: code = %s", j, d.Code)
			}
		}
	})
	t.Run("inactive subscription: new work stops, finishing issued documents continues", func(t *testing.T) {
		tenant := e.tenant("activa")
		e.subscribe(tenant, e.plan("p-idle", `{}`, "facturacion-electronica"), "morosa")
		if d := denial(t, s.CheckJob(ctx(t), tenant, entitlements.Job{Module: "facturacion-electronica"})); d.Code != entitlements.CodeSubscriptionIdle {
			t.Fatalf("code = %s", d.Code)
		}
		// El módulo no está habilitado con la suscripción inactiva (ningún módulo lo está), así
		// que el envío de un CPE ya emitido se declara sin módulo: el job de Khipu es del kernel.
		if err := s.CheckJob(ctx(t), tenant, entitlements.Job{Module: entitlements.NoModule, FinishesIssued: true}); err != nil {
			t.Fatalf("a job that finishes issued documents was stopped: %v", err)
		}
	})
	t.Run("finishing issued documents with a module is a programming error, and the module would be rejected anyway", func(t *testing.T) {
		tenant := e.tenant("activa")
		e.subscribe(tenant, e.plan("p-idle2", `{}`, "facturacion-electronica"), "suspendida")
		err := s.CheckJob(ctx(t), tenant, entitlements.Job{Module: "facturacion-electronica", FinishesIssued: true})
		if !errors.Is(err, entitlements.ErrFinishesNeedsNoModule) {
			t.Fatalf("err = %v, want ErrFinishesNeedsNoModule", err)
		}
		// Con una suscripción activa tampoco se acepta: el error no depende del estado.
		if err := s.CheckJob(ctx(t), e.seeded("ventas"), entitlements.Job{Module: "ventas", FinishesIssued: true}); !errors.Is(err, entitlements.ErrFinishesNeedsNoModule) {
			t.Fatalf("active: err = %v", err)
		}
	})
	t.Run("a job must declare its module", func(t *testing.T) {
		if err := s.CheckJob(ctx(t), e.seeded("ventas"), entitlements.Job{}); err == nil {
			t.Fatal("a job without a module ran")
		}
	})
	t.Run("an unreadable entitlement keeps the job from running", func(t *testing.T) {
		err := s.CheckJob(ctx(t), pgtype.UUID{}, entitlements.Job{Module: entitlements.NoModule})
		if !errors.Is(err, db.ErrNoTenant) {
			t.Fatalf("err = %v, want db.ErrNoTenant", err)
		}
		var d *entitlements.Denial
		if errors.As(err, &d) {
			t.Fatal("a read error is not a denial")
		}
	})
}
