package jobs

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/riverqueue/river/rivermigrate"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

type tickArgs struct{}

func (tickArgs) Kind() string { return "test.tick" }

// Criterio 7: con dos réplicas del worker, un job periódico corre una vez por ciclo (líder de River).
func TestPeriodicJobRunsOncePerCycleWithTwoReplicas(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	admin := d.Admin(ctx, t)
	var runs atomic.Int32
	setup := func(r *Registry) {
		if err := Register(r, func(ctx context.Context, j Job[tickArgs]) error { runs.Add(1); return nil }); err != nil {
			t.Fatal(err)
		}
		if err := r.AddPeriodic(500*time.Millisecond, func() Args { return tickArgs{} }, WithQueue(QueueMaintenance)); err != nil {
			t.Fatal(err)
		}
	}
	// Dos réplicas: cada una con su pool y su cliente de River.
	startWorker(t, openPool(t, d, dbtest.Worker, 5), Config{}, setup)
	startWorker(t, openPool(t, d, dbtest.Worker, 5), Config{}, setup)

	var leaders int
	eventually(t, 10*time.Second, "a leader to be elected", func() bool {
		return admin.QueryRow(ctx, `SELECT count(*) FROM ops.river_leader`).Scan(&leaders) == nil && leaders == 1
	})
	eventually(t, 30*time.Second, "5 periodic jobs", func() bool { return len(jobsOfKind(ctx, t, admin, "test.tick")) >= 5 })
	// Una réplica sin líder duplicaría cada ciclo: dos jobs con casi la misma hora de creación.
	rows, err := admin.Query(ctx, `SELECT extract(epoch FROM created_at - lag(created_at) OVER (ORDER BY created_at)) FROM ops.river_job WHERE kind = 'test.tick' ORDER BY created_at`)
	if err != nil {
		t.Fatal(err)
	}
	var gaps []float64
	for rows.Next() {
		var g *float64
		if err := rows.Scan(&g); err != nil {
			t.Fatal(err)
		}
		if g != nil {
			gaps = append(gaps, *g)
		}
	}
	rows.Close()
	t.Logf("%d periodic jobs, gaps between them (s): %v, handler runs %d", len(gaps)+1, gaps, runs.Load())
	for _, g := range gaps {
		if g < 0.3 {
			t.Fatalf("two periodic jobs %.3f s apart on a 500 ms cycle: both replicas enqueued the same cycle", g)
		}
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM ops.river_leader`).Scan(&leaders); err != nil || leaders != 1 {
		t.Fatalf("%d leaders (%v), want 1", leaders, err)
	}
}

func TestAddPeriodicRejectsBadInput(t *testing.T) {
	d := migrated(t)
	r := NewRegistry(openPool(t, d, dbtest.Worker, 2))
	if err := r.AddPeriodic(0, func() Args { return tickArgs{} }, WithQueue(QueueMaintenance)); err == nil {
		t.Error("a zero interval was accepted")
	}
	if err := r.AddPeriodic(time.Second, func() Args { return tickArgs{} }); err == nil {
		t.Error("a periodic job with no queue was accepted")
	}
}

func TestRegisterRejectsADuplicateKind(t *testing.T) {
	d := migrated(t)
	r := NewRegistry(openPool(t, d, dbtest.Worker, 2))
	h := func(context.Context, Job[tickArgs]) error { return nil }
	if err := Register(r, h); err != nil {
		t.Fatal(err)
	}
	if err := Register(r, h); err == nil {
		t.Error("a second handler for the same kind was accepted")
	}
}

// Criterio 8: completados 7 días, cancelados 7, descartados 30. La configuración...
func TestRetentionConfig(t *testing.T) {
	if CompletedRetention != 7*24*time.Hour || CancelledRetention != 7*24*time.Hour || DiscardedRetention != 30*24*time.Hour {
		t.Fatalf("retention: completed %s, cancelled %s, discarded %s", CompletedRetention, CancelledRetention, DiscardedRetention)
	}
}

// ... y el JobCleaner del worker la aplica: borra lo más viejo que el plazo y conserva lo más nuevo.
func TestJobCleanerDeletesFinishedJobsPastTheirRetention(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	admin := d.Admin(ctx, t)
	insert := func(key, state string, age time.Duration) {
		t.Helper()
		_, err := admin.Exec(ctx, `INSERT INTO ops.river_job (state, kind, args, queue, finalized_at, max_attempts)
			VALUES ($1::ops.river_job_state, 'test.old', jsonb_build_object('key', $2::text), 'maintenance', now() - $3::interval, 1)`,
			state, key, age.String())
		if err != nil {
			t.Fatal(err)
		}
	}
	day := 24 * time.Hour
	for _, c := range []struct {
		key, state string
		age        time.Duration
		gone       bool
	}{
		{"completed-8d", "completed", 8 * day, true}, {"completed-6d", "completed", 6 * day, false},
		{"cancelled-8d", "cancelled", 8 * day, true}, {"cancelled-6d", "cancelled", 6 * day, false},
		{"discarded-31d", "discarded", 31 * day, true}, {"discarded-29d", "discarded", 29 * day, false},
		{"discarded-8d", "discarded", 8 * day, false}, // un descartado vive más que un completado
	} {
		insert(c.key, c.state, c.age)
	}
	startWorker(t, openPool(t, d, dbtest.Worker, 5), Config{}, func(r *Registry) {})

	left := func() map[string]bool {
		rows, err := admin.Query(ctx, `SELECT args->>'key' FROM ops.river_job WHERE kind = 'test.old'`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		m := map[string]bool{}
		for rows.Next() {
			var k string
			if err := rows.Scan(&k); err != nil {
				t.Fatal(err)
			}
			m[k] = true
		}
		return m
	}
	eventually(t, 30*time.Second, "the cleaner to delete the expired jobs", func() bool {
		m := left()
		return !m["completed-8d"] && !m["cancelled-8d"] && !m["discarded-31d"]
	})
	m := left()
	if !m["completed-6d"] || !m["cancelled-6d"] || !m["discarded-29d"] || !m["discarded-8d"] || len(m) != 4 {
		t.Fatalf("jobs left after cleaning: %v, want the four within their retention", m)
	}
}

// El esquema de ops creado por goose es el que crea el migrador de River (y no se desvía al subir River).
func TestSchemaMatchesRiversOwnMigrations(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	admin := d.Admin(ctx, t)
	if _, err := admin.Exec(ctx, `CREATE SCHEMA river_ref`); err != nil {
		t.Fatal(err)
	}
	// Como superusuario: el migrador de River crea el esquema de referencia con sus propias migraciones.
	super, err := openSuper(t, d)
	if err != nil {
		t.Fatal(err)
	}
	m, err := rivermigrate.New(super.RiverDriver(), &rivermigrate.Config{Schema: "river_ref", Logger: discardLog()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatalf("running River's own migrations: %v", err)
	}
	latest := m.AllVersions()[len(m.AllVersions())-1].Version

	// Columnas, tipos, defaults, índices, constraints y funciones, con el nombre del esquema normalizado.
	queries := map[string]string{
		"columns": `SELECT table_name || '.' || column_name || ' ' || data_type || ' ' || udt_name || ' ' || is_nullable || ' ' || coalesce(column_default, '')
			FROM information_schema.columns WHERE table_schema = $1 ORDER BY table_name, ordinal_position`,
		"indexes": `SELECT replace(indexdef, $1 || '.', '') FROM pg_indexes WHERE schemaname = $1 ORDER BY indexname`,
		"constraints": `SELECT c.conrelid::regclass::text || ' ' || c.conname || ' ' || pg_get_constraintdef(c.oid)
			FROM pg_constraint c JOIN pg_namespace n ON n.oid = c.connamespace WHERE n.nspname = $1 ORDER BY 1`,
		"enum": `SELECT e.enumlabel FROM pg_enum e JOIN pg_type t ON t.oid = e.enumtypid JOIN pg_namespace n ON n.oid = t.typnamespace
			WHERE n.nspname = $1 ORDER BY e.enumsortorder`,
		"persistence": `SELECT c.relname::text || ' ' || c.relpersistence::text FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = $1 AND c.relkind = 'r' ORDER BY 1`,
		"functions": `SELECT p.proname || ' ' || pg_get_function_arguments(p.oid) || ' ' || p.provolatile::text FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE n.nspname = $1 ORDER BY 1`,
		"function bodies": `SELECT p.proname::text || ' ' || pg_get_functiondef(p.oid) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE n.nspname = $1 ORDER BY 1`,
		"versions": `SELECT line || ' ' || version FROM ` + `%s.river_migration ORDER BY 1`,
	}
	norm := func(in []string, schema string) []string {
		out := make([]string, len(in))
		for i, v := range in {
			v = strings.ReplaceAll(v, schema+".", "")
			out[i] = strings.ReplaceAll(v, "river_migration_pkey1", "river_migration_pkey") // River renombra la tabla en su migración 4
		}
		return out
	}
	fetch := func(q, schema string) []string {
		t.Helper()
		if q == queries["versions"] {
			q = `SELECT line || ' ' || version FROM ` + schema + `.river_migration ORDER BY 1`
			rows, err := admin.Query(ctx, q)
			if err != nil {
				t.Fatal(err)
			}
			return scanStrings(t, rows)
		}
		rows, err := admin.Query(ctx, q, schema)
		if err != nil {
			t.Fatal(err)
		}
		return scanStrings(t, rows)
	}
	for name, q := range queries {
		ops, ref := norm(fetch(q, "ops"), "ops"), norm(fetch(q, "river_ref"), "river_ref")
		if name != "enum" && name != "versions" {
			// Fuera del esquema de River, ops tiene tablas propias del kernel (ops.exchange_rates, #170): solo river_*.
			// Los enums no se filtran: sus etiquetas no nombran la tabla y ops no tiene otros.
			ops, ref = onlyRiver(ops), onlyRiver(ref)
		}
		if len(ops) == 0 || len(ops) != len(ref) {
			t.Errorf("%s: ops has %d entries, River's migrations create %d\nops: %v\nref: %v", name, len(ops), len(ref), ops, ref)
			continue
		}
		for i := range ops {
			if ops[i] != ref[i] {
				t.Errorf("%s differs:\n  ops:  %s\n  ref:  %s", name, ops[i], ref[i])
			}
		}
	}
	// goose dejó registradas todas las versiones de la línea main: river migrate-list ve la base al día.
	var maxVersion int
	if err := admin.QueryRow(ctx, `SELECT max(version) FROM ops.river_migration WHERE line = 'main'`).Scan(&maxVersion); err != nil || maxVersion != latest {
		t.Errorf("ops.river_migration is at version %d (%v), River's latest is %d: add a goose migration", maxVersion, err, latest)
	}
}
