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
	for _, svc := range []config.Service{config.Migrator, config.API, config.Platform, config.Worker, config.Webhooks} {
		t.Run(svc.Name, func(t *testing.T) {
			// La URL de otro servicio no debe satisfacer a este.
			other := map[string]string{"DATABASE_URL": "x", "DATABASE_URL_OTHER": "x"}
			_, err := config.Load(env(other), svc)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), svc.URLVar) {
				t.Fatalf("error %q does not name %s", err, svc.URLVar)
			}
		})
	}
}

func TestServiceVariablesAndPoolDefaults(t *testing.T) {
	cases := []struct {
		svc  config.Service
		name string
		pool int32
	}{
		{config.API, "DATABASE_URL_APP", 20},
		{config.Worker, "DATABASE_URL_WORKER", 10},
		{config.Webhooks, "DATABASE_URL_WEBHOOKS", 5},
		{config.Platform, "DATABASE_URL_PLATFORM", 5},
		{config.Migrator, "DATABASE_URL_MIGRATOR", 1},
	}
	for _, tc := range cases {
		t.Run(tc.svc.Name, func(t *testing.T) {
			if tc.svc.URLVar != tc.name {
				t.Fatalf("URLVar = %q, want %q", tc.svc.URLVar, tc.name)
			}
			cfg, err := config.Load(env(map[string]string{tc.name: "postgres://x"}), tc.svc)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.DatabaseURL != "postgres://x" || cfg.DBPoolMax != tc.pool {
				t.Fatalf("unexpected config: %+v", cfg)
			}
		})
	}
}

func TestPoolMaxOverrideAndValidation(t *testing.T) {
	base := map[string]string{"DATABASE_URL_APP": "x"}
	with := func(v string) map[string]string {
		m := map[string]string{config.EnvDBPoolMax: v}
		for k, val := range base {
			m[k] = val
		}
		return m
	}
	cfg, err := config.Load(env(with("7")), config.API)
	if err != nil || cfg.DBPoolMax != 7 {
		t.Fatalf("override: %+v, %v", cfg, err)
	}
	for _, bad := range []string{"0", "-1", "abc", "1.5", "201", "9999999999"} {
		_, err := config.Load(env(with(bad)), config.API)
		if err == nil || !strings.Contains(err.Error(), config.EnvDBPoolMax) {
			t.Fatalf("%q: want error naming %s, got %v", bad, config.EnvDBPoolMax, err)
		}
		if strings.Contains(err.Error(), bad) && len(bad) > 3 {
			t.Fatalf("%q: error echoes the value: %v", bad, err)
		}
	}
}

func TestLoadNeverPrintsValues(t *testing.T) {
	const secret = "postgres://user:s3cr3t-pass@db/x"
	_, err := config.Load(env(map[string]string{
		config.API.URLVar:  secret,
		config.EnvLogLevel: secret, // nivel inválido que lleva un valor con apariencia de secreto
	}), config.API)
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
	if strings.Contains(err.Error(), "s3cr3t") {
		t.Fatalf("error leaks a value: %q", err)
	}
	if !strings.Contains(err.Error(), config.EnvLogLevel) {
		t.Fatalf("error %q does not name %s", err, config.EnvLogLevel)
	}

	cfg, err := config.Load(env(map[string]string{config.API.URLVar: secret}), config.API)
	if err != nil {
		t.Fatal(err)
	}
	if s := cfg.String(); strings.Contains(s, "s3cr3t") {
		t.Fatalf("String leaks the secret: %q", s)
	}
}

func TestLoadDefaultsAndOverrides(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{config.API.URLVar: "x"}), config.API)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != config.DefaultHTTPAddr || cfg.LogLevel != slog.LevelInfo {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	cfg, err = config.Load(env(map[string]string{
		config.API.URLVar: "x", config.EnvHTTPAddr: "127.0.0.1:9", config.EnvLogLevel: "DEBUG",
	}), config.API)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9" || cfg.LogLevel != slog.LevelDebug {
		t.Fatalf("unexpected overrides: %+v", cfg)
	}
}

// El nombre de la variable es el que fijan ADR 0006 y ADR 0007 y el que usará
// el admin en Railway: se prueba con el literal, no con la constante.
func TestPoolMaxVariableNameMatchesTheDeploymentDocs(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{"DATABASE_URL_APP": "x", "DB_POOL_MAX_CONNS": "7"}), config.API)
	if err != nil || cfg.DBPoolMax != 7 {
		t.Fatalf("DB_POOL_MAX_CONNS=7 gave %+v, %v", cfg, err)
	}
}

func TestMailerAndEnvironmentDefaultsAreFailSafe(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{"DATABASE_URL_APP": "x"}), config.API)
	if err != nil {
		t.Fatal(err)
	}
	// Sin APP_ENV se asume producción: un olvido avisa de más, nunca de menos.
	if cfg.Mailer != config.MailerStub || !cfg.Production {
		t.Fatalf("defaults: mailer=%q production=%v", cfg.Mailer, cfg.Production)
	}
}

func TestMailerAndEnvironmentOverrides(t *testing.T) {
	cases := []struct {
		mailer, appEnv string
		wantMailer     string
		wantProd       bool
	}{
		{"stub", "production", "stub", true},
		{"none", "development", "none", false},
		{"NONE", "Development", "none", false},
		{"", "development", "stub", false},
		{"", "staging", "stub", true}, // staging cuenta como producción (ADR 0007)
		{"none", "STAGING", "none", true},
	}
	for _, tc := range cases {
		cfg, err := config.Load(env(map[string]string{"DATABASE_URL_APP": "x", "MAILER": tc.mailer, "APP_ENV": tc.appEnv}), config.API)
		if err != nil {
			t.Fatalf("%+v: %v", tc, err)
		}
		if cfg.Mailer != tc.wantMailer || cfg.Production != tc.wantProd {
			t.Errorf("%+v: got mailer=%q production=%v", tc, cfg.Mailer, cfg.Production)
		}
	}
}

func TestInvalidMailerOrEnvironmentNamesVariableNotValue(t *testing.T) {
	for _, v := range []struct{ name, value string }{{"MAILER", "smtp-S3CRET"}, {"APP_ENV", "qa-S3CRET"}} {
		_, err := config.Load(env(map[string]string{"DATABASE_URL_APP": "x", v.name: v.value}), config.API)
		if err == nil || !strings.Contains(err.Error(), v.name) || strings.Contains(err.Error(), "S3CRET") {
			t.Errorf("%s: error = %v", v.name, err)
		}
	}
}

func TestTrustClientIP(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
		bad   bool
	}{{"", false, false}, {"false", false, false}, {"true", true, false}, {"TRUE", true, false}, {"1", false, true}, {"yes-S3CRET", false, true}} {
		cfg, err := config.Load(env(map[string]string{"DATABASE_URL_APP": "x", "API_TRUST_CLIENT_IP": tc.value}), config.API)
		if tc.bad {
			if err == nil || !strings.Contains(err.Error(), "API_TRUST_CLIENT_IP") || strings.Contains(err.Error(), "S3CRET") {
				t.Errorf("%q: error = %v, want one that names the variable and not the value", tc.value, err)
			}
			continue
		}
		if err != nil || cfg.TrustClientIP != tc.want {
			t.Errorf("%q: TrustClientIP = %v, err = %v, want %v", tc.value, cfg.TrustClientIP, err, tc.want)
		}
	}
}
