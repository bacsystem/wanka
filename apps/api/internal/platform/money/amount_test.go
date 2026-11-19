package money_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/bacsystem/wanka/apps/api/internal/platform/money"
)

func TestMarshalKeepsScale(t *testing.T) {
	tests := []struct {
		name string
		a    money.Amount
		want string
	}{
		{"amount", money.New(decimal.RequireFromString("118"), money.ScaleAmount), `"118.00"`},
		{"price", money.New(decimal.RequireFromString("2.5"), money.ScalePrice), `"2.5000"`},
		{"rate", money.New(decimal.RequireFromString("3.756"), money.ScaleRate), `"3.756"`},
		{"negative", money.New(decimal.RequireFromString("-0.5"), money.ScaleAmount), `"-0.50"`},
		{"zero", money.New(decimal.Zero, money.ScaleAmount), `"0.00"`},
		{"never lose digits", money.New(decimal.RequireFromString("1.2345"), money.ScaleAmount), `"1.2345"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.a)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tc.want {
				t.Fatalf("got %s, want %s", b, tc.want)
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{`"118.00"`, "118.00", false},
		{`"0.5"`, "0.5", false},
		{`"-3"`, "-3", false},
		{`"1.2345678901"`, "1.2345678901", false},
		{`118.00`, "", true}, // número JSON: pierde precisión
		{`null`, "", true},   // no se acepta nulo
		{`""`, "", true},
		{`"1e3"`, "", true},
		{`"+1"`, "", true},
		{`" 1"`, "", true},
		{`"1,000.00"`, "", true},
		{`"1."`, "", true},
		{`".5"`, "", true},
		{`"--1"`, "", true},
		{`"1.23456789012"`, "", true}, // más de 10 decimales
		{`"NaN"`, "", true},
		{`"00012.50"`, "", true}, // ceros a la izquierda
		{`"01"`, "", true},
		{`"-0"`, "", true}, // cero negativo
		{`"-0.00"`, "", true},
		{`"0"`, "0", false},
		{`"0.00"`, "0.00", false},
		{`"-0.01"`, "-0.01", false},
		{`"10"`, "10", false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			var a money.Amount
			err := json.Unmarshal([]byte(tc.in), &a)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %s", a)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if a.String() != tc.want {
				t.Fatalf("got %s, want %s", a, tc.want)
			}
		})
	}
}

func TestParseMaxScale(t *testing.T) {
	if _, err := money.Parse("1.234", money.ScaleRate); err != nil {
		t.Fatal(err)
	}
	_, err := money.Parse("1.2345", money.ScaleRate)
	if !errors.Is(err, money.ErrInvalid) {
		t.Fatalf("got %v, want ErrInvalid", err)
	}
}

// El tipo no expone float64: ningún método público lo recibe ni lo devuelve.
func TestNoFloatInPublicAPI(t *testing.T) {
	ty := reflect.TypeOf(money.Amount{})
	for i := 0; i < ty.NumMethod(); i++ {
		m := ty.Method(i).Type
		for j := 0; j < m.NumIn(); j++ {
			if k := m.In(j).Kind(); k == reflect.Float32 || k == reflect.Float64 {
				t.Fatalf("%s takes a float", ty.Method(i).Name)
			}
		}
		for j := 0; j < m.NumOut(); j++ {
			if k := m.Out(j).Kind(); k == reflect.Float32 || k == reflect.Float64 {
				t.Fatalf("%s returns a float", ty.Method(i).Name)
			}
		}
	}
}
