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
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2

	shutdownTimeout = 10 * time.Second
)

var planes = map[string]bool{"tenant": true, "platform": true, "webhooks": true}

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
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "wanka %s: unexpected argument %q\n", sub, fs.Arg(0))
		return exitUsage
	}
	if sub == "serve" && !planes[plane] {
		if plane == "" {
			fmt.Fprintln(stderr, "wanka serve: --plane is required (tenant, platform or webhooks)")
		} else {
			fmt.Fprintf(stderr, "wanka serve: unknown plane %q (want tenant, platform or webhooks)\n", plane)
		}
		return exitUsage
	}

	cfg, err := config.Load(getenv)
	if err != nil {
		fmt.Fprintf(stderr, "wanka %s: configuration error: %v\n", sub, err)
		return exitError
	}
	log := logging.New(stdout, cfg.LogLevel).With("command", sub)

	if sub == "serve" {
		err = serve(ctx, log, cfg, plane)
	} else {
		err = idle(ctx, log, sub)
	}
	if err != nil {
		log.Error("terminated with error", "err", err.Error())
		return exitError
	}
	return exitOK
}

// serve runs the HTTP server of one plane until ctx is cancelled, then shuts
// it down gracefully.
func serve(ctx context.Context, log *slog.Logger, cfg config.Config, plane string) error {
	log = log.With("plane", plane)
	// Every plane mounts only the health routes for now; the module routes
	// arrive with their stories (tenant never mounts plataforma, ADR 0002).
	handler := httpx.Chain(httpx.NewMux(httpx.NotReady), log)
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

// idle is the stub for worker and migrate: it starts, waits for the stop
// signal and exits cleanly. Their real work lands with the jobs and database
// stories.
func idle(ctx context.Context, log *slog.Logger, name string) error {
	log.Info("started", "note", name+" is a stub until its story lands")
	<-ctx.Done()
	log.Info("stopped")
	return nil
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage:
  wanka serve --plane=tenant|platform|webhooks
  wanka worker
  wanka migrate
`)
}
