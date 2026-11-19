package pgsession

import (
	"context"
	"crypto/sha256"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

func migratedPool(t *testing.T) (*dbtest.DB, *db.Pool) {
	t.Helper()
	d := dbtest.New(t)
	ctx := context.Background()
	d.Migrate(ctx, t)
	p, err := db.Open(ctx, d.URL(dbtest.App), db.Options{MaxConns: 2, AppName: "pgsession-contract"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return d, p
}

// column es una columna de iam.session_lookup y el campo de auth.SessionRecord que la recibe.
type column struct{ name, field string }

// functionColumns son las 12 columnas que devuelve iam.session_lookup, en orden.
var functionColumns = []column{
	{"session_id", "SessionID"}, {"user_id", "UserID"}, {"tenant_id", "TenantID"}, {"site_id", "SiteID"},
	{"cash_register_id", "CashRegisterID"}, {"membership_id", "MembershipID"},
	{"session_authz_version", "SessionAuthzVersion"}, {"mfa_satisfied", "MFASatisfied"}, {"mfa_method", "MFAMethod"},
	{"impersonator_id", "ImpersonatorID"}, {"current_authz_version", "CurrentAuthzVersion"},
	{"membership_status", "MembershipStatus"},
}

// oidOf es el tipo de PostgreSQL que escanea cada tipo de campo del record.
var oidOf = map[reflect.Type]uint32{
	reflect.TypeFor[pgtype.UUID](): pgtype.UUIDOID,
	reflect.TypeFor[pgtype.Int8](): pgtype.Int8OID,
	reflect.TypeFor[pgtype.Text](): pgtype.TextOID,
	reflect.TypeFor[bool]():        pgtype.BoolOID,
	reflect.TypeFor[string]():      pgtype.TextOID,
}

// Contrato entre iam.session_lookup y auth.SessionRecord: las columnas de la
// función, en orden y tipo, son las del SELECT de Lookup y cada una se escanea
// en su campo. Si alguien agrega, quita, reordena o cambia el tipo de una
// columna, o agrega un campo al record sin columna, este test falla.
func TestLookupColumnsMatchTheFunction(t *testing.T) {
	d, _ := migratedPool(t)
	ctx := context.Background()
	rows, err := d.Admin(ctx, t).Query(ctx, `SELECT * FROM iam.session_lookup($1)`, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	fds := rows.FieldDescriptions()
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	var rec auth.SessionRecord
	rv := reflect.ValueOf(&rec).Elem()
	// ImpersonationScope no es una columna: iam.session_lookup no la devuelve hasta la ola 6 (#176).
	if got := rv.NumField() - 1; got != len(functionColumns) {
		t.Fatalf("auth.SessionRecord has %d column fields, the contract lists %d columns", got, len(functionColumns))
	}
	if len(fds) != len(functionColumns) {
		t.Fatalf("iam.session_lookup returns %d columns, SessionRecord maps %d", len(fds), len(functionColumns))
	}
	targets := scanTargets(&rec)
	if len(targets) != len(functionColumns) {
		t.Fatalf("scanTargets returns %d destinations for %d columns", len(targets), len(functionColumns))
	}
	for i, c := range functionColumns {
		if fds[i].Name != c.name {
			t.Errorf("column %d of iam.session_lookup is %q, want %q", i+1, fds[i].Name, c.name)
		}
		f := rv.FieldByName(c.field)
		if !f.IsValid() {
			t.Fatalf("auth.SessionRecord has no field %s", c.field)
		}
		oid, ok := oidOf[f.Type()]
		if !ok {
			t.Fatalf("field %s has the unmapped type %s", c.field, f.Type())
		}
		if fds[i].DataTypeOID != oid {
			t.Errorf("column %s has type OID %d, field %s (%s) scans OID %d", c.name, fds[i].DataTypeOID, c.field, f.Type(), oid)
		}
		if reflect.ValueOf(targets[i]).Pointer() != f.Addr().Pointer() {
			t.Errorf("scan target %d (column %s) is not &rec.%s", i+1, c.name, c.field)
		}
	}
	// El SELECT de Lookup nombra las mismas columnas, en el mismo orden.
	if sel := selectedColumns(t, d); !reflect.DeepEqual(sel, columnNames()) {
		t.Errorf("lookupSQL selects %v, want %v", sel, columnNames())
	}
}

func columnNames() []string {
	names := make([]string, len(functionColumns))
	for i, c := range functionColumns {
		names[i] = c.name
	}
	return names
}

// selectedColumns ejecuta lookupSQL y devuelve los nombres de sus columnas.
func selectedColumns(t *testing.T, d *dbtest.DB) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := d.Admin(ctx, t).Query(ctx, lookupSQL, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for _, fd := range rows.FieldDescriptions() {
		names = append(names, fd.Name)
	}
	return names
}

func uuidText(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return u.String()
}

func hashOf(token string) [32]byte { return sha256.Sum256([]byte(token)) }

// Cada columna llega a su campo con su valor (no solo con su tipo): una sede no
// se confunde con una caja ni la versión de la sesión con la de la membresía.
func TestLookupMapsEveryColumnToItsField(t *testing.T) {
	d, p := migratedPool(t)
	r := New(p)
	ctx := context.Background()

	t.Run("bound", func(t *testing.T) {
		s := d.SeedSession(t, dbtest.SessionSpec{MembershipStatus: "suspendido", SessionVersion: 2, MembershipVersion: 5})
		rec, found, err := r.Lookup(ctx, hashOf(s.Token))
		if err != nil || !found {
			t.Fatalf("Lookup = found %v, err %v", found, err)
		}
		for name, got := range map[string][2]string{
			"SessionID": {uuidText(rec.SessionID), s.SessionID}, "UserID": {uuidText(rec.UserID), s.UserID},
			"TenantID": {uuidText(rec.TenantID), s.TenantID}, "SiteID": {uuidText(rec.SiteID), s.SiteID},
			"CashRegisterID": {uuidText(rec.CashRegisterID), s.CashRegisterID}, "MembershipID": {uuidText(rec.MembershipID), s.MembershipID},
			"ImpersonatorID": {uuidText(rec.ImpersonatorID), ""}, "MembershipStatus": {rec.MembershipStatus.String, "suspendido"},
			"MFAMethod": {rec.MFAMethod, "none"},
		} {
			if got[0] != got[1] {
				t.Errorf("%s = %q, want %q", name, got[0], got[1])
			}
		}
		if rec.SessionAuthzVersion != (pgtype.Int8{Int64: 2, Valid: true}) || rec.CurrentAuthzVersion != (pgtype.Int8{Int64: 5, Valid: true}) {
			t.Errorf("versions = %+v / %+v, want 2 / 5", rec.SessionAuthzVersion, rec.CurrentAuthzVersion)
		}
		if !rec.MFASatisfied || !rec.MembershipStatus.Valid {
			t.Errorf("MFASatisfied = %v, MembershipStatus.Valid = %v", rec.MFASatisfied, rec.MembershipStatus.Valid)
		}
	})

	t.Run("impersonated", func(t *testing.T) {
		s := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionImpersonated})
		rec, found, err := r.Lookup(ctx, hashOf(s.Token))
		if err != nil || !found {
			t.Fatalf("Lookup = found %v, err %v", found, err)
		}
		if uuidText(rec.ImpersonatorID) != s.ImpersonatorID || uuidText(rec.TenantID) != s.TenantID || rec.MFAMethod != "platform" {
			t.Errorf("impersonated record = %+v", rec)
		}
		if rec.UserID.Valid || rec.MembershipID.Valid || rec.SessionAuthzVersion.Valid || rec.CurrentAuthzVersion.Valid || rec.MembershipStatus.Valid {
			t.Errorf("an impersonated session has no user, membership or versions: %+v", rec)
		}
	})

	t.Run("no row", func(t *testing.T) {
		rec, found, err := r.Lookup(ctx, hashOf("wk_at_not-in-the-database"))
		if err != nil || found || rec.SessionID.Valid {
			t.Fatalf("Lookup of an unknown hash = %+v, found %v, err %v", rec, found, err)
		}
	})
}

func detachRow(t *testing.T, d *dbtest.DB, id string) (tenant, membership, site, cash *string, version *int64, revoked bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := d.Admin(ctx, t).QueryRow(ctx, `SELECT tenant_id::text, membership_id::text, site_id::text, cash_register_id::text, authz_version,
		revoked_at IS NOT NULL FROM iam.sessions WHERE id = $1`, id).Scan(&tenant, &membership, &site, &cash, &version, &revoked)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func mustUUID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatal(err)
	}
	return u
}

// El UPDATE de desligar actualiza una sola vez y solo con las guardas del ADR
// 0011: la membresía de la sesión y una sesión viva.
func TestDetachGuards(t *testing.T) {
	d, p := migratedPool(t)
	r := New(p)
	ctx := context.Background()

	t.Run("detaches once, then 0 rows", func(t *testing.T) {
		s := d.SeedSession(t, dbtest.SessionSpec{})
		sid, mid := mustUUID(t, s.SessionID), mustUUID(t, s.MembershipID)
		if ok, err := r.Detach(ctx, sid, mid); err != nil || !ok {
			t.Fatalf("first Detach = %v, %v; want true", ok, err)
		}
		tenant, membership, site, cash, version, revoked := detachRow(t, d, s.SessionID)
		if tenant != nil || membership != nil || site != nil || cash != nil || version != nil || revoked {
			t.Fatalf("detached row = %v %v %v %v %v revoked=%v; want all NULL and not revoked", tenant, membership, site, cash, version, revoked)
		}
		if ok, err := r.Detach(ctx, sid, mid); err != nil || ok {
			t.Fatalf("second Detach = %v, %v; want false, nil", ok, err)
		}
	})

	t.Run("another membership does not detach", func(t *testing.T) {
		s := d.SeedSession(t, dbtest.SessionSpec{})
		ok, err := r.Detach(ctx, mustUUID(t, s.SessionID), mustUUID(t, "aaaaaaaa-0000-4000-8000-00000000000b"))
		if err != nil || ok {
			t.Fatalf("Detach with another membership = %v, %v", ok, err)
		}
		if tenant, _, _, _, _, _ := detachRow(t, d, s.SessionID); tenant == nil {
			t.Fatal("the session was detached by another membership id")
		}
	})

	t.Run("a revoked session is left as it is", func(t *testing.T) {
		s := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionRevoked})
		ok, err := r.Detach(ctx, mustUUID(t, s.SessionID), mustUUID(t, s.MembershipID))
		if err != nil || ok {
			t.Fatalf("Detach of a revoked session = %v, %v", ok, err)
		}
		if tenant, _, _, _, _, _ := detachRow(t, d, s.SessionID); tenant == nil {
			t.Fatal("a revoked session was modified")
		}
	})
}

// Lookup corre en una transacción READ ONLY (InNoTenantReadTx). Como el resultado
// no cambia, se hace observable con un envoltorio de prueba, creado como
// superusuario, que ocupa el nombre de iam.session_lookup y falla si la
// transacción que lo llama admite escrituras.
func TestLookupRunsInAReadOnlyTransaction(t *testing.T) {
	d, p := migratedPool(t)
	ctx := context.Background()
	_, err := d.Admin(ctx, t).Exec(ctx, `
		ALTER FUNCTION iam.session_lookup(bytea) RENAME TO session_lookup_real;
		CREATE FUNCTION iam.session_lookup(p_access_hash bytea)
		  RETURNS TABLE (session_id uuid, user_id uuid, tenant_id uuid, site_id uuid, cash_register_id uuid, membership_id uuid,
		                 session_authz_version bigint, mfa_satisfied boolean, mfa_method text, impersonator_id uuid,
		                 current_authz_version bigint, membership_status text)
		  LANGUAGE plpgsql AS $$
		BEGIN
		  IF pg_catalog.current_setting('transaction_read_only') <> 'on' THEN
		    RAISE EXCEPTION 'the session lookup ran in a read-write transaction' USING ERRCODE = 'XX999';
		  END IF;
		  RETURN QUERY SELECT * FROM iam.session_lookup_real(p_access_hash);
		END $$;
		GRANT EXECUTE ON FUNCTION iam.session_lookup(bytea) TO wanka_app`)
	if err != nil {
		t.Fatal(err)
	}
	s := d.SeedSession(t, dbtest.SessionSpec{})
	rec, found, err := New(p).Lookup(ctx, hashOf(s.Token))
	if err != nil || !found || uuidText(rec.SessionID) != s.SessionID {
		t.Fatalf("Lookup = %+v, found %v, err %v; want the session, read through a READ ONLY transaction", rec, found, err)
	}
}

// Detach no toca una sesión impersonada aunque llegue con una membresía: la
// guarda impersonator_id IS NULL del UPDATE. Sin ella, el trigger de la base
// (42501) lo pararía con un error, no con 0 filas.
func TestDetachLeavesAnImpersonatedSessionAlone(t *testing.T) {
	d, p := migratedPool(t)
	ctx := context.Background()
	s := d.SeedSession(t, dbtest.SessionSpec{Kind: dbtest.SessionImpersonated})
	const membership = "aaaaaaaa-0000-4000-8000-00000000000c"
	// Como superusuario (el trigger no le aplica): una impersonada con membresía, que ni existe en producción.
	if _, err := d.Admin(ctx, t).Exec(ctx, `UPDATE iam.sessions SET membership_id = $2 WHERE id = $1`, s.SessionID, membership); err != nil {
		t.Fatal(err)
	}
	ok, err := New(p).Detach(ctx, mustUUID(t, s.SessionID), mustUUID(t, membership))
	if err != nil || ok {
		t.Fatalf("Detach of an impersonated session = %v, %v; want false, nil", ok, err)
	}
	if tenant, mem, _, _, _, _ := detachRow(t, d, s.SessionID); tenant == nil || mem == nil {
		t.Fatalf("an impersonated session was detached: tenant %v, membership %v", tenant, mem)
	}
}
