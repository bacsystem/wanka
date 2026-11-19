package pricing_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/pricing"
)

type wantLine struct{ unit, base, igv, total string }

func checkLine(t *testing.T, got pricing.LineResult, w wantLine) {
	t.Helper()
	if w.unit != "" && got.UnitValue.String() != w.unit {
		t.Errorf("unitValue = %s, want %s", got.UnitValue, w.unit)
	}
	if got.Base.String() != w.base || got.IGV.String() != w.igv || got.Total.String() != w.total {
		t.Errorf("line = base %s igv %s total %s, want base %s igv %s total %s",
			got.Base, got.IGV, got.Total, w.base, w.igv, w.total)
	}
}

func TestAfectaciones(t *testing.T) {
	tests := []struct {
		name string
		line pricing.Line
		want wantLine
		// totales esperados: gravado, exonerado, inafecto, gratuito, igv, freeIGV, total
		tot [7]string
	}{
		{"gravado 18", line(t, "1", "118", pricing.Gravado, "0"),
			wantLine{"100.0000", "100.00", "18.00", "118.00"},
			[7]string{"100.00", "0.00", "0.00", "0.00", "18.00", "0.00", "118.00"}},
		{"gravado quantity and discount", line(t, "2", "59", pricing.Gravado, "18"),
			wantLine{"50.0000", "84.75", "15.25", "100.00"},
			[7]string{"84.75", "0.00", "0.00", "0.00", "15.25", "0.00", "100.00"}},
		{"exonerado", line(t, "2", "50", pricing.Exonerado, "0"),
			wantLine{"", "100.00", "0.00", "100.00"},
			[7]string{"0.00", "100.00", "0.00", "0.00", "0.00", "0.00", "100.00"}},
		{"inafecto", line(t, "3", "10", pricing.Inafecto, "5"),
			wantLine{"", "25.00", "0.00", "25.00"},
			[7]string{"0.00", "0.00", "25.00", "0.00", "0.00", "0.00", "25.00"}},
		{"gratuito: informative IGV, adds nothing", line(t, "1", "118", pricing.Gratuito, "0"),
			wantLine{"", "100.00", "18.00", "0.00"},
			[7]string{"0.00", "0.00", "0.00", "100.00", "0.00", "18.00", "0.00"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := compute(t, pen(t, tc.line))
			checkLine(t, r.Lines[0], tc.want)
			tt := r.Totals
			got := [7]string{tt.Gravado.String(), tt.Exonerado.String(), tt.Inafecto.String(), tt.Gratuito.String(),
				tt.IGV.String(), tt.FreeIGV.String(), tt.Total.String()}
			if got != tc.tot {
				t.Errorf("totals = %v, want %v", got, tc.tot)
			}
		})
	}
}

func TestRoundingHalfUp(t *testing.T) {
	tests := []struct {
		name       string
		qty, price string
		aff        pricing.Afectacion
		total      string
	}{
		{"0.005 rounds up", "0.5", "0.01", pricing.Exonerado, "0.01"},
		{"1.005 rounds up", "1", "1.005", pricing.Inafecto, "1.01"},
		{"0.0049 rounds down", "1", "0.0049", pricing.Exonerado, "0.00"},
		{"2.675 rounds up", "1", "2.675", pricing.Exonerado, "2.68"},
		{"0.25 x 0.02", "0.25", "0.02", pricing.Exonerado, "0.01"},
		{"zero price", "5", "0", pricing.Exonerado, "0.00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := compute(t, pen(t, line(t, tc.qty, tc.price, tc.aff, "0")))
			if got := r.Totals.Total.String(); got != tc.total {
				t.Fatalf("total = %s, want %s", got, tc.total)
			}
		})
	}
}

func TestRoundingGravadoBase(t *testing.T) {
	// 0.59 / 1.18 = 0.5 exacto; 0.03 / 1.18 = 0.0254 -> base 0.03, IGV 0.00
	r := compute(t, pen(t, line(t, "1", "0.03", pricing.Gravado, "0")))
	checkLine(t, r.Lines[0], wantLine{"0.0254", "0.03", "0.00", "0.03"})
	// 1.77 / 1.18 = 1.5 exacto; 0.885 -> total 0.89 (half-up), base 0.75, IGV 0.14
	r = compute(t, pen(t, line(t, "1", "0.885", pricing.Gravado, "0")))
	checkLine(t, r.Lines[0], wantLine{"", "0.75", "0.14", "0.89"})
}

func TestZeroTotals(t *testing.T) {
	r := compute(t, pen(t, line(t, "1", "0", pricing.Gravado, "0")))
	if r.Totals.Total.String() != "0.00" || r.Totals.IGV.String() != "0.00" {
		t.Fatalf("totals = %+v", r.Totals)
	}
	r = compute(t, pen(t, line(t, "2", "50", pricing.Gravado, "100")))
	if r.Totals.Total.String() != "0.00" {
		t.Fatalf("fully discounted line: total = %s", r.Totals.Total)
	}
}

func TestModeSwitchAt050(t *testing.T) {
	// Cada línea de 0.10 desvía 0.01 entre modos: 50 líneas = 0.50 (no cruza), 51 = 0.51 (cruza).
	tests := []struct {
		n    int
		mode pricing.Mode
		tot  string
	}{
		{49, pricing.ModePrice, "4.90"},
		{50, pricing.ModePrice, "5.00"},
		{51, pricing.ModeBaseRate, "4.59"},
	}
	for _, tc := range tests {
		t.Run(string(tc.mode), func(t *testing.T) {
			lines := make([]pricing.Line, tc.n)
			for i := range lines {
				lines[i] = line(t, "1", "0.10", pricing.Gravado, "0")
			}
			r := compute(t, pen(t, lines...))
			if r.Mode != tc.mode || r.Totals.Total.String() != tc.tot {
				t.Fatalf("mode %s total %s, want %s %s", r.Mode, r.Totals.Total, tc.mode, tc.tot)
			}
		})
	}
}

func TestModeIgnoresNonGravado(t *testing.T) {
	lines := make([]pricing.Line, 0, 100)
	for range 30 {
		lines = append(lines, line(t, "1", "0.10", pricing.Gravado, "0"))
	}
	for range 70 {
		lines = append(lines, line(t, "1", "0.10", pricing.Exonerado, "0"), line(t, "1", "0.10", pricing.Gratuito, "0"))
	}
	if r := compute(t, pen(t, lines...)); r.Mode != pricing.ModePrice {
		t.Fatalf("mode = %s, want price", r.Mode)
	}
}

func TestGlobalDiscount(t *testing.T) {
	in := pen(t, line(t, "1", "118", pricing.Gravado, "0"), line(t, "1", "50", pricing.Exonerado, "0"))
	in.GlobalDiscount = amt(t, "16.80") // 118:50 -> 11.80 y 5.00
	r := compute(t, in)
	tt := r.Totals
	// gravado neto 106.20 -> base 90.00 (baja 10.00), IGV 16.20; exonerado 45.00 (baja 5.00)
	if tt.Subtotal.String() != "150.00" || tt.Discounts.String() != "15.00" || tt.IGV.String() != "16.20" || tt.Total.String() != "151.20" {
		t.Fatalf("totals = %+v", tt)
	}
}

func TestGlobalDiscountSplitTieIsStable(t *testing.T) {
	// 0.01 entre tres grupos iguales: el centavo lo gana el primero (gravado), siempre.
	in := pen(t, line(t, "1", "10", pricing.Inafecto, "0"), line(t, "1", "10", pricing.Exonerado, "0"), line(t, "1", "10", pricing.Gravado, "0"))
	in.GlobalDiscount = amt(t, "0.01")
	a := compute(t, in)
	in.Lines[0], in.Lines[2] = in.Lines[2], in.Lines[0]
	b := compute(t, in)
	if fmt.Sprintf("%+v", a.Totals) != fmt.Sprintf("%+v", b.Totals) {
		t.Fatalf("order changed totals:\n%+v\n%+v", a.Totals, b.Totals)
	}
	if a.Totals.Total.String() != "29.99" {
		t.Fatalf("total = %s", a.Totals.Total)
	}
}

func TestOtherChargesAdd(t *testing.T) {
	in := pen(t, line(t, "1", "118", pricing.Gravado, "0"))
	in.OtherCharges = amt(t, "2.50")
	if got := compute(t, in).Totals.Total.String(); got != "120.50" {
		t.Fatalf("total = %s", got)
	}
}

func TestCurrency(t *testing.T) {
	l := line(t, "1", "118", pricing.Gravado, "0")
	tests := []struct {
		name  string
		cur   pricing.Currency
		rate  string
		code  string
		field string
	}{
		{"usd without rate", pricing.USD, "", pricing.CodeExchangeRateMissing, "exchangeRate"},
		{"pen with rate", pricing.PEN, "3.756", pricing.CodeValidationFailed, "exchangeRate"},
		{"usd 4 decimals", pricing.USD, "3.7561", pricing.CodeValidationFailed, "exchangeRate"},
		{"usd zero", pricing.USD, "0", pricing.CodeValidationFailed, "exchangeRate"},
		{"usd negative", pricing.USD, "-3.7", pricing.CodeValidationFailed, "exchangeRate"},
		{"usd too big", pricing.USD, "1000", pricing.CodeValidationFailed, "exchangeRate"},
		{"unknown currency", pricing.Currency("EUR"), "", pricing.CodeValidationFailed, "currency"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := pen(t, l)
			in.Currency = tc.cur
			if tc.rate != "" {
				a := amt(t, tc.rate)
				in.ExchangeRate = &a
			}
			_, err := pricing.Compute(in)
			var e *pricing.Error
			if !errors.As(err, &e) || e.Code != tc.code || e.Field != tc.field {
				t.Fatalf("err = %v, want %s on %s", err, tc.code, tc.field)
			}
		})
	}

	t.Run("usd keeps its own currency and rate", func(t *testing.T) {
		in := pen(t, l)
		in.Currency = pricing.USD
		a := amt(t, "3.756")
		in.ExchangeRate = &a
		r := compute(t, in)
		if r.Currency != pricing.USD || r.ExchangeRate == nil || r.ExchangeRate.String() != "3.756" {
			t.Fatalf("result = %+v", r)
		}
		// El monto no se convierte: los 118 siguen siendo 118 de la moneda del documento.
		if r.Totals.Total.String() != "118.00" {
			t.Fatalf("total = %s", r.Totals.Total)
		}
	})
}

func TestTotalsMismatch(t *testing.T) {
	in := pen(t, line(t, "1", "118", pricing.Gravado, "0"))
	bad, ok := amt(t, "117.99"), amt(t, "118.00")

	in.Declared = &pricing.Declared{Total: &ok}
	compute(t, in) // cuadra

	in.Declared = &pricing.Declared{Total: &bad}
	_, err := pricing.Compute(in)
	var e *pricing.Error
	if !errors.As(err, &e) || e.Code != pricing.CodeTotalsMismatch || e.Field != "declared.total" {
		t.Fatalf("err = %v", err)
	}
	if e.Expected.String() != "118.00" || e.Got.String() != "117.99" || e.Difference.String() != "-0.01" {
		t.Fatalf("detail = %+v", e)
	}

	igv := amt(t, "18.01")
	in.Declared = &pricing.Declared{Total: &ok, IGV: &igv}
	_, err = pricing.Compute(in)
	if !errors.As(err, &e) || e.Field != "declared.igv" || e.Difference.String() != "0.01" {
		t.Fatalf("err = %v / %+v", err, e)
	}
}

func TestValidation(t *testing.T) {
	big := "9999999999.9999"
	tests := []struct {
		name  string
		mod   func(*pricing.Input)
		field string
	}{
		{"no lines", func(in *pricing.Input) { in.Lines = nil }, "lines"},
		{"zero quantity", func(in *pricing.Input) { in.Lines[0].Quantity = amt(t, "0") }, "lines[0].quantity"},
		{"negative quantity", func(in *pricing.Input) { in.Lines[0].Quantity = amt(t, "-1") }, "lines[0].quantity"},
		{"5 decimals quantity", func(in *pricing.Input) { in.Lines[0].Quantity = amt(t, "1.00001") }, "lines[0].quantity"},
		{"negative price", func(in *pricing.Input) { in.Lines[0].UnitPrice = amt(t, "-1") }, "lines[0].unitPrice"},
		{"5 decimals price", func(in *pricing.Input) { in.Lines[0].UnitPrice = amt(t, "1.00001") }, "lines[0].unitPrice"},
		{"negative discount", func(in *pricing.Input) { in.Lines[0].Discount = amt(t, "-1") }, "lines[0].discount"},
		{"3 decimals discount", func(in *pricing.Input) { in.Lines[0].Discount = amt(t, "0.001") }, "lines[0].discount"},
		{"discount over gross", func(in *pricing.Input) { in.Lines[0].Discount = amt(t, "100.01") }, "lines[0].discount"},
		{"unknown afectacion", func(in *pricing.Input) { in.Lines[0].Afectacion = "x" }, "lines[0].afectacion"},
		{"line over numeric(14,2)", func(in *pricing.Input) {
			in.Lines[0].Quantity, in.Lines[0].UnitPrice = amt(t, big), amt(t, big)
		}, "lines[0].unitPrice"},
		{"global discount over lines", func(in *pricing.Input) { in.GlobalDiscount = amt(t, "100.01") }, "globalDiscount"},
		{"negative global discount", func(in *pricing.Input) { in.GlobalDiscount = amt(t, "-1") }, "globalDiscount"},
		{"negative other charges", func(in *pricing.Input) { in.OtherCharges = amt(t, "-0.01") }, "otherCharges"},
		{"3 decimals other charges", func(in *pricing.Input) { in.OtherCharges = amt(t, "0.001") }, "otherCharges"},
		{"total over numeric(14,2)", func(in *pricing.Input) { in.OtherCharges = amt(t, "999999999999.99") }, "totals.total"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := pen(t, line(t, "1", "100", pricing.Exonerado, "0"))
			tc.mod(&in)
			_, err := pricing.Compute(in)
			var e *pricing.Error
			if !errors.As(err, &e) || e.Code != pricing.CodeValidationFailed || e.Field != tc.field {
				t.Fatalf("err = %v, want validation_failed on %s", err, tc.field)
			}
		})
	}

	t.Run("max values are accepted", func(t *testing.T) {
		in := pen(t, line(t, "1", big, pricing.Exonerado, "0"))
		if got := compute(t, in).Totals.Total.String(); got != "10000000000.00" {
			t.Fatalf("total = %s", got)
		}
	})
	t.Run("too many lines", func(t *testing.T) {
		lines := make([]pricing.Line, pricing.MaxLines+1)
		for i := range lines {
			lines[i] = line(t, "1", "1", pricing.Exonerado, "0")
		}
		if _, err := pricing.Compute(pen(t, lines...)); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestDoesNotMutateInput(t *testing.T) {
	in := pen(t, line(t, "1", "118", pricing.Gravado, "0"), line(t, "2", "5", pricing.Exonerado, "1"))
	before := make([]pricing.Line, len(in.Lines))
	copy(before, in.Lines)
	compute(t, in)
	for i := range before {
		if before[i] != in.Lines[i] {
			t.Fatalf("line %d changed", i)
		}
	}
}
