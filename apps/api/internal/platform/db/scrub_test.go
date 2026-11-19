package db

import (
	"errors"
	"strings"
	"testing"
)

func TestScrubRedactsThePasswordOnlyWhereItIsASecret(t *testing.T) {
	cases := []struct {
		name, url, in, want string
	}{
		{"full URL", "postgres://u:s3cret@h/db", "dial postgres://u:s3cret@h/db failed", "dial <database url> failed"},
		{"userinfo of another form of the URL", "postgres://u:s3cret@h/db", "cannot parse `postgres://u:s3cret@h:bad/db`", "cannot parse `postgres://u:[REDACTED]@h:bad/db`"},
		{"keyword value", "postgres://u:s3cret@h/db", "bad conn: host=h password=s3cret user=u", "bad conn: host=h password=[REDACTED] user=u"},
		{"one-letter password keeps the rest intact", "postgres://u:x@h/db", "context deadline exceeded at host=h user=u", "context deadline exceeded at host=h user=u"},
		{"one-letter password in userinfo", "postgres://u:x@h/db", "see postgres://u:x@h:bad/db", "see postgres://u:[REDACTED]@h:bad/db"},
		{"password like a common word", "postgres://postgres:postgres@h/db", "failed to connect to host=h user=postgres", "failed to connect to host=h user=postgres"},
		{"no password", "postgres://u@h/db", "plain error", "plain error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := scrubbed(scrubber(tc.url), errors.New(tc.in)).Error()
			if got != tc.want {
				t.Fatalf("got  %q\nwant %q", got, tc.want)
			}
			if strings.Contains(tc.url, "s3cret") && strings.Contains(got, "s3cret") {
				t.Fatalf("password leaked: %q", got)
			}
		})
	}
}
