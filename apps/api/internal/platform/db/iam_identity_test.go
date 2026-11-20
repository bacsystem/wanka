package db_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// Pruebas de la identidad global de la ola 1 (#231): columnas de iam.users, iam.user_totp, iam.mfa_enrollments e
// iam.user_tokens. Las listas esperadas están escritas a mano desde data-model §6 y el ADR 0001, no calculadas.

func execErr(c *pgx.Conn, sql string, args ...any) error {
	_, err := c.Exec(context.Background(), sql, args...)
	return err
}

// newUser inserta un usuario como superusuario y devuelve su id.
func newUser(t *testing.T, admin *pgx.Conn, email string) string {
	t.Helper()
	return queryOne[string](t, admin, `INSERT INTO iam.users (id, email) VALUES (gen_random_uuid(), $1) RETURNING id::text`, email)
}

// columnsOf devuelve «columna → tipo nullable» de una tabla de iam, con el tipo que imprime format_type.
func columnsOf(t *testing.T, admin *pgx.Conn, table string) map[string]string {
	t.Helper()
	rows, err := admin.Query(context.Background(), `
		SELECT a.attname, format_type(a.atttypid, a.atttypmod) || CASE WHEN a.attnotnull THEN ' not null' ELSE ' null' END
		  FROM pg_attribute a WHERE a.attrelid = ('iam.' || $1)::regclass AND a.attnum > 0 AND NOT a.attisdropped`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var c, ty string
		if err := rows.Scan(&c, &ty); err != nil {
			t.Fatal(err)
		}
		got[c] = ty
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestIdentityTablesHaveExactlyTheContractColumns(t *testing.T) {
	d := migrated(t)
	admin := d.Admin(context.Background(), t)
	const tstz = "timestamp with time zone"
	want := map[string]map[string]string{
		"users": {
			"id": "uuid not null", "status": "text not null", "created_at": tstz + " not null", "updated_at": tstz + " not null",
			"email": "text not null", "full_name": "text null", "phone": "text null", "password_hash": "text null",
			"email_verified": "boolean not null", "password_changed_at": tstz + " null",
			"failed_logins": "smallint not null", "login_window_started_at": tstz + " null", "reset_mfa_failures": "smallint not null",
			"locked_until": tstz + " null", "login_lockouts": "smallint not null",
			"mfa_failed_attempts": "smallint not null", "mfa_locked_until": tstz + " null", "mfa_lockouts": "smallint not null",
		},
		"user_totp": {
			"user_id": "uuid not null", "secret_enc": "bytea not null", "confirmed_at": tstz + " null",
			"last_used_step": "bigint null", "recovery_codes_hash": "text[] not null",
		},
		"mfa_enrollments": {
			"id": "uuid not null", "user_id": "uuid not null", "token_hash": "bytea not null", "email_code_hmac": "bytea not null",
			"email_sends": "smallint not null", "email_sent_at": tstz + " not null", "totp_secret_enc": "bytea null",
			"expires_at": tstz + " not null", "consumed_at": tstz + " null", "created_at": tstz + " not null",
		},
		"user_tokens": {
			"id": "uuid not null", "user_id": "uuid not null", "purpose": "text not null", "token_hash": "bytea not null",
			"membership_id": "uuid null", "new_email": "text null", "expires_at": tstz + " not null",
			"consumed_at": tstz + " null", "created_at": tstz + " not null",
		},
	}
	for table, cols := range want {
		got := columnsOf(t, admin, table)
		for c, ty := range cols {
			if got[c] != ty {
				t.Errorf("iam.%s.%s = %q, want %q", table, c, got[c], ty)
			}
		}
		for c := range got {
			if _, ok := cols[c]; !ok {
				t.Errorf("iam.%s has the unexpected column %s", table, c)
			}
		}
		// Identidad global: ni tenant_id ni RLS (la prueba de CI de pg_catalog solo mira tablas con tenant_id).
		if _, ok := got["tenant_id"]; ok {
			t.Errorf("iam.%s must not have tenant_id", table)
		}
		if queryOne[bool](t, admin, `SELECT relrowsecurity OR relforcerowsecurity FROM pg_class WHERE oid = ('iam.' || $1)::regclass`, table) {
			t.Errorf("iam.%s must not have RLS", table)
		}
	}
}

func TestNewUserGetsTheDocumentedDefaults(t *testing.T) {
	d := migrated(t)
	admin := d.Admin(context.Background(), t)
	id := newUser(t, admin, "defaults@test.pe")
	var verified bool
	var failed, reset, lockouts, mfaFailed, mfaLockouts int
	var nulls int
	err := admin.QueryRow(context.Background(), `
		SELECT email_verified, failed_logins, reset_mfa_failures, login_lockouts, mfa_failed_attempts, mfa_lockouts,
		       (full_name IS NULL)::int + (phone IS NULL)::int + (password_hash IS NULL)::int + (password_changed_at IS NULL)::int
		       + (login_window_started_at IS NULL)::int + (locked_until IS NULL)::int + (mfa_locked_until IS NULL)::int
		  FROM iam.users WHERE id = $1`, id).Scan(&verified, &failed, &reset, &lockouts, &mfaFailed, &mfaLockouts, &nulls)
	if err != nil {
		t.Fatal(err)
	}
	if verified || failed != 0 || reset != 0 || lockouts != 0 || mfaFailed != 0 || mfaLockouts != 0 || nulls != 7 {
		t.Errorf("defaults: verified=%v counters=%d,%d,%d,%d,%d nulls=%d; want false, all 0 and 7 nulls",
			verified, failed, reset, lockouts, mfaFailed, mfaLockouts, nulls)
	}
}

func TestUserEmailRules(t *testing.T) {
	d := migrated(t)
	app := connectAs(t, d, dbtest.App)
	long := func(n int) string { return strings.Repeat("a", n-5) + "@b.pe" }
	for _, c := range []struct {
		name, email, state string
	}{
		{"lowercase", "ana@test.pe", ""},
		{"uppercase", "Ana@test.pe", "23514"},
		{"two chars", "ab", "23514"},
		{"three chars", "a@b", ""},
		{"254 chars", long(254), ""},
		{"255 chars", long(255), "23514"},
		{"empty", "", "23514"},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := execErr(app, `INSERT INTO iam.users (id, email) VALUES (gen_random_uuid(), $1)`, c.email)
			if c.state == "" && err != nil {
				t.Fatalf("rejected: %v", err)
			}
			if c.state != "" {
				wantState(t, err, c.state, "email "+c.name)
			}
		})
	}
	wantState(t, execErr(app, `INSERT INTO iam.users (id) VALUES (gen_random_uuid())`), "23502", "email is required")
	wantState(t, execErr(app, `INSERT INTO iam.users (id, email) VALUES (gen_random_uuid(), 'ana@test.pe')`), "23505", "email is unique")
}

// Invariante: un correo no se duplica aunque dos altas lleguen a la vez.
func TestConcurrentSignupsOfTheSameEmailLeaveOne(t *testing.T) {
	d := migrated(t)
	const n = 4
	conns := make([]*pgx.Conn, n)
	for i := range conns {
		conns[i] = connectAs(t, d, dbtest.App)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, n)
	for i := range conns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = execErr(conns[i], `INSERT INTO iam.users (id, email) VALUES (gen_random_uuid(), 'race@test.pe')`)
		}()
	}
	close(start)
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else {
			wantState(t, err, "23505", "losing concurrent insert")
		}
	}
	if ok != 1 {
		t.Errorf("%d inserts of the same email succeeded, want exactly 1", ok)
	}
}

func TestUserProfileAndCounterRanges(t *testing.T) {
	d := migrated(t)
	app := connectAs(t, d, dbtest.App)
	admin := d.Admin(context.Background(), t)
	id := newUser(t, admin, "ranges@test.pe")
	type c struct {
		col string
		val any
		ok  bool
	}
	cases := []c{
		{"full_name", nil, true}, {"full_name", "A", true}, {"full_name", strings.Repeat("n", 120), true},
		{"full_name", "", false}, {"full_name", strings.Repeat("n", 121), false},
		{"phone", nil, true}, {"phone", "+51987654321", true}, {"phone", "987654", true}, {"phone", "123456789012345", true},
		{"phone", "12345", false}, {"phone", "1234567890123456", false}, {"phone", "+", false}, {"phone", "9876a54321", false},
		{"phone", "++519876543", false}, {"phone", "987654\n", false}, {"phone", "", false},
		{"failed_logins", 0, true}, {"failed_logins", 5, true}, {"failed_logins", 6, false}, {"failed_logins", -1, false},
		{"reset_mfa_failures", 10, true}, {"reset_mfa_failures", 11, false}, {"reset_mfa_failures", -1, false},
		{"login_lockouts", 10, true}, {"login_lockouts", 11, false}, {"login_lockouts", -1, false},
		{"mfa_failed_attempts", 4, true}, {"mfa_failed_attempts", 5, false}, {"mfa_failed_attempts", -1, false},
		{"mfa_lockouts", 3, true}, {"mfa_lockouts", 4, false}, {"mfa_lockouts", -1, false},
		{"password_hash", nil, true}, {"password_hash", "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA", true},
	}
	for _, tc := range cases {
		err := execErr(app, `UPDATE iam.users SET `+tc.col+` = $1 WHERE id = $2`, tc.val, id)
		name := fmt.Sprintf("%s=%v", tc.col, tc.val)
		if tc.ok && err != nil {
			t.Errorf("%s rejected: %v", name, err)
		}
		if !tc.ok {
			wantState(t, err, "23514", name)
		}
	}
}

func TestUserTOTP(t *testing.T) {
	d := migrated(t)
	app := connectAs(t, d, dbtest.App)
	admin := d.Admin(context.Background(), t)
	u := newUser(t, admin, "totp@test.pe")
	if err := execErr(app, `INSERT INTO iam.user_totp (user_id, secret_enc) VALUES ($1, '\x0102')`, u); err != nil {
		t.Fatal(err)
	}
	if queryOne[bool](t, admin, `SELECT confirmed_at IS NOT NULL OR last_used_step IS NOT NULL OR cardinality(recovery_codes_hash) <> 0 FROM iam.user_totp WHERE user_id = $1`, u) {
		t.Error("a fresh TOTP row must be unconfirmed, with no step and no recovery codes")
	}
	wantState(t, execErr(app, `INSERT INTO iam.user_totp (user_id, secret_enc) VALUES ($1, '\x03')`, u), "23505", "one TOTP row per user")
	wantState(t, execErr(app, `INSERT INTO iam.user_totp (user_id, secret_enc) VALUES (gen_random_uuid(), '\x03')`), "23503", "FK to iam.users")
	wantState(t, execErr(app, `INSERT INTO iam.user_totp (user_id, secret_enc) VALUES ($1, NULL)`, newUser(t, admin, "totp2@test.pe")), "23502", "secret is required")
	wantState(t, execErr(app, `UPDATE iam.user_totp SET recovery_codes_hash = ARRAY['v1:aa', NULL] WHERE user_id = $1`, u), "23514", "no NULL recovery code")
	good := "v1:" + strings.Repeat("A", 43)
	for _, bad := range []string{"ABCD-EFGH", "v1:aa", "v0:" + strings.Repeat("A", 43), "V1:" + strings.Repeat("A", 43), "v1:" + strings.Repeat("A", 42),
		"v1:" + strings.Repeat("A", 44), "v1:" + strings.Repeat("+", 43), good + "\n", good + "," + good} {
		wantState(t, execErr(app, `UPDATE iam.user_totp SET recovery_codes_hash = ARRAY[$1::text] WHERE user_id = $2`, bad, u), "23514", fmt.Sprintf("recovery code %q", bad))
	}
	wantState(t, execErr(app, `UPDATE iam.user_totp SET recovery_codes_hash = ARRAY[$1::text, 'plain'] WHERE user_id = $2`, good, u), "23514", "one bad code among good ones")
	for _, ok := range []string{"{}", "{" + good + "}", "{" + good + ",v12:" + strings.Repeat("a_-", 13) + "abcd}"} {
		if err := execErr(app, `UPDATE iam.user_totp SET recovery_codes_hash = $1::text[] WHERE user_id = $2`, ok, u); err != nil {
			t.Errorf("well-formed recovery codes %q rejected: %v", ok, err)
		}
	}
	if err := execErr(app, `UPDATE iam.user_totp SET confirmed_at = now(), last_used_step = 9223372036854775807, recovery_codes_hash = ARRAY['v1:'||repeat('A',43),'v2:'||repeat('b',43)] WHERE user_id = $1`, u); err != nil {
		t.Errorf("confirming TOTP: %v", err)
	}
	wantState(t, execErr(app, `DELETE FROM iam.user_totp`), "42501", "app DELETE")
}

// insertToken inserta con el rol de la app (el trigger fija created_at) y devuelve el id.
func insertToken(app *pgx.Conn, purpose, hash, ttl string) error {
	cols, vals := "", ""
	switch purpose {
	case "invitation":
		cols, vals = ", membership_id", ", gen_random_uuid()"
	case "email_change":
		cols, vals = ", new_email", ", 'nuevo@test.pe'"
	}
	return execErr(app, `INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, expires_at`+cols+`)
		SELECT gen_random_uuid(), id, $1, sha256($2::text::bytea), now() + $3::interval`+vals+` FROM iam.users LIMIT 1`, purpose, hash, ttl)
}

func TestUserTokenRules(t *testing.T) {
	d := migrated(t)
	app := connectAs(t, d, dbtest.App)
	admin := d.Admin(context.Background(), t)
	newUser(t, admin, "tokens@test.pe")
	n := 0
	for _, c := range []struct {
		purpose, ttl, state string
	}{
		{"password_reset", "15 minutes", ""}, {"password_reset", "15 minutes 1 second", "23514"},
		{"invitation", "168 hours", ""}, {"invitation", "168 hours 1 second", "23514"},
		{"email_change", "24 hours", ""}, {"email_change", "24 hours 1 second", "23514"},
		{"password_reset", "1 microsecond", ""}, {"password_reset", "0 seconds", "23514"}, {"password_reset", "-1 minute", "23514"},
		{"other", "5 minutes", "23514"},
	} {
		n++
		err := insertToken(app, c.purpose, fmt.Sprint("tok", n), c.ttl)
		name := c.purpose + " " + c.ttl
		if c.state == "" && err != nil {
			t.Errorf("%s rejected: %v", name, err)
		}
		if c.state != "" {
			wantState(t, err, c.state, name)
		}
	}
	// membership_id solo en invitation y new_email solo en email_change, y los dos obligatorios en los suyos.
	const base = `INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, expires_at%s)
		SELECT gen_random_uuid(), id, $1, sha256($2::text::bytea), now() + interval '5 minutes'%s FROM iam.users LIMIT 1`
	for i, c := range []struct{ purpose, cols, vals string }{
		{"invitation", "", ""},
		{"password_reset", ", membership_id", ", gen_random_uuid()"},
		{"email_change", ", membership_id", ", gen_random_uuid()"},
		{"email_change", "", ""},
		{"password_reset", ", new_email", ", 'a@test.pe'"},
		{"invitation", ", new_email", ", 'a@test.pe'"},
		{"email_change", ", new_email", ", 'Mixed@Case.pe'"},
		{"email_change", ", new_email", ", 'ab'"},
	} {
		wantState(t, execErr(app, fmt.Sprintf(base, c.cols, c.vals), c.purpose, fmt.Sprint("bad", i)), "23514", fmt.Sprintf("%s%s", c.purpose, c.cols))
	}
	wantState(t, insertToken(app, "password_reset", "tok1", "5 minutes"), "23505", "token_hash is unique")
	wantState(t, execErr(app, `INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, expires_at)
		SELECT gen_random_uuid(), id, 'password_reset', '\x00', now() + interval '5 minutes' FROM iam.users LIMIT 1`), "23514", "token_hash is 32 bytes")
	// El CHECK de vigencia no depende del trigger: un superusuario (que lo salta) con created_at propio también topa.
	wantState(t, execErr(admin, `INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, created_at, expires_at)
		SELECT gen_random_uuid(), id, 'password_reset', sha256('x'), now() - interval '1 hour', now() - interval '44 minutes' FROM iam.users LIMIT 1`), "23514", "ttl without the trigger")
	// created_at lo fija el servidor: uno futuro no estira la vigencia.
	wantState(t, execErr(app, `INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, created_at, expires_at)
		SELECT gen_random_uuid(), id, 'password_reset', sha256('y'), now() + interval '30 days', now() + interval '30 days 10 minutes' FROM iam.users LIMIT 1`),
		"23514", "future created_at")
}

// El consumo es condicional: dos consumos simultáneos del mismo token dejan pasar a uno.
func TestUserTokenConsumeIsConditionalAndRaceSafe(t *testing.T) {
	d := migrated(t)
	admin := d.Admin(context.Background(), t)
	newUser(t, admin, "consume@test.pe")
	a, b := connectAs(t, d, dbtest.App), connectAs(t, d, dbtest.App)
	const consume = `UPDATE iam.user_tokens SET consumed_at = now()
		WHERE token_hash = sha256($1::text::bytea) AND purpose = 'password_reset' AND consumed_at IS NULL AND expires_at > now() RETURNING id`
	for round := 0; round < 20; round++ {
		tok := fmt.Sprint("race", round)
		if err := insertToken(a, "password_reset", tok, "10 minutes"); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		start := make(chan struct{})
		got := make([]int, 2)
		for i, c := range []*pgx.Conn{a, b} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				rows, err := c.Query(context.Background(), consume, tok)
				if err != nil {
					t.Error(err)
					return
				}
				for rows.Next() {
					got[i]++
				}
				rows.Close()
				if rows.Err() != nil {
					t.Error(rows.Err())
				}
			}()
		}
		close(start)
		wg.Wait()
		if got[0]+got[1] != 1 {
			t.Fatalf("round %d: consumed %d+%d times, want exactly once", round, got[0], got[1])
		}
	}
	// Vencido: no se consume.
	if _, err := admin.Exec(context.Background(), `INSERT INTO iam.user_tokens (id, user_id, purpose, token_hash, created_at, expires_at)
		SELECT gen_random_uuid(), id, 'password_reset', sha256('expired'::bytea), now() - interval '20 minutes', now() - interval '5 minutes' FROM iam.users LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	tag, err := a.Exec(context.Background(), `UPDATE iam.user_tokens SET consumed_at = now() WHERE token_hash = sha256('expired'::bytea) AND consumed_at IS NULL AND expires_at > now()`)
	if err != nil || tag.RowsAffected() != 0 {
		t.Errorf("consuming an expired token = %v, %v; want 0 rows", tag, err)
	}
}

// Invariante: un token consumido no vuelve a quedar sin consumir, y el instante lo fija el servidor.
func TestUserTokenConsumptionIsIrreversible(t *testing.T) {
	d := migrated(t)
	admin := d.Admin(context.Background(), t)
	newUser(t, admin, "irrev@test.pe")
	app := connectAs(t, d, dbtest.App)
	if err := insertToken(app, "email_change", "irrev", "1 hour"); err != nil {
		t.Fatal(err)
	}
	if err := execErr(app, `UPDATE iam.user_tokens SET consumed_at = now() + interval '1 hour'`); err != nil {
		t.Fatal(err)
	}
	first := queryOne[bool](t, admin, `SELECT consumed_at <= now() FROM iam.user_tokens`)
	if !first {
		t.Error("consumed_at must be set by the server, not taken from the caller")
	}
	wantState(t, execErr(app, `UPDATE iam.user_tokens SET consumed_at = NULL`), "42501", "reopen")
	// Otro consumo, en otra transacción, conserva el instante viejo.
	old := queryOne[string](t, admin, `UPDATE iam.user_tokens SET created_at = created_at - interval '2 hours', consumed_at = consumed_at - interval '1 hour' RETURNING consumed_at::text`)
	if err := execErr(app, `UPDATE iam.user_tokens SET consumed_at = now()`); err != nil {
		t.Fatal(err)
	}
	if got := queryOne[string](t, admin, `SELECT consumed_at::text FROM iam.user_tokens`); got != old {
		t.Errorf("consuming again moved consumed_at from %s to %s", old, got)
	}
}

func TestUserTokenGrantsForAppAndWorker(t *testing.T) {
	d := migrated(t)
	admin := d.Admin(context.Background(), t)
	newUser(t, admin, "grants@test.pe")
	app, w := connectAs(t, d, dbtest.App), connectAs(t, d, dbtest.Worker)
	if err := insertToken(app, "password_reset", "grants", "10 minutes"); err != nil {
		t.Fatal(err)
	}
	// wanka_app: solo consumed_at se actualiza; el resto da 42501.
	for _, col := range []string{"id", "user_id", "purpose", "token_hash", "membership_id", "new_email", "expires_at", "created_at"} {
		wantState(t, execErr(app, `UPDATE iam.user_tokens SET `+col+` = `+col), "42501", "app UPDATE "+col)
	}
	wantState(t, execErr(app, `DELETE FROM iam.user_tokens`), "42501", "app DELETE")
	wantState(t, execErr(app, `TRUNCATE iam.user_tokens`), "42501", "app TRUNCATE")
	// wanka_worker: DELETE y SELECT (id, expires_at); nunca el hash ni nada más.
	for _, col := range []string{"user_id", "purpose", "token_hash", "membership_id", "new_email", "consumed_at", "created_at"} {
		wantState(t, execErr(w, `SELECT `+col+` FROM iam.user_tokens`), "42501", "worker SELECT "+col)
	}
	wantState(t, execErr(w, `INSERT INTO iam.user_tokens DEFAULT VALUES`), "42501", "worker INSERT")
	wantState(t, execErr(w, `UPDATE iam.user_tokens SET consumed_at = now()`), "42501", "worker UPDATE")
	tag, err := w.Exec(context.Background(), `DELETE FROM iam.user_tokens WHERE expires_at < now() + interval '1 day'`)
	if err != nil || tag.RowsAffected() != 1 {
		t.Errorf("worker purge = %v, %v; want 1 row", tag, err)
	}
	// wanka_webhooks y wanka_platform no ven nada de las tres tablas nuevas.
	for _, role := range []string{dbtest.Webhooks, dbtest.Platform} {
		c := connectAs(t, d, role)
		for _, tbl := range []string{"user_totp", "mfa_enrollments", "user_tokens"} {
			wantState(t, execErr(c, `SELECT * FROM iam.`+tbl), "42501", role+" SELECT iam."+tbl)
		}
	}
}

// Los GRANT por columna del ADR 0001 (matriz y nota del 2026-11-19), contra una lista escrita a mano.
func TestIdentityTablesColumnGrantsMatchTheADR(t *testing.T) {
	d := migrated(t)
	admin := d.Admin(context.Background(), t)
	rows, err := admin.Query(context.Background(), `
		SELECT grantee || '|' || table_name || '|' || privilege_type, string_agg(column_name, ',' ORDER BY column_name)
		  FROM information_schema.column_privileges
		 WHERE table_schema = 'iam' AND table_name IN ('user_totp', 'mfa_enrollments', 'user_tokens') AND grantee <> 'wanka_migrator'
		 GROUP BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var k, cols string
		if err := rows.Scan(&k, &cols); err != nil {
			t.Fatal(err)
		}
		got[k] = cols
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	totp := "confirmed_at,last_used_step,recovery_codes_hash,secret_enc,user_id"
	mfa := "consumed_at,created_at,email_code_hmac,email_sends,email_sent_at,expires_at,id,token_hash,totp_secret_enc,user_id"
	tokens := "consumed_at,created_at,expires_at,id,membership_id,new_email,purpose,token_hash,user_id"
	want := map[string]string{
		"wanka_app|user_totp|SELECT": totp, "wanka_app|user_totp|INSERT": totp, "wanka_app|user_totp|UPDATE": totp,
		"wanka_app|mfa_enrollments|SELECT": mfa, "wanka_app|mfa_enrollments|INSERT": mfa,
		"wanka_app|mfa_enrollments|UPDATE": "consumed_at,email_code_hmac,email_sends,email_sent_at,totp_secret_enc",
		"wanka_app|user_tokens|SELECT":     tokens, "wanka_app|user_tokens|INSERT": tokens, "wanka_app|user_tokens|UPDATE": "consumed_at",
		"wanka_worker|mfa_enrollments|SELECT": "expires_at,id",
		"wanka_worker|user_tokens|SELECT":     "expires_at,id",
	}
	var keys []string
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected column grant %s = %s", k, got[k])
		}
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("column grant %s = %q, want %q", k, got[k], w)
		}
	}
}

// El trigger de iam.mfa_enrollments viene ahora de la migración real (antes solo de rls-check).
func TestMFAEnrollmentComesFromTheRealMigration(t *testing.T) {
	d := migrated(t)
	admin := d.Admin(context.Background(), t)
	u := newUser(t, admin, "mfa@test.pe")
	app := connectAs(t, d, dbtest.App)
	ins := `INSERT INTO iam.mfa_enrollments (id, user_id, token_hash, email_code_hmac, expires_at%s)
		VALUES (gen_random_uuid(), $1, sha256($2::text::bytea), sha256('h'), now() + interval '10 minutes'%s)`
	if err := execErr(app, fmt.Sprintf(ins, "", ""), u, "e1"); err != nil {
		t.Fatal(err)
	}
	wantState(t, execErr(app, `INSERT INTO iam.mfa_enrollments (id, user_id, token_hash, email_code_hmac, expires_at)
		VALUES (gen_random_uuid(), $1, sha256('e2'), sha256('h'), now() + interval '11 minutes')`, u), "23514", "ttl over 10 minutes")
	wantState(t, execErr(app, `INSERT INTO iam.mfa_enrollments (id, user_id, token_hash, email_code_hmac, created_at, expires_at)
		VALUES (gen_random_uuid(), $1, sha256('e3'), sha256('h'), now() + interval '30 days', now() + interval '30 days 5 minutes')`, u), "23514", "future created_at")
	if err := execErr(app, `UPDATE iam.mfa_enrollments SET consumed_at = now()`); err != nil {
		t.Fatal(err)
	}
	wantState(t, execErr(app, `UPDATE iam.mfa_enrollments SET consumed_at = NULL`), "42501", "reopen")
	wantState(t, execErr(app, `DELETE FROM iam.mfa_enrollments`), "42501", "app DELETE")
}
