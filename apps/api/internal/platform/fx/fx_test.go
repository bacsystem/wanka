package fx_test

import (
	"context"
	"encoding/json"
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

func day(y int, m time.Month, d int) fx.Day { return fx.Day{Year: y, Month: m, Dom: d} }

// nth devuelve el día n-ésimo después del 1 de enero de 2026 (para sembrar muchas fechas distintas).
func nth(n int) fx.Day {
	t := time.Date(2026, 1, 1+n, 0, 0, 0, 0, time.UTC)
	return day(t.Year(), t.Month(), t.Day())
}

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

func save(t *testing.T, w *fx.Writer, date fx.Day, buy, sell, source string) {
	t.Helper()
	err := w.Save(context.Background(), fx.Quote{Date: date, Currency: fx.USD, Buy: dec(t, buy), Sell: dec(t, sell), Source: source})
	if err != nil {
		t.Fatalf("saving %s: %v", date.String(), err)
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

	tests := []struct {
		name     string
		date     fx.Day
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.r.Rate(ctx, tt.date, fx.USD)
			if err != nil {
				t.Fatal(err)
			}
			if got.Buy.String() != tt.buy || got.Sell.String() != tt.sell || got.Source != tt.source ||
				got.Date.String() != tt.rateDate || got.Currency != fx.USD {
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
			if got.Source != fx.SourceFallback || got.Buy.String() != "3.750" {
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
		date := nth(i)
		t.Run(tt.in, func(t *testing.T) {
			save(t, e.w, date, tt.in, tt.in, fx.SourceOfficial)
			got, err := e.r.Rate(ctx, date, fx.USD)
			if err != nil {
				t.Fatal(err)
			}
			if got.Buy.String() != tt.want || got.Sell.String() != tt.want {
				t.Errorf("stored %s as buy=%s sell=%s, want %s", tt.in, got.Buy.String(), got.Sell.String(), tt.want)
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
		if got.Buy.String() != "3.457" || got.Sell.String() != "3.501" {
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
		date := nth(i)
		want := roundHalfUp3(in)
		save(t, e.w, date, in, in, fx.SourceOfficial)
		got, err := e.r.Rate(ctx, date, fx.USD)
		if err != nil {
			t.Fatal(err)
		}
		if got.Buy.String() != want || got.Sell.String() != want {
			t.Fatalf("input %s stored as %s/%s, want %s", in, got.Buy.String(), got.Sell.String(), want)
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
		r    fx.Quote
	}{
		{"currency EUR", fx.Quote{Date: date, Currency: "EUR", Buy: dec(t, "3.7"), Sell: dec(t, "3.8"), Source: fx.SourceOfficial}},
		{"source unknown", fx.Quote{Date: date, Currency: fx.USD, Buy: dec(t, "3.7"), Sell: dec(t, "3.8"), Source: "sbs"}},
		{"buy zero", fx.Quote{Date: date, Currency: fx.USD, Buy: dec(t, "0"), Sell: dec(t, "3.8"), Source: fx.SourceOfficial}},
		{"sell negative", fx.Quote{Date: date, Currency: fx.USD, Buy: dec(t, "3.7"), Sell: dec(t, "-3.8"), Source: fx.SourceOfficial}},
		{"buy rounds to zero", fx.Quote{Date: date, Currency: fx.USD, Buy: dec(t, "0.0004"), Sell: dec(t, "3.8"), Source: fx.SourceOfficial}},
		{"rounds past numeric(6,3)", fx.Quote{Date: date, Currency: fx.USD, Buy: dec(t, "3.7"), Sell: dec(t, "999.9995"), Source: fx.SourceOfficial}},
		{"zero value decimal", fx.Quote{Date: date, Currency: fx.USD, Source: fx.SourceOfficial}},
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
	err := e.w.Save(ctx, fx.Quote{Date: date, Currency: fx.USD, Buy: dec(t, "9.9"), Sell: dec(t, "9.9"), Source: fx.SourceManual})
	if !errors.Is(err, fx.ErrAlreadyExists) {
		t.Errorf("second Save err = %v, want ErrAlreadyExists", err)
	}
	got, err := e.r.Rate(ctx, date, fx.USD)
	if err != nil || got.Buy.String() != "3.700" || got.Source != fx.SourceOfficial {
		t.Errorf("after a duplicate Save: %+v (err=%v), want the first row untouched", got, err)
	}
}

func TestToday(t *testing.T) {
	lima := time.FixedZone("any", -5*3600)
	tests := []struct {
		name string
		now  time.Time
		want fx.Day
	}{
		{"20:30 of the 19th in Lima, written in UTC (already the 20th there)", time.Date(2026, 11, 20, 1, 30, 0, 0, time.UTC), day(2026, 11, 19)},
		{"20:30 of the 19th in Lima, written in Lima", time.Date(2026, 11, 19, 20, 30, 0, 0, lima), day(2026, 11, 19)},
		{"23:59 in Lima is still the 19th", time.Date(2026, 11, 19, 23, 59, 0, 0, lima), day(2026, 11, 19)},
		{"23:59 in Lima, written in UTC", time.Date(2026, 11, 20, 4, 59, 0, 0, time.UTC), day(2026, 11, 19)},
		{"00:01 in Lima is the 20th", time.Date(2026, 11, 20, 0, 1, 0, 0, lima), day(2026, 11, 20)},
		{"00:01 in Lima, written in UTC", time.Date(2026, 11, 20, 5, 1, 0, 0, time.UTC), day(2026, 11, 20)},
		{"a zone ahead of UTC does not move the day", time.Date(2026, 11, 20, 3, 0, 0, 0, time.FixedZone("far", 9*3600)), day(2026, 11, 19)},
		{"year end", time.Date(2027, 1, 1, 4, 59, 0, 0, time.UTC), day(2026, 12, 31)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fx.Today(tt.now); got != tt.want {
				t.Errorf("Today = %v, want %v", got, tt.want)
			}
		})
	}
}

// Un consumidor que pasa el instante actual por Today pide el día de Lima aunque el host esté en UTC.
func TestRateForALimaInstantInUTC(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	save(t, e.w, day(2026, 11, 19), "3.750", "3.760", fx.SourceManual)
	save(t, e.w, day(2026, 11, 20), "3.800", "3.810", fx.SourceOfficial)
	tests := []struct {
		name       string
		now        time.Time
		buy, src   string
		wantRateOn string
	}{
		{"20:30 of the 19th in Lima, in UTC", time.Date(2026, 11, 20, 1, 30, 0, 0, time.UTC), "3.750", fx.SourceManual, "2026-11-19"},
		{"23:59 of the 19th in Lima, in UTC", time.Date(2026, 11, 20, 4, 59, 0, 0, time.UTC), "3.750", fx.SourceManual, "2026-11-19"},
		{"00:01 of the 20th in Lima, in UTC", time.Date(2026, 11, 20, 5, 1, 0, 0, time.UTC), "3.800", fx.SourceOfficial, "2026-11-20"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.r.Rate(ctx, fx.Today(tt.now), fx.USD)
			if err != nil {
				t.Fatal(err)
			}
			if got.Buy.String() != tt.buy || got.Source != tt.src || got.Date.String() != tt.wantRateOn {
				t.Errorf("Rate = %+v, want buy=%s source=%s on %s", got, tt.buy, tt.src, tt.wantRateOn)
			}
		})
	}
}

func TestInvalidDay(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	for _, d := range []fx.Day{{}, day(2026, 2, 30), day(2026, 13, 1), day(2026, 11, 0)} {
		if _, err := e.r.Rate(ctx, d, fx.USD); !errors.Is(err, fx.ErrInvalidRate) {
			t.Errorf("Rate(%v) err = %v, want ErrInvalidRate", d, err)
		}
		err := e.w.Save(ctx, fx.Quote{Date: d, Currency: fx.USD, Buy: dec(t, "3.7"), Sell: dec(t, "3.8"), Source: fx.SourceOfficial})
		if !errors.Is(err, fx.ErrInvalidRate) {
			t.Errorf("Save(%v) err = %v, want ErrInvalidRate", d, err)
		}
	}
}

// La salida (String y JSON) lleva siempre 3 decimales: 3.750, no 3.75 (api/conventions.md §6).
func TestOutputKeepsThreeDecimals(t *testing.T) {
	e := setup(t)
	save(t, e.w, day(2026, 11, 19), "3.75", "3.7", fx.SourceOfficial)
	got, err := e.r.Rate(context.Background(), day(2026, 11, 19), fx.USD)
	if err != nil {
		t.Fatal(err)
	}
	if got.Buy.String() != "3.750" || got.Sell.String() != "3.700" {
		t.Errorf("String = %s / %s, want 3.750 / 3.700", got.Buy, got.Sell)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["Buy"] != "3.750" || m["Sell"] != "3.700" {
		t.Errorf("JSON = %s, want Buy \"3.750\" and Sell \"3.700\"", b)
	}
	if got.Buy.Decimal().Cmp(dec(t, "3.75")) != 0 {
		t.Errorf("Decimal() = %s", got.Buy.Decimal())
	}
}

// Un Rate sobrevive la ida y vuelta por JSON: el día como yyyy-mm-dd y los valores con 3 decimales.
func TestRateJSONRoundTrip(t *testing.T) {
	e := setup(t)
	save(t, e.w, day(2026, 11, 19), "3.75", "3.7", fx.SourceManual)
	want, err := e.r.Rate(context.Background(), day(2026, 11, 20), fx.USD) // con retroceso
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	const wantJSON = `{"Date":"2026-11-19","Currency":"USD","Buy":"3.750","Sell":"3.700","Source":"fallback"}`
	if string(b) != wantJSON {
		t.Errorf("JSON = %s, want %s", b, wantJSON)
	}
	var got fx.Rate
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Date != want.Date || got.Currency != want.Currency || got.Source != want.Source ||
		got.Buy.String() != want.Buy.String() || got.Sell.String() != want.Sell.String() ||
		!got.Buy.Decimal().Equal(want.Buy.Decimal()) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestJSONRejectsInvalid(t *testing.T) {
	t.Run("day", func(t *testing.T) {
		for _, in := range []string{`"0000-01-01"`, `"2026-02-30"`, `"2026-13-01"`, `"2026-11-00"`, `"2026-1-5"`, `"20261119"`, `""`, `"2026-11-19T00:00:00Z"`, `20261119`, `null`} {
			var d fx.Day
			if err := json.Unmarshal([]byte(in), &d); err == nil && in != `null` {
				t.Errorf("Day %s accepted as %v", in, d)
			}
		}
		var d fx.Day
		if err := json.Unmarshal([]byte(`"2026-11-19"`), &d); err != nil || d != day(2026, 11, 19) {
			t.Errorf("valid day = %v (err=%v)", d, err)
		}
		if _, err := json.Marshal(day(2026, 2, 30)); err == nil {
			t.Error("an impossible day was marshaled")
		}
	})
	t.Run("value", func(t *testing.T) {
		for _, in := range []string{`"3.7501"`, `"0"`, `"0.000"`, `"-1"`, `"-3.750"`, `"1000"`, `"999.9990"`, `"1e2"`, `" 3.7"`, `"3.7 "`, `"+3.7"`, `"3."`, `".5"`, `""`, `"abc"`, `"3,7"`, `3.75`, `null`} {
			var v fx.Value
			if err := json.Unmarshal([]byte(in), &v); err == nil && in != `null` {
				t.Errorf("Value %s accepted as %v", in, v)
			}
		}
		for in, want := range map[string]string{`"3.75"`: "3.750", `"3"`: "3.000", `"0.001"`: "0.001", `"999.999"`: "999.999"} {
			var v fx.Value
			if err := json.Unmarshal([]byte(in), &v); err != nil || v.String() != want {
				t.Errorf("Value %s = %v (err=%v), want %s", in, v, err, want)
			}
		}
	})
}
