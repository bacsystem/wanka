// Package fx lee y guarda el tipo de cambio diario de ops.exchange_rates
// (data-model §2, ADR 0008). La tabla es global: no tiene tenant ni RLS.
//
// El único redondeo del tipo de cambio ocurre en Save, half-up a 3 decimales.
// Rate devuelve lo guardado sin tocarlo, y el documento y el envío a Khipu usan
// ese mismo valor: ningún consumidor debe volver a redondearlo.
package fx

import (
	"context"
	"errors"
	"fmt"
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

// Rate es una fila de ops.exchange_rates. Buy y Sell tienen exactamente 3 decimales.
type Rate struct {
	// Date es el día de la tasa (UTC, sin hora). En una lectura con retroceso es el de la fila
	// encontrada, no el pedido.
	Date     time.Time
	Currency string
	Buy      decimal.Decimal
	Sell     decimal.Decimal
	Source   string
}

// Querier es lo que Reader necesita de una transacción: pgx.Tx lo cumple.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// civil devuelve el día de t en su propia zona horaria como texto yyyy-mm-dd.
// La fecha es un día de calendario, no un instante: no se convierte de zona.
func civil(t time.Time) string { return t.Format(time.DateOnly) }

// Reader lee tipos de cambio con su propia transacción de solo lectura.
type Reader struct{ pool *db.Pool }

// NewReader devuelve un Reader sobre pool (api, platform-api o cualquier servicio con SELECT).
func NewReader(pool *db.Pool) *Reader { return &Reader{pool: pool} }

// Rate devuelve el tipo de cambio de date y currency. Si no hay fila de ese día devuelve la última
// anterior con Source = "fallback" (nunca una posterior); si no hay ninguna, ErrExchangeRateRequired.
func (r *Reader) Rate(ctx context.Context, date time.Time, currency string) (Rate, error) {
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
func RateIn(ctx context.Context, q Querier, date time.Time, currency string) (Rate, error) {
	if currency != USD {
		return Rate{}, fmt.Errorf("%w: currency %q", ErrInvalidRate, currency)
	}
	var (
		r         Rate
		buy, sell string
	)
	err := q.QueryRow(ctx, `SELECT rate_date, buy::text, sell::text, source FROM ops.exchange_rates
		WHERE currency = $1 AND rate_date <= $2::date ORDER BY rate_date DESC LIMIT 1`,
		currency, civil(date)).Scan(&r.Date, &buy, &sell, &r.Source)
	if errors.Is(err, pgx.ErrNoRows) {
		return Rate{}, ErrExchangeRateRequired
	}
	if err != nil {
		return Rate{}, fmt.Errorf("reading exchange rate: %w", err)
	}
	r.Currency = currency
	if r.Buy, err = decimal.NewFromString(buy); err != nil {
		return Rate{}, fmt.Errorf("parsing buy %q: %w", buy, err)
	}
	if r.Sell, err = decimal.NewFromString(sell); err != nil {
		return Rate{}, fmt.Errorf("parsing sell %q: %w", sell, err)
	}
	if civil(r.Date) != civil(date) {
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
func (w *Writer) Save(ctx context.Context, r Rate) error {
	buy, sell, err := r.rounded()
	if err != nil {
		return err
	}
	return w.pool.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO ops.exchange_rates (rate_date, currency, buy, sell, source)
			VALUES ($1::date, $2, $3::numeric, $4::numeric, $5)`, civil(r.Date), r.Currency, buy, sell, r.Source)
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
func (r Rate) rounded() (buy, sell string, err error) {
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
