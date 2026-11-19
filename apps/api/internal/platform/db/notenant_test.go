package db_test

import (
	"context"
	_ "embed"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

//go:embed testdata/99990101000001_no_tenant_probe.sql
var noTenantProbeSQL []byte

const userV = "22222222-2222-2222-2222-222222222222"

// noTenantDB es tenantDB más la función y la tabla de apoyo de InNoTenantTx.
func noTenantDB(t *testing.T, maxConns int32) (*dbtest.DB, *db.Pool) {
	t.Helper()
	d := dbtest.New(t)
	d.MigrateWith(context.Background(), t, fstest.MapFS{
		"99990101000000_tenant_probe.sql":    {Data: tenantProbeSQL},
		"99990101000001_no_tenant_probe.sql": {Data: noTenantProbeSQL},
	})
	return d, open(t, d.URL(dbtest.App), db.Options{MaxConns: maxConns, AcquireTimeout: 2 * time.Second})
}

type noTenantDoor struct {
	name     string
	run      func(*db.Pool, context.Context, db.NoTenantScope, func(pgx.Tx) error) error
	readOnly bool
}

// doors son las dos puertas sin tenant; casi todo se prueba en las dos.
func doors() []noTenantDoor {
	return []noTenantDoor{
		{"InNoTenantTx", (*db.Pool).InNoTenantTx, false},
		{"InNoTenantReadTx", (*db.Pool).InNoTenantReadTx, true},
	}
}

// leakSession deja las cuatro variables fijadas a nivel de sesión en la
// conexión física del pool, como si otro código hubiera usado set_config(..., false).
func leakSession(t *testing.T, p *db.Pool, tenant, user, kind, imp string) {
	t.Helper()
	ctx := context.Background()
	conn, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	_, err = conn.Exec(ctx, `SELECT set_config('app.tenant_id', $1, false), set_config('app.user_id', $2, false),
		set_config('app.session_kind', $3, false), set_config('app.impersonator_id', $4, false)`, tenant, user, kind, imp)
	if err != nil {
		t.Fatal(err)
	}
}

func backendPID(ctx context.Context, tx pgx.Tx) (pid int, err error) {
	err = tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid)
	return pid, err
}

// Criterio 1: la firma es exactamente la de la historia y el scope no admite
// tenant ni Kind.
func TestNoTenantAPIShape(t *testing.T) {
	var _ func(context.Context, db.NoTenantScope, func(pgx.Tx) error) error = (*db.Pool)(nil).InNoTenantTx
	var _ func(context.Context, db.NoTenantScope, func(pgx.Tx) error) error = (*db.Pool)(nil).InNoTenantReadTx
	typ := reflect.TypeOf(db.NoTenantScope{})
	if typ.NumField() != 1 || typ.Field(0).Name != "UserID" || typ.Field(0).Type != reflect.TypeOf(pgtype.UUID{}) {
		t.Fatalf("NoTenantScope = %v, want a struct with only UserID pgtype.UUID", typ)
	}
}

type stmt struct {
	sql  string
	args []any
}

type recorder struct {
	mu    sync.Mutex
	stmts []stmt
}

func (r *recorder) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stmts = append(r.stmts, stmt{strings.Join(strings.Fields(d.SQL), " "), d.Args})
	return ctx
}
func (r *recorder) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// Criterio 2: tras el BEGIN va una sola sentencia set_config y el total es de
// 4 sentencias (BEGIN, set_config, consulta de fn, COMMIT), las mismas que InTenantTx.
func TestNoTenantTxFirstStatementIsOneSetConfig(t *testing.T) {
	d, _ := noTenantDB(t, 1)
	const want = `SELECT set_config('app.tenant_id', '', true), set_config('app.user_id', $1, true), set_config('app.session_kind', '', true), set_config('app.impersonator_id', '', true)`
	for _, door := range doors() {
		for _, c := range []struct {
			name  string
			scope db.NoTenantScope
			arg   string
		}{
			{"without user", db.NoTenantScope{}, ""},
			{"with user", db.NoTenantScope{UserID: uid(t, userV)}, userV},
			{"zero uuid is empty", db.NoTenantScope{UserID: uid(t, "00000000-0000-0000-0000-000000000000")}, ""},
		} {
			t.Run(door.name+"/"+c.name, func(t *testing.T) {
				rec := &recorder{}
				p, err := db.OpenTraced(context.Background(), d.URL(dbtest.App), rec)
				if err != nil {
					t.Fatal(err)
				}
				defer p.Close()
				ctx := context.Background()
				err = door.run(p, ctx, c.scope, func(tx pgx.Tx) error {
					var n int
					return tx.QueryRow(ctx, `SELECT 1`).Scan(&n)
				})
				if err != nil {
					t.Fatal(err)
				}
				if len(rec.stmts) != 4 {
					t.Fatalf("%d statements %v, want 4 (BEGIN, set_config, fn, COMMIT)", len(rec.stmts), rec.stmts)
				}
				begin := "begin read write"
				if door.readOnly {
					begin = "begin read only"
				}
				if !strings.EqualFold(rec.stmts[0].sql, begin) {
					t.Fatalf("first statement = %q, want %q", rec.stmts[0].sql, begin)
				}
				if rec.stmts[1].sql != want {
					t.Fatalf("second statement = %q, want %q", rec.stmts[1].sql, want)
				}
				if len(rec.stmts[1].args) != 1 || rec.stmts[1].args[0] != c.arg {
					t.Fatalf("set_config args = %v, want [%q]", rec.stmts[1].args, c.arg)
				}
				if rec.stmts[2].sql != `SELECT 1` || !strings.EqualFold(rec.stmts[3].sql, "commit") {
					t.Fatalf("statements after set_config = %v", rec.stmts[2:])
				}
			})
		}
	}
}

// Criterio 3: una conexión con app.* de sesión (mismo pg_backend_pid) no se
// filtra; con UserID solo cambia app.user_id.
func TestNoTenantTxClearsSessionVariablesOfTheSamePhysicalConnection(t *testing.T) {
	_, p := noTenantDB(t, 1)
	ctx := context.Background()
	seed(t, p)
	var pidFirst int
	if err := p.InTenantTx(ctx, actor(t, tenantA, db.KindUser), func(tx pgx.Tx) (err error) {
		pidFirst, err = backendPID(ctx, tx)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for _, door := range doors() {
		for _, c := range []struct {
			name  string
			scope db.NoTenantScope
			user  string
		}{
			{"without user", db.NoTenantScope{}, ""},
			{"with user", db.NoTenantScope{UserID: uid(t, userV)}, userV},
		} {
			t.Run(door.name+"/"+c.name, func(t *testing.T) {
				leakSession(t, p, tenantA, userU, "user", tenantB)
				var got [4]string
				var pid, rows int
				var tenantNull bool
				err := door.run(p, ctx, c.scope, func(tx pgx.Tx) error {
					return tx.QueryRow(ctx, `SELECT current_setting('app.tenant_id'), current_setting('app.user_id'),
						current_setting('app.session_kind'), current_setting('app.impersonator_id'), pg_backend_pid(),
						app.current_tenant() IS NULL, (SELECT count(*) FROM rlstest.items)`).
						Scan(&got[0], &got[1], &got[2], &got[3], &pid, &tenantNull, &rows)
				})
				if err != nil {
					t.Fatal(err)
				}
				if pid != pidFirst {
					t.Fatalf("test bug: the physical connection changed (%d -> %d)", pidFirst, pid)
				}
				if want := [4]string{"", c.user, "", ""}; got != want {
					t.Fatalf("variables = %q, want %q", got, want)
				}
				if !tenantNull || rows != 0 {
					t.Fatalf("current_tenant NULL=%v rows=%d: the tenant leaked", tenantNull, rows)
				}
			})
		}
	}
}

// Criterio 3 (parte NULL): sin UserID, app.current_user_id() es NULL.
func TestNoTenantTxCurrentUserIDIsNullWithoutScopeUser(t *testing.T) {
	_, p := noTenantDB(t, 1)
	ctx := context.Background()
	for _, door := range doors() {
		leakSession(t, p, tenantA, userU, "user", tenantB)
		var isNull bool
		err := door.run(p, ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT app.current_user_id() IS NULL`).Scan(&isNull)
		})
		if err != nil || !isNull {
			t.Fatalf("%s: err=%v current_user_id IS NULL=%v", door.name, err, isNull)
		}
	}
}

// Criterio 4: una función que guarda y restaura app.tenant_id como
// iam.session_lookup no devuelve el tenant viejo de la conexión.
func TestNoTenantTxSessionLookupPatternRestoresEmpty(t *testing.T) {
	_, p := noTenantDB(t, 1)
	ctx := context.Background()
	for _, door := range doors() {
		t.Run(door.name, func(t *testing.T) {
			leakSession(t, p, tenantA, userU, "user", "")
			var after string
			err := door.run(p, ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, `SELECT rlstest.lookup_like($1)`, tenantB); err != nil {
					return err
				}
				return tx.QueryRow(ctx, `SELECT current_setting('app.tenant_id')`).Scan(&after)
			})
			if err != nil {
				t.Fatal(err)
			}
			if after != "" {
				t.Fatalf("app.tenant_id after the lookup = %q, want empty (it kept the stale %s)", after, tenantA)
			}
		})
	}
}

// Criterio 5: pool lleno es ErrOverloaded sin abrir transacción; base caída, ErrUnavailable sin la clave.
func TestNoTenantTxAcquireErrors(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	for _, door := range doors() {
		t.Run(door.name+"/pool full", func(t *testing.T) {
			p := open(t, d.URL(dbtest.App), db.Options{MaxConns: 1, AcquireTimeout: 50 * time.Millisecond})
			held, err := p.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer held.Release()
			called := false
			err = door.run(p, ctx, db.NoTenantScope{}, func(pgx.Tx) error { called = true; return nil })
			if !errors.Is(err, db.ErrOverloaded) || called {
				t.Fatalf("err=%v fn called=%v, want ErrOverloaded and no fn", err, called)
			}
		})
		t.Run(door.name+"/database down", func(t *testing.T) {
			p := open(t, closedPortURL(t), db.Options{MaxConns: 1})
			called := false
			err := door.run(p, ctx, db.NoTenantScope{}, func(pgx.Tx) error { called = true; return nil })
			if !errors.Is(err, db.ErrUnavailable) || errors.Is(err, db.ErrOverloaded) || called {
				t.Fatalf("err=%v fn called=%v, want ErrUnavailable and no fn", err, called)
			}
		})
	}
}

func writeRow(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `INSERT INTO rlstest.writes VALUES ('x')`)
	return err
}

func countWrites(t *testing.T, d *dbtest.DB) int {
	t.Helper()
	var n int
	if err := d.Admin(context.Background(), t).QueryRow(context.Background(), `SELECT count(*) FROM rlstest.writes`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Criterios 6 y 7: los errores salen tipados con la misma función que InTenantTx,
// hay ROLLBACK y la conexión vuelve al pool.
func TestNoTenantTxClassifiesFailuresAndRollsBack(t *testing.T) {
	for _, door := range doors() {
		if door.readOnly {
			continue // las escrituras de estas pruebas solo existen en la puerta de escritura
		}
		t.Run(door.name, func(t *testing.T) {
			d, p := noTenantDB(t, 1)
			ctx := context.Background()
			scope := db.NoTenantScope{}

			t.Run("statement timeout", func(t *testing.T) {
				err := door.run(p, ctx, scope, func(tx pgx.Tx) error {
					if err := writeRow(ctx, tx); err != nil {
						return err
					}
					if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout = '50ms'`); err != nil {
						return err
					}
					_, err := tx.Exec(ctx, `SELECT pg_sleep(3)`)
					return err
				})
				if !errors.Is(err, db.ErrStatementTimeout) || pgCode(err) != "57014" {
					t.Fatalf("err = %v, want ErrStatementTimeout keeping SQLSTATE 57014", err)
				}
			})
			t.Run("pg_cancel_backend", func(t *testing.T) {
				err := door.run(p, ctx, scope, func(tx pgx.Tx) error {
					if err := writeRow(ctx, tx); err != nil {
						return err
					}
					var pid int
					if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
						return err
					}
					admin := d.Admin(ctx, t)
					done := make(chan struct{})
					go func() {
						defer close(done)
						time.Sleep(100 * time.Millisecond)
						_, _ = admin.Exec(ctx, `SELECT pg_cancel_backend($1)`, pid)
					}()
					defer func() { <-done }()
					_, err := tx.Exec(ctx, `SELECT pg_sleep(3)`)
					return err
				})
				if !errors.Is(err, db.ErrStatementTimeout) {
					t.Fatalf("err = %v, want ErrStatementTimeout", err)
				}
			})
			t.Run("caller context canceled", func(t *testing.T) {
				cctx, cancel := context.WithCancel(ctx)
				err := door.run(p, cctx, scope, func(tx pgx.Tx) error {
					if err := writeRow(cctx, tx); err != nil {
						return err
					}
					cancel()
					_, err := tx.Exec(cctx, `SELECT pg_sleep(3)`)
					return err
				})
				if err == nil || errors.Is(err, db.ErrStatementTimeout) || errors.Is(err, db.ErrUnavailable) {
					t.Fatalf("err = %v, want the context's own error", err)
				}
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("err = %v, want context.Canceled", err)
				}
			})
			t.Run("caller deadline expired", func(t *testing.T) {
				cctx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
				defer cancel()
				err := door.run(p, cctx, scope, func(tx pgx.Tx) error {
					_, err := tx.Exec(cctx, `SELECT pg_sleep(3)`)
					return err
				})
				if err == nil || errors.Is(err, db.ErrStatementTimeout) || errors.Is(err, db.ErrUnavailable) {
					t.Fatalf("err = %v, want the context's own error", err)
				}
			})
			t.Run("pg_terminate_backend", func(t *testing.T) {
				err := door.run(p, ctx, scope, func(tx pgx.Tx) error {
					_, err := tx.Exec(ctx, `SELECT pg_terminate_backend(pg_backend_pid())`)
					return err
				})
				if !errors.Is(err, db.ErrUnavailable) || errors.Is(err, db.ErrCommitUncertain) {
					t.Fatalf("err = %v, want ErrUnavailable and not commit-uncertain", err)
				}
			})
			t.Run("connection lost at COMMIT", func(t *testing.T) {
				err := door.run(p, ctx, scope, func(tx pgx.Tx) error {
					return tx.Conn().PgConn().Conn().Close()
				})
				if !errors.Is(err, db.ErrUnavailable) || !errors.Is(err, db.ErrCommitUncertain) {
					t.Fatalf("err = %v, want ErrUnavailable and ErrCommitUncertain", err)
				}
			})
			t.Run("error of fn", func(t *testing.T) {
				boom := errors.New("boom")
				var pidIn, pidOut int
				err := door.run(p, ctx, scope, func(tx pgx.Tx) (err error) {
					if pidIn, err = backendPID(ctx, tx); err != nil {
						return err
					}
					if err := writeRow(ctx, tx); err != nil {
						return err
					}
					return boom
				})
				if !errors.Is(err, boom) {
					t.Fatalf("err = %v, want fn's error", err)
				}
				// Con ROLLBACK la conexión sigue sana y vuelve al pool; sin él, el pool la destruye.
				if err := door.run(p, ctx, scope, func(tx pgx.Tx) (err error) {
					pidOut, err = backendPID(ctx, tx)
					return err
				}); err != nil || pidOut != pidIn {
					t.Fatalf("connection pid %d -> %d (err %v): no ROLLBACK, the pool destroyed it", pidIn, pidOut, err)
				}
			})
			t.Run("panic", func(t *testing.T) {
				defer func() {
					if r := recover(); r != "kaboom" {
						t.Fatalf("recovered %v, want the original panic", r)
					}
				}()
				_ = door.run(p, ctx, scope, func(tx pgx.Tx) error {
					if err := writeRow(ctx, tx); err != nil {
						return err
					}
					panic("kaboom")
				})
			})

			if n := countWrites(t, d); n != 0 {
				t.Fatalf("%d rows written, want 0: a failed transaction was not rolled back", n)
			}
			if n := p.AcquiredConns(); n != 0 {
				t.Fatalf("AcquiredConns = %d, want 0", n)
			}
			if err := door.run(p, ctx, scope, func(tx pgx.Tx) error { return writeRow(ctx, tx) }); err != nil {
				t.Fatalf("pool unusable after the failures: %v", err)
			}
		})
	}
}

// El commit de la puerta de escritura persiste; la de lectura rechaza escribir.
func TestNoTenantTxWriteModes(t *testing.T) {
	d, p := noTenantDB(t, 2)
	ctx := context.Background()
	if err := p.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error { return writeRow(ctx, tx) }); err != nil {
		t.Fatal(err)
	}
	if n := countWrites(t, d); n != 1 {
		t.Fatalf("rows = %d, want 1 after COMMIT", n)
	}
	var ro string
	err := p.InNoTenantReadTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT current_setting('transaction_read_only')`).Scan(&ro); err != nil {
			return err
		}
		return writeRow(ctx, tx)
	})
	if ro != "on" || pgCode(err) != "25006" {
		t.Fatalf("read tx: transaction_read_only=%q err=%v, want on and SQLSTATE 25006", ro, err)
	}
}

// La causa de PostgreSQL no lleva la contraseña de la URL.
func TestNoTenantTxErrorsDoNotLeakThePassword(t *testing.T) {
	p := open(t, closedPortURL(t), db.Options{MaxConns: 1})
	err := p.InNoTenantTx(context.Background(), db.NoTenantScope{}, func(pgx.Tx) error { return nil })
	if err == nil || strings.Contains(err.Error(), "postgres://") {
		t.Fatalf("err = %v, want an error without the database URL", err)
	}
}

// Invariante: sea cual sea el estado de la conexión, una transacción sin
// tenant ve todo vacío; y bajo concurrencia (-race) mezclada con InTenantTx.
func TestNoTenantTxNeverSeesAStaleTenant(t *testing.T) {
	_, p := noTenantDB(t, 2)
	ctx := context.Background()
	seed(t, p)
	t.Run("any leaked state", func(t *testing.T) {
		for _, door := range doors() {
			for _, lk := range [][4]string{
				{tenantA, userU, "user", ""}, {tenantB, userU, "impersonated", tenantA},
				{tenantA, "", "system", ""}, {"", "", "", tenantA}, {"", userU, "", ""}, {"not-a-uuid", "x", "weird", "y"},
			} {
				for range 3 { // el pool de 2 reparte las dos conexiones
					leakSession(t, p, lk[0], lk[1], lk[2], lk[3])
				}
				var got [3]string
				var rows int
				err := door.run(p, ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
					return tx.QueryRow(ctx, `SELECT current_setting('app.tenant_id'), current_setting('app.session_kind'),
						current_setting('app.impersonator_id'), (SELECT count(*) FROM rlstest.items)`).Scan(&got[0], &got[1], &got[2], &rows)
				})
				if err != nil || got != [3]string{} || rows != 0 {
					t.Fatalf("%s leaked %v: err=%v got=%q rows=%d", door.name, lk, err, got, rows)
				}
			}
		}
	})
	t.Run("concurrent with InTenantTx", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make(chan error, 64)
		for i := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 10 {
					if i%2 == 0 {
						err := p.InTenantReadTx(ctx, actor(t, tenantA, db.KindUser), func(tx pgx.Tx) error { _, err := countItems(ctx, tx); return err })
						if err != nil {
							errs <- err
						}
						continue
					}
					var tn string
					var rows int
					err := p.InNoTenantReadTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
						return tx.QueryRow(ctx, `SELECT current_setting('app.tenant_id'), (SELECT count(*) FROM rlstest.items)`).Scan(&tn, &rows)
					})
					if err != nil || tn != "" || rows != 0 {
						errs <- errors.Join(err, errors.New("no-tenant tx saw tenant "+tn))
					}
				}
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Error(err)
		}
	})
}

// Criterio 9: el comentario del paquete y el README nombran las dos puertas.
func TestDocsNameBothDoors(t *testing.T) {
	for _, f := range []string{"db.go", "../../../README.md"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"InTenantTx", "InNoTenantTx", "InNoTenantReadTx", "iam.list_my_memberships"} {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s does not mention %s", f, want)
			}
		}
	}
}
