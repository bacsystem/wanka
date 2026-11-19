// Package pricing calcula líneas, IGV y totales de un comprobante (ADR 0008,
// data-model §2). Es una función pura sobre decimales: sin base de datos, sin
// reloj, sin float64. POS, proformas, notas y órdenes de trabajo la comparten.
//
// Algoritmo (precios con IGV incluido, tasa 18 %):
//
//   - Gravado, modo "price" (el que se usa por defecto): total de línea =
//     round2(cantidad × precio − descuento); base = round2(total / 1.18);
//     IGV = total − base.
//   - Gravado, modo "base_rate" (el de Khipu): valor unitario = round10(precio / 1.18);
//     base = round2(cantidad × valor unitario − descuento / 1.18);
//     IGV = round2(base × 0.18); total = base + IGV.
//   - Se calculan ambos modos por línea gravada. Si la suma de |total price −
//     total base_rate| supera 0.50, TODO el comprobante usa base_rate.
//   - Exonerado e inafecto: base = total = round2(cantidad × precio − descuento); IGV 0.
//   - Gratuito: la base y el IGV son informativos (Totals.Gratuito, Totals.FreeIGV);
//     no suman al total ni reciben descuento global.
//   - Descuento global (con IGV, como los precios): se reparte entre los grupos
//     gravado, exonerado e inafecto en proporción a su total, por mayor resto en
//     centavos con empate por orden fijo de grupo; así no depende del orden de las
//     líneas. Se resta a la base del grupo y el IGV se recalcula sobre la base nueva.
//
// Redondeo: half-up (mitad se aleja de cero), como BigDecimal.HALF_UP de Khipu.
// Lo que los documentos no fijan (reparto del descuento global, definición del
// desvío) se confirma con los vectores del spike de #56; si Khipu BETA difiere,
// se corrige aquí.
package pricing

import (
	"fmt"
	"sort"

	"github.com/shopspring/decimal"

	"github.com/bacsystem/wanka/apps/api/internal/platform/money"
)

// Códigos de error, los mismos de api/conventions.md §3.
const (
	CodeValidationFailed    = "validation_failed"
	CodeExchangeRateMissing = "exchange_rate_required"
	CodeTotalsMismatch      = "totals_mismatch"
)

// MaxLines acota el tamaño de un comprobante venido de afuera.
const MaxLines = 1000

// Afectacion es el tipo de afectación del IGV de una línea.
type Afectacion string

// Afectaciones soportadas.
const (
	Gravado   Afectacion = "gravado"
	Exonerado Afectacion = "exonerado"
	Inafecto  Afectacion = "inafecto"
	Gratuito  Afectacion = "gratuito"
)

// Currency es la moneda del comprobante.
type Currency string

// Monedas soportadas.
const (
	PEN Currency = "PEN"
	USD Currency = "USD"
)

// Mode es el algoritmo con que se calculó el comprobante.
type Mode string

// Modos de cálculo.
const (
	ModePrice    Mode = "price"
	ModeBaseRate Mode = "base_rate"
)

// Line es una línea de entrada. UnitPrice incluye IGV y Discount también.
// Todo es positivo: una nota de crédito se calcula con importes positivos.
type Line struct {
	Quantity   money.Amount `json:"quantity"`  // hasta 4 decimales, > 0
	UnitPrice  money.Amount `json:"unitPrice"` // hasta 4 decimales, con IGV
	Discount   money.Amount `json:"discount"`  // hasta 2 decimales, con IGV, <= cantidad × precio
	Afectacion Afectacion   `json:"afectacion"`
}

// Declared son los totales que envió el cliente, para compararlos con los recalculados.
type Declared struct {
	Total *money.Amount `json:"total,omitempty"`
	IGV   *money.Amount `json:"igv,omitempty"`
}

// Input es el comprobante a calcular.
type Input struct {
	Currency       Currency      `json:"currency"`
	ExchangeRate   *money.Amount `json:"exchangeRate,omitempty"` // USD: obligatorio, hasta 3 decimales; PEN: prohibido
	Lines          []Line        `json:"lines"`
	GlobalDiscount money.Amount  `json:"globalDiscount"` // con IGV, hasta 2 decimales
	OtherCharges   money.Amount  `json:"otherCharges"`   // hasta 2 decimales, suman tal cual al total
	Declared       *Declared     `json:"declared,omitempty"`
}

// LineResult es el resultado de una línea, en el orden de la entrada.
type LineResult struct {
	UnitValue money.Amount `json:"unitValue"` // precio / 1.18 a 10 decimales
	Base      money.Amount `json:"base"`
	IGV       money.Amount `json:"igv"`
	Total     money.Amount `json:"total"` // base + IGV; 0 si es gratuita
}

// Totals son los totales del comprobante, todos en la moneda de Input.Currency.
// Gravado, Exonerado, Inafecto y Subtotal son bases ANTES del descuento global;
// Discounts es ese descuento expresado en base (sin IGV) e IGV es el ya descontado:
// Total = Subtotal − Discounts + IGV + OtherCharges.
type Totals struct {
	Gravado      money.Amount `json:"gravado"`
	Exonerado    money.Amount `json:"exonerado"`
	Inafecto     money.Amount `json:"inafecto"`
	Gratuito     money.Amount `json:"gratuito"` // informativo
	Subtotal     money.Amount `json:"subtotal"`
	Discounts    money.Amount `json:"discounts"`
	IGV          money.Amount `json:"igv"`
	FreeIGV      money.Amount `json:"freeIgv"` // informativo
	OtherCharges money.Amount `json:"otherCharges"`
	Total        money.Amount `json:"total"`
}

// Result es el comprobante calculado.
type Result struct {
	Currency     Currency      `json:"currency"`
	ExchangeRate *money.Amount `json:"exchangeRate,omitempty"`
	Mode         Mode          `json:"mode"`
	Lines        []LineResult  `json:"lines"`
	Totals       Totals        `json:"totals"`
}

// Error es un rechazo de Compute. Code es un code de conventions.md §3; Field
// nombra el campo de entrada. En totals_mismatch, Expected es lo recalculado,
// Got lo enviado y Difference = Got − Expected.
type Error struct {
	Code       string
	Field      string
	Detail     string
	Expected   money.Amount
	Got        money.Amount
	Difference money.Amount
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.Code, e.Field, e.Detail)
}

func invalid(field, format string, a ...any) *Error {
	return &Error{Code: CodeValidationFailed, Field: field, Detail: fmt.Sprintf(format, a...)}
}

var (
	igvRate = decimal.RequireFromString("0.18")
	divisor = decimal.RequireFromString("1.18")
	half    = decimal.RequireFromString("0.50")
	maxQty  = decimal.RequireFromString("9999999999.9999") // numeric(14,4)
	maxAmt  = decimal.RequireFromString("999999999999.99") // numeric(14,2)
	maxRate = decimal.RequireFromString("999.999")
	hundred = decimal.NewFromInt(100)
)

func r2(d decimal.Decimal) decimal.Decimal { return d.Round(2) }

// m2 fija la escala de un importe ya exacto a 2 decimales (Round no cambia el valor).
func m2(d decimal.Decimal) money.Amount { return money.New(d.Round(2), money.ScaleAmount) }

// Compute calcula el comprobante. No modifica la entrada y es determinista.
func Compute(in Input) (Result, error) {
	rate, err := validate(in)
	if err != nil {
		return Result{}, err
	}

	n := len(in.Lines)
	type calc struct {
		qty, price, disc decimal.Decimal
		unit             decimal.Decimal // valor unitario a 10 decimales
		priceTotal       decimal.Decimal // modo price: total de línea
		rateBase         decimal.Decimal // modo base_rate
		rateIGV          decimal.Decimal
	}
	cs := make([]calc, n)
	deviation := decimal.Zero
	for i, l := range in.Lines {
		c := calc{qty: l.Quantity.Decimal(), price: l.UnitPrice.Decimal(), disc: l.Discount.Decimal()}
		c.priceTotal = r2(c.qty.Mul(c.price).Sub(c.disc))
		c.unit = c.price.DivRound(divisor, money.ScaleUnit)
		if l.Afectacion == Gravado {
			discNet := c.disc.DivRound(divisor, money.ScaleUnit)
			c.rateBase = r2(c.qty.Mul(c.unit).Sub(discNet))
			if c.rateBase.IsNegative() {
				c.rateBase = decimal.Zero
			}
			c.rateIGV = r2(c.rateBase.Mul(igvRate))
			deviation = deviation.Add(c.priceTotal.Sub(c.rateBase.Add(c.rateIGV)).Abs())
		}
		cs[i] = c
	}
	mode := ModePrice
	if deviation.GreaterThan(half) {
		mode = ModeBaseRate
	}

	res := Result{Currency: in.Currency, Mode: mode, Lines: make([]LineResult, n)}
	if rate != nil {
		a := money.New(*rate, money.ScaleRate)
		res.ExchangeRate = &a
	}

	var gravBase, gravGross, exoGross, inaGross, freeBase, freeIGV decimal.Decimal
	for i, l := range in.Lines {
		c := cs[i]
		var base, igv, total decimal.Decimal
		switch l.Afectacion {
		case Gravado:
			if mode == ModePrice {
				total = c.priceTotal
				base = r2(total.DivRound(divisor, 2))
				igv = total.Sub(base)
			} else {
				base, igv = c.rateBase, c.rateIGV
				total = base.Add(igv)
			}
			gravBase = gravBase.Add(base)
			gravGross = gravGross.Add(total)
		case Exonerado:
			base, total = c.priceTotal, c.priceTotal
			exoGross = exoGross.Add(total)
		case Inafecto:
			base, total = c.priceTotal, c.priceTotal
			inaGross = inaGross.Add(total)
		case Gratuito:
			base = r2(c.priceTotal.DivRound(divisor, 2))
			igv = c.priceTotal.Sub(base)
			freeBase = freeBase.Add(base)
			freeIGV = freeIGV.Add(igv)
			total = decimal.Zero
		}
		res.Lines[i] = LineResult{
			UnitValue: money.New(c.unit, money.ScaleUnit),
			Base:      m2(base),
			IGV:       m2(igv),
			Total:     m2(total),
		}
	}

	grossSum := gravGross.Add(exoGross).Add(inaGross)
	gd := in.GlobalDiscount.Decimal()
	if gd.GreaterThan(grossSum) {
		return Result{}, invalid("globalDiscount", "exceeds the total of the discountable lines")
	}
	shares := splitDiscount(gd, []decimal.Decimal{gravGross, exoGross, inaGross})

	// Grupo gravado: la base baja en round2(descuento / 1.18) y el IGV se recalcula.
	gravAfter, gravIGV := gravBase, gravGross.Sub(gravBase)
	if mode == ModeBaseRate {
		gravIGV = decimal.Zero
		for _, lr := range res.Lines {
			gravIGV = gravIGV.Add(lr.IGV.Decimal())
		}
	}
	if !shares[0].IsZero() {
		if shares[0].Equal(gravGross) {
			gravAfter, gravIGV = decimal.Zero, decimal.Zero
		} else if mode == ModePrice {
			net := gravGross.Sub(shares[0])
			gravAfter = r2(net.DivRound(divisor, 2))
			gravIGV = net.Sub(gravAfter)
		} else {
			gravAfter = gravBase.Sub(r2(shares[0].DivRound(divisor, 2)))
			if gravAfter.IsNegative() {
				gravAfter = decimal.Zero
			}
			gravIGV = r2(gravAfter.Mul(igvRate))
		}
	}
	discounts := gravBase.Sub(gravAfter).Add(shares[1]).Add(shares[2])
	subtotal := gravBase.Add(exoGross).Add(inaGross)
	oc := in.OtherCharges.Decimal()
	total := subtotal.Sub(discounts).Add(gravIGV).Add(oc)
	if total.GreaterThan(maxAmt) {
		return Result{}, invalid("total", "exceeds the maximum amount")
	}

	res.Totals = Totals{
		Gravado:      m2(gravBase),
		Exonerado:    m2(exoGross),
		Inafecto:     m2(inaGross),
		Gratuito:     m2(freeBase),
		Subtotal:     m2(subtotal),
		Discounts:    m2(discounts),
		IGV:          m2(gravIGV),
		FreeIGV:      m2(freeIGV),
		OtherCharges: m2(oc),
		Total:        m2(total),
	}

	if d := in.Declared; d != nil {
		if d.Total != nil {
			if e := mismatch("declared.total", total, d.Total.Decimal()); e != nil {
				return Result{}, e
			}
		}
		if d.IGV != nil {
			if e := mismatch("declared.igv", gravIGV, d.IGV.Decimal()); e != nil {
				return Result{}, e
			}
		}
	}
	return res, nil
}

func mismatch(field string, want, got decimal.Decimal) *Error {
	if want.Equal(got) {
		return nil
	}
	return &Error{
		Code:       CodeTotalsMismatch,
		Field:      field,
		Detail:     fmt.Sprintf("expected %s, got %s", want.StringFixed(2), got.StringFixed(2)),
		Expected:   m2(want),
		Got:        money.New(got, money.ScaleAmount),
		Difference: money.New(got.Sub(want), money.ScaleAmount),
	}
}

// splitDiscount reparte d entre los grupos en proporción a gross, en centavos y por
// mayor resto; el empate lo gana el grupo de menor índice, no el orden de las líneas.
func splitDiscount(d decimal.Decimal, gross []decimal.Decimal) []decimal.Decimal {
	out := make([]decimal.Decimal, len(gross))
	sum := decimal.Zero
	for _, g := range gross {
		sum = sum.Add(g)
	}
	if d.IsZero() || sum.IsZero() {
		return out
	}
	dc, sc := d.Mul(hundred), sum.Mul(hundred)
	type part struct {
		idx  int
		cent decimal.Decimal // parte entera en centavos
		rest decimal.Decimal // resto de la división, mismo denominador en todos
	}
	parts := make([]part, len(gross))
	given := decimal.Zero
	for i, g := range gross {
		q, r := dc.Mul(g.Mul(hundred)).QuoRem(sc, 0)
		parts[i] = part{i, q, r}
		given = given.Add(q)
	}
	left := dc.Sub(given).IntPart()
	sort.SliceStable(parts, func(a, b int) bool { return parts[a].rest.GreaterThan(parts[b].rest) })
	for k := range parts {
		if left > 0 && !gross[parts[k].idx].IsZero() {
			parts[k].cent = parts[k].cent.Add(decimal.NewFromInt(1))
			left--
		}
		out[parts[k].idx] = parts[k].cent.Div(hundred)
	}
	return out
}

// validate revisa la entrada y devuelve el tipo de cambio ya validado (nil en PEN).
func validate(in Input) (*decimal.Decimal, error) {
	var rate *decimal.Decimal
	switch in.Currency {
	case PEN:
		if in.ExchangeRate != nil {
			return nil, invalid("exchangeRate", "must be absent when currency is PEN")
		}
	case USD:
		if in.ExchangeRate == nil {
			return nil, &Error{Code: CodeExchangeRateMissing, Field: "exchangeRate", Detail: "required when currency is USD"}
		}
		r := in.ExchangeRate.Decimal()
		if !r.IsPositive() || r.GreaterThan(maxRate) || !r.Equal(r.Round(money.ScaleRate)) {
			return nil, invalid("exchangeRate", "must be > 0, at most 999.999, with at most 3 decimals")
		}
		rate = &r
	default:
		return nil, invalid("currency", "must be PEN or USD")
	}
	if err := validateLines(in); err != nil {
		return nil, err
	}
	return rate, nil
}

func validateLines(in Input) error {
	if len(in.Lines) == 0 || len(in.Lines) > MaxLines {
		return invalid("lines", "must have between 1 and %d lines", MaxLines)
	}
	for i, l := range in.Lines {
		f := func(name string) string { return fmt.Sprintf("lines[%d].%s", i, name) }
		q, p, d := l.Quantity.Decimal(), l.UnitPrice.Decimal(), l.Discount.Decimal()
		switch {
		case !q.IsPositive() || q.GreaterThan(maxQty) || !q.Equal(q.Round(4)):
			return invalid(f("quantity"), "must be > 0 with at most 4 decimals")
		case p.IsNegative() || p.GreaterThan(maxQty) || !p.Equal(p.Round(4)):
			return invalid(f("unitPrice"), "must be >= 0 with at most 4 decimals")
		case d.IsNegative() || !d.Equal(d.Round(2)):
			return invalid(f("discount"), "must be >= 0 with at most 2 decimals")
		}
		gross := q.Mul(p)
		if gross.GreaterThan(maxAmt) {
			return invalid(f("unitPrice"), "line amount exceeds the maximum amount")
		}
		if d.GreaterThan(gross) {
			return invalid(f("discount"), "exceeds quantity x unitPrice")
		}
		switch l.Afectacion {
		case Gravado, Exonerado, Inafecto, Gratuito:
		default:
			return invalid(f("afectacion"), "must be gravado, exonerado, inafecto or gratuito")
		}
	}
	for _, f := range []struct {
		name string
		v    decimal.Decimal
	}{{"globalDiscount", in.GlobalDiscount.Decimal()}, {"otherCharges", in.OtherCharges.Decimal()}} {
		if f.v.IsNegative() || !f.v.Equal(f.v.Round(2)) || f.v.GreaterThan(maxAmt) {
			return invalid(f.name, "must be >= 0 with at most 2 decimals")
		}
	}
	return nil
}
