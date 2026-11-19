package idempotency

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCanonicalBody(t *testing.T) {
	json := "application/json; charset=utf-8"
	cases := []struct {
		name, ct, a, b string
		same           bool
	}{
		{"field order", json, `{"a":1,"b":2}`, `{"b":2,"a":1}`, true},
		{"whitespace", json, `{"a": [1, 2]}`, "{\n\"a\":[1,2]\n}", true},
		{"nested field order", json, `{"x":{"a":1,"b":2},"y":[{"q":1,"r":2}]}`, `{"y":[{"r":2,"q":1}],"x":{"b":2,"a":1}}`, true},
		{"unicode escape", json, `{"a":"é"}`, `{"a":"é"}`, true},
		{"array order matters", json, `{"a":[1,2]}`, `{"a":[2,1]}`, false},
		{"value matters", json, `{"a":1}`, `{"a":2}`, false},
		{"big numbers are not rounded", json, `{"a":12345678901234567890}`, `{"a":12345678901234567891}`, false},
		{"string versus number", json, `{"a":"1"}`, `{"a":1}`, false},
		{"html characters are not escaped differently", json, `{"a":"<b>&"}`, `{"a":"<b>&"}`, true},
		{"invalid json is compared as bytes", json, `{"a":1`, `{ "a":1`, false},
		{"trailing data is compared as bytes", json, `{"a":1} x`, `{"a": 1} x`, false},
		{"not json is compared as bytes", "text/plain", `{"a":1,"b":2}`, `{"b":2,"a":1}`, false},
		{"no content type is compared as bytes", "", `{"a":1,"b":2}`, `{"b":2,"a":1}`, false},
		{"empty body", json, ``, ``, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bytes.Equal(canonicalBody(tc.ct, []byte(tc.a)), canonicalBody(tc.ct, []byte(tc.b)))
			if got != tc.same {
				t.Fatalf("canonical(%q) == canonical(%q) is %v, want %v", tc.a, tc.b, got, tc.same)
			}
		})
	}
}

func TestFingerprintCoversMethodPathQueryAndBody(t *testing.T) {
	mk := func(method, target, body string) []byte {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		return fingerprint(r, []byte(body))
	}
	base := mk(http.MethodPost, "/v1/sales", `{"a":1}`)
	if !bytes.Equal(base, mk(http.MethodPost, "/v1/sales", `{ "a" : 1 }`)) {
		t.Error("the same request with other spacing changed the fingerprint")
	}
	for name, other := range map[string][]byte{
		"path":   mk(http.MethodPost, "/v1/notes", `{"a":1}`),
		"query":  mk(http.MethodPost, "/v1/sales?x=1", `{"a":1}`),
		"method": mk(http.MethodPut, "/v1/sales", `{"a":1}`),
		"body":   mk(http.MethodPost, "/v1/sales", `{"a":2}`),
	} {
		if bytes.Equal(base, other) {
			t.Errorf("a different %s kept the fingerprint", name)
		}
	}
	if len(base) != 32 {
		t.Errorf("fingerprint is %d bytes, want 32", len(base))
	}
}

func TestParseKey(t *testing.T) {
	const ok = "0190C1A2-1111-7000-8000-00000000000A"
	if k, valid := parseKey([]string{ok}); !valid || k != strings.ToLower(ok) {
		t.Errorf("parseKey(%q) = %q, %v", ok, k, valid)
	}
	for _, bad := range [][]string{nil, {}, {""}, {"x"}, {ok, ok}, {ok + " "}, {" " + ok}, {strings.Replace(ok, "-", "", 1)}, {strings.Replace(ok, "A", "G", 1)}} {
		if k, valid := parseKey(bad); valid {
			t.Errorf("parseKey(%q) = %q, true", bad, k)
		}
	}
}
