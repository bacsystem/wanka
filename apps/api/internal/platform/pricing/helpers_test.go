package pricing_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/bacsystem/wanka/apps/api/internal/platform/money"
	"github.com/bacsystem/wanka/apps/api/internal/platform/pricing"
)

func amt(t testing.TB, s string) money.Amount {
	t.Helper()
	a, err := money.Parse(s, money.ScaleUnit)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func line(t testing.TB, qty, price string, aff pricing.Afectacion, disc string) pricing.Line {
	t.Helper()
	return pricing.Line{Quantity: amt(t, qty), UnitPrice: amt(t, price), Discount: amt(t, disc), Afectacion: aff}
}

func pen(t testing.TB, lines ...pricing.Line) pricing.Input {
	t.Helper()
	return pricing.Input{Currency: pricing.PEN, Lines: lines, GlobalDiscount: amt(t, "0"), OtherCharges: amt(t, "0")}
}

func compute(t testing.TB, in pricing.Input) pricing.Result {
	t.Helper()
	r, err := pricing.Compute(in)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	return r
}

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }
