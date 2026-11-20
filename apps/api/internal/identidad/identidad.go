// Package identidad es el módulo de identidad (ADR 0002): login, 2FA, sesión y usuarios. Esta es su fachada;
// los demás módulos no importan domain, app ni adapters. Hoy expone el login (POST /v1/auth/login, #24).
package identidad

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/adapters/argon2"
	"github.com/bacsystem/wanka/apps/api/internal/identidad/adapters/httpapi"
	"github.com/bacsystem/wanka/apps/api/internal/identidad/adapters/postgres"
	"github.com/bacsystem/wanka/apps/api/internal/identidad/app"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/jobs"
	"github.com/bacsystem/wanka/apps/api/internal/platform/mailer"
)

// Espera acotada del semáforo del hash (ADR 0004): 2 x núcleos en cola y 300 ms.
const (
	GateWait          = 300 * time.Millisecond
	GateQueuePerCore  = 2
	loginPattern      = "POST /v1/auth/login"
	lockoutNoticeKind = app.LockoutNoticeKind
)

// Options reemplaza los valores de producción; los ceros son el contrato. Lo usan los tests.
type Options struct {
	Hash         argon2.Params
	GateSize     int
	GateQueue    int
	GateWait     time.Duration
	Clock        clock.Clock
	EmailLimiter httpx.LimiterConfig
	IPLimiter    httpx.LimiterConfig
	Log          *slog.Logger
}

// Mount registra las rutas de identidad en routes. pool puede ser nil solo en los tests de cableado, que no
// llegan a la base.
func Mount(routes *auth.Routes, pool *db.Pool, o Options) error {
	if o.Hash == (argon2.Params{}) {
		o.Hash = argon2.Default
	}
	if o.GateSize == 0 {
		o.GateSize = runtime.GOMAXPROCS(0)
	}
	if o.GateQueue == 0 {
		o.GateQueue = GateQueuePerCore * o.GateSize
	}
	if o.GateWait == 0 {
		o.GateWait = GateWait
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	hasher, err := argon2.New(o.Hash)
	if err != nil {
		return fmt.Errorf("creating the password hasher: %w", err)
	}
	var enq jobs.Enqueuer = noEnqueuer{}
	if pool != nil {
		if enq, err = jobs.NewEnqueuer(pool); err != nil {
			return err
		}
	}
	svc := app.NewService(postgres.New(pool, enq), hasher, app.NewGate(o.GateSize, o.GateQueue, o.GateWait), o.Log)
	login, err := httpapi.NewLogin(svc, httpapi.Options{ClientIP: routes.ClientIP, Clock: o.Clock, Log: o.Log,
		EmailLimiter: o.EmailLimiter, IPLimiter: o.IPLimiter})
	if err != nil {
		return fmt.Errorf("creating the login handler: %w", err)
	}
	routes.Handle(loginPattern, auth.Public, auth.Spec{Module: auth.NoModule, Permission: auth.NoPermission}, login)
	return nil
}

// RegisterJobs registra los handlers de identidad en el worker: hoy, el aviso del bloqueo del login.
func RegisterJobs(reg *jobs.Registry, pool *db.Pool, m mailer.Mailer) error {
	if err := jobs.Register(reg, app.LockoutNotice(postgres.NewEmails(pool), m)); err != nil {
		return fmt.Errorf("registering %s: %w", lockoutNoticeKind, err)
	}
	return nil
}

// noEnqueuer es el encolador de los tests de cableado sin base: encolar es un error, nunca un silencio.
type noEnqueuer struct{}

func (noEnqueuer) InsertTx(context.Context, pgx.Tx, jobs.Args, ...jobs.Option) error {
	return errors.New("identidad: no job enqueuer: the module was mounted without a database")
}
