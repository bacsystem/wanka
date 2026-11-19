package entitlements_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
)

// seeded devuelve una empresa activa con el plan dado.
func (e *env) seeded(modules ...string) pgtype.UUID {
	e.t.Helper()
	tenant := e.tenant("activa")
	e.subscribe(tenant, e.plan("p-"+tenant.String(), `{}`, modules...), "activa")
	return tenant
}

// Criterio 7: dos lecturas dentro del TTL, una sola consulta; vencido el TTL, otra.
func TestSecondReadWithinTheTTLDoesNotTouchTheDatabase(t *testing.T) {
	e := newEnv(t)
	tenant := e.seeded("ventas")
	clk := newFake()
	counter := &countingDB{DB: e.pool(4)}
	s := entitlements.New(counter, entitlements.Options{Clock: clk})

	first := mustFor(t, s, tenant)
	// Un cambio en la base sin NOTIFY escuchado: la caché no debe enterarse todavía.
	e.exec(`UPDATE platform.subscriptions SET status = 'suspendida' WHERE tenant_id = $1`, tenant)
	clk.Advance(entitlements.DefaultTTL - time.Second)
	second := mustFor(t, s, tenant)
	if n := counter.reads.Load(); n != 1 {
		t.Fatalf("reads after two For within the TTL = %d, want 1", n)
	}
	if second.Version != first.Version || !second.Status.Usable() {
		t.Errorf("second read = %+v, want the cached first read", second)
	}

	clk.Advance(2 * time.Second) // 31 s en total
	third := mustFor(t, s, tenant)
	if n := counter.reads.Load(); n != 2 {
		t.Errorf("reads after the TTL = %d, want 2", n)
	}
	if third.Status != entitlements.StatusSuspended {
		t.Errorf("after the TTL Status = %q, want the new suspendida", third.Status)
	}
}

// Lo que devuelve For es una copia: mutarla no cambia la caché.
func TestReturnedValueIsACopy(t *testing.T) {
	e := newEnv(t)
	tenant := e.seeded("ventas", "inventario")
	s := e.service(entitlements.Options{})

	got := mustFor(t, s, tenant)
	got.Modules[0] = "hackeado"
	got.Limits["users"] = 1 << 40
	got.Usage["users"] = -1
	again := mustFor(t, s, tenant)
	if !slices.Equal(again.Modules, []string{"inventario", "ventas"}) || again.Limits["users"] == 1<<40 || again.Usage["users"] == -1 {
		t.Errorf("mutating a returned value changed the cache: %+v", again)
	}
}

// Muchas peticiones con la caché fría para la misma empresa hacen una sola consulta.
func TestConcurrentColdReadsShareOneQuery(t *testing.T) {
	e := newEnv(t)
	tenant := e.seeded("ventas")
	counter := &countingDB{DB: e.pool(4)}
	release := make(chan struct{})
	counter.before = func() { <-release } // mantiene la lectura en vuelo hasta que todos esperan
	s := entitlements.New(counter, entitlements.Options{})

	const callers = 40
	var wg sync.WaitGroup
	results := make([]entitlements.Entitlements, callers)
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], _ = s.For(ctx(t), tenant)
		}()
	}
	time.Sleep(200 * time.Millisecond) // deja que los 40 lleguen a For
	close(release)
	wg.Wait()
	if n := counter.reads.Load(); n != 1 {
		t.Errorf("%d concurrent cold reads made %d queries, want 1", callers, n)
	}
	for i, r := range results {
		if !slices.Equal(r.Modules, []string{"ventas"}) {
			t.Fatalf("caller %d got %v", i, r.Modules)
		}
	}
}

// Una invalidación que llega mientras la lectura está en vuelo impide guardar lo leído (pudo ser lo viejo).
func TestInvalidationDuringALoadPreventsCachingIt(t *testing.T) {
	e := newEnv(t)
	tenant := e.seeded("ventas")
	counter := &countingDB{DB: e.pool(4)}
	s := entitlements.New(counter, entitlements.Options{})
	fire := true
	counter.before = func() {
		if fire {
			fire = false
			s.Invalidate(tenant) // el NOTIFY llega con la lectura en curso
		}
	}
	mustFor(t, s, tenant)
	mustFor(t, s, tenant)
	if n := counter.reads.Load(); n != 2 {
		t.Errorf("reads = %d, want 2: the load that raced an invalidation must not be cached", n)
	}
	mustFor(t, s, tenant)
	if n := counter.reads.Load(); n != 2 {
		t.Errorf("reads = %d after a clean load, want it cached (2)", n)
	}
}

// Una petición que llega después del NOTIFY no se une a la lectura que empezó antes del cambio: lanza una nueva.
func TestRequestAfterAnInvalidationDoesNotJoinTheStaleInflightLoad(t *testing.T) {
	e := newEnv(t)
	tenant := e.seeded("ventas")
	counter := &countingDB{DB: e.pool(4)}
	release := make(chan struct{})
	started := make(chan struct{})
	counter.before = func() {
		if counter.reads.Load() == 1 { // solo la primera lectura espera
			close(started)
			<-release
		}
	}
	s := entitlements.New(counter, entitlements.Options{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _, _ = s.For(ctx(t), tenant) }()
	<-started
	s.Invalidate(tenant) // el cambio ya está hecho y avisado; la primera lectura es anterior
	done := make(chan struct{})
	go func() { defer close(done); _, _ = s.For(ctx(t), tenant) }()
	waitFor(t, "a fresh read instead of joining the stale one", func() bool { return counter.reads.Load() == 2 })
	close(release)
	wg.Wait()
	<-done
}

// Al reconectar, Listen avisa y la caché se vacía: lo que se notificó mientras no había conexión se perdió.
func TestRunDropsTheCacheWhenTheListenerReconnects(t *testing.T) {
	e := newEnv(t)
	tenant := e.seeded("ventas")
	counter := &countingDB{DB: e.pool(4)}
	s := entitlements.New(counter, entitlements.Options{Clock: newFake()}) // el reloj quieto: solo el reconectar puede vaciar
	runCtx, stop := context.WithCancel(ctx(t))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _ = s.Run(runCtx) }()
	t.Cleanup(func() { stop(); wg.Wait() })
	listeners := func() int {
		var n int
		_ = e.admin.QueryRow(ctx(t), `SELECT count(*) FROM pg_stat_activity WHERE query LIKE 'LISTEN %' AND datname = current_database()`).Scan(&n)
		return n
	}
	waitFor(t, "listening", func() bool { return listeners() == 1 })
	mustFor(t, s, tenant)
	mustFor(t, s, tenant)
	if n := counter.reads.Load(); n != 1 {
		t.Fatalf("reads = %d, want 1 (cached)", n)
	}
	e.exec(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE query LIKE 'LISTEN %' AND datname = current_database()`)
	waitFor(t, "cache dropped after reconnecting", func() bool {
		mustFor(t, s, tenant)
		return counter.reads.Load() > 1
	})
}

// La caché tiene tope de entradas.
func TestCacheIsBounded(t *testing.T) {
	e := newEnv(t)
	plan := e.plan("p", `{}`, "ventas")
	counter := &countingDB{DB: e.pool(4)}
	s := entitlements.New(counter, entitlements.Options{MaxEntries: 3})
	var tenants []pgtype.UUID
	for range 6 {
		tn := e.tenant("activa")
		e.subscribe(tn, plan, "activa")
		tenants = append(tenants, tn)
		mustFor(t, s, tn)
	}
	before := counter.reads.Load()
	for _, tn := range tenants {
		mustFor(t, s, tn)
	}
	// Con tope 3 caben a lo sumo 3 de 6: al menos 3 releen.
	if re := counter.reads.Load() - before; re < 3 {
		t.Errorf("only %d re-reads of 6 tenants with a 3-entry cache: the cache is not bounded", re)
	}
}

// Criterio 7, parte 2: un NOTIFY invalida la entrada antes del TTL, en cada réplica (dos instancias).
func TestNotifyInvalidatesBeforeTheTTLOnEveryReplica(t *testing.T) {
	e := newEnv(t)
	tenant := e.seeded("ventas")
	other := e.seeded("inventario")
	clk := newFake() // el reloj no avanza: solo el NOTIFY puede invalidar

	pools := []*countingDB{{DB: e.pool(4)}, {DB: e.pool(4)}}
	replicas := []*entitlements.Service{
		entitlements.New(pools[0], entitlements.Options{Clock: clk}),
		entitlements.New(pools[1], entitlements.Options{Clock: clk}),
	}
	runCtx, stop := context.WithCancel(ctx(t))
	var wg sync.WaitGroup
	for _, r := range replicas {
		wg.Add(1)
		go func() { defer wg.Done(); _ = r.Run(runCtx) }()
	}
	t.Cleanup(func() { stop(); wg.Wait() })

	// Espera a que ambas escuchen: los NOTIFY anteriores al LISTEN no llegan.
	waitFor(t, "both replicas listening", func() bool {
		var n int
		_ = e.admin.QueryRow(ctx(t), `SELECT count(*) FROM pg_stat_activity WHERE query LIKE 'LISTEN %' AND datname = current_database()`).Scan(&n)
		return n == 2
	})

	for _, r := range replicas {
		mustFor(t, r, tenant)
		mustFor(t, r, other)
	}
	e.exec(`INSERT INTO platform.tenant_module_overrides (tenant_id, module_code, enabled) VALUES ($1, 'compras', true)`, tenant)

	for i, r := range replicas {
		waitFor(t, "replica sees the override", func() bool {
			got, err := r.For(ctx(t), tenant)
			return err == nil && got.HasModule("compras")
		})
		t.Logf("replica %d saw the change without the TTL passing", i)
	}
	// La otra empresa no se tocó: sigue en la caché (una sola lectura por réplica).
	for i, p := range pools {
		before := p.reads.Load()
		mustFor(t, replicas[i], other)
		if p.reads.Load() != before {
			t.Errorf("replica %d re-read an unrelated tenant after a tenant-scoped NOTIFY", i)
		}
	}
	// Un cambio de catálogo (payload vacío) invalida a todas las empresas.
	e.exec(`INSERT INTO platform.feature_flags (key, default_on) VALUES ('global', true)`)
	for _, r := range replicas {
		waitFor(t, "global change reaches every tenant", func() bool {
			a, errA := r.For(ctx(t), other)
			b, errB := r.For(ctx(t), tenant)
			return errA == nil && errB == nil && a.HasFlag("global") && b.HasFlag("global")
		})
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for: %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// BenchmarkHotCache mide For con la caché caliente (criterio no funcional: < 1 ms).
func BenchmarkHotCache(b *testing.B) {
	e := newEnv(b)
	tenant := e.seeded("ventas", "inventario", "compras", "rubro.lubricentro")
	e.exec(`INSERT INTO platform.feature_flags (key, default_on) VALUES ('a', true), ('b', true), ('c', false)`)
	s := e.service(entitlements.Options{})
	mustFor(b, s, tenant)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := s.For(context.Background(), tenant); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkColdRead mide la lectura completa (una consulta agregada) contra la base real.
func BenchmarkColdRead(b *testing.B) {
	e := newEnv(b)
	tenant := e.seeded("ventas", "inventario", "compras", "rubro.lubricentro")
	e.exec(`INSERT INTO platform.feature_flags (key, default_on) VALUES ('a', true), ('b', true), ('c', false)`)
	s := e.service(entitlements.Options{TTL: time.Nanosecond})
	mustFor(b, s, tenant)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		time.Sleep(time.Microsecond) // el TTL de 1 ns ya venció: cada For lee de la base
		if _, err := s.For(context.Background(), tenant); err != nil {
			b.Fatal(err)
		}
	}
}
