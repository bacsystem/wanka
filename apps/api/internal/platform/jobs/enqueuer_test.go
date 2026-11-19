package jobs

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

func ctx5(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func newEnqueuer(t testing.TB, p *db.Pool) Enqueuer {
	t.Helper()
	e, err := NewEnqueuer(p)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// Criterio 1: el job existe solo si la transacción hace COMMIT, con ambos finales,
// y es atómico con el cambio de datos de la misma transacción.
func TestInsertTxIsAtomicWithTheCallersTransaction(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	admin := d.Admin(ctx, t)
	if _, err := admin.Exec(ctx, `CREATE TABLE ops.jobs_test_items (id int PRIMARY KEY); GRANT INSERT ON ops.jobs_test_items TO wanka_app`); err != nil {
		t.Fatal(err)
	}
	app := openPool(t, d, dbtest.App, 5)
	enq := newEnqueuer(t, app)
	s := d.SeedSession(t, dbtest.SessionSpec{})
	actor := db.Actor{TenantID: uuidOf(t, s.TenantID), UserID: uuidOf(t, s.UserID), Kind: db.KindUser}
	errRollback := errors.New("the business operation failed")

	counts := func() (items, jobs int) {
		t.Helper()
		if err := admin.QueryRow(ctx, `SELECT (SELECT count(*) FROM ops.jobs_test_items), (SELECT count(*) FROM ops.river_job WHERE kind = 'test.ping')`).Scan(&items, &jobs); err != nil {
			t.Fatal(err)
		}
		return items, jobs
	}
	run := func(id int, fail bool) error {
		return app.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO ops.jobs_test_items VALUES ($1)`, id); err != nil {
				return err
			}
			if err := enq.InsertTx(ctx, tx, pingArgs{TenantID: actor.TenantID, Key: "k"}, WithQueue(QueueMaintenance)); err != nil {
				return err
			}
			if fail {
				return errRollback
			}
			return nil
		})
	}

	if err := run(1, true); !errors.Is(err, errRollback) {
		t.Fatalf("want the rollback error, got %v", err)
	}
	if items, jobs := counts(); items != 0 || jobs != 0 {
		t.Fatalf("after ROLLBACK: %d items and %d jobs, want 0 and 0", items, jobs)
	}
	if err := run(2, false); err != nil {
		t.Fatal(err)
	}
	if items, jobs := counts(); items != 1 || jobs != 1 {
		t.Fatalf("after COMMIT: %d items and %d jobs, want 1 and 1", items, jobs)
	}
}

func TestInsertTxStoresQueueAttemptsAndArgs(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	admin := d.Admin(ctx, t)
	app := openPool(t, d, dbtest.App, 5)
	enq := newEnqueuer(t, app)
	s := d.SeedSession(t, dbtest.SessionSpec{})

	enqueueTx(ctx, t, app, enq, s, pingArgs{TenantID: uuidOf(t, s.TenantID), Key: "a"}, WithQueue(QueueCPE))
	enqueueTx(ctx, t, app, enq, s, pingArgs{TenantID: uuidOf(t, s.TenantID), Key: "b"}, WithQueue(QueueReports), WithMaxAttempts(3))
	var queue, tenant string
	var max int
	if err := admin.QueryRow(ctx, `SELECT queue, max_attempts, args->>'tenant_id' FROM ops.river_job WHERE args->>'key' = 'a'`).Scan(&queue, &max, &tenant); err != nil {
		t.Fatal(err)
	}
	if queue != QueueCPE || max != DefaultMaxAttempts || tenant != s.TenantID {
		t.Fatalf("job a: queue %q, max %d, tenant %q", queue, max, tenant)
	}
	if err := admin.QueryRow(ctx, `SELECT queue, max_attempts FROM ops.river_job WHERE args->>'key' = 'b'`).Scan(&queue, &max); err != nil {
		t.Fatal(err)
	}
	if queue != QueueReports || max != 3 {
		t.Fatalf("job b: queue %q, max %d", queue, max)
	}
}

func TestInsertTxRejectsBadOptions(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	app := openPool(t, d, dbtest.App, 5)
	enq := newEnqueuer(t, app)
	s := d.SeedSession(t, dbtest.SessionSpec{})
	actor := db.Actor{TenantID: uuidOf(t, s.TenantID), UserID: uuidOf(t, s.UserID), Kind: db.KindUser}
	cases := []struct {
		name string
		opts []Option
		want error
	}{
		{"no queue", nil, ErrUnknownQueue},
		{"unknown queue", []Option{WithQueue("default")}, ErrUnknownQueue},
		{"zero attempts", []Option{WithQueue(QueueCPE), WithMaxAttempts(0)}, ErrInvalidOption},
		{"too many attempts", []Option{WithQueue(QueueCPE), WithMaxAttempts(101)}, ErrInvalidOption},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := app.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
				return enq.InsertTx(ctx, tx, pingArgs{TenantID: actor.TenantID}, c.opts...)
			})
			if !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}

// Criterio 6: un job único por clave se encola dos veces y existe uno solo. El
// duplicado no aborta la transacción del llamador.
func TestUniqueJobIsEnqueuedOnce(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	admin := d.Admin(ctx, t)
	if _, err := admin.Exec(ctx, `CREATE TABLE ops.jobs_test_items (id int PRIMARY KEY); GRANT INSERT ON ops.jobs_test_items TO wanka_app`); err != nil {
		t.Fatal(err)
	}
	app := openPool(t, d, dbtest.App, 5)
	enq := newEnqueuer(t, app)
	s := d.SeedSession(t, dbtest.SessionSpec{})
	tenant := uuidOf(t, s.TenantID)
	count := func(key string) int {
		t.Helper()
		var n int
		if err := admin.QueryRow(ctx, `SELECT count(*) FROM ops.river_job WHERE kind = 'test.ping' AND args->>'key' = $1`, key).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	uniq := []Option{WithQueue(QueueMaintenance), WithUnique()}

	// En transacciones distintas.
	enqueueTx(ctx, t, app, enq, s, pingArgs{TenantID: tenant, Key: "x"}, uniq...)
	enqueueTx(ctx, t, app, enq, s, pingArgs{TenantID: tenant, Key: "x"}, uniq...)
	if n := count("x"); n != 1 {
		t.Fatalf("two unique inserts in two transactions left %d jobs, want 1", n)
	}
	// En la misma transacción, y la transacción sigue sirviendo después del duplicado.
	actor := db.Actor{TenantID: tenant, UserID: uuidOf(t, s.UserID), Kind: db.KindUser}
	err := app.InTenantTx(ctx, actor, func(tx pgx.Tx) error {
		for range 2 {
			if err := enq.InsertTx(ctx, tx, pingArgs{TenantID: tenant, Key: "y"}, uniq...); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO ops.jobs_test_items VALUES (7)`)
		return err
	})
	if err != nil {
		t.Fatalf("a duplicate unique insert must not fail or abort the transaction: %v", err)
	}
	if n := count("y"); n != 1 {
		t.Fatalf("two unique inserts in one transaction left %d jobs, want 1", n)
	}
	var items int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM ops.jobs_test_items`).Scan(&items); err != nil || items != 1 {
		t.Fatalf("the business row after the duplicate: %d (%v), want 1", items, err)
	}
	// Otra clave sí se encola, y sin WithUnique no hay deduplicación.
	enqueueTx(ctx, t, app, enq, s, pingArgs{TenantID: tenant, Key: "z"}, uniq...)
	enqueueTx(ctx, t, app, enq, s, pingArgs{TenantID: tenant, Key: "n"}, WithQueue(QueueMaintenance))
	enqueueTx(ctx, t, app, enq, s, pingArgs{TenantID: tenant, Key: "n"}, WithQueue(QueueMaintenance))
	if count("z") != 1 || count("n") != 2 {
		t.Fatalf("other key: %d, non-unique twice: %d; want 1 and 2", count("z"), count("n"))
	}
	// Terminado el job, uno nuevo con la misma clave se acepta (solo cuenta el que está en curso).
	if _, err := admin.Exec(ctx, `UPDATE ops.river_job SET state = 'completed', finalized_at = now() WHERE args->>'key' = 'x'`); err != nil {
		t.Fatal(err)
	}
	enqueueTx(ctx, t, app, enq, s, pingArgs{TenantID: tenant, Key: "x"}, uniq...)
	if n := count("x"); n != 2 {
		t.Fatalf("after the first job completed, a new unique insert left %d rows, want 2", n)
	}
}

// Criterio 2: los GRANT del ADR 0001 sobre las tablas de River.
func TestRiverGrants(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	admin := d.Admin(ctx, t)
	tables := []string{"river_job", "river_leader", "river_queue", "river_notification", "river_migration"}
	want := map[string]map[string]string{ // rol → tabla → privilegios de tabla en orden SELECT,INSERT,UPDATE,DELETE
		dbtest.App:      {"river_job": "-I--"},
		dbtest.Webhooks: {"river_job": "-I--"},
		dbtest.Platform: {"river_job": "-I--"},
		dbtest.Worker:   {"river_job": "SIUD", "river_leader": "SIUD", "river_queue": "SIUD", "river_notification": "SIUD"},
	}
	for _, role := range []string{dbtest.App, dbtest.Webhooks, dbtest.Platform, dbtest.Worker} {
		for _, tbl := range tables {
			var got strings.Builder
			for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
				var ok bool
				if err := admin.QueryRow(ctx, `SELECT has_table_privilege($1, 'ops.' || $2, $3)`, role, tbl, priv).Scan(&ok); err != nil {
					t.Fatal(err)
				}
				if ok {
					got.WriteString(priv[:1])
				} else {
					got.WriteString("-")
				}
			}
			exp := want[role][tbl]
			if exp == "" {
				exp = "----"
			}
			if got.String() != exp {
				t.Errorf("%s on ops.%s: %s, want %s", role, tbl, got.String(), exp)
			}
		}
	}
	// La secuencia de ids: quien inserta la usa.
	for role, exp := range map[string]bool{dbtest.App: true, dbtest.Webhooks: true, dbtest.Platform: true, dbtest.Worker: true} {
		var ok bool
		if err := admin.QueryRow(ctx, `SELECT has_sequence_privilege($1, 'ops.river_job_id_seq', 'USAGE')`, role).Scan(&ok); err != nil || ok != exp {
			t.Errorf("%s USAGE on river_job_id_seq: %v (%v), want %v", role, ok, err, exp)
		}
	}
	// Y las tablas son del migrador, no de un rol de aplicación.
	var notOwned int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'ops' AND c.relkind IN ('r', 'S') AND pg_get_userbyid(c.relowner) <> 'wanka_migrator'`).Scan(&notOwned); err != nil || notOwned != 0 {
		t.Errorf("%d ops tables or sequences not owned by wanka_migrator (%v)", notOwned, err)
	}
}

// Los roles que encolan insertan de verdad con su propio rol (api por InTenantTx;
// webhooks por InTenantTx de sistema; platform-api sin tenant) y no leen la cola.
func TestEnqueueingRolesInsertButCannotReadTheQueue(t *testing.T) {
	ctx := ctx5(t)
	d := migrated(t)
	admin := d.Admin(ctx, t)
	s := d.SeedSession(t, dbtest.SessionSpec{})
	tenant := uuidOf(t, s.TenantID)

	app, webhooks, platform := openPool(t, d, dbtest.App, 3), openPool(t, d, dbtest.Webhooks, 3), openPool(t, d, dbtest.Platform, 3)
	enqueueTx(ctx, t, app, newEnqueuer(t, app), s, pingArgs{TenantID: tenant, Key: "app"}, WithQueue(QueueCPE))

	err := webhooks.InTenantTx(ctx, db.Actor{TenantID: tenant, Kind: db.KindSystem}, func(tx pgx.Tx) error {
		return newEnqueuer(t, webhooks).InsertTx(ctx, tx, pingArgs{TenantID: tenant, Key: "webhooks"}, WithQueue(QueueWebhooks), WithUnique())
	})
	if err != nil {
		t.Fatalf("wanka_webhooks cannot enqueue: %v", err)
	}
	err = platform.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		return newEnqueuer(t, platform).InsertTx(ctx, tx, noTenantArgs{Key: "platform"}, WithQueue(QueueMaintenance))
	})
	if err != nil {
		t.Fatalf("wanka_platform cannot enqueue: %v", err)
	}
	var n int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM ops.river_job WHERE args->>'key' IN ('app', 'webhooks', 'platform')`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("%d jobs enqueued by the three services (%v), want 3", n, err)
	}

	for name, p := range map[string]*db.Pool{"wanka_app": app, "wanka_webhooks": webhooks, "wanka_platform": platform} {
		err := p.InNoTenantReadTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `SELECT args FROM ops.river_job`)
			return err
		})
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("%s reading ops.river_job: %v, want permission denied", name, err)
		}
	}
}

// Criterio 10: la interfaz no expone tipos de River. Se revisa la API exportada
// del paquete (firmas, campos y métodos exportados) en busca de los paquetes de River.
func TestExportedAPIExposesNoRiverTypes(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, ok := pkgs["jobs"]
	if !ok {
		t.Fatal("package jobs not found")
	}
	exported := 0
	check := func(what string, n ast.Node) {
		ast.Inspect(n, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && strings.HasPrefix(id.Name, "river") {
					t.Errorf("exported %s exposes %s.%s", what, id.Name, sel.Sel.Name)
				}
			}
			return true
		})
	}
	for _, f := range pkg.Files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if !d.Name.IsExported() {
					continue
				}
				if d.Recv != nil && !recvExported(d.Recv) {
					continue
				}
				exported++
				check("func "+d.Name.Name, d.Type)
				if d.Recv != nil {
					check("func "+d.Name.Name, d.Recv)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch sp := spec.(type) {
					case *ast.TypeSpec:
						if !sp.Name.IsExported() {
							continue
						}
						exported++
						check("type "+sp.Name.Name, exportedPart(sp.Type))
						if sp.TypeParams != nil {
							check("type "+sp.Name.Name, sp.TypeParams)
						}
					case *ast.ValueSpec:
						for _, n := range sp.Names {
							if n.IsExported() {
								exported++
								check("value "+n.Name, sp)
							}
						}
					}
				}
			}
		}
	}
	if exported < 10 {
		t.Fatalf("only %d exported declarations found: the walk is broken", exported)
	}
}

func recvExported(r *ast.FieldList) bool {
	for _, f := range r.List {
		t := f.Type
		if s, ok := t.(*ast.StarExpr); ok {
			t = s.X
		}
		if ix, ok := t.(*ast.IndexExpr); ok {
			t = ix.X
		}
		if id, ok := t.(*ast.Ident); ok && id.IsExported() {
			return true
		}
	}
	return false
}

// exportedPart deja solo los campos y métodos exportados de un struct o interface.
func exportedPart(e ast.Expr) ast.Node {
	switch t := e.(type) {
	case *ast.StructType:
		out := &ast.FieldList{}
		for _, f := range t.Fields.List {
			for _, n := range f.Names {
				if n.IsExported() {
					out.List = append(out.List, f)
					break
				}
			}
		}
		return out
	default:
		return e
	}
}
