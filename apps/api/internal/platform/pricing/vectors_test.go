package pricing_test

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/pricing"
)

// wantResult es la forma del resultado esperado en testdata/vectors.json.
type wantResult struct {
	Mode  string `json:"mode"`
	Lines []struct {
		UnitValue string `json:"unitValue"`
		Base      string `json:"base"`
		IGV       string `json:"igv"`
		Total     string `json:"total"`
	} `json:"lines"`
	Totals map[string]string `json:"totals"`
}

type vector struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
	Want  *wantResult     `json:"want"`
	Error *struct {
		Code  string `json:"code"`
		Field string `json:"field"`
	} `json:"error"`
}

func loadVectors(t testing.TB) []vector {
	t.Helper()
	b, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vs []vector
	if err := json.Unmarshal(b, &vs); err != nil {
		t.Fatal(err)
	}
	return vs
}

func TestVectors(t *testing.T) {
	vs := loadVectors(t)
	if len(vs) < 200 {
		t.Fatalf("only %d vectors, the story requires at least 200", len(vs))
	}
	modes := map[string]int{}
	for _, v := range vs {
		t.Run(v.Name, func(t *testing.T) {
			var in pricing.Input
			if err := json.Unmarshal(v.Input, &in); err != nil {
				t.Fatal(err)
			}
			got, err := pricing.Compute(in)
			if v.Error != nil {
				var e *pricing.Error
				if !errors.As(err, &e) || e.Code != v.Error.Code || e.Field != v.Error.Field {
					t.Fatalf("err = %v, want %s on %s", err, v.Error.Code, v.Error.Field)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var have wantResult
			b, _ := json.Marshal(got)
			if err := json.Unmarshal(b, &have); err != nil {
				t.Fatal(err)
			}
			modes[have.Mode]++
			if have.Mode != v.Want.Mode || len(have.Lines) != len(v.Want.Lines) {
				t.Fatalf("mode %s (%d lines), want %s (%d lines)", have.Mode, len(have.Lines), v.Want.Mode, len(v.Want.Lines))
			}
			for i := range have.Lines {
				if have.Lines[i] != v.Want.Lines[i] {
					t.Fatalf("line %d = %+v, want %+v", i, have.Lines[i], v.Want.Lines[i])
				}
			}
			for k, w := range v.Want.Totals {
				if have.Totals[k] != w {
					t.Errorf("totals.%s = %s, want %s", k, have.Totals[k], w)
				}
			}
			if len(have.Totals) != len(v.Want.Totals) {
				t.Errorf("totals has %d fields, want %d", len(have.Totals), len(v.Want.Totals))
			}
		})
	}
	// El archivo debe cubrir los dos modos (carritos cuyo desvío cruza 0.50).
	if modes["base_rate"] == 0 || modes["price"] == 0 {
		t.Fatalf("vectors cover modes %v", modes)
	}
}
