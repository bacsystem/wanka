package auth_test

import (
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"regexp"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
)

func TestClientIP(t *testing.T) {
	req := func(remote string, hdr ...string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
		r.RemoteAddr = remote
		for _, h := range hdr {
			r.Header.Add("X-Client-Ip", h)
		}
		return r
	}
	none := netip.Addr{}
	for _, tc := range []struct {
		name  string
		r     *http.Request
		trust bool
		want  netip.Addr
	}{
		{"untrusted: socket", req("198.51.100.1:5000", "203.0.113.7"), false, netip.MustParseAddr("198.51.100.1")},
		{"untrusted: ipv6 socket", req("[2001:db8::1]:5000"), false, netip.MustParseAddr("2001:db8::1")},
		{"untrusted: v4-mapped socket", req("[::ffff:198.51.100.1]:5000"), false, netip.MustParseAddr("198.51.100.1")},
		{"untrusted: unparsable socket", req("garbage"), false, none},
		{"trusted: ipv4", req("198.51.100.1:5000", "203.0.113.7"), true, netip.MustParseAddr("203.0.113.7")},
		{"trusted: ipv6", req("198.51.100.1:5000", "2001:db8::7"), true, netip.MustParseAddr("2001:db8::7")},
		{"trusted: v4-mapped is unmapped", req("198.51.100.1:5000", "::ffff:203.0.113.7"), true, netip.MustParseAddr("203.0.113.7")},
		{"trusted: absent", req("198.51.100.1:5000"), true, none},
		{"trusted: repeated", req("198.51.100.1:5000", "203.0.113.1", "203.0.113.2"), true, none},
		{"trusted: a list", req("198.51.100.1:5000", "203.0.113.1, 203.0.113.2"), true, none},
		{"trusted: with port", req("198.51.100.1:5000", "203.0.113.1:80"), true, none},
		{"trusted: with zone", req("198.51.100.1:5000", "fe80::1%eth0"), true, none},
		{"trusted: not an address", req("198.51.100.1:5000", "unknown"), true, none},
		{"trusted: empty", req("198.51.100.1:5000", ""), true, none},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := auth.ClientIP(tc.r, tc.trust); got != tc.want {
				t.Fatalf("ClientIP = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIssuedTokens(t *testing.T) {
	for name, tc := range map[string]struct {
		issue func() (auth.IssuedToken, error)
		re    *regexp.Regexp
	}{
		"access":    {auth.NewAccessToken, regexp.MustCompile(`^wk_at_[A-Za-z0-9_-]{43}$`)},
		"refresh":   {auth.NewRefreshToken, regexp.MustCompile(`^wk_rt_[A-Za-z0-9_-]{43}$`)},
		"challenge": {auth.NewChallengeToken, regexp.MustCompile(`^wk_ch_[A-Za-z0-9_-]{43}$`)},
	} {
		t.Run(name, func(t *testing.T) {
			a, err := tc.issue()
			if err != nil || !tc.re.MatchString(a.Value) {
				t.Fatalf("token = %q, err = %v", a.Value, err)
			}
			if a.Hash != sha256.Sum256([]byte(a.Value)) {
				t.Fatal("the hash is not sha256 of the whole token, prefix included")
			}
			b, _ := tc.issue()
			if a.Value == b.Value {
				t.Fatal("two tokens are equal")
			}
		})
	}
	// El acceso emitido pasa la validación de Authorization con el mismo hash.
	a, _ := auth.NewAccessToken()
	h, err := auth.ParseAuthorization([]string{"Bearer " + a.Value})
	if err != nil || h != a.Hash {
		t.Fatalf("ParseAuthorization = %x, %v", h, err)
	}
	// Un refresh o un challenge no sirven como acceso.
	for _, f := range []func() (auth.IssuedToken, error){auth.NewRefreshToken, auth.NewChallengeToken} {
		tok, _ := f()
		if _, err := auth.ParseAuthorization([]string{"Bearer " + tok.Value}); err == nil {
			t.Fatalf("%s accepted as an access token", tok.Value[:6])
		}
	}
	if s := (auth.IssuedToken{Value: "wk_at_secret"}).String(); s == "wk_at_secret" {
		t.Fatal("String leaks the token")
	}
}
