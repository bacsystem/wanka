// Package config reads the process configuration from environment variables.
//
// Errors name the offending variable and never include its value.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// Nombres de las variables de entorno que lee el binario, además de la URL de
// base de datos de cada Service.
const (
	EnvHTTPAddr  = "HTTP_ADDR"
	EnvLogLevel  = "LOG_LEVEL"
	EnvDBPoolMax = "DB_POOL_MAX_CONNS"
	EnvMailer    = "MAILER"
	EnvAppEnv    = "APP_ENV"
	// EnvTrustClientIP: la API lee la IP del cliente de X-Client-Ip (api/conventions.md §3).
	EnvTrustClientIP = "API_TRUST_CLIENT_IP"
)

// Valores de MAILER: el adaptador de correo. El proveedor real llega con
// notificaciones (ola 4).
const (
	MailerStub = "stub" // no envía; guarda en memoria y registra destinatario enmascarado y asunto
	MailerNone = "none" // rechaza todo envío
)

// maxPoolMax acota DB_POOL_MAX_CONNS: un error de tipeo no debe abrir cientos de conexiones.
const maxPoolMax = 200

// Service es una identidad de base de datos: cada proceso se conecta con el rol
// de su servicio mediante su propia URL (ADR 0001, "Creación de los roles"; ADR 0007).
type Service struct {
	Name        string
	URLVar      string // variable de entorno con la URL de conexión (un secreto)
	DefaultPool int32  // tamaño máximo de pool por defecto (docs/architecture/capacity.md)
}

// Los servicios del binario. Los tamaños de pool son por réplica.
var (
	Migrator = Service{"migrator", "DATABASE_URL_MIGRATOR", 1}
	API      = Service{"api", "DATABASE_URL_APP", 20}
	Platform = Service{"platform-api", "DATABASE_URL_PLATFORM", 5}
	Worker   = Service{"worker", "DATABASE_URL_WORKER", 10}
	Webhooks = Service{"webhooks", "DATABASE_URL_WEBHOOKS", 5}
)

// DefaultHTTPAddr is used when HTTP_ADDR is not set.
const DefaultHTTPAddr = ":8080"

// Config is the validated process configuration.
type Config struct {
	// DatabaseURL es un secreto: la URL del rol del servicio. Nunca se registra.
	DatabaseURL string
	// DBPoolMax es el tamaño máximo del pool de conexiones del servicio.
	DBPoolMax int32
	HTTPAddr  string
	LogLevel  slog.Level
	// Mailer es MailerStub (por defecto) o MailerNone.
	Mailer string
	// Production es true (también para staging) salvo APP_ENV=development: sin la variable se asume
	// producción, para que un olvido avise de más y nunca de menos.
	Production bool
	// TrustClientIP es API_TRUST_CLIENT_IP (false por defecto): solo donde el BFF es el único cliente que alcanza la API.
	TrustClientIP bool
}

// String hides the secret so a Config printed by mistake leaks nothing.
func (c Config) String() string {
	return fmt.Sprintf("Config{HTTPAddr:%s LogLevel:%s DBPoolMax:%d DatabaseURL:<redacted>}", c.HTTPAddr, c.LogLevel, c.DBPoolMax)
}

// Load construye la Config de svc a partir de getenv (os.Getenv en producción).
// Todos los problemas encontrados se informan juntos en un solo error.
func Load(getenv func(string) string, svc Service) (Config, error) {
	var errs []error
	cfg := Config{
		DatabaseURL: getenv(svc.URLVar),
		DBPoolMax:   svc.DefaultPool,
		HTTPAddr:    getenv(EnvHTTPAddr),
		Production:  true,
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		errs = append(errs, fmt.Errorf("required environment variable %s is not set", svc.URLVar))
	}
	if v := getenv(EnvDBPoolMax); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxPoolMax {
			// El valor no se repite en el mensaje.
			errs = append(errs, fmt.Errorf("environment variable %s must be an integer between 1 and %d", EnvDBPoolMax, maxPoolMax))
		} else {
			cfg.DBPoolMax = int32(n)
		}
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = DefaultHTTPAddr
	}
	switch strings.ToLower(getenv(EnvLogLevel)) {
	case "", "info":
		cfg.LogLevel = slog.LevelInfo
	case "debug":
		cfg.LogLevel = slog.LevelDebug
	case "warn":
		cfg.LogLevel = slog.LevelWarn
	case "error":
		cfg.LogLevel = slog.LevelError
	default:
		// The value is not echoed: it could be anything.
		errs = append(errs, fmt.Errorf("environment variable %s must be one of debug, info, warn, error", EnvLogLevel))
	}
	switch strings.ToLower(getenv(EnvMailer)) {
	case "", MailerStub:
		cfg.Mailer = MailerStub
	case MailerNone:
		cfg.Mailer = MailerNone
	default:
		errs = append(errs, fmt.Errorf("environment variable %s must be one of %s, %s", EnvMailer, MailerStub, MailerNone))
	}
	switch strings.ToLower(getenv(EnvAppEnv)) {
	case "", "production", "staging": // staging se trata como producción (ADR 0007)
	case "development":
		cfg.Production = false
	default:
		errs = append(errs, fmt.Errorf("environment variable %s must be one of production, staging, development", EnvAppEnv))
	}
	switch strings.ToLower(getenv(EnvTrustClientIP)) {
	case "", "false":
	case "true":
		cfg.TrustClientIP = true
	default:
		errs = append(errs, fmt.Errorf("environment variable %s must be one of true, false", EnvTrustClientIP))
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
