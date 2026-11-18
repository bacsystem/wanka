package config_test

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/config"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadMissingRequiredNamesVariable(t *testing.T) {
	_, err := config.Load(env(nil))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), config.EnvDatabaseURL) {
		t.Fatalf("error %q does not name %s", err, config.EnvDatabaseURL)
	}
}

func TestLoadNeverPrintsValues(t *testing.T) {
	const secret = "postgres://user:s3cr3t-pass@db/x"
	cfg, err := config.Load(env(map[string]string{
		config.EnvDatabaseURL: secret,
		config.EnvLogLevel:    secret, // invalid level carrying a secret-looking value
	}))
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
	if strings.Contains(err.Error(), "s3cr3t") {
		t.Fatalf("error leaks a value: %q", err)
	}
	if !strings.Contains(err.Error(), config.EnvLogLevel) {
		t.Fatalf("error %q does not name %s", err, config.EnvLogLevel)
	}

	cfg, err = config.Load(env(map[string]string{config.EnvDatabaseURL: secret}))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{cfg.String()} {
		if strings.Contains(s, "s3cr3t") {
			t.Fatalf("String leaks the secret: %q", s)
		}
	}
}

func TestLoadDefaultsAndOverrides(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{config.EnvDatabaseURL: "x"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != config.DefaultHTTPAddr || cfg.LogLevel != slog.LevelInfo {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	cfg, err = config.Load(env(map[string]string{
		config.EnvDatabaseURL: "x", config.EnvHTTPAddr: "127.0.0.1:9", config.EnvLogLevel: "DEBUG",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9" || cfg.LogLevel != slog.LevelDebug {
		t.Fatalf("unexpected overrides: %+v", cfg)
	}
}
