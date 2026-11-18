package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("log line is not JSON: %v: %q", err, buf.String())
	}
	return m
}

func TestJSONWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(&buf, slog.LevelInfo)
	ctx := logging.WithRequestID(context.Background(), "req-123")
	log.InfoContext(ctx, "hello", "k", "v")
	m := decode(t, &buf)
	if m["request_id"] != "req-123" || m["msg"] != "hello" || m["k"] != "v" {
		t.Fatalf("unexpected log: %v", m)
	}
}

func TestNoRequestIDWhenAbsent(t *testing.T) {
	var buf bytes.Buffer
	logging.New(&buf, slog.LevelInfo).Info("hello")
	if _, ok := decode(t, &buf)["request_id"]; ok {
		t.Fatal("request_id must be omitted when there is none")
	}
}

func TestRedactsSensitiveKeys(t *testing.T) {
	keys := []string{
		"authorization", "Authorization", "token", "access_token", "refresh_token", "password", "passwd",
		"secret", "client_secret", "api_key", "apikey", "cookie", "set-cookie", "database_url", "otp", "email", "dni", "phone",
	}
	for _, k := range keys {
		t.Run(k, func(t *testing.T) {
			var buf bytes.Buffer
			log := logging.New(&buf, slog.LevelInfo)
			log.Info("x", k, "SENSITIVE-VALUE")
			log.Info("x", slog.Group("req", slog.String(k, "SENSITIVE-VALUE")))
			if strings.Contains(buf.String(), "SENSITIVE-VALUE") {
				t.Fatalf("key %q leaked: %s", k, buf.String())
			}
		})
	}
}

func TestLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	logging.New(&buf, slog.LevelWarn).Info("hidden")
	if buf.Len() != 0 {
		t.Fatalf("info should be filtered: %q", buf.String())
	}
}
