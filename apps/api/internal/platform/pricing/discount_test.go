package pricing_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/bacsystem/wanka/apps/api/internal/platform/money"
	"github.com/bacsystem/wanka/apps/api/internal/platform/pricing"
)

func repeat(t testing.TB, n int, qty, price string, aff pricing.Afectacion) []pricing.Line {
	t.Helper()
	ls := make([]pricing.Line, n)
	for i := range ls {
		ls[i] = line(t, qty, price, aff, "0")
	}
	return ls
}

// Regresión de H1 (#204): un descuento global de 0.01 nunca sube el IGV ni el total,
// ni produce un descuento negativo. Se parte de las bases ya sumadas por línea.
func TestGlobalDiscountNeverRaisesTaxOrTotal(t *testing.T) {
	tests := []struct {
		name                 string
		lines                []pricing.Line
		igv, total, discount string
		mode                 pricing.Mode
	}{
		{"50 x 1.50 price", repeat(t, 50, "1", "1.50", pricing.Gravado), "11.50", "74.99", "0.01", pricing.ModePrice},
		{"30 x 3.50 price", repeat(t, 30, "1", "3.50", pricing.Gravado), "15.90", "104.99", "0.01", pricing.ModePrice},
		{"200 x 0.43 base_rate", repeat(t, 200, "1", "0.43", pricing.Gravado), "12.00", "83.99", "0.01", pricing.ModeBaseRate},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := pen(t, tc.lines...)
			in.GlobalDiscount = amt(t, "0.01")
			r := compute(t, in)
			if r.Mode != tc.mode {
				t.Fatalf("mode = %s, want %s", r.Mode, tc.mode)
			}
			if r.Totals.IGV.String() != tc.igv || r.Totals.Total.String() != tc.total || r.Totals.Discounts.String() != tc.discount {
				t.Fatalf("igv %s total %s discounts %s, want %s %s %s", r.Totals.IGV, r.Totals.Total, r.Totals.Discounts, tc.igv, tc.total, tc.discount)
			}
		})
	}
}

func TestGlobalDiscountSplitTieGoesToFirstGroup(t *testing.T) {
	// 0.10 entre tres grupos de 118: 4, 3 y 3 centavos. El centavo sobrante lo gana
	// el gravado (primer grupo): su base baja 0.03 y el IGV baja 0.01 (17.99). Si lo
	// ganara otro grupo, el gravado recibiría 0.03 y el IGV seguiría en 18.00.
	in := pen(t, line(t, "1", "118", pricing.Gravado, "0"), line(t, "1", "118", pricing.Exonerado, "0"), line(t, "1", "118", pricing.Inafecto, "0"))
	in.GlobalDiscount = amt(t, "0.10")
	r := compute(t, in)
	if r.Totals.IGV.String() != "17.99" || r.Totals.Total.String() != "353.90" || r.Totals.Discounts.String() != "0.09" {
		t.Fatalf("totals = %+v", r.Totals)
	}
}

func TestTotalsCapped(t *testing.T) {
	big := "9999999999"
	tests := []struct {
		name  string
		in    func() pricing.Input
		field string
	}{
		{"exonerado over the cap", func() pricing.Input {
			in := pen(t, line(t, "99", big, pricing.Exonerado, "0"), line(t, "99", big, pricing.Exonerado, "0"))
			in.GlobalDiscount = amt(t, "989999999901.00")
			return in
		}, "totals.exonerado"},
		{"gratuito over the cap", func() pricing.Input {
			ls := make([]pricing.Line, 0, 20)
			for range 20 {
				ls = append(ls, line(t, "99", big, pricing.Gratuito, "0"))
			}
			return pen(t, ls...)
		}, "totals.gratuito"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pricing.Compute(tc.in())
			e, ok := err.(*pricing.Error)
			if !ok || e.Code != pricing.CodeValidationFailed || e.Field != tc.field {
				t.Fatalf("err = %v, want validation_failed on %s", err, tc.field)
			}
		})
	}
}

func TestUnitValueExposedWith4Decimals(t *testing.T) {
	r := compute(t, pen(t, line(t, "1", "1", pricing.Gravado, "0")))
	if got := r.Lines[0].UnitValue.String(); got != "0.8475" {
		t.Fatalf("unitValue = %s, want 0.8475", got)
	}
}

// En base_rate la línea gratuita sigue el mismo modo que el resto del comprobante.
func TestFreeLineFollowsMode(t *testing.T) {
	ls := append(repeat(t, 60, "1", "0.10", pricing.Gravado), line(t, "1", "0.10", pricing.Gratuito, "0"))
	r := compute(t, pen(t, ls...))
	if r.Mode != pricing.ModeBaseRate {
		t.Fatalf("mode = %s", r.Mode)
	}
	// modo price daría base 0.08 e IGV 0.02; base_rate: base 0.08, IGV round2(0.08*0.18) = 0.01.
	f := r.Lines[60]
	if f.Base.String() != "0.08" || f.IGV.String() != "0.01" || f.Total.String() != "0.00" {
		t.Fatalf("free line = %+v", f)
	}
}

// Invariantes de negocio con descuentos chicos y carritos de muchas líneas baratas:
// descuento >= 0, IGV y total con descuento <= sin descuento, identidad del total.
func TestPropertyDiscountMonotone(t *testing.T) {
	r := rand.New(rand.NewSource(204))
	for i := 0; i < 600; i++ {
		var in pricing.Input
		if i%2 == 0 {
			in = cheapCart(t, r)
		} else {
			in = randInput(t, r)
		}
		base, err := pricing.Compute(in)
		if err != nil {
			continue
		}
		disc := base.Totals.Subtotal.Decimal().Add(base.Totals.IGV.Decimal())
		for _, gd := range discounts(r, disc) {
			in.GlobalDiscount = money.New(gd, 2)
			res, err := pricing.Compute(in)
			if err != nil {
				t.Fatalf("iter %d gd %s: %v", i, gd, err)
			}
			tt := res.Totals
			ctx := fmt.Sprintf("iter %d gd %s mode %s lines %d", i, gd, res.Mode, len(in.Lines))
			if tt.Discounts.Decimal().IsNegative() {
				t.Fatalf("%s: negative discounts %s", ctx, tt.Discounts)
			}
			if tt.IGV.Decimal().GreaterThan(base.Totals.IGV.Decimal()) {
				t.Fatalf("%s: IGV rose %s -> %s", ctx, base.Totals.IGV, tt.IGV)
			}
			if tt.Total.Decimal().GreaterThan(base.Totals.Total.Decimal()) {
				t.Fatalf("%s: total rose %s -> %s", ctx, base.Totals.Total, tt.Total)
			}
			want := tt.Subtotal.Decimal().Add(tt.IGV.Decimal()).Add(tt.OtherCharges.Decimal()).Sub(tt.Discounts.Decimal())
			if !tt.Total.Decimal().Equal(want) {
				t.Fatalf("%s: identity broken", ctx)
			}
			// el total baja exactamente lo descontado (con IGV)
			if !base.Totals.Total.Decimal().Sub(tt.Total.Decimal()).Equal(gd) {
				t.Fatalf("%s: total fell %s, want %s", ctx, base.Totals.Total.Decimal().Sub(tt.Total.Decimal()), gd)
			}
		}
	}
}

// cheapCart arma carritos de 20 a 250 líneas baratas, donde se acumula el redondeo.
func cheapCart(t testing.TB, r *rand.Rand) pricing.Input {
	t.Helper()
	n := 20 + r.Intn(231)
	prices := []string{"0.10", "0.16", "0.43", "0.99", "1.50", "3.50", "2.95"}
	p := prices[r.Intn(len(prices))]
	affs := []pricing.Afectacion{pricing.Gravado, pricing.Gravado, pricing.Gravado, pricing.Exonerado, pricing.Gratuito}
	ls := make([]pricing.Line, n)
	for k := range ls {
		a := pricing.Gravado
		if r.Intn(4) == 0 {
			a = affs[r.Intn(len(affs))]
		}
		ls[k] = line(t, "1", p, a, "0")
	}
	in := pen(t, ls...)
	if r.Intn(2) == 0 {
		in.Lines[0] = line(t, "1", prices[r.Intn(len(prices))], pricing.Gravado, "0")
	}
	return in
}

// discounts devuelve descuentos a probar: chicos (0.01, 0.02, ...), de 1.00 y proporcionales.
func discounts(r *rand.Rand, max decimal.Decimal) []decimal.Decimal {
	out := []decimal.Decimal{dec("0.01"), dec("0.02"), dec("0.05"), dec("0.99"), dec("1.00")}
	out = append(out, max.Mul(decimal.New(int64(r.Intn(101)), -2)).Floor())
	var ok []decimal.Decimal
	for _, d := range out {
		if d.IsPositive() && !d.GreaterThan(max) {
			ok = append(ok, d)
		}
	}
	return ok
}

// El IGV baja lo que el descuento no se lleva de la base; sin recortes absurdos.
func TestGlobalDiscountGroupEdges(t *testing.T) {
	tests := []struct {
		name                       string
		lines                      []pricing.Line
		gd                         string
		base, igv, total, discount string // Totals.IGV, Total y Discounts tras el descuento
	}{
		{"one line, half discount", []pricing.Line{line(t, "1", "118", pricing.Gravado, "0")}, "50",
			"", "10.37", "68.00", "42.37"},
		// 10 líneas de 0.03: base 0.30 e IGV 0.00; el descuento 0.20 baja la base 0.17, el resto no cabe en el IGV
		{"igv cannot go negative", repeat(t, 10, "1", "0.03", pricing.Gravado), "0.20",
			"", "0.00", "0.10", "0.20"},
		// 100 líneas de 0.12: base 10.00, IGV 2.00; con 11.99 de descuento quedan 0.01 de total
		{"igv cannot exceed the remaining total", repeat(t, 100, "1", "0.12", pricing.Gravado), "11.99",
			"", "0.01", "0.01", "10.00"},
		{"full discount", repeat(t, 100, "1", "0.12", pricing.Gravado), "12.00",
			"", "0.00", "0.00", "10.00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := pen(t, tc.lines...)
			in.GlobalDiscount = amt(t, tc.gd)
			r := compute(t, in)
			tt := r.Totals
			if tt.IGV.String() != tc.igv || tt.Total.String() != tc.total || tt.Discounts.String() != tc.discount {
				t.Fatalf("igv %s total %s discounts %s, want %s %s %s", tt.IGV, tt.Total, tt.Discounts, tc.igv, tc.total, tc.discount)
			}
		})
	}
}

// Exonerado e inafecto no llevan IGV: su valor unitario es el precio; gravado y
// gratuito lo dividen entre 1.18.
func TestUnitValuePerAfectacion(t *testing.T) {
	tests := []struct {
		aff  pricing.Afectacion
		want string
	}{
		{pricing.Gravado, "0.8475"},
		{pricing.Gratuito, "0.8475"},
		{pricing.Exonerado, "1.0000"},
		{pricing.Inafecto, "1.0000"},
	}
	for _, tc := range tests {
		t.Run(string(tc.aff), func(t *testing.T) {
			r := compute(t, pen(t, line(t, "1", "1", tc.aff, "0")))
			if got := r.Lines[0].UnitValue.String(); got != tc.want {
				t.Fatalf("unitValue = %s, want %s", got, tc.want)
			}
		})
	}
}

// "A lo sumo n decimales" se mide sobre el valor, no sobre los dígitos escritos:
// "1.00000" es 1 y se acepta como cantidad.
func TestDecimalsMeasuredOnValue(t *testing.T) {
	r := compute(t, pen(t, line(t, "1.00000", "10.000000", pricing.Exonerado, "0.000")))
	if r.Totals.Total.String() != "10.00" {
		t.Fatalf("total = %s", r.Totals.Total)
	}
	if _, err := pricing.Compute(pen(t, line(t, "1.00001", "10", pricing.Exonerado, "0"))); err == nil {
		t.Fatal("1.00001 must be rejected")
	}
}
