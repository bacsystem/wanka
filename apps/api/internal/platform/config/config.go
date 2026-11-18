// Package config reads the process configuration from environment variables.
//
// Errors name the offending variable and never include its value.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// Names of the environment variables the binary reads.
const (
	EnvDatabaseURL = "DATABASE_URL"
	EnvHTTPAddr    = "HTTP_ADDR"
	EnvLogLevel    = "LOG_LEVEL"
)

// DefaultHTTPAddr is used when HTTP_ADDR is not set.
const DefaultHTTPAddr = ":8080"

// Config is the validated process configuration.
type Config struct {
	// DatabaseURL is a secret. It is not used until the database story lands;
	// it is required now so a deployment cannot start without it. Never log it.
	DatabaseURL string
	HTTPAddr    string
	LogLevel    slog.Level
}

// String hides the secret so a Config printed by mistake leaks nothing.
func (c Config) String() string {
	return fmt.Sprintf("Config{HTTPAddr:%s LogLevel:%s DatabaseURL:<redacted>}", c.HTTPAddr, c.LogLevel)
}

// Load builds a Config from getenv (os.Getenv in production).
// Every problem found is reported, joined in a single error.
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	cfg := Config{
		DatabaseURL: getenv(EnvDatabaseURL),
		HTTPAddr:    getenv(EnvHTTPAddr),
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		errs = append(errs, fmt.Errorf("required environment variable %s is not set", EnvDatabaseURL))
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
	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
