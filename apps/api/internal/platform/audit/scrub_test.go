package audit

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Criterio 6: la lista de campos prohibidos enmascara a cualquier profundidad.
func TestScrubDiffMasksForbiddenFields(t *testing.T) {
	in := map[string]any{
		"name":          "Ana",
		"password":      "hunter2",
		"Password":      "hunter3",
		"new_password":  "hunter4",
		"contraseña":    "hunter5",
		"clave_sol":     "hunter6",
		"token":         "abc",
		"refreshToken":  "abc",
		"access-token":  "abc",
		"api_key":       "abc",
		"apiKey":        "abc",
		"API-KEY":       "abc",
		"secret":        "abc",
		"client_secret": "abc",
		"certificate":   "-----BEGIN CERTIFICATE-----",
		"certificado":   "MIIB...",
		"private_key":   "abc",
		"password_hash": "abc",
		"authorization": "Bearer abc",
		"nested":        map[string]any{"deep": map[string]any{"password": "p", "ok": "fine"}},
		"list":          []any{map[string]any{"token": "t", "n": 1}, "plain"},
		"by_value":      "wk_at_abcdefghijklmnopqrstuvwxyz",
		"khipu":         "fk_live_123",
		"qty":           12,
	}
	raw, err := scrubDiff(in)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"password", "Password", "new_password", "contraseña", "clave_sol", "token", "refreshToken", "access-token",
		"api_key", "apiKey", "API-KEY", "secret", "client_secret", "certificate", "certificado", "private_key", "password_hash", "authorization", "by_value", "khipu"} {
		if out[k] != Redacted {
			t.Errorf("%q = %v, want redacted", k, out[k])
		}
	}
	if out["name"] != "Ana" || out["qty"] != float64(12) {
		t.Errorf("harmless fields changed: %v", out)
	}
	nested := out["nested"].(map[string]any)["deep"].(map[string]any)
	if nested["password"] != Redacted || nested["ok"] != "fine" {
		t.Errorf("nested = %v", nested)
	}
	item := out["list"].([]any)[0].(map[string]any)
	if item["token"] != Redacted || item["n"] != float64(1) || out["list"].([]any)[1] != "plain" {
		t.Errorf("list = %v", out["list"])
	}
	for _, leak := range []string{"hunter", "BEGIN CERTIFICATE", "MIIB", "fk_live", "wk_at_abc"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("leaks %q: %s", leak, raw)
		}
	}
}

// Un struct con tags json también pasa por la lista; los enteros grandes conservan su precisión.
func TestScrubDiffStructsAndNumbers(t *testing.T) {
	type payload struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Big      int64  `json:"big"`
	}
	raw, err := scrubDiff(payload{User: "ana", Password: "x", Big: 9007199254740993})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(raw), `{"big":9007199254740993,"password":"[REDACTED]","user":"ana"}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestScrubDiffNilAndLimits(t *testing.T) {
	if raw, err := scrubDiff(nil); raw != nil || err != nil {
		t.Errorf("nil diff = %s, %v", raw, err)
	}
	var m map[string]any
	if raw, err := scrubDiff(m); raw != nil || err != nil {
		t.Errorf("nil map = %s, %v", raw, err)
	}
	deep := map[string]any{}
	cur := deep
	for i := 0; i < 40; i++ {
		next := map[string]any{}
		cur["a"] = next
		cur = next
	}
	if _, err := scrubDiff(deep); err == nil {
		t.Error("a diff nested 40 levels was accepted")
	}
}

func TestForbiddenKey(t *testing.T) {
	for k, want := range map[string]bool{
		"password": true, "PASSWORD": true, "Contraseña": true, "x-api-key": true, "tokens": true, "email": false, "name": false,
		"entity_id": false, "status": false, "total": false, "footprint": false,
	} {
		if got := forbiddenKey(k); got != want {
			t.Errorf("forbiddenKey(%q) = %v, want %v", k, got, want)
		}
	}
}

// H1: un diff ya serializado (string, []byte o json.RawMessage) se sanea igual que un objeto.
func TestScrubDiffSanitizesSerializedJSON(t *testing.T) {
	const doc = `{"password":"hunter2","nested":{"api_key":"k"},"ok":1}`
	for name, in := range map[string]any{"string": doc, "bytes": []byte(doc), "raw message": json.RawMessage(doc)} {
		t.Run(name, func(t *testing.T) {
			raw, err := scrubDiff(in)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "hunter2") || strings.Contains(string(raw), `"k"`) || !strings.Contains(string(raw), `"ok":1`) {
				t.Errorf("not sanitized: %s", raw)
			}
		})
	}
}

func TestScrubDiffRequiresAnObject(t *testing.T) {
	for name, in := range map[string]any{
		"array": []any{1}, "string json scalar": `"hunter2"`, "number": 7, "plain text": "password=hunter2",
		"bytes array": []byte(`[{"password":"x"}]`), "trailing data": `{"a":1} {"password":"x"}`, "bool": true,
	} {
		t.Run(name, func(t *testing.T) {
			if raw, err := scrubDiff(in); !errors.Is(err, ErrInvalidEvent) {
				t.Errorf("got %s, %v; want ErrInvalidEvent", raw, err)
			}
		})
	}
}

// H2: nombres cortos por palabra, hmac, access key, y valores con forma de credencial.
func TestScrubDiffExtraNamesAndValueShapes(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2ln"
	raw, err := scrubDiff(map[string]any{
		"email_code_hmac": "h", "access_key": "k", "accessKey": "k", "otp": "123456", "email_otp": "1", "emailOtp": "1", "pin": "1234", "jwt": "x",
		"note": "Bearer abc.def", "lower": "bearer abc", "shape": jwt, "footprint": "keep", "shipping": "keep", "mapping": "keep", "plain": "eyJ.only.one",
	})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"email_code_hmac", "access_key", "accessKey", "otp", "email_otp", "emailOtp", "pin", "jwt", "note", "lower", "shape"} {
		if out[k] != Redacted {
			t.Errorf("%q = %v, want redacted", k, out[k])
		}
	}
	for _, k := range []string{"footprint", "shipping", "mapping", "plain"} {
		if out[k] == Redacted {
			t.Errorf("%q was masked", k)
		}
	}
}
