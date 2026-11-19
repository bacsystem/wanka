package pricing_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/bacsystem/wanka/apps/api/internal/platform/money"
	"github.com/bacsystem/wanka/apps/api/internal/platform/pricing"
)

func randInput(t testing.TB, r *rand.Rand) pricing.Input {
	t.Helper()
	affs := []pricing.Afectacion{pricing.Gravado, pricing.Gravado, pricing.Gravado, pricing.Exonerado, pricing.Inafecto, pricing.Gratuito}
	n := 1 + r.Intn(40)
	lines := make([]pricing.Line, n)
	for i := range lines {
		qty := decimal.New(int64(1+r.Intn(500000)), -int32(r.Intn(5)))
		if qty.GreaterThan(dec("99999")) {
			qty = qty.Round(0)
		}
		qty = qty.Round(4)
		price := decimal.New(int64(r.Intn(100000000)), -int32(r.Intn(5))).Round(4)
		disc := decimal.Zero
		if r.Intn(3) == 0 {
			disc = qty.Mul(price).Mul(decimal.New(int64(r.Intn(101)), -2)).Round(2)
			if disc.GreaterThan(qty.Mul(price)) {
				disc = decimal.Zero
			}
		}
		lines[i] = pricing.Line{
			Quantity: money.New(qty, 4), UnitPrice: money.New(price, 4), Discount: money.New(disc, 2),
			Afectacion: affs[r.Intn(len(affs))],
		}
	}
	in := pricing.Input{Currency: pricing.PEN, Lines: lines, GlobalDiscount: amt(t, "0"), OtherCharges: money.New(decimal.New(int64(r.Intn(5000)), -2), 2)}
	if r.Intn(2) == 0 {
		in.Currency = pricing.USD
		rate := money.New(decimal.New(int64(1+r.Intn(9999)), -3), 3)
		in.ExchangeRate = &rate
	}
	return in
}

func fits2(d decimal.Decimal) bool { return d.Equal(d.Round(2)) }

// total = subtotal + igv + otros − descuentos y ningún importe pasa de 2 decimales,
// para cualquier entrada; con descuento global y sin él.
func TestPropertyIdentityAndScale(t *testing.T) {
	r := rand.New(rand.NewSource(167))
	for i := 0; i < 5000; i++ {
		in := randInput(t, r)
		first, err := pricing.Compute(in)
		if err != nil {
			continue // fuera de rango; no es lo que se prueba aquí
		}
		// segunda pasada con descuento global sobre lo que sí admite
		if r.Intn(2) == 0 {
			sum := first.Totals.Subtotal.Decimal().Add(first.Totals.IGV.Decimal())
			in.GlobalDiscount = money.New(sum.Mul(decimal.New(int64(r.Intn(101)), -2)).Floor().Round(2), 2)
		}
		res, err := pricing.Compute(in)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		tt := res.Totals
		want := tt.Subtotal.Decimal().Add(tt.IGV.Decimal()).Add(tt.OtherCharges.Decimal()).Sub(tt.Discounts.Decimal())
		if !tt.Total.Decimal().Equal(want) {
			t.Fatalf("iter %d: total %s != subtotal+igv+other-discounts %s", i, tt.Total, want)
		}
		sub := tt.Gravado.Decimal().Add(tt.Exonerado.Decimal()).Add(tt.Inafecto.Decimal())
		if !tt.Subtotal.Decimal().Equal(sub) {
			t.Fatalf("iter %d: subtotal %s != sum of afectaciones %s", i, tt.Subtotal, sub)
		}
		all := []money.Amount{tt.Gravado, tt.Exonerado, tt.Inafecto, tt.Gratuito, tt.Subtotal, tt.Discounts, tt.IGV, tt.FreeIGV, tt.OtherCharges, tt.Total}
		for _, l := range res.Lines {
			all = append(all, l.Base, l.IGV, l.Total)
			if !l.Base.Decimal().Add(l.IGV.Decimal()).Equal(l.Total.Decimal()) && l.Total.Decimal().IsPositive() && res.Mode == pricing.ModePrice {
				t.Fatalf("iter %d: line base+igv != total", i)
			}
		}
		for _, a := range all {
			if !fits2(a.Decimal()) || a.Decimal().IsNegative() {
				t.Fatalf("iter %d: amount %s has more than 2 decimals or is negative", i, a)
			}
		}
	}
}

// Misma entrada en cualquier orden de líneas da los mismos totales y las mismas líneas.
func TestPropertyOrderIndependent(t *testing.T) {
	r := rand.New(rand.NewSource(1670))
	for i := 0; i < 1000; i++ {
		in := randInput(t, r)
		base, err := pricing.Compute(in)
		if err != nil {
			continue
		}
		in.GlobalDiscount = money.New(base.Totals.Subtotal.Decimal().Mul(decimal.New(int64(r.Intn(60)), -2)).Floor(), 2)
		a, err := pricing.Compute(in)
		if err != nil {
			t.Fatal(err)
		}
		perm := r.Perm(len(in.Lines))
		shuffled := in
		shuffled.Lines = make([]pricing.Line, len(in.Lines))
		for to, from := range perm {
			shuffled.Lines[to] = in.Lines[from]
		}
		b, err := pricing.Compute(shuffled)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%+v", a.Totals) != fmt.Sprintf("%+v", b.Totals) || a.Mode != b.Mode {
			t.Fatalf("iter %d: order changed the result\n%+v\n%+v", i, a.Totals, b.Totals)
		}
		for to, from := range perm {
			if fmt.Sprintf("%+v", b.Lines[to]) != fmt.Sprintf("%+v", a.Lines[from]) {
				t.Fatalf("iter %d: line %d differs after shuffling", i, from)
			}
		}
	}
}

// Misma entrada, mismo resultado, también al repetirla.
func TestDeterministic(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	in := randInput(t, r)
	a, errA := pricing.Compute(in)
	b, errB := pricing.Compute(in)
	if fmt.Sprint(errA) != fmt.Sprint(errB) || fmt.Sprintf("%+v", a) != fmt.Sprintf("%+v", b) {
		t.Fatal("Compute is not deterministic")
	}
}
