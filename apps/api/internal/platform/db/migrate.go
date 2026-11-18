package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

const (
	// migrationLockTimeout hace que una migración que espera un lock falle en
	// vez de congelar producción (data-model §6).
	migrationLockTimeout = 5 * time.Second
	// migrationStatementTimeout acota cada sentencia de una migración.
	migrationStatementTimeout = 5 * time.Minute
)

// Migrate aplica las migraciones pendientes de fsys con el rol de rawURL y
// devuelve cuántas aplicó; correrlo otra vez no aplica ninguna. Se niega a
// correr como superusuario o con un rol BYPASSRLS, y serializa las corridas
// concurrentes con un advisory lock. La sesión tiene lock_timeout = 5s.
func Migrate(ctx context.Context, rawURL string, fsys fs.FS, log *slog.Logger) (int, error) {
	scrub := scrubber(rawURL)
	cc, err := pgx.ParseConfig(rawURL)
	if err != nil {
		return 0, scrubbed(scrub, fmt.Errorf("parsing database URL: %w", err))
	}
	cc.RuntimeParams["lock_timeout"] = fmt.Sprintf("%dms", migrationLockTimeout.Milliseconds())
	cc.RuntimeParams["statement_timeout"] = fmt.Sprintf("%dms", migrationStatementTimeout.Milliseconds())
	cc.RuntimeParams["application_name"] = "wanka-migrator"
	cc.ConnectTimeout = 10 * time.Second

	sqlDB := stdlib.OpenDB(*cc)
	defer sqlDB.Close()
	// Una conexión sostiene el advisory lock y la otra corre la migración.
	sqlDB.SetMaxOpenConns(2)
	sqlDB.SetConnMaxLifetime(time.Hour)

	n, err := migrate(ctx, sqlDB, fsys, log)
	if err != nil {
		return n, scrubbed(scrub, err)
	}
	return n, nil
}

func migrate(ctx context.Context, sqlDB *sql.DB, fsys fs.FS, log *slog.Logger) (int, error) {
	if err := requireUnprivileged(ctx, sqlDB); err != nil {
		return 0, err
	}
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(1, 300))
	if err != nil {
		return 0, fmt.Errorf("creating migration lock: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return 0, fmt.Errorf("creating migration provider: %w", err)
	}
	results, err := provider.Up(ctx)
	for _, r := range results {
		log.InfoContext(ctx, "migration applied", "version", r.Source.Version, "file", r.Source.Path,
			"duration_ms", r.Duration.Milliseconds())
	}
	if err != nil {
		return len(results), fmt.Errorf("applying migrations: %w", err)
	}
	return len(results), nil
}

// requireUnprivileged impone el invariante de que ninguna migración corre como
// superusuario ni con BYPASSRLS: cualquiera de los dos haría que RLS parezca
// correcto sin estar aplicándose.
func requireUnprivileged(ctx context.Context, sqlDB *sql.DB) error {
	var user string
	var super, bypass bool
	err := sqlDB.QueryRowContext(ctx,
		`SELECT current_user, rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user`).
		Scan(&user, &super, &bypass)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("checking migration role: current role not found")
	}
	if err != nil {
		return fmt.Errorf("checking migration role: %w", err)
	}
	if super || bypass {
		return fmt.Errorf("refusing to migrate as role %q: it has SUPERUSER or BYPASSRLS", strings.TrimSpace(user))
	}
	return nil
}
