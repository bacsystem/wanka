package audit_test

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bacsystem/wanka/apps/api/internal/platform/audit"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/logging"
)

var noScope = db.NoTenantScope{}

var ev = audit.Event{Action: "ventas.documento:void", EntityType: "sales.document"}

func sqlState(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// Criterio 1: la tabla, sus columnas, la RLS y los GRANT.
func TestTableShapeAndGrants(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	c := e.d.Admin(ctx, t)

	var strategy string
	var key string
	if err := c.QueryRow(ctx, `SELECT partstrat::text, (SELECT attname FROM pg_attribute WHERE attrelid = 'audit.events'::regclass AND attnum = partattrs[0])
		FROM pg_partitioned_table WHERE partrelid = 'audit.events'::regclass`).Scan(&strategy, &key); err != nil {
		t.Fatal(err)
	}
	if strategy != "r" || key != "at" {
		t.Errorf("partitioned by strategy %q on %q, want range on at", strategy, key)
	}

	want := map[string]string{
		"id": "uuid", "tenant_id": "uuid", "actor_type": "text", "actor_user_id": "uuid", "impersonator_id": "uuid",
		"action": "text", "entity_type": "text", "entity_id": "uuid", "result": "text", "ip": "inet",
		"request_id": "text", "diff": "jsonb", "at": "timestamp with time zone",
	}
	rows, err := c.Query(ctx, `SELECT attname, format_type(atttypid, atttypmod), attnotnull FROM pg_attribute
		WHERE attrelid = 'audit.events'::regclass AND attnum > 0 AND NOT attisdropped`)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	notNull := map[string]bool{}
	for rows.Next() {
		var n, ty string
		var nn bool
		if err := rows.Scan(&n, &ty, &nn); err != nil {
			t.Fatal(err)
		}
		got[n], notNull[n] = ty, nn
	}
	rows.Close()
	for col, ty := range want {
		if got[col] != ty {
			t.Errorf("column %s is %q, want %q", col, got[col], ty)
		}
	}
	if len(got) != len(want) {
		t.Errorf("audit.events has %d columns, want %d: %v", len(got), len(want), got)
	}
	for _, col := range []string{"tenant_id", "actor_type", "action", "result", "at"} {
		if !notNull[col] {
			t.Errorf("%s must be NOT NULL", col)
		}
	}

	if count(t, ctx, c, `SELECT count(*) FROM pg_class WHERE oid = 'audit.events'::regclass AND relrowsecurity AND relforcerowsecurity`) != 1 {
		t.Error("audit.events needs ENABLE and FORCE ROW LEVEL SECURITY")
	}
	if count(t, ctx, c, `SELECT count(*) FROM pg_policies WHERE schemaname = 'audit' AND tablename = 'events' AND policyname = 'tenant_isolation' AND cmd = 'ALL'
		AND qual LIKE '%tenant_id = app.current_tenant()%' AND with_check LIKE '%tenant_id = app.current_tenant()%'`) != 1 {
		t.Error("audit.events needs the standard tenant_isolation policy")
	}
	for _, role := range []string{"wanka_app", "wanka_worker"} {
		for priv, allowed := range map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": false, "DELETE": false, "TRUNCATE": false, "REFERENCES": false, "TRIGGER": false} {
			var has bool
			if err := c.QueryRow(ctx, `SELECT has_table_privilege($1, 'audit.events', $2)`, role, priv).Scan(&has); err != nil {
				t.Fatal(err)
			}
			if has != allowed {
				t.Errorf("%s %s on audit.events = %v, want %v", role, priv, has, allowed)
			}
		}
	}
	for _, role := range []string{"wanka_webhooks", "wanka_platform"} {
		if count(t, ctx, c, `SELECT count(*) FROM pg_class c WHERE c.oid = 'audit.events'::regclass AND has_table_privilege($1, c.oid, 'SELECT,INSERT,UPDATE,DELETE')`, role) != 0 {
			t.Errorf("%s must have no privilege on audit.events", role)
		}
	}
	// Las particiones no conceden nada: solo se llega a ellas por el padre.
	if n := count(t, ctx, c, `SELECT count(*) FROM pg_inherits i JOIN pg_class p ON p.oid = i.inhrelid, unnest(ARRAY['wanka_app','wanka_worker','wanka_webhooks','wanka_platform']) r
		WHERE i.inhparent = 'audit.events'::regclass AND has_table_privilege(r, p.oid, 'SELECT,INSERT,UPDATE,DELETE')`); n != 0 {
		t.Errorf("%d role/partition pairs hold privileges on a partition", n)
	}
}

// Criterio 1: los CHECK, incluido el del architect (una fila user sin usuario es imposible).
func TestActorChecks(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	c := e.d.Admin(ctx, t)
	for _, tc := range []struct {
		name, sql string
		want      string // "" = debe aceptarse; si no, el nombre del CHECK
	}{
		{"user with user", `('user', '` + userA + `')`, ""},
		{"user without user", `('user', NULL)`, "events_user_has_user"},
		{"system without user", `('system', NULL)`, ""},
		{"system with user", `('system', '` + userA + `')`, "events_system_has_no_user"},
		{"platform_staff without user", `('platform_staff', NULL)`, ""},
		{"unknown actor", `('robot', NULL)`, "events_actor_type_check"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.Exec(ctx, `INSERT INTO audit.events (tenant_id, actor_type, actor_user_id, action) SELECT '`+tenantA+`', v.t, v.u::uuid, 'x.y:z' FROM (VALUES `+tc.sql+`) v(t, u)`)
			var pe *pgconn.PgError
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("rejected: %v", err)
			case tc.want != "" && !errors.As(err, &pe):
				t.Fatalf("accepted, want a violation of %s", tc.want)
			case tc.want != "" && pe.ConstraintName != tc.want:
				t.Fatalf("violated %q, want %q", pe.ConstraintName, tc.want)
			}
		})
	}
	if _, err := c.Exec(ctx, `INSERT INTO audit.events (tenant_id, actor_type, action, result) VALUES ($1, 'system', 'x.y:z', 'quizas')`, tenantA); sqlState(err) != "23514" {
		t.Errorf("result outside ok|denegado: %v", err)
	}
}

// Criterio 2: el evento confirma o se revierte con el cambio.
func TestRecordIsInTheSameTransactionAsTheChange(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	c := e.d.Admin(ctx, t)
	if _, err := c.Exec(ctx, `CREATE TABLE public.change (id int)`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, `GRANT ALL ON public.change TO wanka_app`); err != nil {
		t.Fatal(err)
	}
	actor := userActor(t, tenantA, userA)
	boom := errors.New("boom")

	t.Run("commit keeps the change and the event", func(t *testing.T) {
		err := e.app.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO public.change VALUES (1)`); err != nil {
				return err
			}
			return e.w.Record(ctx, tx, ev)
		})
		if err != nil {
			t.Fatal(err)
		}
		if n := count(t, ctx, c, `SELECT count(*) FROM audit.events`); n != 1 {
			t.Errorf("%d events after commit, want 1", n)
		}
		if n := count(t, ctx, c, `SELECT count(*) FROM public.change`); n != 1 {
			t.Errorf("%d changes after commit, want 1", n)
		}
	})
	t.Run("rollback after Record leaves neither", func(t *testing.T) {
		err := e.app.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO public.change VALUES (2)`); err != nil {
				return err
			}
			if err := e.w.Record(ctx, tx, ev); err != nil {
				return err
			}
			return boom
		})
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want boom", err)
		}
		if n := count(t, ctx, c, `SELECT count(*) FROM audit.events`); n != 1 {
			t.Errorf("%d events after rollback, want still 1", n)
		}
		if n := count(t, ctx, c, `SELECT count(*) FROM public.change WHERE id = 2`); n != 0 {
			t.Error("the rolled back change is visible")
		}
	})
	t.Run("a failing insert aborts the whole transaction", func(t *testing.T) {
		err := e.app.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO public.change VALUES (3)`); err != nil {
				return err
			}
			return e.w.Record(ctx, tx, audit.Event{Action: "no es una accion"})
		})
		if !errors.Is(err, audit.ErrInvalidEvent) {
			t.Fatalf("err = %v, want ErrInvalidEvent", err)
		}
		if n := count(t, ctx, c, `SELECT count(*) FROM public.change WHERE id = 3`); n != 0 {
			t.Error("the change survived an event that was refused")
		}
	})
}

// Criterio 3: tenant, usuario, request_id e ip salen de la transacción y del contexto.
func TestEventTakesTheActorFromTheTransactionAndContext(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	ctx = logging.WithRequestID(ctx, "req-123")
	ctx = audit.WithClientIP(ctx, netip.MustParseAddr("::ffff:203.0.113.7")) // IPv4 mapeada: se guarda como IPv4
	entity := uid(t, "99999999-0000-0000-0000-000000000001")

	err := e.app.InTenantTx(ctx, userActor(t, tenantA, userA), func(tx pgx.Tx) error {
		return e.w.Record(ctx, tx, audit.Event{Action: "ventas.documento:void", EntityType: "sales.document", EntityID: entity,
			Diff: map[string]any{"status": map[string]string{"from": "emitido", "to": "anulado"}}})
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := events(t, ctx, e)
	if len(rows) != 1 {
		t.Fatalf("%d events, want 1", len(rows))
	}
	r := rows[0]
	if r.TenantID != tenantA || r.ActorType != "user" || deref(r.ActorUserID) != userA || r.ImpersonatorID != nil ||
		r.Result != "ok" || deref(r.RequestID) != "req-123" || deref(r.IP) != "203.0.113.7" ||
		deref(r.EntityType) != "sales.document" || deref(r.EntityID) != "99999999-0000-0000-0000-000000000001" {
		t.Errorf("unexpected event %+v", r)
	}
	if string(r.Diff) != `{"status": {"to": "anulado", "from": "emitido"}}` {
		t.Errorf("diff = %s", r.Diff)
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// Criterio 5 y la decisión del architect: system sin usuario, impersonada como platform_staff.
func TestActorTypePerSessionKind(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	for _, tc := range []struct {
		name         string
		actor        db.Actor
		wantType     string
		wantUser     string
		wantImperson string
	}{
		{"user", userActor(t, tenantA, userA), "user", userA, "<nil>"},
		{"system job has no user", systemActor(t, tenantA), "system", "<nil>", "<nil>"},
		{"impersonated session", db.Actor{TenantID: uid(t, tenantA), UserID: uid(t, staff), Kind: db.KindImpersonated, ImpersonatorID: uid(t, staff)},
			"platform_staff", "<nil>", staff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := e.d.Admin(ctx, t).Exec(ctx, `TRUNCATE audit.events`); err != nil {
				t.Fatal(err)
			}
			err := e.app.InTenantTx(ctx, tc.actor, func(tx pgx.Tx) error { return e.w.Record(ctx, tx, ev) })
			if err != nil {
				t.Fatal(err)
			}
			rows := events(t, ctx, e)
			if len(rows) != 1 {
				t.Fatalf("%d events", len(rows))
			}
			r := rows[0]
			if r.ActorType != tc.wantType || deref(r.ActorUserID) != tc.wantUser || deref(r.ImpersonatorID) != tc.wantImperson {
				t.Errorf("actor_type=%s actor_user_id=%s impersonator_id=%s, want %s/%s/%s",
					r.ActorType, deref(r.ActorUserID), deref(r.ImpersonatorID), tc.wantType, tc.wantUser, tc.wantImperson)
			}
		})
	}
}

// El worker (wanka_worker, Kind system) también escribe.
func TestWorkerRecordsAsSystem(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	w := audit.NewWriter(e.worker)
	err := e.worker.InTenantTx(ctx, systemActor(t, tenantA), func(tx pgx.Tx) error {
		return w.Record(ctx, tx, audit.Event{Action: "kernel.job:run", Diff: map[string]any{"job_id": 42}})
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := events(t, ctx, e)
	if len(rows) != 1 || rows[0].ActorType != "system" || rows[0].ActorUserID != nil || string(rows[0].Diff) != `{"job_id": 42}` {
		t.Errorf("unexpected %+v", rows)
	}
}

// Sin tenant en la transacción no hay evento: falla cerrado, nunca con un tenant inventado.
func TestRecordWithoutATenantFails(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	err := e.app.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error { return e.w.Record(ctx, tx, ev) })
	if err == nil {
		t.Fatal("an event was recorded without a tenant")
	}
	if n := len(events(t, ctx, e)); n != 0 {
		t.Errorf("%d events", n)
	}
}

// Criterio 4: la denegación se escribe sola, aunque no haya cambio.
func TestRecordDeniedPersistsOnItsOwn(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	if err := e.w.RecordDenied(ctx, userActor(t, tenantA, userA), audit.Event{Action: "ventas.documento:void", EntityType: "sales.document"}); err != nil {
		t.Fatal(err)
	}
	rows := events(t, ctx, e)
	if len(rows) != 1 || rows[0].Result != "denegado" || rows[0].TenantID != tenantA || deref(rows[0].ActorUserID) != userA {
		t.Fatalf("unexpected %+v", rows)
	}
	// El resultado de Record dentro de un cambio sigue siendo ok.
	if err := e.app.InTenantTx(ctx, userActor(t, tenantA, userA), func(tx pgx.Tx) error { return e.w.Record(ctx, tx, ev) }); err != nil {
		t.Fatal(err)
	}
	var ok, denied int
	for _, r := range events(t, ctx, e) {
		if r.Result == "ok" {
			ok++
		} else {
			denied++
		}
	}
	if ok != 1 || denied != 1 {
		t.Errorf("ok=%d denied=%d, want 1 and 1", ok, denied)
	}
}

// Decisión del #173: si el insert del denegado falla se devuelve el error (el middleware lo loguea y responde 403 igual).
func TestRecordDeniedReturnsTheError(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	if err := e.w.RecordDenied(ctx, userActor(t, tenantA, userA), audit.Event{Action: "mal formato"}); !errors.Is(err, audit.ErrInvalidEvent) {
		t.Errorf("err = %v, want ErrInvalidEvent", err)
	}
	// Sin partición el insert falla y el error sube.
	c := e.d.Admin(ctx, t)
	dropAllPartitions(t, ctx, c)
	if err := e.w.RecordDenied(ctx, userActor(t, tenantA, userA), ev); sqlState(err) != "23514" {
		t.Errorf("err = %v, want SQLSTATE 23514 (no partition)", err)
	}
}

// Criterio 7: nadie de la aplicación modifica ni borra.
func TestEventsAreAppendOnly(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	if err := e.app.InTenantTx(ctx, userActor(t, tenantA, userA), func(tx pgx.Tx) error { return e.w.Record(ctx, tx, ev) }); err != nil {
		t.Fatal(err)
	}
	for name, pool := range map[string]*db.Pool{"wanka_app": e.app, "wanka_worker": e.worker} {
		for _, stmt := range []string{
			`UPDATE audit.events SET action = 'x.y:z'`,
			`UPDATE audit.events SET diff = NULL`,
			`UPDATE audit.events SET tenant_id = tenant_id`,
			`DELETE FROM audit.events`,
			`TRUNCATE audit.events`,
		} {
			err := pool.InTenantTx(ctx, systemActor(t, tenantA), func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, stmt)
				return err
			})
			if sqlState(err) != "42501" {
				t.Errorf("%s %q: err = %v, want permission denied (42501)", name, stmt, err)
			}
		}
		// Tampoco directo en una partición.
		err := pool.InTenantTx(ctx, systemActor(t, tenantA), func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `DELETE FROM audit.events_y`+currentPartitionSuffix())
			return err
		})
		if sqlState(err) != "42501" {
			t.Errorf("%s DELETE on a partition: err = %v, want 42501", name, err)
		}
	}
	if n := len(events(t, ctx, e)); n != 1 {
		t.Errorf("%d events, want the original 1", n)
	}
}

// Criterio 9: A no ve los de B.
func TestTenantsDoNotSeeEachOthersEvents(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	for _, a := range []struct{ tenant, user, action string }{
		{tenantA, userA, "a.recurso:uno"}, {tenantA, userA, "a.recurso:dos"}, {tenantB, userB, "b.recurso:uno"},
	} {
		action := a.action
		if err := e.app.InTenantTx(ctx, userActor(t, a.tenant, a.user), func(tx pgx.Tx) error {
			return e.w.Record(ctx, tx, audit.Event{Action: action})
		}); err != nil {
			t.Fatal(err)
		}
	}
	seen := func(tenant, user string) []string {
		var actions []string
		err := e.app.InTenantReadTx(ctx, userActor(t, tenant, user), func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `SELECT action FROM audit.events ORDER BY action`)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var a string
				if err := rows.Scan(&a); err != nil {
					return err
				}
				actions = append(actions, a)
			}
			return rows.Err()
		})
		if err != nil {
			t.Fatal(err)
		}
		return actions
	}
	if got := strings.Join(seen(tenantA, userA), ","); got != "a.recurso:dos,a.recurso:uno" {
		t.Errorf("tenant A sees %q", got)
	}
	if got := strings.Join(seen(tenantB, userB), ","); got != "b.recurso:uno" {
		t.Errorf("tenant B sees %q", got)
	}
	// Sin tenant no hay filas, y un insert hacia otro tenant lo rechaza el WITH CHECK.
	err := e.app.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM audit.events`).Scan(&n); err != nil {
			return err
		}
		if n != 0 {
			t.Errorf("a no-tenant transaction sees %d events", n)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = e.app.InTenantTx(ctx, userActor(t, tenantA, userA), func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO audit.events (tenant_id, actor_type, actor_user_id, action) VALUES ($1, 'user', $2, 'x.y:z')`, tenantB, userA)
		return err
	})
	if sqlState(err) != "42501" {
		t.Errorf("insert for another tenant: err = %v, want 42501 (RLS WITH CHECK)", err)
	}
}

// Los eventos concurrentes de varios tenants quedan cada uno en el suyo (con -race).
func TestConcurrentRecords(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tenant, user := tenantA, userA
			if i%2 == 1 {
				tenant, user = tenantB, userB
			}
			errs <- e.app.InTenantTx(ctx, userActor(t, tenant, user), func(tx pgx.Tx) error { return e.w.Record(ctx, tx, ev) })
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	c := e.d.Admin(ctx, t)
	if n := count(t, ctx, c, `SELECT count(*) FROM audit.events WHERE tenant_id = $1 AND actor_user_id = $2`, tenantA, userA); n != 20 {
		t.Errorf("tenant A has %d events, want 20", n)
	}
	if n := count(t, ctx, c, `SELECT count(*) FROM audit.events WHERE tenant_id = $1 AND actor_user_id = $2`, tenantB, userB); n != 20 {
		t.Errorf("tenant B has %d events, want 20", n)
	}
	if n := count(t, ctx, c, `SELECT count(*) FROM audit.events WHERE (tenant_id = $1) <> (actor_user_id = $2)`, tenantA, userA); n != 0 {
		t.Errorf("%d events cross a tenant with the other tenant's user", n)
	}
}

// El diff del evento no guarda secretos aunque el módulo los pase (criterio 6, ida y vuelta por la base).
func TestRecordedDiffHasNoSecrets(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	diff := map[string]any{
		"name":     "Ana",
		"password": "hunter2",
		"changes":  []any{map[string]any{"api_key": "fk_live_abc", "ok": 1}},
		"headers":  map[string]any{"Authorization": "Bearer wk_at_zzz", "note": "wk_rt_yyy"},
	}
	err := e.app.InTenantTx(ctx, userActor(t, tenantA, userA), func(tx pgx.Tx) error {
		return e.w.Record(ctx, tx, audit.Event{Action: "iam.usuario:update", Diff: diff})
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(events(t, ctx, e)[0].Diff)
	for _, leak := range []string{"hunter2", "fk_live_abc", "wk_at_zzz", "wk_rt_yyy"} {
		if strings.Contains(got, leak) {
			t.Errorf("the stored diff leaks %q: %s", leak, got)
		}
	}
	if !strings.Contains(got, `"name": "Ana"`) || !strings.Contains(got, audit.Redacted) {
		t.Errorf("the stored diff lost its harmless fields or the marker: %s", got)
	}
}

func TestInvalidEvents(t *testing.T) {
	e := newEnv(t)
	ctx := testCtx(t)
	for name, evt := range map[string]audit.Event{
		"empty action":     {},
		"uppercase action": {Action: "Ventas.documento:void"},
		"no verb":          {Action: "ventas.documento"},
		"bad entity type":  {Action: "a.b:c", EntityType: "Sales Document; drop"},
		"unserializable":   {Action: "a.b:c", Diff: make(chan int)},
		"too big":          {Action: "a.b:c", Diff: map[string]string{"blob": strings.Repeat("x", 70<<10)}},
	} {
		t.Run(name, func(t *testing.T) {
			err := e.app.InTenantTx(ctx, userActor(t, tenantA, userA), func(tx pgx.Tx) error { return e.w.Record(ctx, tx, evt) })
			if !errors.Is(err, audit.ErrInvalidEvent) {
				t.Errorf("err = %v, want ErrInvalidEvent", err)
			}
		})
	}
	if n := len(events(t, ctx, e)); n != 0 {
		t.Errorf("%d events were stored from invalid input", n)
	}
}

var _ = context.Background
