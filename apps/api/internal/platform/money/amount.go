package money

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// Escalas de data-model §2.
const (
	ScaleAmount int32 = 2  // importes de documento
	ScalePrice  int32 = 4  // precios y cantidades
	ScaleRate   int32 = 3  // tipo de cambio
	ScaleUnit   int32 = 10 // valor unitario (UBL admite hasta 10 decimales)
)

// ErrInvalid indica un texto que no es un decimal en notación plana.
var ErrInvalid = errors.New("invalid decimal")

// Amount es un decimal con la escala con que se serializa. El valor cero es
// "0" de escala 0; úsese New o Parse para fijar la escala.
type Amount struct {
	d     decimal.Decimal
	scale int32
}

// New devuelve d con la escala indicada para su serialización. No redondea:
// quien llama decide cómo; si d tiene más decimales que scale, String los mantiene.
func New(d decimal.Decimal, scale int32) Amount {
	if scale < 0 {
		scale = 0
	}
	if d.Exponent() < -scale {
		scale = -d.Exponent()
	}
	return Amount{d: d, scale: scale}
}

// Parse lee un decimal plano ("118.00", "-0.5"). Rechaza vacío, signo +, exponente,
// espacios, separadores de miles y más de maxScale decimales. La escala del
// resultado es la cantidad de decimales escritos.
func Parse(s string, maxScale int32) (Amount, error) {
	if err := validPlain(s); err != nil {
		return Amount{}, err
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Amount{}, fmt.Errorf("%w: %q", ErrInvalid, s)
	}
	scale := int32(0)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		scale = int32(len(s) - i - 1)
	}
	if scale > maxScale {
		return Amount{}, fmt.Errorf("%w: %q has more than %d decimals", ErrInvalid, s, maxScale)
	}
	return Amount{d: d, scale: scale}, nil
}

// validPlain acepta -?dígitos(.dígitos)? y nada más.
func validPlain(s string) error {
	t := strings.TrimPrefix(s, "-")
	whole, frac, hasDot := strings.Cut(t, ".")
	if whole == "" || (hasDot && frac == "") || len(s) > 64 {
		return fmt.Errorf("%w: %q", ErrInvalid, s)
	}
	for _, part := range []string{whole, frac} {
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return fmt.Errorf("%w: %q", ErrInvalid, s)
			}
		}
	}
	return nil
}

// Decimal devuelve el valor exacto.
func (a Amount) Decimal() decimal.Decimal { return a.d }

// Scale devuelve la escala con que se serializa.
func (a Amount) Scale() int32 { return a.scale }

// IsZero informa si vale cero.
func (a Amount) IsZero() bool { return a.d.IsZero() }

// String devuelve el decimal con su escala ("118.00").
func (a Amount) String() string { return a.d.StringFixed(a.scale) }

// MarshalJSON serializa como string decimal: "118.00", nunca como número.
func (a Amount) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

// UnmarshalJSON solo acepta un string JSON con decimal plano; un número JSON se
// rechaza porque ya perdió precisión en el cliente. Admite hasta ScaleUnit decimales.
func (a *Amount) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '"' {
		return fmt.Errorf("%w: a JSON string is required", ErrInvalid)
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	p, err := Parse(s, ScaleUnit)
	if err != nil {
		return err
	}
	*a = p
	return nil
}
