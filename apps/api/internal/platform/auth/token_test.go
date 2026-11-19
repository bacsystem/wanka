package auth_test

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
)

// body32 es el cuerpo de un token válido: 32 bytes en base64url sin relleno.
func body32(b byte) string {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = b + byte(i)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func TestParseAuthorizationHashesTheWholeTokenWithItsPrefix(t *testing.T) {
	tok := "wk_at_" + body32(7)
	got, err := auth.ParseAuthorization([]string{"Bearer " + tok})
	if err != nil {
		t.Fatal(err)
	}
	if want := sha256.Sum256([]byte(tok)); got != auth.TokenHash(want) {
		t.Fatalf("hash = %x, want sha256 of the full token %x", got, want)
	}
	if noPrefix := sha256.Sum256([]byte(body32(7))); got == auth.TokenHash(noPrefix) {
		t.Fatal("the hash ignores the prefix")
	}
}

func TestParseAuthorizationSchemeIsCaseInsensitive(t *testing.T) {
	tok := "wk_at_" + body32(1)
	for _, scheme := range []string{"Bearer", "bearer", "BEARER", "bEaReR"} {
		if _, err := auth.ParseAuthorization([]string{scheme + " " + tok}); err != nil {
			t.Errorf("%s: %v", scheme, err)
		}
	}
}

func TestParseAuthorizationRejectsEverythingElseWithoutAHash(t *testing.T) {
	good := body32(3)
	// El último carácter de un cuerpo de 43 caracteres solo puede llevar 2 bits
	// útiles: cambiarlo por uno con bits sueltos no decodifica de forma canónica.
	nonCanonical := good[:42] + "B"
	if raw, err := base64.RawURLEncoding.Strict().DecodeString(nonCanonical); err == nil {
		t.Fatalf("test setup: %q decodes canonically to %x", nonCanonical, raw)
	}
	cases := []struct {
		name   string
		values []string
	}{
		{"no header", nil},
		{"empty header", []string{""}},
		{"two headers", []string{"Bearer wk_at_" + good, "Bearer wk_at_" + good}},
		{"two headers, second empty", []string{"Bearer wk_at_" + good, ""}},
		{"other scheme", []string{"Basic wk_at_" + good}},
		{"scheme only", []string{"Bearer"}},
		{"scheme and space only", []string{"Bearer "}},
		{"no scheme", []string{"wk_at_" + good}},
		{"two spaces", []string{"Bearer  wk_at_" + good}},
		{"tab separator", []string{"Bearer\twk_at_" + good}},
		{"trailing space", []string{"Bearer wk_at_" + good + " "}},
		{"trailing text", []string{"Bearer wk_at_" + good + " extra"}},
		{"leading space", []string{" Bearer wk_at_" + good}},
		{"newline", []string{"Bearer wk_at_" + good + "\n"}},
		{"refresh token as access", []string{"Bearer wk_rt_" + good}},
		{"platform access token", []string{"Bearer wka_at_" + good}},
		{"platform refresh token", []string{"Bearer wka_rt_" + good}},
		{"prefix in upper case", []string{"Bearer WK_AT_" + good}},
		{"no prefix", []string{"Bearer " + good}},
		{"prefix only", []string{"Bearer wk_at_"}},
		{"42 characters", []string{"Bearer wk_at_" + good[:42]}},
		{"44 characters", []string{"Bearer wk_at_" + good + "A"}},
		{"padding", []string{"Bearer wk_at_" + good + "="}},
		{"standard alphabet", []string{"Bearer wk_at_" + strings.Repeat("+", 43)}},
		{"non-ASCII", []string{"Bearer wk_at_" + good[:42] + "é"}},
		{"non-canonical last character", []string{"Bearer wk_at_" + nonCanonical}},
		{"two prefixes", []string{"Bearer wk_at_wk_at_" + good}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := auth.ParseAuthorization(tc.values)
			if !errors.Is(err, auth.ErrMalformedToken) {
				t.Fatalf("err = %v, want ErrMalformedToken", err)
			}
			if h != (auth.TokenHash{}) {
				t.Fatal("a rejected token returned a hash")
			}
		})
	}
}

// El error no repite el valor recibido: así no llega a un log.
func TestParseAuthorizationErrorDoesNotEchoTheInput(t *testing.T) {
	tok := "wk_rt_" + body32(9)
	_, err := auth.ParseAuthorization([]string{"Bearer " + tok})
	if err == nil || strings.Contains(err.Error(), tok) || strings.Contains(err.Error(), body32(9)) {
		t.Fatalf("error echoes the token: %v", err)
	}
}
