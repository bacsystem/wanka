// Package fx lee y guarda el tipo de cambio diario de ops.exchange_rates
// (data-model §2, ADR 0008). La tabla es global: no tiene tenant ni RLS.
//
// El único redondeo del tipo de cambio ocurre en Save, half-up a 3 decimales.
// Rate devuelve lo guardado sin tocarlo, y el documento y el envío a Khipu usan
// ese mismo valor: ningún consumidor debe volver a redondearlo.
//
// La fecha es un día civil de Lima (Day), no un instante: ningún método recibe un
// time.Time, así que la zona del llamador no puede correr el día. Today(now)
// convierte un instante a ese día.
package fx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

// Monedas y fuentes que acepta la tabla (sus CHECK).
const (
	USD = "USD"

	SourceOfficial = "official"
	SourceManual   = "manual"
	SourceFallback = "fallback"
)

// scale son los decimales de numeric(6,3).
const scale = 3

// Errores del paquete.
var (
	// ErrExchangeRateRequired: no hay tipo de cambio de esa fecha ni anterior.
	// La capa HTTP lo responde como 422 exchange_rate_required.
	ErrExchangeRateRequired = errors.New("fx: exchange_rate_required")
	// ErrInvalidRate: moneda o fuente no admitidas, o un valor que no es > 0 una vez redondeado
	// o que no cabe en numeric(6,3). No toca la base.
	ErrInvalidRate = errors.New("fx: invalid exchange rate")
	// ErrAlreadyExists: ya hay una tasa de esa fecha y moneda; el job puede reintentar sin duplicar.
	ErrAlreadyExists = errors.New("fx: exchange rate already stored")
)

// limaZone es la hora de Lima (UTC-5, sin horario de verano): los días de las fechas fiscales se cuentan ahí.
var limaZone = time.FixedZone("America/Lima", -5*60*60)

// Day es un día de calendario (año, mes y día), sin hora ni zona. El valor cero no es un día válido.
type Day struct {
	Year  int
	Month time.Month
	Dom   int // día del mes
}

// Today devuelve el día de Lima en el instante now.
func Today(now time.Time) Day {
	n := now.In(limaZone)
	return Day{Year: n.Year(), Month: n.Month(), Dom: n.Day()}
}

// String devuelve el día como yyyy-mm-dd.
func (d Day) String() string { return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Dom) }

// valid dice si d existe en el calendario (rechaza el valor cero y el 30 de febrero).
func (d Day) valid() bool {
	t := time.Date(d.Year, d.Month, d.Dom, 0, 0, 0, 0, time.UTC)
	return d.Year >= 1 && t.Year() == d.Year && t.Month() == d.Month && t.Day() == d.Dom
}

// MarshalText escribe el día como yyyy-mm-dd; un día inexistente es un error.
func (d Day) MarshalText() ([]byte, error) {
	if !d.valid() {
		return nil, fmt.Errorf("%w: day %v", ErrInvalidRate, d)
	}
	return []byte(d.String()), nil
}

// UnmarshalText lee yyyy-mm-dd y rechaza lo que no sea un día existente (por ejemplo 2026-02-30).
func (d *Day) UnmarshalText(b []byte) error {
	t, err := time.Parse(time.DateOnly, string(b))
	if err != nil {
		return fmt.Errorf("%w: day %q", ErrInvalidRate, b)
	}
	got := Day{Year: t.Year(), Month: t.Month(), Dom: t.Day()}
	if !got.valid() {
		return fmt.Errorf("%w: day %q", ErrInvalidRate, b)
	}
	*d = got
	return nil
}

// Value es un tipo de cambio con exactamente 3 decimales: String y JSON lo escriben con ellos
// ("3.750", no "3.75"), como pide api/conventions.md §6.
type Value struct{ d decimal.Decimal }

// Decimal devuelve el valor para operar con él.
func (v Value) Decimal() decimal.Decimal { return v.d }

// String devuelve el valor con 3 decimales.
func (v Value) String() string { return v.d.StringFixed(scale) }

// MarshalJSON escribe el valor como cadena JSON con 3 decimales.
func (v Value) MarshalJSON() ([]byte, error) { return json.Marshal(v.String()) }

// UnmarshalJSON lee una cadena decimal (solo dígitos y un punto) con a lo sumo 3 decimales,
// mayor que 0 y de hasta 999.999. Un número JSON se rechaza: nunca pasa por float64.
func (v *Value) UnmarshalJSON(b []byte) error {
	var text string
	if err := json.Unmarshal(b, &text); err != nil {
		return fmt.Errorf("%w: value must be a JSON string", ErrInvalidRate)
	}
	whole, frac, dot := strings.Cut(text, ".")
	if whole == "" || (dot && frac == "") || len(frac) > scale || !onlyDigits(whole) || !onlyDigits(frac) {
		return fmt.Errorf("%w: value %q", ErrInvalidRate, text)
	}
	d, err := decimal.NewFromString(text)
	if err != nil || d.Sign() <= 0 || d.GreaterThan(maxRate) {
		return fmt.Errorf("%w: value %q", ErrInvalidRate, text)
	}
	v.d = d
	return nil
}

func onlyDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Rate es una fila de ops.exchange_rates.
type Rate struct {
	// Date es el día de la fila encontrada; en una lectura con retroceso no es el pedido.
	Date     Day
	Currency string
	Buy      Value
	Sell     Value
	Source   string
}

// Quote es un tipo de cambio por ingerir: Buy y Sell pueden traer más de 3 decimales, Save los redondea.
type Quote struct {
	Date     Day
	Currency string
	Buy      decimal.Decimal
	Sell     decimal.Decimal
	Source   string
}

// Querier es lo que Reader necesita de una transacción: pgx.Tx lo cumple.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Reader lee tipos de cambio con su propia transacción de solo lectura.
type Reader struct{ pool *db.Pool }

// NewReader devuelve un Reader sobre pool (api, platform-api o cualquier servicio con SELECT).
func NewReader(pool *db.Pool) *Reader { return &Reader{pool: pool} }

// Rate devuelve el tipo de cambio de date y currency. Si no hay fila de ese día devuelve la última
// anterior con Source = "fallback" (nunca una posterior); si no hay ninguna, ErrExchangeRateRequired.
func (r *Reader) Rate(ctx context.Context, date Day, currency string) (Rate, error) {
	var out Rate
	err := r.pool.InNoTenantReadTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		var err error
		out, err = RateIn(ctx, tx, date, currency)
		return err
	})
	return out, err
}

// RateIn es Rate dentro de una transacción ya abierta, para quien ya está en InTenantTx
// (la tabla no tiene RLS, así que se lee igual con un tenant fijado).
func RateIn(ctx context.Context, q Querier, date Day, currency string) (Rate, error) {
	if !date.valid() {
		return Rate{}, fmt.Errorf("%w: day %v", ErrInvalidRate, date)
	}
	if currency != USD {
		return Rate{}, fmt.Errorf("%w: currency %q", ErrInvalidRate, currency)
	}
	var (
		r         Rate
		buy, sell string
		found     time.Time
	)
	err := q.QueryRow(ctx, `SELECT rate_date, buy::text, sell::text, source FROM ops.exchange_rates
		WHERE currency = $1 AND rate_date <= $2::date ORDER BY rate_date DESC LIMIT 1`,
		currency, date.String()).Scan(&found, &buy, &sell, &r.Source)
	if errors.Is(err, pgx.ErrNoRows) {
		return Rate{}, ErrExchangeRateRequired
	}
	if err != nil {
		return Rate{}, fmt.Errorf("reading exchange rate: %w", err)
	}
	r.Currency = currency
	// pgx entrega la columna date como medianoche UTC: se lee su día de calendario tal cual.
	r.Date = Day{Year: found.Year(), Month: found.Month(), Dom: found.Day()}
	if r.Buy.d, err = decimal.NewFromString(buy); err != nil {
		return Rate{}, fmt.Errorf("parsing buy %q: %w", buy, err)
	}
	if r.Sell.d, err = decimal.NewFromString(sell); err != nil {
		return Rate{}, fmt.Errorf("parsing sell %q: %w", sell, err)
	}
	if r.Date != date {
		r.Source = SourceFallback
	}
	return r, nil
}

// Writer guarda tipos de cambio; lo usa el job de ingesta con la conexión de wanka_worker.
type Writer struct{ pool *db.Pool }

// NewWriter devuelve un Writer sobre pool.
func NewWriter(pool *db.Pool) *Writer { return &Writer{pool: pool} }

// Save redondea Buy y Sell half-up a 3 decimales (el único redondeo del tipo de cambio) y los inserta.
// Rechaza con ErrInvalidRate una moneda o fuente no admitida, un valor que redondeado no es > 0 o que no
// cabe en numeric(6,3); con ErrAlreadyExists, una fecha y moneda ya guardadas.
func (w *Writer) Save(ctx context.Context, r Quote) error {
	buy, sell, err := r.rounded()
	if err != nil {
		return err
	}
	return w.pool.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO ops.exchange_rates (rate_date, currency, buy, sell, source)
			VALUES ($1::date, $2, $3::numeric, $4::numeric, $5)`, r.Date.String(), r.Currency, buy, sell, r.Source)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyExists
		}
		if err != nil {
			return fmt.Errorf("storing exchange rate: %w", err)
		}
		return nil
	})
}

// maxRate es el mayor valor de numeric(6,3).
var maxRate = decimal.New(999999, -scale)

// rounded valida r y devuelve buy y sell con 3 decimales, ya como texto para la base.
func (r Quote) rounded() (buy, sell string, err error) {
	if !r.Date.valid() {
		return "", "", fmt.Errorf("%w: day %v", ErrInvalidRate, r.Date)
	}
	if r.Currency != USD {
		return "", "", fmt.Errorf("%w: currency %q", ErrInvalidRate, r.Currency)
	}
	switch r.Source {
	case SourceOfficial, SourceManual, SourceFallback:
	default:
		return "", "", fmt.Errorf("%w: source %q", ErrInvalidRate, r.Source)
	}
	var out [2]string
	for i, v := range []decimal.Decimal{r.Buy, r.Sell} {
		// Round redondea half away from zero: para valores positivos es half-up. Los demás se rechazan abajo.
		v = v.Round(scale)
		if v.Sign() <= 0 || v.GreaterThan(maxRate) {
			return "", "", fmt.Errorf("%w: value out of range", ErrInvalidRate)
		}
		out[i] = v.StringFixed(scale)
	}
	return out[0], out[1], nil
}
