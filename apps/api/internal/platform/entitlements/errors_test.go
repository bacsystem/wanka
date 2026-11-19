package entitlements_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
)

// Una base caída da un error tipado y NUNCA un resultado que abra el acceso ni uno guardado.
func TestUnavailableDatabaseGivesATypedErrorAndOpensNothing(t *testing.T) {
	e := newEnv(t)
	tenant := e.seeded("ventas")
	// Un puerto en el que no escucha nadie.
	dead, err := db.Open(context.Background(), "postgres://wanka_app:x@127.0.0.1:1/wanka", db.Options{MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(dead.Close)
	s := entitlements.New(dead, entitlements.Options{})

	for range 2 { // dos veces: el fallo no se guarda como resultado
		got, err := s.For(ctx(t), tenant)
		if !errors.Is(err, db.ErrUnavailable) {
			t.Fatalf("err = %v, want db.ErrUnavailable", err)
		}
		if len(got.Modules) != 0 || len(got.Flags) != 0 || got.Status != "" || got.Version != "" {
			t.Errorf("an error came with a non-zero result: %+v", got)
		}
	}
}

// Pool lleno: ErrOverloaded (el middleware responde 503 overloaded).
func TestFullPoolGivesErrOverloaded(t *testing.T) {
	e := newEnv(t)
	tenant := e.seeded("ventas")
	p, err := db.Open(context.Background(), e.d.URL(dbtest.App), db.Options{MaxConns: 1, AcquireTimeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	hold, release := make(chan struct{}), make(chan struct{})
	go func() {
		_ = p.InNoTenantTx(context.Background(), db.NoTenantScope{}, func(pgx.Tx) error { close(hold); <-release; return nil })
	}()
	<-hold
	defer close(release)

	_, err = entitlements.New(p, entitlements.Options{}).For(ctx(t), tenant)
	if !errors.Is(err, db.ErrOverloaded) {
		t.Errorf("err = %v, want db.ErrOverloaded", err)
	}
}

// Una lectura que la base cancela por statement_timeout (57014) sale como db.ErrStatementTimeout y no se guarda.
func TestStatementTimeoutIsTypedAndNotCached(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for the 5 s statement_timeout of wanka_app")
	}
	e := newEnv(t)
	tenant := e.seeded("ventas")
	s := e.service(entitlements.Options{})

	// Otro cliente tiene la tabla bloqueada: la lectura espera hasta que el rol la cancela.
	locker, err := pgx.Connect(ctx(t), e.d.SuperuserURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = locker.Close(context.Background()) })
	tx, err := locker.Begin(ctx(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx(t), `LOCK TABLE platform.subscriptions IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = s.For(ctx(t), tenant)
	if !errors.Is(err, db.ErrStatementTimeout) {
		t.Fatalf("err = %v after %v, want db.ErrStatementTimeout", err, time.Since(start))
	}
	if err := tx.Rollback(ctx(t)); err != nil {
		t.Fatal(err)
	}
	if got := mustFor(t, s, tenant); !got.HasModule("ventas") {
		t.Errorf("after the lock was released For = %+v, want the real entitlements", got)
	}
}

func TestInvalidInputs(t *testing.T) {
	e := newEnv(t)
	s := e.service(entitlements.Options{})

	for name, tenant := range map[string]pgtype.UUID{"zero value": {}, "all zeros": {Valid: true}} {
		if _, err := s.For(ctx(t), tenant); !errors.Is(err, db.ErrNoTenant) {
			t.Errorf("%s: err = %v, want db.ErrNoTenant", name, err)
		}
	}
	// Una empresa que no existe no es «sin plan»: es un dato roto.
	if _, err := s.For(ctx(t), e.uuid()); !errors.Is(err, entitlements.ErrTenantNotFound) {
		t.Errorf("unknown tenant: err = %v, want ErrTenantNotFound", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.For(canceled, e.seeded("ventas")); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled context: err = %v", err)
	}
}

// Un plan con límites que no son números cierra con error en vez de abrir sin tope.
func TestMalformedPlanLimitsFailClosed(t *testing.T) {
	e := newEnv(t)
	tenant := e.tenant("activa")
	e.subscribe(tenant, e.plan("bad", `{"users": "muchos"}`, "ventas"), "activa")
	got, err := e.service(entitlements.Options{}).For(ctx(t), tenant)
	if err == nil {
		t.Fatalf("a plan with a non-numeric limit was accepted: %+v", got)
	}
	if len(got.Modules) != 0 {
		t.Error("an error came with modules")
	}
}
