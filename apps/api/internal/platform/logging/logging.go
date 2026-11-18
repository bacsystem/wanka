// Package logging builds the process logger: JSON from log/slog, with the
// request id taken from the context and sensitive attributes redacted by name.
package logging

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

type ctxKey struct{}

// WithRequestID returns a context that carries id; loggers built by New add it
// to every record emitted with that context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestID returns the request id stored in ctx, or "".
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// New returns a JSON logger writing to w at the given level.
func New(w io.Writer, level slog.Level) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level, ReplaceAttr: redact})
	return slog.New(requestIDHandler{h})
}

const redacted = "[REDACTED]"

// Substrings that mark a key as a secret, and exact keys that are personal data.
var (
	secretParts = []string{"token", "secret", "password", "passwd", "authorization", "cookie", "api_key", "apikey", "database_url", "otp"}
	personal    = map[string]bool{"email": true, "dni": true, "ruc": true, "phone": true, "telefono": true, "address": true, "direccion": true}
)

func redact(_ []string, a slog.Attr) slog.Attr {
	k := strings.ToLower(strings.ReplaceAll(a.Key, "-", "_"))
	sensitive := personal[k]
	for _, p := range secretParts {
		if strings.Contains(k, p) {
			sensitive = true
			break
		}
	}
	if sensitive {
		return slog.String(a.Key, redacted)
	}
	return a
}

// requestIDHandler adds request_id when the context carries one.
type requestIDHandler struct{ slog.Handler }

func (h requestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return requestIDHandler{h.Handler.WithAttrs(attrs)}
}

func (h requestIDHandler) WithGroup(name string) slog.Handler {
	return requestIDHandler{h.Handler.WithGroup(name)}
}
