// Command wanka is the single backend binary: serve (one plane per process),
// worker and migrate. See docs/architecture/adr/0002-modular-monolith-go.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/config"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
	"github.com/bacsystem/wanka/apps/api/migrations"
)

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2

	shutdownTimeout = 10 * time.Second
)

// planes asocia cada plano con el servicio (rol de base de datos) que lo atiende.
var planes = map[string]config.Service{
	"tenant":   config.API,
	"platform": config.Platform,
	"webhooks": config.Webhooks,
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run executes one invocation and returns the process exit code. Logs go to
// stdout; usage and configuration errors go to stderr.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}
	sub, rest := args[0], args[1:]

	switch sub {
	case "help", "-h", "--help":
		usage(stdout)
		return exitOK
	case "serve", "worker", "migrate":
	default:
		fmt.Fprintf(stderr, "wanka: unknown subcommand %q\n", sub)
		usage(stderr)
		return exitUsage
	}

	fs := flag.NewFlagSet("wanka "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var plane string
	if sub == "serve" {
		fs.StringVar(&plane, "plane", "", "plane to serve: tenant, platform or webhooks")
	}
	if err := fs.Parse(rest); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if sub == "migrate" {
		if fs.NArg() != 1 || fs.Arg(0) != "up" {
			fmt.Fprintln(stderr, "wanka migrate: the only action is \"up\"")
			return exitUsage
		}
	} else if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "wanka %s: unexpected argument %q\n", sub, fs.Arg(0))
		return exitUsage
	}
	svc := config.Worker
	switch sub {
	case "migrate":
		svc = config.Migrator
	case "serve":
		var ok bool
		if svc, ok = planes[plane]; !ok {
			if plane == "" {
				fmt.Fprintln(stderr, "wanka serve: --plane is required (tenant, platform or webhooks)")
			} else {
				fmt.Fprintf(stderr, "wanka serve: unknown plane %q (want tenant, platform or webhooks)\n", plane)
			}
			return exitUsage
		}
	}

	cfg, err := config.Load(getenv, svc)
	if err != nil {
		fmt.Fprintf(stderr, "wanka %s: configuration error: %v\n", sub, err)
		return exitError
	}
	log := logging.New(stdout, cfg.LogLevel).With("command", sub)

	switch sub {
	case "serve":
		err = serve(ctx, log, cfg, svc, plane)
	case "migrate":
		err = migrate(ctx, log, cfg)
	default:
		err = worker(ctx, log, cfg, svc)
	}
	if err != nil {
		log.Error("terminated with error", "err", err.Error())
		return exitError
	}
	return exitOK
}

// serve runs the HTTP server of one plane until ctx is cancelled, then shuts
// it down gracefully.
func serve(ctx context.Context, log *slog.Logger, cfg config.Config, svc config.Service, plane string) error {
	log = log.With("plane", plane)
	// El pool no conecta al abrirse: con la base caída el proceso arranca
	// igual y /readyz responde 503 hasta que la base conteste.
	pool, err := db.Open(ctx, cfg.DatabaseURL, db.Options{MaxConns: cfg.DBPoolMax, AppName: "wanka-" + svc.Name})
	if err != nil {
		return fmt.Errorf("opening database pool: %w", err)
	}
	defer pool.Close()
	ready := func(ctx context.Context) error {
		err := pool.Ping(ctx)
		if err != nil {
			log.WarnContext(ctx, "readiness check failed", "err", err.Error())
		}
		return err
	}
	// Every plane mounts only the health routes for now; the module routes
	// arrive with their stories (tenant never mounts plataforma, ADR 0002).
	handler := httpx.Chain(httpx.NewMux(ready), log)
	srv := httpx.NewServer(cfg.HTTPAddr, handler)

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listening on HTTP address: %w", err)
	}
	log.Info("listening", "addr", ln.Addr().String())

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	select {
	case err := <-errc:
		return fmt.Errorf("serving: %w", err)
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("shutting down: %w", err)
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving: %w", err)
	}
	log.Info("stopped")
	return nil
}

// migrate aplica las migraciones pendientes como wanka_migrator y termina.
// Correrlo otra vez no cambia nada.
func migrate(ctx context.Context, log *slog.Logger, cfg config.Config) error {
	n, err := db.Migrate(ctx, cfg.DatabaseURL, migrations.FS, log)
	if err != nil {
		return fmt.Errorf("migrating: %w", err)
	}
	log.Info("migrations up to date", "applied", n)
	return nil
}

// worker es el stub del ejecutor de jobs: abre su pool (una URL o un tamaño
// inválidos lo detienen al arrancar), espera la señal de parada y sale limpio.
// Los jobs llegan con su historia.
func worker(ctx context.Context, log *slog.Logger, cfg config.Config, svc config.Service) error {
	pool, err := db.Open(ctx, cfg.DatabaseURL, db.Options{MaxConns: cfg.DBPoolMax, AppName: "wanka-" + svc.Name})
	if err != nil {
		return fmt.Errorf("opening database pool: %w", err)
	}
	defer pool.Close()
	log.Info("started", "note", "worker is a stub until its story lands")
	<-ctx.Done()
	log.Info("stopped")
	return nil
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage:
  wanka serve --plane=tenant|platform|webhooks
  wanka worker
  wanka migrate up
`)
}
