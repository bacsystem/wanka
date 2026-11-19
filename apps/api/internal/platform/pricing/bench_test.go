package pricing_test

import (
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/pricing"
)

func lines200(t testing.TB) pricing.Input {
	t.Helper()
	lines := make([]pricing.Line, 200)
	affs := []pricing.Afectacion{pricing.Gravado, pricing.Gravado, pricing.Exonerado, pricing.Inafecto, pricing.Gratuito}
	for i := range lines {
		lines[i] = line(t, "2.5", "17.3333", affs[i%len(affs)], "1.10")
	}
	in := pen(t, lines...)
	in.GlobalDiscount = amt(t, "25.50")
	return in
}

func BenchmarkCompute200Lines(b *testing.B) {
	in := lines200(b)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := pricing.Compute(in); err != nil {
			b.Fatal(err)
		}
	}
}

// Criterio: un comprobante de 200 líneas en menos de 5 ms. Se toma el mejor de varias
// corridas para que un tropiezo del planificador de la CI no lo vuelva intermitente.
func TestCompute200LinesUnder5ms(t *testing.T) {
	in := lines200(t)
	best := time.Hour
	for range 20 {
		start := time.Now()
		if _, err := pricing.Compute(in); err != nil {
			t.Fatal(err)
		}
		if d := time.Since(start); d < best {
			best = d
		}
	}
	if best > 5*time.Millisecond {
		t.Fatalf("200 lines took %s, want < 5ms", best)
	}
}
