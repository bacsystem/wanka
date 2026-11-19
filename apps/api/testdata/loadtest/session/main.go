// Arnés de carga con sesiones reales (#196). Dos subcomandos:
//
//	seed   crea la base de carga, aplica el bootstrap de roles y las migraciones y
//	       siembra TENANTS empresas con PER_TENANT sesiones cada una; escribe los
//	       tokens (uno por línea) en TOKENS_FILE.
//	serve  levanta el kernel del plano tenant tal como lo cablea `wanka serve`
//	       (pgsession + auth.NewRoutes con los límites por defecto + Chain) con una
//	       ruta Tenant de prueba que no hace otra consulta, y una ruta de control sin
//	       sesión ni base. El log de acceso va a ACCESS_LOG; /stats (puerto STATS_ADDR)
//	       da las búsquedas de sesión y el uso del pool.
//
// Variables (los valores nunca van en el repo): LOAD_ADMIN_URL (superusuario, base
// "postgres"), LOAD_DB_HOST (host:puerto), LOAD_ROLE_PASSWORD (clave de los roles
// wanka_migrator y wanka_app), TOKENS_FILE, ACCESS_LOG, DB_POOL_MAX_CONNS,
// HTTP_ADDR, STATS_ADDR, TENANTS, PER_TENANT. Ver README.md.
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/bootstrap"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth/pgsession"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
	"github.com/bacsystem/wanka/apps/api/internal/platform/metrics"
	"github.com/bacsystem/wanka/apps/api/migrations"
)

const dbName = "wanka_load"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: session seed|serve")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "seed":
		err = seed()
	case "serve":
		err = serve()
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func must(k string) (string, error) {
	if v := os.Getenv(k); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%s is not set", k)
}

func roleURL(role, pass, host string) string {
	u := url.URL{Scheme: "postgres", User: url.UserPassword(role, pass), Host: host, Path: "/" + dbName}
	return u.String()
}

func seed() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	adminURL, err := must("LOAD_ADMIN_URL")
	if err != nil {
		return err
	}
	pass, err := must("LOAD_ROLE_PASSWORD")
	if err != nil {
		return err
	}
	host, err := must("LOAD_DB_HOST")
	if err != nil {
		return err
	}
	tokensFile, err := must("TOKENS_FILE")
	if err != nil {
		return err
	}
	tenants, _ := strconv.Atoi(env("TENANTS", "24"))
	per, _ := strconv.Atoi(env("PER_TENANT", "15"))

	au, err := url.Parse(adminURL)
	if err != nil {
		return fmt.Errorf("LOAD_ADMIN_URL is not a URL")
	}
	root, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return fmt.Errorf("connecting as superuser: %w", err)
	}
	defer root.Close(ctx)
	if _, err = root.Exec(ctx, `DROP DATABASE IF EXISTS `+dbName+` WITH (FORCE)`); err != nil {
		return err
	}
	if _, err = root.Exec(ctx, `CREATE DATABASE `+dbName+` TEMPLATE template0`); err != nil {
		return err
	}
	du := *au
	du.Path = "/" + dbName
	admin, err := pgx.Connect(ctx, du.String())
	if err != nil {
		return fmt.Errorf("connecting to the load database: %w", err)
	}
	defer admin.Close(ctx)
	if _, err = admin.Exec(ctx, bootstrap.RolesSQL); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	for _, r := range []string{"wanka_migrator", "wanka_app"} {
		if _, err = admin.Exec(ctx, `ALTER ROLE `+r+` PASSWORD `+quote(pass)); err != nil {
			return err
		}
	}
	n, err := db.Migrate(ctx, roleURL("wanka_migrator", pass, host), migrations.FS, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		return fmt.Errorf("migrating: %w", err)
	}
	fmt.Println("migrations applied:", n)

	// Misma forma que dbtest.SeedSession (sesión ligada, membresía activa, versión 1),
	// con una empresa por grupo de sesiones y un usuario y una membresía por sesión.
	f, err := os.OpenFile(tokensFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for t := 0; t < tenants; t++ {
		var tid, rid string
		if err = admin.QueryRow(ctx, `WITH t AS (INSERT INTO org.tenants (id) VALUES (gen_random_uuid()) RETURNING id),
			r AS (INSERT INTO iam.roles (id, tenant_id, name) SELECT gen_random_uuid(), t.id, 'Rol' FROM t RETURNING id)
			SELECT t.id::text, r.id::text FROM t, r`).Scan(&tid, &rid); err != nil {
			return fmt.Errorf("seeding tenant: %w", err)
		}
		batch := &pgx.Batch{}
		for s := 0; s < per; s++ {
			var raw [32]byte
			_, _ = rand.Read(raw[:])
			token := "wk_at_" + base64.RawURLEncoding.EncodeToString(raw[:])
			hash := sha256.Sum256([]byte(token))
			fmt.Fprintln(w, token)
			batch.Queue(`WITH u AS (INSERT INTO iam.users (id) VALUES (gen_random_uuid()) RETURNING id),
				m AS (INSERT INTO iam.memberships (id, tenant_id, user_id, role_id, status, authz_version)
				      SELECT gen_random_uuid(), $1::uuid, u.id, $2::uuid, 'activo', 1 FROM u RETURNING id, user_id)
				INSERT INTO iam.sessions (id, user_id, tenant_id, site_id, cash_register_id, membership_id, access_hash, access_expires_at,
				       authz_version, mfa_satisfied, mfa_method, refresh_family)
				SELECT gen_random_uuid(), m.user_id, $1::uuid, gen_random_uuid(), gen_random_uuid(), m.id, $3, now() + interval '4 hours',
				       1, true, 'none', gen_random_uuid() FROM m`, tid, rid, hash[:])
		}
		br := admin.SendBatch(ctx, batch)
		for s := 0; s < per; s++ {
			if _, err = br.Exec(); err != nil {
				_ = br.Close()
				return fmt.Errorf("seeding session: %w", err)
			}
		}
		if err = br.Close(); err != nil {
			return err
		}
	}
	if err = w.Flush(); err != nil {
		return err
	}
	var ns, nt int
	_ = admin.QueryRow(ctx, `SELECT count(*), count(DISTINCT tenant_id) FROM iam.sessions`).Scan(&ns, &nt)
	fmt.Printf("seeded %d sessions in %d tenants\n", ns, nt)
	_, _ = admin.Exec(ctx, `VACUUM ANALYZE`)
	return nil
}

func quote(s string) string { return "'" + replaceQuote(s) + "'" }

func replaceQuote(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'')
		}
		out = append(out, s[i])
	}
	return string(out)
}

// timed mide la duración de cada Lookup (pool + 4 idas y vueltas + consulta).
type timed struct {
	inner auth.SessionResolver
	mu    sync.Mutex
	durs  []time.Duration
}

func (t *timed) Lookup(ctx context.Context, h [32]byte) (auth.SessionRecord, bool, error) {
	s := time.Now()
	r, ok, err := t.inner.Lookup(ctx, h)
	d := time.Since(s)
	t.mu.Lock()
	t.durs = append(t.durs, d)
	t.mu.Unlock()
	return r, ok, err
}

func (t *timed) Detach(ctx context.Context, a, b pgtype.UUID) (bool, error) {
	return t.inner.Detach(ctx, a, b)
}

func pct(d []time.Duration, p float64) float64 {
	if len(d) == 0 {
		return 0
	}
	return float64(d[int(float64(len(d)-1)*p)].Microseconds()) / 1000
}

func serve() error {
	pass, err := must("LOAD_ROLE_PASSWORD")
	if err != nil {
		return err
	}
	host, err := must("LOAD_DB_HOST")
	if err != nil {
		return err
	}
	logPath, err := must("ACCESS_LOG")
	if err != nil {
		return err
	}
	pool, _ := strconv.Atoi(env("DB_POOL_MAX_CONNS", "20"))
	lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer lf.Close()
	bw := bufio.NewWriterSize(lf, 1<<20)
	sw := &syncWriter{w: bw}
	log := logging.New(sw, slog.LevelInfo)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	p, err := db.Open(ctx, roleURL("wanka_app", pass, host), db.Options{MaxConns: int32(pool), AppName: "wanka-loadtest"})
	if err != nil {
		return err
	}
	defer p.Close()

	rt := httpx.NewMux(func(ctx context.Context) error { return p.Ping(ctx) })
	res := &timed{inner: pgsession.New(p)}
	routes, err := auth.NewRoutes(rt, res, auth.Options{}) // límites por defecto: lo que se despliega
	if err != nil {
		return err
	}
	body := []byte(`{"ok":true}` + "\n")
	h := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
	routes.HandleFunc("GET /v1/loadtest/ping", auth.Tenant, h) // ruta Tenant que no hace otra consulta
	rt.HandleFunc("GET /ctl", h)                               // control: misma cadena, sin sesión ni base

	var peak, sum, n atomic.Int64
	go func() {
		t := time.NewTicker(20 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				a := int64(p.AcquiredConns())
				sum.Add(a)
				n.Add(1)
				for {
					old := peak.Load()
					if a <= old || peak.CompareAndSwap(old, a) {
						break
					}
				}
			}
		}
	}()
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
			res.mu.Lock()
			d := append([]time.Duration(nil), res.durs...)
			if r.URL.Query().Get("reset") == "1" {
				res.durs = res.durs[:0]
			}
			res.mu.Unlock()
			sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
			pk, s, c := peak.Load(), sum.Load(), n.Load()
			if r.URL.Query().Get("reset") == "1" {
				peak.Store(0)
				sum.Store(0)
				n.Store(0)
			}
			mean := 0.0
			if c > 0 {
				mean = float64(s) / float64(c)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"lookups": len(d), "lookup_p50_ms": pct(d, .5), "lookup_p95_ms": pct(d, .95),
				"lookup_p99_ms": pct(d, .99), "lookup_max_ms": pct(d, 1), "pool_max": pool, "pool_acquired_peak": pk, "pool_acquired_mean": mean})
		})
		mux.HandleFunc("/flush", func(http.ResponseWriter, *http.Request) { sw.flush() })
		_ = http.ListenAndServe(env("STATS_ADDR", "127.0.0.1:18091"), mux)
	}()

	srv := httpx.NewServer(env("HTTP_ADDR", "127.0.0.1:18090"), httpx.Chain(rt, log, metrics.Nop{}))
	go func() {
		<-ctx.Done()
		sctx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = srv.Shutdown(sctx)
	}()
	fmt.Println("listening on", srv.Addr)
	err = srv.ListenAndServe()
	sw.flush()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

type syncWriter struct {
	mu sync.Mutex
	w  *bufio.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}
func (s *syncWriter) flush() { s.mu.Lock(); _ = s.w.Flush(); s.mu.Unlock() }
