package audit_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/platform/audit"
)

// monthStart devuelve el primer día (UTC) del mes de now más n meses.
func monthStart(now time.Time, n int) time.Time {
	u := now.UTC()
	return time.Date(u.Year(), u.Month()+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
}

func suffix(m time.Time) string { return fmt.Sprintf("%04dm%02d", m.Year(), int(m.Month())) }

func currentPartitionSuffix() string { return suffix(monthStart(time.Now(), 0)) }

func partitionNames(t testing.TB, ctx context.Context, c *pgx.Conn) []string {
	t.Helper()
	rows, err := c.Query(ctx, `SELECT c.relname FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = 'audit.events'::regclass ORDER BY c.relname`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func dropAllPartitions(t testing.TB, ctx context.Context, c *pgx.Conn) {
	t.Helper()
	for _, n := range partitionNames(t, ctx, c) {
		if _, err := c.Exec(ctx, `DROP TABLE audit.`+n); err != nil {
			t.Fatal(err)
		}
	}
}

// Criterio 8: la migración deja el mes actual y los 3 siguientes.
func TestMigrationCreatesTheCurrentAndNextThreePartitions(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	got := partitionNames(t, ctx, e.d.Admin(ctx, t))
	var want []string
	for i := 0; i <= 3; i++ {
		want = append(want, "events_y"+suffix(monthStart(time.Now(), i)))
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("partitions %v, want %v", got, want)
	}
}

// Criterio 8: el job crea la del mes siguiente y es idempotente.
func TestEnsurePartitionsCreatesTheNextMonthAndIsIdempotent(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	c := e.d.Admin(ctx, t)
	// Un mes después: hace falta una nueva (la 4ª por delante).
	later := monthStart(time.Now(), 1).Add(24 * time.Hour)
	n, err := audit.EnsurePartitions(ctx, e.worker, later, audit.PartitionsAhead)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("created %d partitions, want 1 (the month after the current +3)", n)
	}
	want := "events_y" + suffix(monthStart(time.Now(), 4))
	if names := partitionNames(t, ctx, c); names[len(names)-1] != want || len(names) != 5 {
		t.Errorf("partitions %v, want 5 ending in %s", names, want)
	}
	// Otra vez, y otra: nada nuevo y sin error.
	for i := 0; i < 2; i++ {
		if n, err := audit.EnsurePartitions(ctx, e.worker, later, audit.PartitionsAhead); err != nil || n != 0 {
			t.Errorf("repeat %d: created %d, err %v; want 0, nil", i, n, err)
		}
	}
	if len(partitionNames(t, ctx, c)) != 5 {
		t.Error("a repeat created or dropped partitions")
	}
	// Los eventos del mes nuevo caen en su partición.
	if _, err := c.Exec(ctx, `INSERT INTO audit.events (tenant_id, actor_type, action, at) VALUES ($1, 'system', 'a.b:c', $2)`, tenantA, monthStart(time.Now(), 4).Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if count(t, ctx, c, `SELECT count(*) FROM audit.`+want) != 1 {
		t.Error("the event went to another partition")
	}
}

// Las particiones son de UTC: el límite de mes no depende de la zona horaria de la sesión que las crea.
func TestPartitionBoundsAreUTC(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	c := e.d.Admin(ctx, t)
	// Se crea el mes +6 desde una sesión en Lima (UTC-5): si los límites usaran la zona de la sesión, empezarían a las 05:00 UTC.
	if _, err := c.Exec(ctx, `SET timezone = 'America/Lima'`); err != nil {
		t.Fatal(err)
	}
	m6, m7 := monthStart(time.Now(), 6), monthStart(time.Now(), 7)
	if _, err := c.Exec(ctx, `SELECT audit.ensure_partitions($1::date, 1)`, m6.Format("2006-01-02")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		at   time.Time
		part string
	}{
		{m6, "events_y" + suffix(m6)},                   // 00:00 UTC del primer día
		{m7.Add(-time.Second), "events_y" + suffix(m6)}, // 23:59:59 UTC del último día
		{m7, "events_y" + suffix(m7)},                   // 00:00 UTC del mes siguiente
	} {
		if _, err := c.Exec(ctx, `INSERT INTO audit.events (tenant_id, actor_type, action, at) VALUES ($1, 'system', 'a.b:c', $2)`, tenantA, tc.at); err != nil {
			t.Fatalf("inserting at %s: %v", tc.at, err)
		}
		var got string
		if err := c.QueryRow(ctx, `SELECT tableoid::regclass::text FROM audit.events WHERE at = $1`, tc.at).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != "audit."+tc.part {
			t.Errorf("event at %s is in %s, want audit.%s", tc.at.Format(time.RFC3339), got, tc.part)
		}
	}
}

// Criterio 8: un insert sin partición falla de forma visible y revierte todo; no se descarta en silencio.
func TestInsertWithoutAPartitionFailsLoudly(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	c := e.d.Admin(ctx, t)
	if _, err := c.Exec(ctx, `CREATE TABLE public.change (id int); GRANT ALL ON public.change TO wanka_app`); err != nil {
		t.Fatal(err)
	}
	dropAllPartitions(t, ctx, c)
	err := e.app.InTenantTx(ctx, userActor(t, tenantA, userA), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO public.change VALUES (1)`); err != nil {
			return err
		}
		return e.w.Record(ctx, tx, ev)
	})
	if err == nil || sqlState(err) != "23514" {
		t.Fatalf("err = %v, want SQLSTATE 23514 (no partition of relation found)", err)
	}
	if count(t, ctx, c, `SELECT count(*) FROM public.change`) != 0 {
		t.Error("the change was committed although its event had nowhere to go")
	}
}

// El worker es el único con EXECUTE; la función rechaza argumentos fuera de rango.
func TestEnsurePartitionsFunctionAccessAndRange(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	if _, err := audit.EnsurePartitions(ctx, e.app, time.Now(), 1); sqlState(errors.Unwrap(err)) != "42501" && sqlState(err) != "42501" {
		t.Errorf("wanka_app executed audit.ensure_partitions: %v", err)
	}
	c := e.d.Admin(ctx, t)
	for _, role := range []string{"wanka_app", "wanka_webhooks", "wanka_platform", "wanka_definer", "public"} {
		var has bool
		if err := c.QueryRow(ctx, `SELECT has_function_privilege($1, 'audit.ensure_partitions(date,integer)', 'EXECUTE')`, role).Scan(&has); err != nil {
			t.Fatal(err)
		}
		if has {
			t.Errorf("%s can execute audit.ensure_partitions", role)
		}
	}
	for name, args := range map[string][2]any{
		"too many months ahead": {time.Now().UTC().Format("2006-01-02"), 13},
		"negative":              {time.Now().UTC().Format("2006-01-02"), -1},
		"too old":               {"2025-12-31", 1},
		"too far":               {time.Now().AddDate(3, 0, 0).UTC().Format("2006-01-02"), 1},
	} {
		err := e.worker.InNoTenantTx(ctx, noScope, func(tx pgx.Tx) error {
			var n int
			return tx.QueryRow(ctx, `SELECT audit.ensure_partitions($1::date, $2::int)`, args[0], args[1]).Scan(&n)
		})
		if sqlState(err) != "22023" {
			t.Errorf("%s: err = %v, want SQLSTATE 22023", name, err)
		}
	}
}

// Dos workers a la vez no se pisan.
func TestEnsurePartitionsConcurrent(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	errs := make(chan error, 8)
	later := monthStart(time.Now(), 6)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := audit.EnsurePartitions(ctx, e.worker, later, audit.PartitionsAhead)
			mu.Lock()
			total += n
			mu.Unlock()
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	// De +6 a +9 meses hay cuatro, y se crean exactamente una vez entre los 8.
	if total != 4 {
		t.Errorf("8 concurrent runs created %d partitions in total, want 4", total)
	}
	if n := len(partitionNames(t, ctx, e.d.Admin(ctx, t))); n != 8 {
		t.Errorf("%d partitions, want 8", n)
	}
}

// Una partición nueva es del padre: mismos índices y sin permisos para los roles de aplicación.
func TestNewPartitionHasNoGrantsAndKeepsTheKey(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	if _, err := audit.EnsurePartitions(ctx, e.worker, monthStart(time.Now(), 6), 0); err != nil {
		t.Fatal(err)
	}
	c := e.d.Admin(ctx, t)
	name := "events_y" + suffix(monthStart(time.Now(), 6))
	if count(t, ctx, c, `SELECT count(*) FROM pg_index i WHERE i.indrelid = ('audit.' || $1)::regclass AND i.indisprimary`, name) != 1 {
		t.Error("the new partition has no primary key")
	}
	if count(t, ctx, c, `SELECT count(*) FROM unnest(ARRAY['wanka_app','wanka_worker']) r WHERE has_table_privilege(r, ('audit.' || $1)::regclass, 'SELECT,INSERT,UPDATE,DELETE')`, name) != 0 {
		t.Error("a role holds privileges on the new partition")
	}
}
