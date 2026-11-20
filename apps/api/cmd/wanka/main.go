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

	"github.com/bacsystem/wanka/apps/api/internal/platform/audit"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth/pgsession"
	"github.com/bacsystem/wanka/apps/api/internal/platform/clock"
	"github.com/bacsystem/wanka/apps/api/internal/platform/config"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/jobs"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
	"github.com/bacsystem/wanka/apps/api/internal/platform/mailer"
	"github.com/bacsystem/wanka/apps/api/internal/platform/mailer/stub"
	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
	"github.com/bacsystem/wanka/apps/api/internal/platform/passwordpolicy"
	"github.com/bacsystem/wanka/apps/api/migrations"
)

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2

	shutdownTimeout = 10 * time.Second
)

// wiring es lo que serve toma de afuera para armar el plano: lo fija el
// contrato y los tests lo reemplazan.
type wiring struct {
	// sweepInterval es cada cuánto se barren los buckets llenos de los limitadores (#196).
	sweepInterval time.Duration
	// auth son las opciones de auth.NewRoutes; vacías toman los valores de conventions.md §7.
	auth auth.Options
	// entitlements son las opciones de entitlements.New (TTL 30 s por defecto);
	// el Logger lo pone serve.
	entitlements entitlements.Options
	// wrapSweeper, si no es nil, envuelve o reemplaza el barredor (solo los tests).
	wrapSweeper func(sweeper) sweeper
	// mount, si no es nil, registra rutas en el registrador del plano tenant
	// (solo los tests; los módulos llegan con sus historias).
	mount func(*auth.Routes)
	// wrapListener, si no es nil, envuelve o reemplaza el escucha de NOTIFY (solo los tests).
	wrapListener func(listener) listener
	// breachedList, si no es nil, reemplaza el archivo embebido de contraseñas filtradas (solo los tests).
	breachedList []byte
	// passwords es la política cargada por withPasswords; serve la guarda aquí para que
	// las rutas de #26, #28 y #30 usen este mismo objeto (nada la vuelve a cargar).
	passwords *passwordpolicy.Policy
}

// withPasswords devuelve w con la política de contraseñas cargada (la lista embebida, o
// breachedList si no es nil). Falla cerrado: si el archivo falta o está dañado, serve no arranca (#238).
func (w wiring) withPasswords() (wiring, error) {
	p, err := loadPasswordPolicy(w.breachedList)
	if err != nil {
		return w, fmt.Errorf("loading breached password list: %w", err)
	}
	w.passwords = p
	return w, nil
}

// defaultWiring es el cableado de producción.
var defaultWiring = wiring{sweepInterval: 10 * time.Second}

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

	if sub != "migrate" {
		// Aún ningún módulo lo consume; cuando lleguen, reciben el puerto aquí.
		_ = newMailer(log, cfg)
	}

	switch sub {
	case "serve":
		err = serve(ctx, log, cfg, svc, plane, defaultWiring)
	case "migrate":
		err = migrate(ctx, log, cfg)
	default:
		err = worker(ctx, log, cfg, svc, getenv)
	}
	if err != nil {
		log.Error("terminated with error", "err", err.Error())
		return exitError
	}
	return exitOK
}

// newMailer elige el adaptador de correo según MAILER. El stub no entrega nada:
// En producción (y staging), tanto el stub como none avisan con WARN hasta que exista
// un adaptador real.
func newMailer(log *slog.Logger, cfg config.Config) mailer.Mailer {
	if cfg.Mailer == config.MailerNone {
		if cfg.Production {
			log.Warn("mailer is disabled: every send fails until a real mailer adapter exists", "mailer", cfg.Mailer)
		}
		return mailer.Disabled{}
	}
	if cfg.Production {
		log.Warn("mailer is the stub: no email is delivered until a real mailer adapter exists", "mailer", cfg.Mailer)
	}
	return stub.New(log, stub.DefaultMaxSent)
}

// serve runs the HTTP server of one plane until ctx is cancelled, then shuts
// it down gracefully.
func serve(ctx context.Context, log *slog.Logger, cfg config.Config, svc config.Service, plane string, w wiring) error {
	log = log.With("plane", plane)
	// La lista de contraseñas filtradas se valida antes de abrir nada: si falta o está
	// dañada, serve no arranca (cierra; no se degrada a «sin lista», #238).
	w, err := w.withPasswords()
	if err != nil {
		return err
	}
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
	// Every plane mounts the health routes; the tenant plane also mounts the
	// session chain. The module routes arrive with their stories (tenant never
	// mounts plataforma, ADR 0002).
	rt := httpx.NewMux(ready)
	w.entitlements.Logger = log
	sw, ls, err := mountPlane(plane, rt, pool, w)
	if err != nil {
		return err
	}
	if sw != nil && w.wrapSweeper != nil {
		sw = w.wrapSweeper(sw)
	}
	if ls != nil && w.wrapListener != nil {
		ls = w.wrapListener(ls)
	}
	handler := httpx.Chain(rt, log, metrics.Nop{})
	srv := httpx.NewServer(cfg.HTTPAddr, handler)

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listening on HTTP address: %w", err)
	}
	log.Info("listening", "addr", ln.Addr().String())

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	if sw != nil {
		// El bucle termina con el apagado: se espera a que salga antes de volver.
		sweepCtx, stopSweep := context.WithCancel(ctx)
		sweepDone := startSweepLoop(sweepCtx, sw, w.sweepInterval)
		defer func() { stopSweep(); <-sweepDone }()
	}
	if ls != nil {
		// Igual con el escucha de entitlements_changed: sin él la caché sigue
		// siendo correcta, solo tarda hasta el TTL en enterarse de un cambio.
		listenCtx, stopListen := context.WithCancel(ctx)
		listenDone := startListener(listenCtx, ls, log)
		defer func() { stopListen(); <-listenDone }()
	}

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

// loadPasswordPolicy carga la lista embebida, o data si no es nil.
func loadPasswordPolicy(data []byte) (*passwordpolicy.Policy, error) {
	if data != nil {
		return passwordpolicy.Parse(data)
	}
	return passwordpolicy.NewEmbedded()
}

// sweeper es lo que el bucle de barrido necesita de auth.Routes.
type sweeper interface{ Sweep() int }

// listener es el escucha de NOTIFY que invalida la caché de entitlements.
type listener interface {
	Run(ctx context.Context) error
}

// mountPlane monta en rt lo propio de cada plano. En el plano tenant arma el
// servicio de entitlements, el registrador de rutas /v1 con el resolver de
// PostgreSQL, los dos limitadores con los valores de conventions.md §7 y el
// paso de entitlements, y devuelve quien barre los buckets y quien escucha los
// NOTIFY; en los demás no monta nada y devuelve nil. El *auth.Routes no sale de
// aquí hacia los módulos todavía: cuando haya rutas /v1 de módulo, se les
// pasará solo ese registrador y nunca rt, para que ninguna ruta /v1 quede sin clase.
func mountPlane(plane string, rt *httpx.Router, pool *db.Pool, w wiring) (sweeper, listener, error) {
	if plane != "tenant" {
		return nil, nil, nil
	}
	ent := entitlements.New(pool, w.entitlements)
	routes, err := auth.NewRoutes(rt, pgsession.New(pool), ent, auth.DenyAllPermissions{}, audit.NewWriter(pool), w.auth)
	if err != nil {
		return nil, nil, fmt.Errorf("mounting the session chain: %w", err)
	}
	if w.mount != nil {
		w.mount(routes)
	}
	return routes, ent, nil
}

// startListener corre l.Run hasta que ctx termine; el canal devuelto se cierra
// cuando salió. Run solo devuelve error si no puede seguir: se registra y la
// caché queda acotada por su TTL.
func startListener(ctx context.Context, l listener, log *slog.Logger) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := l.Run(ctx); err != nil && ctx.Err() == nil {
			log.Error("entitlements listener stopped, the cache falls back to its TTL", "err", err.Error())
		}
	}()
	return done
}

// startSweepLoop llama a s.Sweep cada interval hasta que ctx termine; el canal
// devuelto se cierra cuando el bucle salió.
func startSweepLoop(ctx context.Context, s sweeper, interval time.Duration) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.Sweep()
			}
		}
	}()
	return done
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

// worker ejecuta los jobs (internal/platform/jobs): abre su pool (una URL o un
// tamaño inválidos lo detienen al arrancar), levanta las colas y, al recibir la
// señal de parada, espera a los jobs en curso hasta JOBS_SHUTDOWN_TIMEOUT. Registra
// los jobs de mantenimiento del kernel; los handlers de cada módulo se registran
// aquí cuando llegan sus historias.
func worker(ctx context.Context, log *slog.Logger, cfg config.Config, svc config.Service, getenv func(string) string) error {
	jcfg, err := jobs.ConfigFromEnv(getenv)
	if err != nil {
		return fmt.Errorf("job configuration: %w", err)
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL, db.Options{MaxConns: cfg.DBPoolMax, AppName: "wanka-" + svc.Name})
	if err != nil {
		return fmt.Errorf("opening database pool: %w", err)
	}
	defer pool.Close()
	reg := jobs.NewRegistry(pool)
	if err := jobs.RegisterMaintenance(reg, pool); err != nil {
		return err
	}
	clk := clock.System{}
	if err := registerKernelJobs(reg, pool, clk, log); err != nil {
		return err
	}
	// Al arrancar se aseguran las particiones sin esperar al ciclo del periódico (River lo reinicia con
	// cada despliegue). Si falla, el job lo reintenta; las que creó la migración cubren unos meses.
	if _, err := audit.EnsurePartitions(ctx, pool, clk.Now(), audit.PartitionsAhead); err != nil {
		log.Warn("could not ensure the audit partitions at startup", "err", err.Error())
	}
	w, err := jobs.NewWorker(pool, reg, jcfg, log)
	if err != nil {
		return err
	}
	if err := w.Run(ctx); err != nil {
		return err
	}
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
