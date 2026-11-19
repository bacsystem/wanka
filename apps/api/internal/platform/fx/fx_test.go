package fx_test

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/fx"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	v, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// env es una base migrada con un Reader sobre wanka_app y un Writer sobre wanka_worker.
type env struct {
	d *dbtest.DB
	r *fx.Reader
	w *fx.Writer
}

func setup(t *testing.T) env {
	t.Helper()
	d := dbtest.New(t)
	ctx := context.Background()
	d.Migrate(ctx, t)
	open := func(role string) *db.Pool {
		p, err := db.Open(ctx, d.URL(role), db.Options{MaxConns: 3})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(p.Close)
		return p
	}
	return env{d: d, r: fx.NewReader(open(dbtest.App)), w: fx.NewWriter(open(dbtest.Worker))}
}

func connectAs(t *testing.T, d *dbtest.DB, role string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), d.URL(role))
	if err != nil {
		t.Fatalf("connecting as %s: %v", role, err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

func wantState(t *testing.T, err error, code, what string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Errorf("%s: err = %v, want SQLSTATE %s", what, err, code)
	}
}

func save(t *testing.T, w *fx.Writer, date time.Time, buy, sell, source string) {
	t.Helper()
	err := w.Save(context.Background(), fx.Rate{Date: date, Currency: fx.USD, Buy: dec(t, buy), Sell: dec(t, sell), Source: source})
	if err != nil {
		t.Fatalf("saving %s: %v", date.Format(time.DateOnly), err)
	}
}

// Criterio 1: forma de la tabla, sin RLS y matriz de GRANT del ADR 0001.
func TestTableShapeAndGrants(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := e.d.Admin(ctx, t)

	var cols string
	err := admin.QueryRow(ctx, `SELECT string_agg(attname || ':' || format_type(atttypid, atttypmod) || ':' || attnotnull, ', ' ORDER BY attnum)
		FROM pg_attribute WHERE attrelid = 'ops.exchange_rates'::regclass AND attnum > 0 AND NOT attisdropped`).Scan(&cols)
	if err != nil {
		t.Fatal(err)
	}
	const wantCols = "rate_date:date:true, currency:text:true, buy:numeric(6,3):true, sell:numeric(6,3):true, source:text:true"
	if cols != wantCols {
		t.Errorf("columns = %s, want %s", cols, wantCols)
	}

	var pk string
	if err := admin.QueryRow(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid = 'ops.exchange_rates'::regclass AND contype = 'p'`).Scan(&pk); err != nil {
		t.Fatal(err)
	}
	if pk != "PRIMARY KEY (rate_date, currency)" {
		t.Errorf("primary key = %s", pk)
	}

	var rls, force bool
	if err := admin.QueryRow(ctx, `SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE oid = 'ops.exchange_rates'::regclass`).Scan(&rls, &force); err != nil {
		t.Fatal(err)
	}
	if rls || force {
		t.Errorf("RLS enabled=%v forced=%v, want neither: the table is global", rls, force)
	}

	// Matriz exacta: cada rol con cada privilegio de tabla.
	want := map[string]map[string]bool{
		dbtest.App:      {"SELECT": true},
		dbtest.Worker:   {"INSERT": true},
		dbtest.Platform: {"SELECT": true},
		dbtest.Webhooks: {},
		dbtest.Migrator: {"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true, "TRUNCATE": true, "REFERENCES": true, "TRIGGER": true},
	}
	for role, grants := range want {
		for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
			var has bool
			if err := admin.QueryRow(ctx, `SELECT has_table_privilege($1, 'ops.exchange_rates', $2)`, role, priv).Scan(&has); err != nil {
				t.Fatal(err)
			}
			if has != grants[priv] {
				t.Errorf("%s %s = %v, want %v", role, priv, has, grants[priv])
			}
		}
	}
}

// Criterios 1 y 5: la base rechaza moneda, fuente y valores fuera de rango, aunque el servicio no los valide.
func TestDatabaseRejectsInvalidRows(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	w := connectAs(t, e.d, dbtest.Worker)

	tests := []struct {
		name, sql, state string
	}{
		{"currency EUR", `INSERT INTO ops.exchange_rates VALUES ('2026-11-19', 'EUR', 3.7, 3.8, 'official')`, "23514"},
		{"currency PEN", `INSERT INTO ops.exchange_rates VALUES ('2026-11-19', 'PEN', 1, 1, 'official')`, "23514"},
		{"source unknown", `INSERT INTO ops.exchange_rates VALUES ('2026-11-19', 'USD', 3.7, 3.8, 'sbs')`, "23514"},
		{"buy zero", `INSERT INTO ops.exchange_rates VALUES ('2026-11-19', 'USD', 0, 3.8, 'official')`, "23514"},
		{"sell zero", `INSERT INTO ops.exchange_rates VALUES ('2026-11-19', 'USD', 3.7, 0, 'official')`, "23514"},
		{"buy negative", `INSERT INTO ops.exchange_rates VALUES ('2026-11-19', 'USD', -3.7, 3.8, 'official')`, "23514"},
		{"sell negative", `INSERT INTO ops.exchange_rates VALUES ('2026-11-19', 'USD', 3.7, -0.001, 'official')`, "23514"},
		{"buy null", `INSERT INTO ops.exchange_rates VALUES ('2026-11-19', 'USD', NULL, 3.8, 'official')`, "23502"},
		{"does not fit numeric(6,3)", `INSERT INTO ops.exchange_rates VALUES ('2026-11-19', 'USD', 1000, 3.8, 'official')`, "22003"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := w.Exec(ctx, tt.sql)
			wantState(t, err, tt.state, tt.name)
		})
	}
	var n int
	if err := e.d.Admin(ctx, t).QueryRow(ctx, `SELECT count(*) FROM ops.exchange_rates`).Scan(&n); err != nil || n != 0 {
		t.Errorf("rows = %d (err=%v), want 0 after every rejection", n, err)
	}
	// La fila válida del mismo estilo sí entra: los rechazos de arriba son por su dato, no por el rol.
	if _, err := w.Exec(ctx, `INSERT INTO ops.exchange_rates VALUES ('2026-11-19', 'USD', 3.7, 3.8, 'manual')`); err != nil {
		t.Errorf("valid insert as worker: %v", err)
	}
}

// Criterio 6: wanka_app solo lee.
func TestAppCannotWrite(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	save(t, e.w, day(2026, 11, 19), "3.7", "3.8", fx.SourceOfficial)
	app := connectAs(t, e.d, dbtest.App)

	_, err := app.Exec(ctx, `INSERT INTO ops.exchange_rates VALUES ('2026-11-20', 'USD', 3.7, 3.8, 'official')`)
	wantState(t, err, "42501", "app INSERT")
	_, err = app.Exec(ctx, `UPDATE ops.exchange_rates SET buy = 9`)
	wantState(t, err, "42501", "app UPDATE")
	_, err = app.Exec(ctx, `DELETE FROM ops.exchange_rates`)
	wantState(t, err, "42501", "app DELETE")

	var buy string
	if err := app.QueryRow(ctx, `SELECT buy::text FROM ops.exchange_rates`).Scan(&buy); err != nil || buy != "3.700" {
		t.Errorf("app SELECT = %q (err=%v), want 3.700", buy, err)
	}
	pl := connectAs(t, e.d, dbtest.Platform)
	if err := pl.QueryRow(ctx, `SELECT buy::text FROM ops.exchange_rates`).Scan(&buy); err != nil || buy != "3.700" {
		t.Errorf("platform SELECT = %q (err=%v), want 3.700", buy, err)
	}
	_, err = pl.Exec(ctx, `INSERT INTO ops.exchange_rates VALUES ('2026-11-20', 'USD', 3.7, 3.8, 'official')`)
	wantState(t, err, "42501", "platform INSERT")
}

// Criterios 2 y 3.
func TestRate(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	save(t, e.w, day(2026, 11, 16), "3.701", "3.711", fx.SourceOfficial) // lunes
	save(t, e.w, day(2026, 11, 19), "3.750", "3.760", fx.SourceManual)
	save(t, e.w, day(2026, 11, 25), "3.800", "3.810", fx.SourceOfficial) // posterior: nunca debe usarse antes de su día

	lima := time.FixedZone("America/Lima", -5*3600)
	tests := []struct {
		name     string
		date     time.Time
		buy      string
		sell     string
		source   string
		rateDate string
	}{
		{"exact row keeps its own source", day(2026, 11, 19), "3.750", "3.760", fx.SourceManual, "2026-11-19"},
		{"exact row official", day(2026, 11, 16), "3.701", "3.711", fx.SourceOfficial, "2026-11-16"},
		{"gap falls back to the previous day", day(2026, 11, 20), "3.750", "3.760", fx.SourceFallback, "2026-11-19"},
		{"gap falls back to the closest previous one", day(2026, 11, 18), "3.701", "3.711", fx.SourceFallback, "2026-11-16"},
		{"the day before a later row does not use it", day(2026, 11, 24), "3.750", "3.760", fx.SourceFallback, "2026-11-19"},
		{"a calendar day, not an instant: late evening in Lima", time.Date(2026, 11, 19, 23, 30, 0, 0, lima), "3.750", "3.760", fx.SourceManual, "2026-11-19"},
		{"a calendar day, not an instant: early morning in Lima", time.Date(2026, 11, 19, 0, 30, 0, 0, lima), "3.750", "3.760", fx.SourceManual, "2026-11-19"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.r.Rate(ctx, tt.date, fx.USD)
			if err != nil {
				t.Fatal(err)
			}
			if got.Buy.StringFixed(3) != tt.buy || got.Sell.StringFixed(3) != tt.sell || got.Source != tt.source ||
				got.Date.Format(time.DateOnly) != tt.rateDate || got.Currency != fx.USD {
				t.Errorf("Rate = %+v, want buy=%s sell=%s source=%s date=%s", got, tt.buy, tt.sell, tt.source, tt.rateDate)
			}
		})
	}

	t.Run("nothing on or before the date", func(t *testing.T) {
		_, err := e.r.Rate(ctx, day(2026, 11, 15), fx.USD)
		if !errors.Is(err, fx.ErrExchangeRateRequired) {
			t.Errorf("err = %v, want ErrExchangeRateRequired (a later row must not answer)", err)
		}
	})
	t.Run("currency other than USD", func(t *testing.T) {
		for _, c := range []string{"PEN", "EUR", "", "usd"} {
			if _, err := e.r.Rate(ctx, day(2026, 11, 19), c); !errors.Is(err, fx.ErrInvalidRate) {
				t.Errorf("Rate(%q) err = %v, want ErrInvalidRate", c, err)
			}
		}
	})
	t.Run("RateIn inside an open transaction", func(t *testing.T) {
		d := e.d
		p, err := db.Open(ctx, d.URL(dbtest.App), db.Options{MaxConns: 1})
		if err != nil {
			t.Fatal(err)
		}
		defer p.Close()
		err = p.InNoTenantReadTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
			got, err := fx.RateIn(ctx, tx, day(2026, 11, 22), fx.USD)
			if err != nil {
				return err
			}
			if got.Source != fx.SourceFallback || got.Buy.StringFixed(3) != "3.750" {
				t.Errorf("RateIn = %+v", got)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestRateEmptyTable(t *testing.T) {
	e := setup(t)
	_, err := e.r.Rate(context.Background(), day(2026, 11, 19), fx.USD)
	if !errors.Is(err, fx.ErrExchangeRateRequired) {
		t.Errorf("err = %v, want ErrExchangeRateRequired", err)
	}
}

// Criterio 4: half-up a 3 decimales una sola vez, y lo leído es lo guardado.
func TestSaveRoundsHalfUpOnce(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	tests := []struct{ in, want string }{
		{"3.4565", "3.457"}, // empate hacia arriba (con float64 sería 3.456 o 3.457 según la representación)
		{"3.4564", "3.456"},
		{"3.4555", "3.456"},
		{"3.4995", "3.500"},
		{"3.456", "3.456"},
		{"3.4", "3.400"},
		{"3", "3.000"},
		{"0.0005", "0.001"},
		{"3.45650000001", "3.457"},
		{"3.45649999999", "3.456"},
		{"999.9994", "999.999"},
	}
	for i, tt := range tests {
		date := day(2026, 1, 1).AddDate(0, 0, i)
		t.Run(tt.in, func(t *testing.T) {
			save(t, e.w, date, tt.in, tt.in, fx.SourceOfficial)
			got, err := e.r.Rate(ctx, date, fx.USD)
			if err != nil {
				t.Fatal(err)
			}
			if got.Buy.StringFixed(3) != tt.want || got.Sell.StringFixed(3) != tt.want {
				t.Errorf("stored %s as buy=%s sell=%s, want %s", tt.in, got.Buy.StringFixed(3), got.Sell.StringFixed(3), tt.want)
			}
		})
	}

	// Lo guardado ya redondeado vuelve igual: leer no redondea ni cambia la escala.
	t.Run("read does not change what is stored", func(t *testing.T) {
		date := day(2027, 1, 1)
		if _, err := e.d.Admin(ctx, t).Exec(ctx, `INSERT INTO ops.exchange_rates VALUES ($1, 'USD', 3.457, 3.501, 'official')`, date); err != nil {
			t.Fatal(err)
		}
		got, err := e.r.Rate(ctx, date, fx.USD)
		if err != nil {
			t.Fatal(err)
		}
		if got.Buy.StringFixed(3) != "3.457" || got.Sell.StringFixed(3) != "3.501" {
			t.Errorf("Rate = %s / %s", got.Buy, got.Sell)
		}
	})
}

// roundHalfUp3 es el oráculo: aritmética exacta con math/big, independiente de shopspring.
func roundHalfUp3(s string) string {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		panic("bad rational " + s)
	}
	r.Mul(r, big.NewRat(1000, 1))
	r.Add(r, big.NewRat(1, 2))
	n := new(big.Int).Div(r.Num(), r.Denom()) // floor (valores positivos)
	return fmt.Sprintf("%d.%03d", new(big.Int).Div(n, big.NewInt(1000)), new(big.Int).Mod(n, big.NewInt(1000)))
}

// Invariante: ante cualquier entrada válida, lo guardado es exactamente el half-up de 3 decimales del valor dado.
func TestSaveMatchesExactHalfUpForAnyInput(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	rng := rand.New(rand.NewSource(170))
	for i := 0; i < 300; i++ {
		decimals := rng.Intn(8) // 0..7 decimales
		in := fmt.Sprintf("%d", 1+rng.Intn(998))
		if decimals > 0 {
			in += "." + fmt.Sprintf("%0*d", decimals, rng.Int63n(pow10(decimals)))
		}
		// Empates exactos con frecuencia: terminan en 5 en el cuarto decimal.
		if i%3 == 0 {
			in = fmt.Sprintf("%d.%03d5", 1+rng.Intn(998), rng.Intn(1000))
		}
		date := day(2026, 1, 1).AddDate(0, 0, i)
		want := roundHalfUp3(in)
		save(t, e.w, date, in, in, fx.SourceOfficial)
		got, err := e.r.Rate(ctx, date, fx.USD)
		if err != nil {
			t.Fatal(err)
		}
		if got.Buy.StringFixed(3) != want || got.Sell.StringFixed(3) != want {
			t.Fatalf("input %s stored as %s/%s, want %s", in, got.Buy.StringFixed(3), got.Sell.StringFixed(3), want)
		}
	}
}

func pow10(n int) int64 {
	v := int64(1)
	for ; n > 0; n-- {
		v *= 10
	}
	return v
}

// Save rechaza antes de tocar la base, y un reintento no duplica ni pisa.
func TestSaveRejectsInvalidAndIsRetrySafe(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	date := day(2026, 11, 19)
	bad := []struct {
		name string
		r    fx.Rate
	}{
		{"currency EUR", fx.Rate{Date: date, Currency: "EUR", Buy: dec(t, "3.7"), Sell: dec(t, "3.8"), Source: fx.SourceOfficial}},
		{"source unknown", fx.Rate{Date: date, Currency: fx.USD, Buy: dec(t, "3.7"), Sell: dec(t, "3.8"), Source: "sbs"}},
		{"buy zero", fx.Rate{Date: date, Currency: fx.USD, Buy: dec(t, "0"), Sell: dec(t, "3.8"), Source: fx.SourceOfficial}},
		{"sell negative", fx.Rate{Date: date, Currency: fx.USD, Buy: dec(t, "3.7"), Sell: dec(t, "-3.8"), Source: fx.SourceOfficial}},
		{"buy rounds to zero", fx.Rate{Date: date, Currency: fx.USD, Buy: dec(t, "0.0004"), Sell: dec(t, "3.8"), Source: fx.SourceOfficial}},
		{"rounds past numeric(6,3)", fx.Rate{Date: date, Currency: fx.USD, Buy: dec(t, "3.7"), Sell: dec(t, "999.9995"), Source: fx.SourceOfficial}},
		{"zero value decimal", fx.Rate{Date: date, Currency: fx.USD, Source: fx.SourceOfficial}},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			if err := e.w.Save(ctx, tt.r); !errors.Is(err, fx.ErrInvalidRate) {
				t.Errorf("err = %v, want ErrInvalidRate", err)
			}
		})
	}
	if _, err := e.r.Rate(ctx, date, fx.USD); !errors.Is(err, fx.ErrExchangeRateRequired) {
		t.Fatalf("a rejected Save stored something: %v", err)
	}

	save(t, e.w, date, "3.7", "3.8", fx.SourceOfficial)
	err := e.w.Save(ctx, fx.Rate{Date: date, Currency: fx.USD, Buy: dec(t, "9.9"), Sell: dec(t, "9.9"), Source: fx.SourceManual})
	if !errors.Is(err, fx.ErrAlreadyExists) {
		t.Errorf("second Save err = %v, want ErrAlreadyExists", err)
	}
	got, err := e.r.Rate(ctx, date, fx.USD)
	if err != nil || got.Buy.StringFixed(3) != "3.700" || got.Source != fx.SourceOfficial {
		t.Errorf("after a duplicate Save: %+v (err=%v), want the first row untouched", got, err)
	}
}
