package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bacsystem/wanka/apps/api/internal/identidad"
	"github.com/bacsystem/wanka/apps/api/internal/identidad/adapters/argon2"
	"github.com/bacsystem/wanka/apps/api/internal/platform/config"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

const (
	// envWankaEnv debe valer "local" para que dev-seed corra: la semilla escribe usuarios con una contraseña conocida.
	envWankaEnv = "WANKA_ENV"
	// devSeedPassword es la contraseña de prueba común (solo local; no está en la lista de contraseñas filtradas).
	devSeedPassword = "Demo-Wanka-2026-local"
)

// errNotLocal: dev-seed se niega fuera de local aunque alguien apunte la URL a otra base.
var errNotLocal = errors.New("dev-seed only runs locally: set WANKA_ENV=local and point DATABASE_URL_SEED at a loopback database")

// checkLocal exige WANKA_ENV=local y que TODO destino que pgx usaría (host principal y alternos) sea la propia máquina:
// IP de loopback, socket Unix o un nombre cuyas direcciones resueltas sean todas de loopback. Se valida la
// configuración que pgx arma (pgconn.ParseConfig), no solo el host de la URL: ?host=, hosts separados por comas y
// service/passfile (que traen el host de un archivo) cambiarían el destino real.
func checkLocal(getenv func(string) string, databaseURL string) error {
	if getenv(envWankaEnv) != "local" {
		return errNotLocal
	}
	u, err := url.Parse(databaseURL)
	if err != nil {
		return errNotLocal
	}
	for k := range u.Query() {
		switch strings.ToLower(k) {
		case "service", "passfile", "hostaddr":
			return errNotLocal
		}
	}
	cfg, err := pgconn.ParseConfig(databaseURL)
	if err != nil {
		return errNotLocal
	}
	hosts := []string{cfg.Host}
	for _, fb := range cfg.Fallbacks {
		hosts = append(hosts, fb.Host)
	}
	for _, h := range hosts {
		if !isLocalHost(h) {
			return errNotLocal
		}
	}
	return nil
}

// isLocalHost: socket Unix, IP de loopback o nombre que resuelve solo a loopback.
func isLocalHost(h string) bool {
	if strings.HasPrefix(h, "/") {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	if h == "" {
		return false
	}
	ips, err := net.LookupIP(h)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if !ip.IsLoopback() {
			return false
		}
	}
	return true
}

// devSeed siembra la empresa de prueba y sus usuarios (identidad.DevSeed) e imprime la tabla de acceso.
func devSeed(ctx context.Context, log *slog.Logger, cfg config.Config, getenv func(string) string, stdout io.Writer) error {
	if err := checkLocal(getenv, cfg.DatabaseURL); err != nil {
		return err
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL, db.Options{MaxConns: cfg.DBPoolMax, AppName: "wanka-dev-seed"})
	if err != nil {
		return fmt.Errorf("opening database pool: %w", err)
	}
	defer pool.Close()
	users, err := identidad.DevSeed(ctx, pool, argon2.Default, devSeedPassword)
	if err != nil {
		return err
	}
	log.Info("dev seed done", "users", len(users))
	fmt.Fprintf(stdout, "\nEmpresa de prueba: %s (RUC %s)\nContrasena de prueba (solo local): %s\n\n%-14s %s\n", identidad.DevTenantLegalName, identidad.DevTenantRUC, devSeedPassword, "ROL", "CORREO")
	for _, u := range users {
		fmt.Fprintf(stdout, "%-14s %s\n", u.Role, u.Email)
	}
	return nil
}
