package identidad_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/identidad"
	"github.com/bacsystem/wanka/apps/api/internal/identidad/adapters/postgres"
	"github.com/bacsystem/wanka/apps/api/internal/identidad/app"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/jobs"
	"github.com/bacsystem/wanka/apps/api/internal/platform/mailer/stub"
)

// El worker lee el correo del destinatario con el GRANT por columna de la migración 20261120130000 (id y email).
func TestWorkerReadsTheRecipientEmail(t *testing.T) {
	e := newEnv(t, envOpts{})
	uid := e.addUser("ana@x.pe", goodPassword)
	wp, err := db.Open(context.Background(), e.d.URL(dbtest.Worker), db.Options{MaxConns: 2, AppName: "identidad-worker-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(wp.Close)
	dir := postgres.NewEmails(wp)
	got, err := dir.EmailOf(context.Background(), uid)
	if err != nil || got != "ana@x.pe" {
		t.Fatalf("EmailOf = %q, %v", got, err)
	}
	if _, err := dir.EmailOf(context.Background(), "00000000-0000-4000-8000-000000000000"); !errors.Is(err, app.ErrUserNotFound) {
		t.Fatalf("EmailOf for a missing user = %v, want ErrUserNotFound", err)
	}
}

func TestJobsAreRegisteredOnceInTheWorker(t *testing.T) {
	e := newEnv(t, envOpts{})
	reg := jobs.NewRegistry(e.pool)
	m := stub.New(nil, 10)
	if err := identidad.RegisterJobs(reg, e.pool, m); err != nil {
		t.Fatal(err)
	}
	if err := identidad.RegisterJobs(reg, e.pool, m); err == nil {
		t.Fatal("the same Kind was registered twice")
	}
}
