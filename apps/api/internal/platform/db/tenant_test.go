package db_test

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// Migración solo de test: la tabla rlstest.items con la política estándar.
//
//go:embed testdata/99990101000000_tenant_probe.sql
var tenantProbeSQL []byte

func uid(t testing.TB, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatal(err)
	}
	return u
}

const (
	tenantA = "aaaaaaaa-0000-0000-0000-00000000000a"
	tenantB = "bbbbbbbb-0000-0000-0000-00000000000b"
	userU   = "11111111-1111-1111-1111-111111111111"
)

func actor(t testing.TB, tenant string, kind db.SessionKind) db.Actor {
	t.Helper()
	return db.Actor{TenantID: uid(t, tenant), UserID: uid(t, userU), Kind: kind}
}

// tenantDB devuelve un pool de wanka_app sobre una base migrada con la tabla de prueba.
func tenantDB(t *testing.T, maxConns int32) (*dbtest.DB, *db.Pool) {
	t.Helper()
	d := dbtest.New(t)
	d.MigrateWith(context.Background(), t, fstest.MapFS{
		"99990101000000_tenant_probe.sql": {Data: tenantProbeSQL},
	})
	return d, open(t, d.URL(dbtest.App), db.Options{MaxConns: maxConns, AcquireTimeout: 2 * time.Second})
}

func insertItem(ctx context.Context, tx pgx.Tx, tenant pgtype.UUID, name string) error {
	_, err := tx.Exec(ctx, `INSERT INTO rlstest.items (tenant_id, name) VALUES ($1, $2)`, tenant, name)
	return err
}

func countItems(ctx context.Context, tx pgx.Tx) (n int, err error) {
	err = tx.QueryRow(ctx, `SELECT count(*) FROM rlstest.items`).Scan(&n)
	return n, err
}

// seed deja una fila por tenant, insertada dentro de su propio InTenantTx.
func seed(t *testing.T, p *db.Pool) {
	t.Helper()
	for _, tn := range []string{tenantA, tenantB} {
		err := p.InTenantTx(context.Background(), actor(t, tn, db.KindUser), func(tx pgx.Tx) error {
			return insertItem(context.Background(), tx, uid(t, tn), "row of "+tn[:1])
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// Criterio 1: fija las cuatro variables locales y llama a fn con la transacción.
func TestInTenantTxSetsSessionVariables(t *testing.T) {
	_, p := tenantDB(t, 2)
	ctx := context.Background()
	imp := "cccccccc-0000-0000-0000-00000000000c"
	cases := []struct {
		name  string
		actor db.Actor
		want  [4]string // tenant_id, user_id, session_kind, impersonator_id
	}{
		{"user", actor(t, tenantA, db.KindUser), [4]string{tenantA, userU, "user", ""}},
		{"system", actor(t, tenantA, db.KindSystem), [4]string{tenantA, userU, "system", ""}},
		{"impersonated", db.Actor{TenantID: uid(t, tenantA), UserID: uid(t, userU), Kind: db.KindImpersonated, ImpersonatorID: uid(t, imp)},
			[4]string{tenantA, userU, "impersonated", imp}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got [4]string
			var user string
			var readOnly string
			err := p.InTenantTx(ctx, c.actor, func(tx pgx.Tx) error {
				return tx.QueryRow(ctx, `SELECT current_setting('app.tenant_id'), current_setting('app.user_id'),
					current_setting('app.session_kind'), current_setting('app.impersonator_id'), current_user, current_setting('transaction_read_only')`).
					Scan(&got[0], &got[1], &got[2], &got[3], &user, &readOnly)
			})
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("variables = %v, want %v", got, c.want)
			}
			if user != "wanka_app" || readOnly != "off" {
				t.Fatalf("current_user=%q transaction_read_only=%q", user, readOnly)
			}
		})
	}
}

// Criterio 1: COMMIT con nil, ROLLBACK con error o panic (el panic se relanza).
func TestInTenantTxCommitsOnNilAndRollsBackOnErrorOrPanic(t *testing.T) {
	_, p := tenantDB(t, 1)
	ctx := context.Background()
	a := actor(t, tenantA, db.KindUser)
	count := func() (n int) {
		t.Helper()
		if err := p.InTenantTx(ctx, a, func(tx pgx.Tx) (err error) { n, err = countItems(ctx, tx); return }); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if err := p.InTenantTx(ctx, a, func(tx pgx.Tx) error { return insertItem(ctx, tx, a.TenantID, "kept") }); err != nil {
		t.Fatal(err)
	}
	if n := count(); n != 1 {
		t.Fatalf("after commit: %d rows, want 1", n)
	}

	boom := errors.New("boom")
	err := p.InTenantTx(ctx, a, func(tx pgx.Tx) error {
		if err := insertItem(ctx, tx, a.TenantID, "lost"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want boom", err)
	}
	if n := count(); n != 1 {
		t.Fatalf("after error: %d rows, want 1", n)
	}

	func() {
		defer func() {
			if r := recover(); r != "kaboom" {
				t.Fatalf("recovered %v, want the original panic value", r)
			}
		}()
		_ = p.InTenantTx(ctx, a, func(tx pgx.Tx) error {
			if err := insertItem(ctx, tx, a.TenantID, "lost too"); err != nil {
				return err
			}
			panic("kaboom")
		})
		t.Fatal("InTenantTx swallowed the panic")
	}()
	if n := count(); n != 1 {
		t.Fatalf("after panic: %d rows, want 1", n)
	}
}

// Criterios 2, 3 y 4: un Actor inválido nunca abre transacción ni llama a fn.
// El pool apunta a un puerto cerrado: si InTenantTx intentara conectar, el
// error sería ErrUnavailable y no el del Actor.
func TestInTenantTxRejectsInvalidActorsWithoutOpeningATransaction(t *testing.T) {
	p := open(t, "postgres://wanka_app:x@127.0.0.1:1/none", db.Options{MaxConns: 1})
	ctx := context.Background()
	tenant, user, imp := uid(t, tenantA), uid(t, userU), uid(t, "cccccccc-0000-0000-0000-00000000000c")
	var zeroValid pgtype.UUID
	zeroValid.Valid = true // el UUID cero también cuenta como vacío

	cases := []struct {
		name  string
		actor db.Actor
		want  error
	}{
		{"zero Kind", db.Actor{TenantID: tenant, UserID: user}, db.ErrInvalidActor},
		{"Kind out of range", db.Actor{TenantID: tenant, UserID: user, Kind: db.SessionKind(200)}, db.ErrInvalidActor},
		{"impersonated without impersonator", db.Actor{TenantID: tenant, UserID: user, Kind: db.KindImpersonated}, db.ErrInvalidActor},
		{"impersonated with the zero impersonator", db.Actor{TenantID: tenant, UserID: user, Kind: db.KindImpersonated, ImpersonatorID: zeroValid}, db.ErrInvalidActor},
		{"user with impersonator", db.Actor{TenantID: tenant, UserID: user, Kind: db.KindUser, ImpersonatorID: imp}, db.ErrInvalidActor},
		{"system with impersonator", db.Actor{TenantID: tenant, UserID: user, Kind: db.KindSystem, ImpersonatorID: imp}, db.ErrInvalidActor},
		{"user without UserID", db.Actor{TenantID: tenant, Kind: db.KindUser}, db.ErrInvalidActor},
		{"impersonated without UserID", db.Actor{TenantID: tenant, Kind: db.KindImpersonated, ImpersonatorID: imp}, db.ErrInvalidActor},
		{"user with the zero UserID", db.Actor{TenantID: tenant, UserID: zeroValid, Kind: db.KindUser}, db.ErrInvalidActor},
		{"user without tenant", db.Actor{UserID: user, Kind: db.KindUser}, db.ErrNoTenant},
		{"user with the zero tenant", db.Actor{TenantID: zeroValid, UserID: user, Kind: db.KindUser}, db.ErrNoTenant},
		{"impersonated without tenant", db.Actor{UserID: user, Kind: db.KindImpersonated, ImpersonatorID: imp}, db.ErrNoTenant},
		{"system without tenant", db.Actor{UserID: user, Kind: db.KindSystem}, db.ErrNoTenant},
	}
	for _, c := range cases {
		for _, mode := range []string{"InTenantTx", "InTenantReadTx"} {
			t.Run(c.name+"/"+mode, func(t *testing.T) {
				called := false
				fn := func(pgx.Tx) error { called = true; return nil }
				var err error
				if mode == "InTenantTx" {
					err = p.InTenantTx(ctx, c.actor, fn)
				} else {
					err = p.InTenantReadTx(ctx, c.actor, fn)
				}
				if !errors.Is(err, c.want) {
					t.Fatalf("error = %v, want %v", err, c.want)
				}
				if called {
					t.Fatal("fn was called")
				}
			})
		}
	}
}

// Invariante: ningún camino deja app.session_kind vacío. Se barre todo el
// rango del tipo: solo los tres valores definidos abren transacción y en todos
// la variable llega con un valor conocido.
func TestEverySessionKindValueEitherFailsOrSetsAKnownKind(t *testing.T) {
	_, p := tenantDB(t, 1)
	ctx := context.Background()
	imp := uid(t, "cccccccc-0000-0000-0000-00000000000c")
	known := map[string]bool{"user": true, "impersonated": true, "system": true}
	opened := 0
	for k := 0; k <= 255; k++ {
		a := db.Actor{TenantID: uid(t, tenantA), UserID: uid(t, userU), Kind: db.SessionKind(k)}
		if db.SessionKind(k) == db.KindImpersonated {
			a.ImpersonatorID = imp
		}
		var kind string
		err := p.InTenantTx(ctx, a, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT current_setting('app.session_kind')`).Scan(&kind)
		})
		if err != nil {
			if !errors.Is(err, db.ErrInvalidActor) {
				t.Fatalf("kind %d: unexpected error %v", k, err)
			}
			continue
		}
		opened++
		if !known[kind] {
			t.Fatalf("kind %d produced session_kind %q", k, kind)
		}
	}
	if opened != 3 {
		t.Fatalf("%d kinds opened a transaction, want 3", opened)
	}
}

// Criterio 5: solo se ve el tenant propio y un INSERT de otro tenant viola RLS.
func TestInTenantTxIsolatesTenants(t *testing.T) {
	_, p := tenantDB(t, 2)
	ctx := context.Background()
	seed(t, p)

	for _, tn := range []string{tenantA, tenantB} {
		var tenants []string
		err := p.InTenantTx(ctx, actor(t, tn, db.KindUser), func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `SELECT tenant_id::text FROM rlstest.items`) // sin WHERE
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var s string
				if err := rows.Scan(&s); err != nil {
					return err
				}
				tenants = append(tenants, s)
			}
			return rows.Err()
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(tenants) != 1 || tenants[0] != tn {
			t.Fatalf("InTenantTx(%s) saw %v", tn[:1], tenants)
		}
	}

	err := p.InTenantTx(ctx, actor(t, tenantA, db.KindUser), func(tx pgx.Tx) error {
		return insertItem(ctx, tx, uid(t, tenantB), "forged")
	})
	if pgCode(err) != "42501" {
		t.Fatalf("insert for another tenant: %v, want SQLSTATE 42501", err)
	}
	// Tampoco puede tocar las filas ajenas: UPDATE y DELETE afectan 0 filas.
	err = p.InTenantTx(ctx, actor(t, tenantA, db.KindUser), func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM rlstest.items WHERE tenant_id = $1`, uid(t, tenantB))
		if err == nil && tag.RowsAffected() != 0 {
			err = fmt.Errorf("deleted %d rows of another tenant", tag.RowsAffected())
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Criterio 6: la misma conexión física, ya usada por un tenant, no ve nada sin
// InTenantTx, ni siquiera a través de una transacción normal del pool.
func TestReusedConnectionWithoutTenantSeesNothing(t *testing.T) {
	_, p := tenantDB(t, 1) // un solo hueco: la conexión es siempre la misma
	ctx := context.Background()
	seed(t, p)

	var pidIn int
	err := p.InTenantTx(ctx, actor(t, tenantA, db.KindUser), func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pidIn)
	})
	if err != nil {
		t.Fatal(err)
	}

	conn, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	var pid, rows int
	var tenantNull bool
	var raw string
	err = tx.QueryRow(ctx, `SELECT pg_backend_pid(), (SELECT count(*) FROM rlstest.items),
		app.current_tenant() IS NULL, current_setting('app.tenant_id', true)`).Scan(&pid, &rows, &tenantNull, &raw)
	if err != nil {
		t.Fatal(err)
	}
	if pid != pidIn {
		t.Fatalf("test bug: the connection changed (%d -> %d)", pidIn, pid)
	}
	if rows != 0 || !tenantNull || raw != "" {
		t.Fatalf("rows=%d current_tenant is NULL=%v raw tenant_id=%q: state leaked", rows, tenantNull, raw)
	}
	// Un SELECT y un INSERT sin tenant no dan error de cast: el INSERT lo rechaza RLS.
	if err := insertItem(ctx, tx, uid(t, tenantA), "x"); pgCode(err) != "42501" {
		t.Fatalf("insert without tenant: %v, want SQLSTATE 42501", err)
	}
}

// Criterio 7: fn falla a mitad; no queda escritura y nada se filtra a la
// siguiente transacción de la misma conexión.
func TestFailedFnLeavesNoWritesAndNoStateOnTheConnection(t *testing.T) {
	_, p := tenantDB(t, 1)
	ctx := context.Background()
	imp := uid(t, "cccccccc-0000-0000-0000-00000000000c")
	a := db.Actor{TenantID: uid(t, tenantA), UserID: uid(t, userU), Kind: db.KindImpersonated, ImpersonatorID: imp}

	var pid1 int
	err := p.InTenantTx(ctx, a, func(tx pgx.Tx) error {
		if err := insertItem(ctx, tx, a.TenantID, "half"); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid1); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `SELECT 1/0`) // falla a mitad
		return err
	})
	if pgCode(err) != "22012" {
		t.Fatalf("error = %v, want division_by_zero", err)
	}

	// La siguiente transacción de la misma conexión, sin tenant, no ve ninguna variable.
	conn, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var pid2 int
	var vars [4]string
	err = conn.QueryRow(ctx, `SELECT pg_backend_pid(), current_setting('app.tenant_id', true), current_setting('app.user_id', true),
		current_setting('app.session_kind', true), current_setting('app.impersonator_id', true)`).Scan(&pid2, &vars[0], &vars[1], &vars[2], &vars[3])
	conn.Release()
	if err != nil {
		t.Fatal(err)
	}
	if pid1 != pid2 {
		t.Fatalf("test bug: the connection changed (%d -> %d)", pid1, pid2)
	}
	if vars != [4]string{} {
		t.Fatalf("variables leaked to the next transaction: %q", vars)
	}

	// Y la tabla quedó sin la fila a medias.
	var n int
	err = p.InTenantTx(ctx, a, func(tx pgx.Tx) (err error) { n, err = countItems(ctx, tx); return })
	if err != nil || n != 0 {
		t.Fatalf("rows after the failed fn = %d (err %v), want 0", n, err)
	}
}

// Una cancelación del contexto en medio de fn deshace la transacción y la
// conexión física se conserva: el ROLLBACK no depende del contexto cancelado.
// Con un ROLLBACK fallido el pool destruiría la conexión y la siguiente
// transacción abriría otra, por eso se compara el pid.
func TestCanceledContextRollsBackAndKeepsTheConnection(t *testing.T) {
	_, p := tenantDB(t, 1)
	a := actor(t, tenantA, db.KindUser)
	ctx, cancel := context.WithCancel(context.Background())
	var pidIn int
	err := p.InTenantTx(ctx, a, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pidIn); err != nil {
			return err
		}
		if err := insertItem(ctx, tx, a.TenantID, "x"); err != nil {
			return err
		}
		cancel()
		_, err := tx.Exec(ctx, `SELECT pg_sleep(5)`)
		return err
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	var n, pid int
	err = p.InTenantTx(context.Background(), a, func(tx pgx.Tx) error {
		if err := tx.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
			return err
		}
		n, err = countItems(context.Background(), tx)
		return err
	})
	if err != nil || n != 0 {
		t.Fatalf("rows = %d (err %v), want 0", n, err)
	}
	if pid != pidIn {
		t.Fatalf("the connection was destroyed (%d -> %d): the ROLLBACK depended on the canceled context", pidIn, pid)
	}
}

// Un COMMIT que falla llega al llamador y no deja nada guardado.
func TestFailedCommitReachesTheCaller(t *testing.T) {
	_, p := tenantDB(t, 1)
	ctx := context.Background()
	a := actor(t, tenantA, db.KindUser)

	t.Run("deferred constraint violated at commit", func(t *testing.T) {
		err := p.InTenantTx(ctx, a, func(tx pgx.Tx) error {
			for range 2 { // la restricción es diferida: fn no ve el error
				if err := insertItem(ctx, tx, a.TenantID, "dup"); err != nil {
					return err
				}
			}
			return nil
		})
		if pgCode(err) != "23505" {
			t.Fatalf("error = %v, want unique violation (23505) from COMMIT", err)
		}
	})
	t.Run("fn swallows an error and the transaction is already aborted", func(t *testing.T) {
		err := p.InTenantTx(ctx, a, func(tx pgx.Tx) error {
			_ = insertItem(ctx, tx, uid(t, tenantB), "forged") // 42501 ignorado
			return nil
		})
		if !errors.Is(err, pgx.ErrTxCommitRollback) {
			t.Fatalf("error = %v, want pgx.ErrTxCommitRollback", err)
		}
	})
	var n int
	err := p.InTenantTx(ctx, a, func(tx pgx.Tx) (err error) { n, err = countItems(ctx, tx); return })
	if err != nil || n != 0 {
		t.Fatalf("rows = %d (err %v), want 0", n, err)
	}
}

// Una variable fijada a nivel de sesión dentro de fn (prohibido, ver el doc del
// paquete) sobrevive al COMMIT en esa conexión, pero la siguiente InTenantTx
// vuelve a fijar las cuatro y no la ve; y fuera de este paquete nadie puede
// usar la conexión cruda (Acquire no es exportado).
func TestSessionLevelSetConfigInFnDoesNotReachTheNextTenantTx(t *testing.T) {
	_, p := tenantDB(t, 1)
	ctx := context.Background()
	seed(t, p)
	err := p.InTenantTx(ctx, actor(t, tenantA, db.KindUser), func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, false)`, tenantA)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var tenants []string
	err = p.InTenantTx(ctx, actor(t, tenantB, db.KindUser), func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT tenant_id::text FROM rlstest.items`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				return err
			}
			tenants = append(tenants, s)
		}
		return rows.Err()
	})
	if err != nil || len(tenants) != 1 || tenants[0] != tenantB {
		t.Fatalf("tenant B saw %v (err %v)", tenants, err)
	}
}

// Para Kind = system el ADR 0001 («Workers», webhooks) no pasa UserID: es válido.
func TestSystemActorWithoutUserIDIsValid(t *testing.T) {
	_, p := tenantDB(t, 1)
	ctx := context.Background()
	a := db.Actor{TenantID: uid(t, tenantA), Kind: db.KindSystem}
	var user, kind string
	err := p.InTenantTx(ctx, a, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT current_setting('app.user_id'), current_setting('app.session_kind')`).Scan(&user, &kind)
	})
	if err != nil || user != "" || kind != "system" {
		t.Fatalf("user_id=%q session_kind=%q err=%v", user, kind, err)
	}
}

// H2: el timeout se reconoce por SQLSTATE 57014 y el ctx vivo, nunca por el
// texto del mensaje, que PostgreSQL traduce según lc_messages.
func TestTypedErrClassifiesBySQLStateNotByMessage(t *testing.T) {
	dead, cancel := context.WithCancel(context.Background())
	cancel()
	alive := context.Background()
	pgErr := func(code, msg string) error {
		return fmt.Errorf("fn: %w", &pgconn.PgError{Code: code, Message: msg})
	}
	cases := []struct {
		name      string
		ctx       context.Context
		err       error
		wantTyped bool
	}{
		{"english message", alive, pgErr("57014", "canceling statement due to statement timeout"), true},
		{"spanish message", alive, pgErr("57014", "cancelando la sentencia debido a que se agotó el tiempo de espera de sentencias"), true},
		{"german message", alive, pgErr("57014", "storniere Anfrage wegen Zeitüberschreitung"), true},
		{"caller canceled", dead, pgErr("57014", "canceling statement due to user request"), false},
		{"caller canceled and statement timeout together", dead, pgErr("57014", "canceling statement due to statement timeout"), false},
		{"other SQLSTATE with the same words", alive, pgErr("XX000", "canceling statement due to statement timeout"), false},
		{"not a PostgreSQL error", alive, errors.New("canceling statement due to statement timeout"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := db.TypedErr(c.ctx, c.err)
			if errors.Is(got, db.ErrStatementTimeout) != c.wantTyped {
				t.Fatalf("typed = %v, want %v (err %v)", !c.wantTyped, c.wantTyped, got)
			}
			if !errors.Is(got, c.err) {
				t.Fatalf("the original error was lost: %v", got)
			}
		})
	}
}

// Criterio 8: InTenantReadTx corre en BEGIN READ ONLY.
func TestInTenantReadTxIsReadOnly(t *testing.T) {
	_, p := tenantDB(t, 2)
	ctx := context.Background()
	seed(t, p)
	a := actor(t, tenantA, db.KindUser)

	var n int
	var ro string
	err := p.InTenantReadTx(ctx, a, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT current_setting('transaction_read_only')`).Scan(&ro); err != nil {
			return err
		}
		n, _ = countItems(ctx, tx)
		return nil
	})
	if err != nil || ro != "on" || n != 1 {
		t.Fatalf("read tx: err=%v transaction_read_only=%q rows=%d", err, ro, n)
	}
	err = p.InTenantReadTx(ctx, a, func(tx pgx.Tx) error { return insertItem(ctx, tx, a.TenantID, "nope") })
	if pgCode(err) != "25006" {
		t.Fatalf("insert in a read tx: %v, want SQLSTATE 25006", err)
	}
}

// Criterio 9: el statement_timeout de 5 s del rol rige dentro de la
// transacción y, al vencerse, el error llega tipado conservando la causa.
func TestStatementTimeoutArrivesTyped(t *testing.T) {
	_, p := tenantDB(t, 1)
	ctx := context.Background()
	a := actor(t, tenantA, db.KindUser)

	var setting string
	err := p.InTenantTx(ctx, a, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT current_setting('statement_timeout')`).Scan(&setting)
	})
	if err != nil || setting != "5s" {
		t.Fatalf("statement_timeout = %q (err %v), want 5s", setting, err)
	}

	// Se acorta con SET LOCAL para no esperar 5 s; el camino de error es el mismo.
	err = p.InTenantTx(ctx, a, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout = '100ms'`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `SELECT pg_sleep(3)`)
		return err
	})
	if !errors.Is(err, db.ErrStatementTimeout) {
		t.Fatalf("error = %v, want ErrStatementTimeout", err)
	}
	if pgCode(err) != "57014" {
		t.Fatalf("the PostgreSQL cause was lost: %v", err)
	}

	// Una cancelación del llamador no es un statement_timeout.
	cctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	err = p.InTenantTx(cctx, a, func(tx pgx.Tx) error {
		_, err := tx.Exec(cctx, `SELECT pg_sleep(3)`)
		return err
	})
	if err == nil || errors.Is(err, db.ErrStatementTimeout) {
		t.Fatalf("a canceled context was reported as %v", err)
	}

	// Contexto y statement_timeout vencen a la vez: manda el contexto, no se tipa.
	bctx, bcancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer bcancel()
	err = p.InTenantTx(bctx, a, func(tx pgx.Tx) error {
		if _, err := tx.Exec(bctx, `SET LOCAL statement_timeout = '50ms'`); err != nil {
			return err
		}
		_, err := tx.Exec(bctx, `SELECT pg_sleep(3)`)
		return err
	})
	if err == nil || errors.Is(err, db.ErrStatementTimeout) {
		t.Fatalf("an expired context plus statement_timeout was reported as %v", err)
	}
}

// Invariante: sin estado entre transacciones, bajo concurrencia (-race): cada
// goroutine ve solo su tenant aunque comparta las dos conexiones del pool.
func TestConcurrentTenantsNeverSeeEachOther(t *testing.T) {
	_, p := tenantDB(t, 2)
	ctx := context.Background()
	seed(t, p)
	var wg sync.WaitGroup
	for i := range 40 {
		tn := tenantA
		if i%2 == 1 {
			tn = tenantB
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := p.InTenantReadTx(ctx, actor(t, tn, db.KindUser), func(tx pgx.Tx) error {
				var bad int
				if err := tx.QueryRow(ctx, `SELECT count(*) FROM rlstest.items WHERE tenant_id <> $1`, uid(t, tn)).Scan(&bad); err != nil {
					return err
				}
				n, err := countItems(ctx, tx)
				if err != nil {
					return err
				}
				if bad != 0 || n != 1 {
					return fmt.Errorf("tenant %s saw %d rows, %d of others", tn[:1], n, bad)
				}
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

func TestSessionKindStringAndZeroValue(t *testing.T) {
	var zero db.SessionKind
	if zero.Valid() || zero.String() != "" {
		t.Fatalf("the zero SessionKind must be invalid: valid=%v string=%q", zero.Valid(), zero.String())
	}
	for k, want := range map[db.SessionKind]string{db.KindUser: "user", db.KindImpersonated: "impersonated", db.KindSystem: "system"} {
		if !k.Valid() || k.String() != want {
			t.Fatalf("kind %d: valid=%v string=%q, want %q", k, k.Valid(), k.String(), want)
		}
	}
}
