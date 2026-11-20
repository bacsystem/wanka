package db_test

// Reglas de catálogo de aislamiento (ADR 0001, «Migraciones y pruebas» y
// «Funciones SECURITY DEFINER»; data-model §1 y §6). Corren sobre lo que haya
// migrado: ninguna regla lista tablas a mano. Lo que por diseño se sale de la
// regla vive en las listas blancas de abajo, cada entrada con su motivo y el ADR
// que lo justifica (README de apps/api, «Pruebas del catálogo RLS»).
//
// Cada regla es una función que devuelve las infracciones como texto que nombra
// el objeto. TestCatalogRules las exige vacías; TestCatalogRulesCatchMutations
// aplica una migración de prueba que viola cada regla y exige que la regla la
// nombre, así que una regla que dejara de mirar se detecta.

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// Listas blancas. Agregar una entrada es un PR de arquitectura: se cita el ADR
// (o su erratum) que la justifica y, si hace falta, se actualiza el ADR 0001.

// tenantRoots son las tablas de tenant que se aíslan por otra columna que
// tenant_id: la fila propia del tenant se aísla por su id (ADR 0001).
var tenantRoots = map[string]string{
	"org.tenants": "id",
}

// noRLSTables tienen columna tenant_id y, por diseño, no son de tenant: se saltan
// la regla de RLS y de políticas. Valor: el motivo, que cita el ADR.
var noRLSTables = map[string]string{
	"iam.sessions": "ADR 0001 («sin RLS: identidad global») y ADR 0004: se busca por hash antes de conocer el tenant; su tenant_id es la empresa elegida y puede ser NULL",
	// Tablas de entitlements (#171): platform.* es global y sin RLS; entitlements filtra por app.current_tenant().
	"platform.subscriptions":           "ADR 0001, matriz de GRANT, fila «platform.* (sin RLS)», y ADR 0005: suscripción de la empresa, la lee entitlements filtrando por app.current_tenant()",
	"platform.tenant_module_overrides": "ADR 0001, matriz de GRANT, fila «platform.* (sin RLS)», y ADR 0005: override de módulos por empresa, la lee entitlements filtrando por app.current_tenant()",
	"platform.tenant_flag_overrides":   "ADR 0001, matriz de GRANT, fila «platform.* (sin RLS)», y ADR 0005: override de flags por empresa, la lee entitlements filtrando por app.current_tenant()",
	"platform.tenant_usage":            "ADR 0001, matriz de GRANT, fila «platform.* (sin RLS)», y ADR 0001 «Uso por tenant para la consola»: la escribe wanka_worker y la lee entitlements filtrando por app.current_tenant()",
}

// policyRule describe una política adicional permitida (además de tenant_isolation).
type policyRule struct {
	cmds        []string // pg_policies.cmd admitidos
	roles       []string // TO <roles>; nil = PUBLIC
	userIs      bool     // la rama por usuario debe contener app.current_tenant() IS NULL
	mustContain []string // fragmentos que la expresión debe contener (para las políticas cuyo USING el ADR aún no escribe)
	qual        string   // si no está vacío, p.qual debe ser exactamente este texto (como lo deparsa PostgreSQL 16, espacios colapsados)
	reason      string
}

// purgeQual24h es el predicado de las políticas de limpieza de ADR 0012: sin
// tenant fijado y solo filas de más de 24 h, tal como lo devuelve pg_policies.
const purgeQual24h = "((app.current_tenant() IS NULL) AND (created_at < (now() - '24:00:00'::interval)))"

// policyAllow: clave «esquema.tabla:política». Es la lista blanca del ADR 0001
// (a) más las políticas de limpieza del ADR 0012; incluye tablas que llegan en
// olas siguientes, porque la lista es del ADR y la regla solo la consulta.
var policyAllow = map[string]policyRule{
	"iam.memberships:own_memberships": {cmds: []string{"SELECT"}, roles: []string{"wanka_app"}, userIs: true, qual: qualOwnMemberships, reason: reasonNoTenantPaths},
	// El ADR no escribe el USING de esta; la migración de #232 lo escribe con el mismo filtro de estado y aquí queda fijado.
	"iam.membership_sites:own_memberships":    {cmds: []string{"SELECT"}, roles: []string{"wanka_app"}, userIs: true, mustContain: []string{"status", "invitado", "activo"}, qual: qualMembershipSitesOwn, reason: reasonNoTenantPaths},
	"org.tenants:member_read":                 {cmds: []string{"SELECT"}, roles: []string{"wanka_app"}, userIs: true, qual: qualTenantsMemberRead, reason: reasonNoTenantPaths},
	"iam.roles:member_read":                   {cmds: []string{"SELECT"}, roles: []string{"wanka_app"}, userIs: true, qual: qualRolesMemberRead, reason: reasonNoTenantPaths},
	"org.sites:member_read":                   {cmds: []string{"SELECT"}, roles: []string{"wanka_app"}, userIs: true, qual: qualSitesMemberRead, reason: reasonNoTenantPaths},
	"org.cash_registers:member_read":          {cmds: []string{"SELECT"}, roles: []string{"wanka_app"}, userIs: true, qual: qualCashRegistersMemberRead, reason: reasonNoTenantPaths},
	"org.tenants:platform_rw":                 {cmds: []string{"ALL", "SELECT", "UPDATE"}, roles: []string{"wanka_platform"}, qual: "true", reason: "ADR 0001, matriz de GRANT: única política que no es solo de lectura, limitada TO wanka_platform"},
	"ops.idempotency_keys:purge_expired_read": {cmds: []string{"SELECT"}, roles: []string{"wanka_worker"}, userIs: true, qual: purgeQual24h, reason: "ADR 0012: limpieza sin tenant de wanka_worker, solo filas de más de 24 h"},
	"ops.idempotency_keys:purge_expired":      {cmds: []string{"DELETE"}, roles: []string{"wanka_worker"}, userIs: true, qual: purgeQual24h, reason: "ADR 0012: limpieza sin tenant de wanka_worker, solo filas de más de 24 h"},
}

const reasonNoTenantPaths = "ADR 0001, «Caminos que no tienen un tenant fijado», 1"

// Textos canónicos del ADR 0001 («Caminos que no tienen un tenant fijado», 1; filtro de estado de #187), tal como
// PostgreSQL 16 los deparsa en pg_policies.qual. Cambiar uno es cambiar el ADR.
const (
	qualOwnMemberships = `((app.current_tenant() IS NULL) AND (user_id = app.current_user_id()) AND (status = ANY (ARRAY['invitado'::text, 'activo'::text])))`

	qualMembershipSitesOwn = `((app.current_tenant() IS NULL) AND (membership_id IN ( SELECT memberships.id
	   FROM iam.memberships
	  WHERE ((memberships.user_id = app.current_user_id()) AND (memberships.status = ANY (ARRAY['invitado'::text, 'activo'::text]))))))`

	qualTenantsMemberRead = `((id = app.current_tenant()) OR ((app.current_tenant() IS NULL) AND (id IN ( SELECT memberships.tenant_id
	   FROM iam.memberships
	  WHERE ((memberships.user_id = app.current_user_id()) AND (memberships.status = ANY (ARRAY['invitado'::text, 'activo'::text])))))))`

	qualRolesMemberRead = `((tenant_id = app.current_tenant()) OR ((app.current_tenant() IS NULL) AND (id IN ( SELECT memberships.role_id
	   FROM iam.memberships
	  WHERE ((memberships.user_id = app.current_user_id()) AND (memberships.status = ANY (ARRAY['invitado'::text, 'activo'::text])))))))`

	qualSitesMemberRead = `((tenant_id = app.current_tenant()) OR ((app.current_tenant() IS NULL) AND (EXISTS ( SELECT 1
	   FROM iam.memberships m
	  WHERE ((m.user_id = app.current_user_id()) AND (m.status = 'activo'::text) AND (m.tenant_id = sites.tenant_id) AND (m.all_sites OR (EXISTS ( SELECT 1
	           FROM iam.membership_sites ms
	          WHERE ((ms.membership_id = m.id) AND (ms.site_id = sites.id))))))))))`

	qualCashRegistersMemberRead = `((tenant_id = app.current_tenant()) OR ((app.current_tenant() IS NULL) AND (site_id IN ( SELECT sites.id
	   FROM org.sites))))`
)

// squash colapsa los espacios y saltos de línea: PostgreSQL parte las expresiones largas en varias líneas.
func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

// uniqueAllow: UNIQUE o PK de una tabla con tenant_id que no empieza por
// tenant_id y no es una PK sustituta. Clave «esquema.tabla(col1,col2)».
var uniqueAllow = map[string]string{
	"iam.sessions(access_hash)":  "ADR 0004: el hash del token de acceso es único en todo el sistema y se busca antes de conocer el tenant",
	"iam.sessions(refresh_hash)": "ADR 0004: el hash del token de refresco es único en todo el sistema",
	"audit.events(id,at)":        "ADR 0001: id es PK sustituta (gen_random_uuid); PostgreSQL exige la columna de partición at en toda clave única",
}

// fkAllow: FK entre tablas de tenant que no lleva tenant_id. Clave «esquema.tabla.restricción».
var fkAllow = map[string]string{}

// definerOwnerAllow: función SECURITY DEFINER cuyo dueño es wanka_migrator y no
// wanka_definer, porque hace DDL sobre tablas del migrador. Clave: la firma
// regprocedure («esquema.función(tipos)»).
var definerOwnerAllow = map[string]string{
	"audit.ensure_partitions(date,integer)": "ADR 0001, «Excepción: DDL de particiones», #173",
}

// adrRef exige que el motivo de cada entrada cite un ADR.
var adrRef = regexp.MustCompile(`ADR (\d{4})`)

// adrDir es docs/architecture/adr, visto desde este paquete.
const adrDir = "../../../../../docs/architecture/adr"

// knownADRs devuelve los números de los ADR que existen en el repo.
func knownADRs(t testing.TB) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		t.Fatalf("reading the ADR directory: %v", err)
	}
	out := map[string]bool{}
	for _, e := range entries {
		if len(e.Name()) > 5 && e.Name()[4] == '-' {
			out[e.Name()[:4]] = true
		}
	}
	if len(out) == 0 {
		t.Fatalf("no ADR found in %s", adrDir)
	}
	return out
}

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// scanAll ejecuta sql y reúne una fila por llamada a scan.
func scanAll[T any](ctx context.Context, q queryer, sql string, scan func(pgx.Rows) (T, error)) ([]T, error) {
	rows, err := q.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Esquemas que no son del producto.
const userSchemas = `n.nspname NOT IN ('pg_catalog', 'information_schema', 'goose') AND n.nspname NOT LIKE 'pg\_toast%'`

type tableInfo struct {
	name              string
	rls, force, hasID bool     // hasID: tiene columna tenant_id
	parent            string   // tabla particionada madre, si es una partición
	grantees          []string // roles wanka_* de ejecución (no el migrador) con algún privilegio sobre la tabla o sus columnas
}

// tenantCol devuelve la columna que aísla la tabla, o "" si no es de tenant.
func (t tableInfo) tenantCol() string {
	if t.hasID {
		return "tenant_id"
	}
	return tenantRoots[t.name]
}

func loadTables(ctx context.Context, q queryer) (map[string]tableInfo, error) {
	list, err := scanAll(ctx, q, `
		SELECT n.nspname || '.' || c.relname, c.relrowsecurity, c.relforcerowsecurity,
		       EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped),
		       COALESCE((SELECT pn.nspname || '.' || pc.relname FROM pg_inherits h JOIN pg_class pc ON pc.oid = h.inhparent
		                   JOIN pg_namespace pn ON pn.oid = pc.relnamespace WHERE h.inhrelid = c.oid AND c.relispartition), ''),
		       ARRAY(SELECT r.rolname::text FROM pg_roles r WHERE r.rolname LIKE 'wanka\_%' AND r.rolname <> 'wanka_migrator'
		              AND (has_table_privilege(r.oid, c.oid, 'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')
		                   OR has_any_column_privilege(r.oid, c.oid, 'SELECT,INSERT,UPDATE,REFERENCES')) ORDER BY 1)
		  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE c.relkind IN ('r', 'p') AND `+userSchemas, func(r pgx.Rows) (tableInfo, error) {
		var t tableInfo
		return t, r.Scan(&t.name, &t.rls, &t.force, &t.hasID, &t.parent, &t.grantees)
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]tableInfo, len(list))
	for _, t := range list {
		out[t.name] = t
	}
	return out, nil
}

type policyInfo struct {
	table, name, cmd string
	roles            []string
	qual, check      string
}

func loadPolicies(ctx context.Context, q queryer) ([]policyInfo, error) {
	return scanAll(ctx, q, `
		SELECT schemaname || '.' || tablename, policyname, cmd, roles::text[], COALESCE(qual, ''), COALESCE(with_check, '')
		  FROM pg_policies
		 WHERE schemaname NOT IN ('pg_catalog', 'information_schema', 'goose')
		 ORDER BY 1, 2`, func(r pgx.Rows) (policyInfo, error) {
		var p policyInfo
		return p, r.Scan(&p.table, &p.name, &p.cmd, &p.roles, &p.qual, &p.check)
	})
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var usesCurrentTenant = regexp.MustCompile(`app\.current_tenant\(\)`)

// isolationProblems revisa RLS, FORCE y la política tenant_isolation exacta de una tabla de tenant.
func isolationProblems(name string, t tableInfo, iso map[string]policyInfo) []string {
	col := t.tenantCol()
	want := "(" + col + " = app.current_tenant())"
	var out []string
	if !t.rls || !t.force {
		out = append(out, fmt.Sprintf("%s: relrowsecurity=%v relforcerowsecurity=%v, both must be true (or add it to noRLSTables citing the ADR)", name, t.rls, t.force))
	}
	p, ok := iso[name]
	switch {
	case !ok:
		out = append(out, fmt.Sprintf("%s: no tenant_isolation policy", name))
	case p.cmd != "ALL":
		out = append(out, fmt.Sprintf("%s: tenant_isolation is FOR %s, want ALL", name, p.cmd))
	case squash(p.qual) != want:
		out = append(out, fmt.Sprintf("%s: tenant_isolation USING does not isolate by exactly %s: %q", name, want, p.qual))
	case p.check == "":
		out = append(out, fmt.Sprintf("%s: tenant_isolation has no WITH CHECK, want %s", name, want))
	case squash(p.check) != want:
		out = append(out, fmt.Sprintf("%s: tenant_isolation WITH CHECK is %q, want exactly %s", name, p.check, want))
	}
	return out
}

func isolationPolicies(ctx context.Context, q queryer) (map[string]policyInfo, error) {
	pols, err := loadPolicies(ctx, q)
	if err != nil {
		return nil, err
	}
	iso := map[string]policyInfo{}
	for _, p := range pols {
		if p.name == "tenant_isolation" {
			iso[p.table] = p
		}
	}
	return iso, nil
}

// (a) Toda tabla de tenant tiene RLS, FORCE y la política tenant_isolation exacta del ADR. Las particiones van aparte.
func ruleTenantRLS(ctx context.Context, q queryer) ([]string, error) {
	tables, err := loadTables(ctx, q)
	if err != nil {
		return nil, err
	}
	iso, err := isolationPolicies(ctx, q)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, name := range sortedKeys(tables) {
		t := tables[name]
		if t.tenantCol() == "" || t.parent != "" {
			continue
		}
		if _, skip := noRLSTables[name]; skip {
			continue
		}
		out = append(out, isolationProblems(name, t, iso)...)
	}
	return out, nil
}

// Una consulta directa a una partición aplica las políticas de la partición, no las de su madre. Por eso una
// partición de una tabla de tenant, o no tiene ningún privilegio de rol de ejecución (así se hace con audit.events,
// #173: se llega por la madre), o tiene su propia RLS con FORCE y tenant_isolation.
func rulePartitions(ctx context.Context, q queryer) ([]string, error) {
	tables, err := loadTables(ctx, q)
	if err != nil {
		return nil, err
	}
	iso, err := isolationPolicies(ctx, q)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, name := range sortedKeys(tables) {
		t := tables[name]
		if t.parent == "" || t.tenantCol() == "" || len(t.grantees) == 0 {
			continue
		}
		if _, skip := noRLSTables[t.parent]; skip {
			continue
		}
		for _, v := range isolationProblems(name, t, iso) {
			out = append(out, fmt.Sprintf("partition %s of %s is granted to %v and is not isolated by itself (revoke the privileges or give it RLS): %s", name, t.parent, t.grantees, v))
		}
	}
	return out, nil
}

// (a) Toda otra política es de la lista blanca del ADR, en su tabla, con su comando.
func rulePolicies(ctx context.Context, q queryer) ([]string, error) {
	tables, err := loadTables(ctx, q)
	if err != nil {
		return nil, err
	}
	pols, err := loadPolicies(ctx, q)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range pols {
		if p.name == "tenant_isolation" {
			if tables[p.table].tenantCol() == "" {
				out = append(out, fmt.Sprintf("%s: tenant_isolation on a table that is not a tenant table", p.table))
			}
			continue
		}
		rule, ok := policyAllow[p.table+":"+p.name]
		if !ok {
			out = append(out, fmt.Sprintf("%s: policy %s is not in the ADR 0001 whitelist (own_memberships, member_read, platform_rw) nor ADR 0012 (purge_*)", p.table, p.name))
			continue
		}
		if !contains(rule.cmds, p.cmd) {
			out = append(out, fmt.Sprintf("%s: policy %s is FOR %s, allowed %v", p.table, p.name, p.cmd, rule.cmds))
		}
		wantRoles := rule.roles
		if wantRoles == nil {
			wantRoles = []string{"public"}
		}
		if strings.Join(p.roles, ",") != strings.Join(wantRoles, ",") {
			out = append(out, fmt.Sprintf("%s: policy %s is TO %v, want %v", p.table, p.name, p.roles, wantRoles))
		}
		if rule.qual != "" && squash(p.qual) != squash(rule.qual) {
			out = append(out, fmt.Sprintf("%s: policy %s does not match the canonical expression of ADR 0001: got %q, want %q", p.table, p.name, squash(p.qual), squash(rule.qual)))
		}
		for _, frag := range rule.mustContain {
			if !strings.Contains(p.qual, frag) {
				out = append(out, fmt.Sprintf("%s: policy %s must contain %q (ADR 0001 (a), #187): %q", p.table, p.name, frag, p.qual))
			}
		}
		if rule.userIs && !strings.Contains(p.qual, "app.current_tenant() IS NULL") {
			out = append(out, fmt.Sprintf("%s: policy %s has no app.current_tenant() IS NULL in its user branch: %q", p.table, p.name, p.qual))
		}
	}
	return out, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// (c) Ninguna política usa current_setting sin NULLIF.
func rulePoliciesNullif(ctx context.Context, q queryer) ([]string, error) {
	pols, err := loadPolicies(ctx, q)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range pols {
		for _, expr := range []string{p.qual, p.check} {
			if strings.Count(expr, "current_setting(") > strings.Count(expr, "NULLIF(current_setting(") {
				out = append(out, fmt.Sprintf("%s: policy %s uses current_setting without NULLIF: %q", p.table, p.name, expr))
				break
			}
		}
	}
	return out, nil
}

type uniqueInfo struct {
	table, index          string
	primary               bool
	cols                  []string
	firstType, firstIdent string
}

// (b) Todo UNIQUE o PK de una tabla con tenant_id empieza por tenant_id. Salvo la
// PK sustituta (una sola columna id uuid o identidad) y uniqueAllow.
func ruleUniques(ctx context.Context, q queryer) ([]string, error) {
	tables, err := loadTables(ctx, q)
	if err != nil {
		return nil, err
	}
	idx, err := scanAll(ctx, q, `
		SELECT n.nspname || '.' || c.relname, ic.relname, i.indisprimary,
		       ARRAY(SELECT CASE WHEN k.attnum = 0 THEN '<expr>' ELSE a.attname::text END
		               FROM unnest(i.indkey::int2[]) WITH ORDINALITY k(attnum, ord)
		               LEFT JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
		              WHERE k.ord <= i.indnkeyatts ORDER BY k.ord),
		       COALESCE((SELECT format_type(a.atttypid, NULL) FROM pg_attribute a WHERE a.attrelid = i.indrelid AND a.attnum = i.indkey[0]), ''),
		       COALESCE((SELECT a.attidentity::text FROM pg_attribute a WHERE a.attrelid = i.indrelid AND a.attnum = i.indkey[0]), '')
		  FROM pg_index i
		  JOIN pg_class c ON c.oid = i.indrelid JOIN pg_namespace n ON n.oid = c.relnamespace
		  JOIN pg_class ic ON ic.oid = i.indexrelid
		 WHERE i.indisunique AND NOT c.relispartition AND `+userSchemas+` ORDER BY 1, 2`, func(r pgx.Rows) (uniqueInfo, error) {
		var u uniqueInfo
		return u, r.Scan(&u.table, &u.index, &u.primary, &u.cols, &u.firstType, &u.firstIdent)
	})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, u := range idx {
		if !tables[u.table].hasID || u.cols[0] == "tenant_id" {
			continue
		}
		if u.primary && len(u.cols) == 1 && u.cols[0] == "id" && (u.firstType == "uuid" || u.firstIdent != "") {
			continue // PK sustituta generada por el servidor
		}
		key := u.table + "(" + strings.Join(u.cols, ",") + ")"
		if _, ok := uniqueAllow[key]; ok {
			continue
		}
		out = append(out, fmt.Sprintf("%s: unique index %s on (%s) does not start with tenant_id (add %s to uniqueAllow citing the ADR if it is global by design)", u.table, u.index, strings.Join(u.cols, ", "), key))
	}
	return out, nil
}

type fkInfo struct {
	table, name, refTable string
	cols, refCols         []string
	indexed               bool
}

func loadFKs(ctx context.Context, q queryer) ([]fkInfo, error) {
	return scanAll(ctx, q, `
		SELECT cn.nspname || '.' || cc.relname, con.conname, rn.nspname || '.' || rc.relname,
		       ARRAY(SELECT a.attname::text FROM unnest(con.conkey) WITH ORDINALITY k(attnum, ord)
		               JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum ORDER BY k.ord),
		       ARRAY(SELECT a.attname::text FROM unnest(con.confkey) WITH ORDINALITY k(attnum, ord)
		               JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.attnum ORDER BY k.ord),
		       EXISTS (SELECT 1 FROM pg_index i WHERE i.indrelid = con.conrelid AND i.indisvalid
		               AND (i.indkey::int2[])[0:cardinality(con.conkey) - 1] = con.conkey)
		  FROM pg_constraint con
		  JOIN pg_class cc ON cc.oid = con.conrelid JOIN pg_namespace cn ON cn.oid = cc.relnamespace
		  JOIN pg_class rc ON rc.oid = con.confrelid JOIN pg_namespace rn ON rn.oid = rc.relnamespace
		  JOIN pg_namespace n ON n.oid = con.connamespace
		 WHERE con.contype = 'f' AND con.conparentid = 0 AND `+userSchemas+` ORDER BY 1, 2`, func(r pgx.Rows) (fkInfo, error) {
		var f fkInfo
		return f, r.Scan(&f.table, &f.name, &f.refTable, &f.cols, &f.refCols, &f.indexed)
	})
}

// Toda FK tiene un índice que empieza por sus columnas.
func ruleFKIndexed(ctx context.Context, q queryer) ([]string, error) {
	fks, err := loadFKs(ctx, q)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, f := range fks {
		if !f.indexed {
			out = append(out, fmt.Sprintf("%s: foreign key %s (%s) has no index that starts with its columns", f.table, f.name, strings.Join(f.cols, ", ")))
		}
	}
	return out, nil
}

// Toda FK de una tabla de tenant hacia otra tabla de tenant lleva tenant_id en
// ambos lados, en la misma posición (hacia la raíz, su id).
func ruleFKTenant(ctx context.Context, q queryer) ([]string, error) {
	tables, err := loadTables(ctx, q)
	if err != nil {
		return nil, err
	}
	fks, err := loadFKs(ctx, q)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, f := range fks {
		child, parent := tables[f.table], tables[f.refTable]
		if !child.hasID || parent.tenantCol() == "" {
			continue // un lado es global: no hay mezcla de tenants posible
		}
		if _, ok := fkAllow[f.table+"."+f.name]; ok {
			continue
		}
		pos := -1
		for i, c := range f.cols {
			if c == "tenant_id" {
				pos = i
			}
		}
		if pos < 0 || f.refCols[pos] != parent.tenantCol() {
			out = append(out, fmt.Sprintf("%s: foreign key %s (%s) -> %s (%s) between tenant tables must include tenant_id on both sides", f.table, f.name, strings.Join(f.cols, ", "), f.refTable, strings.Join(f.refCols, ", ")))
		}
	}
	return out, nil
}

// Toda función SECURITY DEFINER: dueño wanka_definer, search_path fijo y sin EXECUTE de PUBLIC.
func ruleDefiners(ctx context.Context, q queryer) ([]string, error) {
	type fn struct {
		sig, owner string
		config     []string
		public     bool
	}
	fns, err := scanAll(ctx, q, `
		SELECT p.oid::regprocedure::text, pg_get_userbyid(p.proowner), COALESCE(p.proconfig, '{}'), has_function_privilege('public', p.oid, 'EXECUTE')
		  FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		 WHERE p.prosecdef AND `+userSchemas+` ORDER BY 1`, func(r pgx.Rows) (fn, error) {
		var f fn
		return f, r.Scan(&f.sig, &f.owner, &f.config, &f.public)
	})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, f := range fns {
		if f.owner != "wanka_definer" && (f.owner != "wanka_migrator" || definerOwnerAllow[f.sig] == "") {
			out = append(out, fmt.Sprintf("%s: SECURITY DEFINER owned by %s, want wanka_definer", f.sig, f.owner))
		}
		if len(f.config) != 1 || f.config[0] != "search_path=pg_catalog, pg_temp" {
			out = append(out, fmt.Sprintf("%s: SECURITY DEFINER proconfig = %v, want exactly search_path=pg_catalog, pg_temp", f.sig, f.config))
		}
		if f.public {
			out = append(out, fmt.Sprintf("%s: PUBLIC has EXECUTE on a SECURITY DEFINER function", f.sig))
		}
	}
	return out, nil
}

// Ningún rol wanka_* tiene BYPASSRLS ni es superusuario; los que no son el migrador no son dueños de tablas.
func ruleRoles(ctx context.Context, q queryer) ([]string, error) {
	type role struct {
		name          string
		super, bypass bool
		owned         []string
	}
	roles, err := scanAll(ctx, q, `
		SELECT r.rolname, r.rolsuper, r.rolbypassrls,
		       ARRAY(SELECT n.nspname || '.' || c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		              WHERE c.relowner = r.oid AND c.relkind IN ('r', 'p', 'v', 'm', 'f') AND `+userSchemas+` ORDER BY 1)
		  FROM pg_roles r WHERE r.rolname LIKE 'wanka\_%' ORDER BY 1`, func(r pgx.Rows) (role, error) {
		var x role
		return x, r.Scan(&x.name, &x.super, &x.bypass, &x.owned)
	})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, r := range roles {
		if r.super || r.bypass {
			out = append(out, fmt.Sprintf("role %s: rolsuper=%v rolbypassrls=%v, both must be false", r.name, r.super, r.bypass))
		}
		if r.name != "wanka_migrator" && len(r.owned) > 0 {
			out = append(out, fmt.Sprintf("role %s owns tables %v, only wanka_migrator may", r.name, r.owned))
		}
	}
	return out, nil
}

type catalogRule struct {
	name  string
	check func(context.Context, queryer) ([]string, error)
}

var catalogRules = []catalogRule{
	{"tenant-tables-have-force-rls-and-policy", ruleTenantRLS},
	{"partitions-of-tenant-tables", rulePartitions},
	{"extra-policies-are-whitelisted", rulePolicies},
	{"unique-and-pk-start-with-tenant-id", ruleUniques},
	{"policies-wrap-current-setting-in-nullif", rulePoliciesNullif},
	{"foreign-keys-are-indexed", ruleFKIndexed},
	{"tenant-foreign-keys-carry-tenant-id", ruleFKTenant},
	{"security-definer-functions", ruleDefiners},
	{"application-roles", ruleRoles},
}

func ruleByName(t *testing.T, name string) catalogRule {
	t.Helper()
	for _, r := range catalogRules {
		if r.name == name {
			return r
		}
	}
	t.Fatalf("no catalog rule %q", name)
	return catalogRule{}
}

// problemsOfExceptions revisa las listas blancas: cada entrada cita un ADR y
// (salvo las de política, que el ADR fija de antemano) apunta a un objeto que existe.
func problemsOfExceptions(ctx context.Context, q queryer, adrs map[string]bool, noRLS, uniq, fks, definers map[string]string, pols map[string]policyRule) ([]string, error) {
	var out []string
	check := func(kind, key, reason string) {
		if !adrRef.MatchString(reason) {
			out = append(out, fmt.Sprintf("%s %s: the reason must cite an ADR (\"ADR NNNN\"): %q", kind, key, reason))
		}
		for _, m := range adrRef.FindAllStringSubmatch(reason, -1) {
			if !adrs[m[1]] {
				out = append(out, fmt.Sprintf("%s %s: ADR %s does not exist in docs/architecture/adr: %q", kind, key, m[1], reason))
			}
		}
	}
	for k, v := range noRLS {
		check("noRLSTables", k, v)
	}
	for k, v := range uniq {
		check("uniqueAllow", k, v)
	}
	for k, v := range fks {
		check("fkAllow", k, v)
	}
	for k, v := range definers {
		check("definerOwnerAllow", k, v)
	}
	for k, v := range pols {
		check("policyAllow", k, v.reason)
	}
	tables, err := loadTables(ctx, q)
	if err != nil {
		return nil, err
	}
	for k := range noRLS {
		if !tables[k].hasID {
			out = append(out, fmt.Sprintf("noRLSTables %s: no such table with a tenant_id column; remove the stale entry", k))
		}
	}
	idx := map[string]bool{}
	if err := collectUniqueKeys(ctx, q, idx); err != nil {
		return nil, err
	}
	for k := range uniq {
		if !idx[k] {
			out = append(out, fmt.Sprintf("uniqueAllow %s: no such unique index; remove the stale entry", k))
		}
	}
	fkList, err := loadFKs(ctx, q)
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, f := range fkList {
		have[f.table+"."+f.name] = true
	}
	for k := range fks {
		if !have[k] {
			out = append(out, fmt.Sprintf("fkAllow %s: no such foreign key; remove the stale entry", k))
		}
	}
	defs, err := scanAll(ctx, q, `
		SELECT p.oid::regprocedure::text FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		 WHERE p.prosecdef AND `+userSchemas, func(r pgx.Rows) (string, error) {
		var s string
		return s, r.Scan(&s)
	})
	if err != nil {
		return nil, err
	}
	haveDef := map[string]bool{}
	for _, d := range defs {
		haveDef[d] = true
	}
	for k := range definers {
		if !haveDef[k] {
			out = append(out, fmt.Sprintf("definerOwnerAllow %s: no such SECURITY DEFINER function; remove the stale entry", k))
		}
	}
	sort.Strings(out)
	return out, nil
}

func collectUniqueKeys(ctx context.Context, q queryer, into map[string]bool) error {
	keys, err := scanAll(ctx, q, `
		SELECT n.nspname || '.' || c.relname || '(' || array_to_string(ARRAY(
		         SELECT CASE WHEN k.attnum = 0 THEN '<expr>' ELSE a.attname::text END
		           FROM unnest(i.indkey::int2[]) WITH ORDINALITY k(attnum, ord)
		           LEFT JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
		          WHERE k.ord <= i.indnkeyatts ORDER BY k.ord), ',') || ')'
		  FROM pg_index i JOIN pg_class c ON c.oid = i.indrelid JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE i.indisunique AND `+userSchemas, func(r pgx.Rows) (string, error) {
		var s string
		return s, r.Scan(&s)
	})
	for _, k := range keys {
		into[k] = true
	}
	return err
}

func runRules(t *testing.T, d *dbtest.DB) {
	t.Helper()
	ctx := context.Background()
	admin := d.Admin(ctx, t)
	for _, r := range catalogRules {
		t.Run(r.name, func(t *testing.T) {
			got, err := r.check(ctx, admin)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range got {
				t.Error(v)
			}
		})
	}
	t.Run("the whitelists cite an ADR and point at live objects", func(t *testing.T) {
		got, err := problemsOfExceptions(ctx, admin, knownADRs(t), noRLSTables, uniqueAllow, fkAllow, definerOwnerAllow, policyAllow)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range got {
			t.Error(v)
		}
	})
}

// Las reglas pasan sobre todo lo que hay migrado: las migraciones de producción
// y, aparte, con las tablas de prueba (rlstest.items) encima.
func TestCatalogRules(t *testing.T) {
	t.Run("production migrations", func(t *testing.T) { runRules(t, migrated(t)) })
	t.Run("with the rlstest tables", func(t *testing.T) {
		d := dbtest.New(t)
		d.MigrateWith(context.Background(), t, fstest.MapFS{"99990101000000_tenant_probe.sql": {Data: tenantProbeSQL}})
		runRules(t, d)
	})
}

// Las reglas no son vacías: ven las tablas, políticas, índices, FK, funciones y roles del kernel.
func TestCatalogRulesSeeTheKernel(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	admin := d.Admin(ctx, t)
	tables, err := loadTables(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"org.tenants", "iam.roles", "iam.memberships"} {
		if tables[want].tenantCol() == "" || !tables[want].force {
			t.Errorf("%s is not seen as a tenant table with FORCE RLS: %+v", want, tables[want])
		}
	}
	if _, ok := noRLSTables["iam.sessions"]; !ok || !tables["iam.sessions"].hasID || tables["iam.sessions"].rls {
		t.Errorf("iam.sessions must be a whitelisted table with tenant_id and no RLS: %+v", tables["iam.sessions"])
	}
	if tables["iam.users"].rls {
		t.Error("iam.users must not have RLS")
	}
	pols, err := loadPolicies(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	if len(pols) < 3 {
		t.Errorf("policies seen = %d, want at least tenant_isolation on org.tenants, iam.roles and iam.memberships", len(pols))
	}
	fks, err := loadFKs(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range fks {
		seen[f.table+">"+f.refTable] = true
	}
	for _, want := range []string{"iam.roles>org.tenants", "iam.memberships>org.tenants", "iam.memberships>iam.users", "iam.memberships>iam.roles", "iam.sessions>iam.users"} {
		if !seen[want] {
			t.Errorf("foreign key %s not seen", want)
		}
	}
	keys := map[string]bool{}
	if err := collectUniqueKeys(ctx, admin, keys); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"iam.memberships(tenant_id,user_id)", "iam.sessions(access_hash)", "iam.sessions(refresh_hash)", "iam.roles(tenant_id,name)"} {
		if !keys[want] {
			t.Errorf("unique index %s not seen", want)
		}
	}
	if n := queryOne[int](t, admin, `SELECT count(*) FROM pg_proc WHERE prosecdef AND pronamespace = 'iam'::regnamespace`); n < 1 {
		t.Error("no SECURITY DEFINER function seen in iam")
	}
	if n := queryOne[int](t, admin, `SELECT count(*) FROM pg_roles WHERE rolname LIKE 'wanka\_%'`); n != 6 {
		t.Errorf("wanka_* roles seen = %d, want 6", n)
	}
}

func TestCatalogWhitelistProblemsAreDetected(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	admin := d.Admin(ctx, t)
	got, err := problemsOfExceptions(ctx, admin, knownADRs(t),
		map[string]string{"iam.ghost": "ADR 0001: gone", "iam.sessions": "no reason", "iam.users": "ADR 9999: invented"},
		map[string]string{"iam.sessions(nope)": "ADR 0004: gone"},
		map[string]string{"iam.sessions.nope_fk": "ADR 0001: gone"},
		map[string]string{"audit.ghost(integer)": "ADR 9998: gone", "audit.ensure_partitions(date,integer)": "no reason"},
		map[string]policyRule{"iam.x:y": {reason: "because"}})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"noRLSTables iam.ghost: no such table",
		"noRLSTables iam.sessions: the reason must cite an ADR",
		"uniqueAllow iam.sessions(nope): no such unique index",
		"fkAllow iam.sessions.nope_fk: no such foreign key",
		"definerOwnerAllow audit.ghost(integer): no such SECURITY DEFINER function",
		"definerOwnerAllow audit.ensure_partitions(date,integer): the reason must cite an ADR",
		"definerOwnerAllow audit.ghost(integer): ADR 9998 does not exist",
		"policyAllow iam.x:y: the reason must cite an ADR",
		"noRLSTables iam.users: ADR 9999 does not exist",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing problem %q in:\n%s", want, joined)
		}
	}
}

// goodTenantTable es una tabla de tenant que cumple todas las reglas.
func goodTenantTable(name string) string {
	return fmt.Sprintf(`
CREATE TABLE mut.%[1]s (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL REFERENCES org.tenants,
  name text NOT NULL,
  UNIQUE (tenant_id, name),
  UNIQUE (tenant_id, id)
);
ALTER TABLE mut.%[1]s ENABLE ROW LEVEL SECURITY;
ALTER TABLE mut.%[1]s FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON mut.%[1]s
  USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());
`, name)
}

// definerFn crea mut.<name>() SECURITY DEFINER; extra va entre la firma y el cuerpo.
func definerFn(name, config string, owned, revokePublic bool) string {
	s := fmt.Sprintf(`
-- +goose StatementBegin
CREATE FUNCTION mut.%[1]s() RETURNS int LANGUAGE sql SECURITY DEFINER %[2]s AS $$ SELECT 1 $$;
-- +goose StatementEnd
`, name, config)
	if owned {
		s += fmt.Sprintf("GRANT CREATE ON SCHEMA mut TO wanka_definer;\nALTER FUNCTION mut.%s() OWNER TO wanka_definer;\nREVOKE CREATE ON SCHEMA mut FROM wanka_definer;\n", name)
	}
	if revokePublic {
		s += fmt.Sprintf("REVOKE ALL ON FUNCTION mut.%s() FROM PUBLIC;\n", name)
	}
	return s
}

// partitionedParent crea una tabla particionada de tenant que cumple todas las reglas.
func partitionedParent(name string) string {
	return fmt.Sprintf(`
CREATE TABLE mut.%[1]s (
  id uuid NOT NULL,
  tenant_id uuid NOT NULL REFERENCES org.tenants,
  name text NOT NULL,
  PRIMARY KEY (tenant_id, id)
) PARTITION BY HASH (tenant_id);
ALTER TABLE mut.%[1]s ENABLE ROW LEVEL SECURITY;
ALTER TABLE mut.%[1]s FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON mut.%[1]s
  USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());
`, name)
}

// partition crea la única partición (hash módulo 1) de mut.pt; isolated le da su propia RLS completa.
func partition(name string, isolated bool) string {
	s := fmt.Sprintf("CREATE TABLE mut.%s PARTITION OF mut.pt FOR VALUES WITH (MODULUS 1, REMAINDER 0);\n", name)
	if isolated {
		s += fmt.Sprintf(`ALTER TABLE mut.%[1]s ENABLE ROW LEVEL SECURITY;
ALTER TABLE mut.%[1]s FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON mut.%[1]s
  USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());
`, name)
	}
	return s
}

const goodSearchPath = "SET search_path = pg_catalog, pg_temp"

// Cada mutación viola una regla; la regla debe nombrar el objeto. Es la prueba de
// que el test de verdad se pone en rojo si alguien rompe la línea clave.
func TestCatalogRulesCatchMutations(t *testing.T) {
	cases := []struct {
		name      string
		rule      string
		up        string   // migración de prueba, sección Up (corre como wanka_migrator)
		admin     string   // SQL como superusuario tras migrar
		exclusive bool     // cambia roles del clúster
		cleanup   string   // SQL como superusuario al terminar
		want      []string // fragmentos que la salida de la regla debe contener
	}{
		// (a) RLS de las tablas de tenant.
		{name: "tenant table without FORCE", rule: "tenant-tables-have-force-rls-and-policy",
			up: goodTenantTable("a") + "ALTER TABLE mut.a NO FORCE ROW LEVEL SECURITY;", want: []string{"mut.a: relrowsecurity=true relforcerowsecurity=false"}},
		{name: "tenant table with RLS disabled", rule: "tenant-tables-have-force-rls-and-policy",
			up: goodTenantTable("a") + "ALTER TABLE mut.a DISABLE ROW LEVEL SECURITY;", want: []string{"mut.a: relrowsecurity=false"}},
		{name: "tenant table with tenant_id and no RLS outside the whitelist", rule: "tenant-tables-have-force-rls-and-policy",
			up:   "CREATE TABLE mut.b (id uuid PRIMARY KEY, tenant_id uuid NOT NULL REFERENCES org.tenants, UNIQUE (tenant_id, id));",
			want: []string{"mut.b: relrowsecurity=false", "mut.b: no tenant_isolation policy"}},
		{name: "tenant table without tenant_isolation", rule: "tenant-tables-have-force-rls-and-policy",
			up: goodTenantTable("a") + "DROP POLICY tenant_isolation ON mut.a;", want: []string{"mut.a: no tenant_isolation policy"}},
		{name: "tenant_isolation without WITH CHECK", rule: "tenant-tables-have-force-rls-and-policy",
			up:   goodTenantTable("a") + "DROP POLICY tenant_isolation ON mut.a; CREATE POLICY tenant_isolation ON mut.a USING (tenant_id = app.current_tenant());",
			want: []string{"mut.a: tenant_isolation has no WITH CHECK"}},
		{name: "tenant_isolation that does not use app.current_tenant", rule: "tenant-tables-have-force-rls-and-policy",
			up:   goodTenantTable("a") + "DROP POLICY tenant_isolation ON mut.a; CREATE POLICY tenant_isolation ON mut.a USING (true) WITH CHECK (true);",
			want: []string{"mut.a: tenant_isolation USING does not isolate"}},
		{name: "org.tenants loses FORCE", rule: "tenant-tables-have-force-rls-and-policy",
			up: "ALTER TABLE org.tenants NO FORCE ROW LEVEL SECURITY;", want: []string{"org.tenants: relrowsecurity=true relforcerowsecurity=false"}},

		// (a) Políticas adicionales.
		{name: "policy outside the whitelist", rule: "extra-policies-are-whitelisted",
			up: goodTenantTable("a") + "CREATE POLICY leak ON mut.a USING (true);", want: []string{"mut.a: policy leak is not in the ADR 0001 whitelist"}},
		{name: "whitelisted name on another table", rule: "extra-policies-are-whitelisted",
			up:   goodTenantTable("a") + "CREATE POLICY member_read ON mut.a FOR SELECT USING (app.current_tenant() IS NULL);",
			want: []string{"mut.a: policy member_read is not in the ADR 0001 whitelist"}},
		{name: "own_memberships without the user branch guard", rule: "extra-policies-are-whitelisted",
			up:   "DROP POLICY IF EXISTS own_memberships ON iam.memberships; CREATE POLICY own_memberships ON iam.memberships FOR SELECT USING (user_id = app.current_user_id());",
			want: []string{"iam.memberships: policy own_memberships has no app.current_tenant() IS NULL"}},
		{name: "own_memberships that is not FOR SELECT", rule: "extra-policies-are-whitelisted",
			up:   "DROP POLICY IF EXISTS own_memberships ON iam.memberships; CREATE POLICY own_memberships ON iam.memberships FOR ALL USING (app.current_tenant() IS NULL AND user_id = app.current_user_id());",
			want: []string{"iam.memberships: policy own_memberships is FOR ALL"}},
		{name: "platform_rw open to PUBLIC", rule: "extra-policies-are-whitelisted",
			up:   "DROP POLICY IF EXISTS platform_rw ON org.tenants; CREATE POLICY platform_rw ON org.tenants USING (true);",
			want: []string{"org.tenants: policy platform_rw is TO [public], want [wanka_platform]"}},
		{name: "purge_expired with a wider predicate", rule: "extra-policies-are-whitelisted",
			up: `DROP POLICY IF EXISTS purge_expired ON ops.idempotency_keys;
			     CREATE POLICY purge_expired ON ops.idempotency_keys FOR DELETE TO wanka_worker
			       USING (app.current_tenant() IS NULL AND created_at < now() - interval '1 hour');`,
			want: []string{"ops.idempotency_keys: policy purge_expired does not match the canonical expression"}},
		{name: "tenant_isolation on a table that is not a tenant table", rule: "extra-policies-are-whitelisted",
			up: `CREATE TABLE mut.g (id uuid PRIMARY KEY); ALTER TABLE mut.g ENABLE ROW LEVEL SECURITY;
			     CREATE POLICY tenant_isolation ON mut.g USING (true);`,
			want: []string{"mut.g: tenant_isolation on a table that is not a tenant table"}},

		// (a) Forma exacta de las políticas (H2) y filtro de estado (H3).
		{name: "tenant_isolation that fails open without a tenant", rule: "tenant-tables-have-force-rls-and-policy",
			up: goodTenantTable("a") + `DROP POLICY tenant_isolation ON mut.a;
			   CREATE POLICY tenant_isolation ON mut.a
			     USING (tenant_id = app.current_tenant() OR app.current_tenant() IS NULL) WITH CHECK (tenant_id = app.current_tenant());`,
			want: []string{"mut.a: tenant_isolation USING does not isolate by exactly (tenant_id = app.current_tenant())"}},
		{name: "tenant_isolation with an inverted WITH CHECK", rule: "tenant-tables-have-force-rls-and-policy",
			up: goodTenantTable("a") + `DROP POLICY tenant_isolation ON mut.a;
			   CREATE POLICY tenant_isolation ON mut.a USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id <> app.current_tenant());`,
			want: []string{"mut.a: tenant_isolation WITH CHECK is"}},
		{name: "own_memberships that opens with OR true", rule: "extra-policies-are-whitelisted",
			up: `DROP POLICY IF EXISTS own_memberships ON iam.memberships;
			   CREATE POLICY own_memberships ON iam.memberships FOR SELECT
			     USING ((app.current_tenant() IS NULL AND user_id = app.current_user_id() AND status IN ('invitado','activo')) OR true);`,
			want: []string{"iam.memberships: policy own_memberships does not match the canonical expression"}},
		{name: "own_memberships without the member state filter", rule: "extra-policies-are-whitelisted",
			up: `DROP POLICY IF EXISTS own_memberships ON iam.memberships;
			   CREATE POLICY own_memberships ON iam.memberships FOR SELECT USING (app.current_tenant() IS NULL AND user_id = app.current_user_id());`,
			want: []string{"iam.memberships: policy own_memberships does not match the canonical expression"}},
		{name: "member_read of org.tenants that lets a suspended member read", rule: "extra-policies-are-whitelisted",
			up: `DROP POLICY IF EXISTS member_read ON org.tenants;
			   CREATE POLICY member_read ON org.tenants FOR SELECT USING (id = app.current_tenant()
			     OR (app.current_tenant() IS NULL AND id IN (SELECT tenant_id FROM iam.memberships WHERE user_id = app.current_user_id())));`,
			want: []string{"org.tenants: policy member_read does not match the canonical expression"}},
		{name: "own_memberships of membership_sites without the state filter", rule: "extra-policies-are-whitelisted",
			up: `DROP POLICY own_memberships ON iam.membership_sites;
			   CREATE POLICY own_memberships ON iam.membership_sites FOR SELECT USING (app.current_tenant() IS NULL AND membership_id IS NOT NULL);`,
			want: []string{`iam.membership_sites: policy own_memberships must contain "status"`}},

		// Particiones (H1): una consulta directa a la partición no pasa por la política de la madre.
		{name: "partition without RLS granted to wanka_app", rule: "partitions-of-tenant-tables",
			up:   partitionedParent("pt") + partition("pt_1", false) + "GRANT SELECT ON mut.pt_1 TO wanka_app;",
			want: []string{"partition mut.pt_1 of mut.pt is granted to [wanka_app]"}},
		{name: "partition granted by column only", rule: "partitions-of-tenant-tables",
			up:   partitionedParent("pt") + partition("pt_1", false) + "GRANT SELECT (name) ON mut.pt_1 TO wanka_worker;",
			want: []string{"partition mut.pt_1 of mut.pt is granted to [wanka_worker]"}},
		{name: "partition with RLS but without FORCE and granted", rule: "partitions-of-tenant-tables",
			up:   partitionedParent("pt") + partition("pt_1", true) + "ALTER TABLE mut.pt_1 NO FORCE ROW LEVEL SECURITY; GRANT SELECT ON mut.pt_1 TO wanka_app;",
			want: []string{"partition mut.pt_1 of mut.pt is granted to [wanka_app]", "relforcerowsecurity=false"}},

		// (c) NULLIF.
		{name: "current_setting without NULLIF", rule: "policies-wrap-current-setting-in-nullif",
			up: goodTenantTable("a") + `DROP POLICY tenant_isolation ON mut.a;
			   CREATE POLICY tenant_isolation ON mut.a
			     USING (tenant_id = app.current_tenant() OR tenant_id = current_setting('app.tenant_id', true)::uuid)
			     WITH CHECK (tenant_id = app.current_tenant());`,
			want: []string{"mut.a: policy tenant_isolation uses current_setting without NULLIF"}},

		// (b) Únicos.
		{name: "UNIQUE without tenant_id", rule: "unique-and-pk-start-with-tenant-id",
			up:   goodTenantTable("a") + "ALTER TABLE mut.a ADD COLUMN code text, ADD CONSTRAINT a_code_key UNIQUE (code);",
			want: []string{"mut.a: unique index a_code_key on (code)"}},
		{name: "unique index with tenant_id in second place", rule: "unique-and-pk-start-with-tenant-id",
			up:   goodTenantTable("a") + "CREATE UNIQUE INDEX a_name_tenant_idx ON mut.a (name, tenant_id);",
			want: []string{"mut.a: unique index a_name_tenant_idx on (name, tenant_id)"}},
		{name: "natural primary key without tenant_id", rule: "unique-and-pk-start-with-tenant-id",
			up: `CREATE TABLE mut.n (code text PRIMARY KEY, tenant_id uuid NOT NULL REFERENCES org.tenants);
			     CREATE INDEX n_tenant_idx ON mut.n (tenant_id);
			     ALTER TABLE mut.n ENABLE ROW LEVEL SECURITY; ALTER TABLE mut.n FORCE ROW LEVEL SECURITY;
			     CREATE POLICY tenant_isolation ON mut.n USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());`,
			want: []string{"mut.n: unique index n_pkey on (code)"}},
		{name: "expression unique index", rule: "unique-and-pk-start-with-tenant-id",
			up:   goodTenantTable("a") + "CREATE UNIQUE INDEX a_lower_idx ON mut.a (lower(name));",
			want: []string{"mut.a: unique index a_lower_idx on (<expr>)"}},

		// Claves foráneas.
		{name: "foreign key without index", rule: "foreign-keys-are-indexed",
			up:   goodTenantTable("a") + "ALTER TABLE mut.a ADD COLUMN owner_id uuid REFERENCES iam.users;",
			want: []string{"mut.a: foreign key a_owner_id_fkey (owner_id) has no index"}},
		{name: "composite foreign key whose index starts with other columns", rule: "foreign-keys-are-indexed",
			up: goodTenantTable("a") + goodTenantTable("c") + `ALTER TABLE mut.c ADD COLUMN ref uuid,
			     ADD CONSTRAINT c_ref_fk FOREIGN KEY (tenant_id, ref) REFERENCES mut.a (tenant_id, id);`,
			want: []string{"mut.c: foreign key c_ref_fk (tenant_id, ref) has no index"}},
		{name: "tenant foreign key without tenant_id", rule: "tenant-foreign-keys-carry-tenant-id",
			up: goodTenantTable("a") + goodTenantTable("c") + `ALTER TABLE mut.c ADD COLUMN ref uuid REFERENCES mut.a (id);
			     CREATE INDEX c_ref_idx ON mut.c (ref);`,
			want: []string{"mut.c: foreign key c_ref_fkey (ref) -> mut.a (id) between tenant tables must include tenant_id"}},
		{name: "foreign key whose tenant_id points at another column", rule: "tenant-foreign-keys-carry-tenant-id",
			up: goodTenantTable("a") + goodTenantTable("c") + `ALTER TABLE mut.c ADD COLUMN ref uuid;
			     ALTER TABLE mut.a ADD CONSTRAINT a_id_tenant_key UNIQUE (id, tenant_id);
			     ALTER TABLE mut.c ADD CONSTRAINT c_ref_fk FOREIGN KEY (tenant_id, ref) REFERENCES mut.a (id, tenant_id);
			     CREATE INDEX c_ref_idx ON mut.c (tenant_id, ref);`,
			want: []string{"mut.c: foreign key c_ref_fk (tenant_id, ref) -> mut.a (id, tenant_id)"}},

		// Funciones SECURITY DEFINER.
		{name: "definer owned by the migrator", rule: "security-definer-functions",
			up:   definerFn("f1", goodSearchPath, false, true),
			want: []string{"mut.f1(): SECURITY DEFINER owned by wanka_migrator"}},
		{name: "definer without search_path", rule: "security-definer-functions",
			up:   definerFn("f2", "", true, true),
			want: []string{"mut.f2(): SECURITY DEFINER proconfig = [], want exactly search_path"}},
		{name: "definer with a search_path that includes public", rule: "security-definer-functions",
			up:   definerFn("f3", "SET search_path = public", true, true),
			want: []string{"mut.f3(): SECURITY DEFINER proconfig = [search_path=public]"}},
		{name: "definer executable by PUBLIC", rule: "security-definer-functions",
			up:   definerFn("f4", goodSearchPath, true, false),
			want: []string{"mut.f4(): PUBLIC has EXECUTE"}},

		// Roles.
		{name: "role with BYPASSRLS", rule: "application-roles", exclusive: true,
			admin: "ALTER ROLE wanka_worker BYPASSRLS", cleanup: "ALTER ROLE wanka_worker NOBYPASSRLS",
			want: []string{"role wanka_worker: rolsuper=false rolbypassrls=true"}},
		{name: "role that is superuser", rule: "application-roles", exclusive: true,
			admin: "ALTER ROLE wanka_webhooks SUPERUSER", cleanup: "ALTER ROLE wanka_webhooks NOSUPERUSER",
			want: []string{"role wanka_webhooks: rolsuper=true"}},
		{name: "application role that owns a table", rule: "application-roles",
			up: goodTenantTable("a"), admin: "ALTER TABLE mut.a OWNER TO wanka_app",
			want: []string{"role wanka_app owns tables [mut.a]"}},
		{name: "definer that owns a table", rule: "application-roles",
			up: goodTenantTable("a"), admin: "ALTER TABLE mut.a OWNER TO wanka_definer",
			want: []string{"role wanka_definer owns tables [mut.a]"}},
	}
	for _, tc := range cases {
		t.Run(tc.rule+"/"+tc.name, func(t *testing.T) {
			ctx := context.Background()
			var d *dbtest.DB
			if tc.exclusive {
				d = dbtest.NewExclusive(t)
			} else {
				d = dbtest.New(t)
			}
			extra := fstest.MapFS{}
			if tc.up != "" {
				extra["99990101000099_catalog_mutation.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE SCHEMA IF NOT EXISTS mut;\n" + tc.up + "\n")}
			}
			d.MigrateWith(ctx, t, extra)
			admin := d.Admin(ctx, t)
			if tc.cleanup != "" {
				t.Cleanup(func() {
					if _, err := admin.Exec(context.Background(), tc.cleanup); err != nil {
						t.Errorf("restoring the cluster role: %v", err)
					}
				})
			}
			if tc.admin != "" {
				if _, err := admin.Exec(ctx, tc.admin); err != nil {
					t.Fatalf("mutating as superuser: %v", err)
				}
			}
			got, err := ruleByName(t, tc.rule).check(ctx, admin)
			if err != nil {
				t.Fatal(err)
			}
			text := strings.Join(got, "\n")
			for _, want := range tc.want {
				if !strings.Contains(text, want) {
					t.Errorf("rule %s did not report %q; it reported:\n%s", tc.rule, want, text)
				}
			}
		})
	}
}

// Una partición sin privilegios de ejecución, o con su propia RLS completa, no viola ninguna regla
// (ni rulePolicies se queja de su tenant_isolation).
func TestCatalogAcceptsPartitionsThatAreSafe(t *testing.T) {
	cases := map[string]string{
		"partition with no privileges for the runtime roles": partitionedParent("pt") + partition("pt_1", false) + "GRANT SELECT ON mut.pt TO wanka_app;",
		"partition with its own full RLS and a grant":        partitionedParent("pt") + partition("pt_1", true) + "GRANT SELECT, INSERT ON mut.pt, mut.pt_1 TO wanka_app;",
	}
	for name, up := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			d := dbtest.New(t)
			d.MigrateWith(ctx, t, fstest.MapFS{"99990101000099_catalog_mutation.sql": {Data: []byte("-- +goose Up\nCREATE SCHEMA mut;\n" + up + "\n")}})
			admin := d.Admin(ctx, t)
			tables, err := loadTables(ctx, admin)
			if err != nil {
				t.Fatal(err)
			}
			if tables["mut.pt_1"].parent != "mut.pt" {
				t.Fatalf("mut.pt_1 is not seen as a partition of mut.pt: %+v", tables["mut.pt_1"])
			}
			for _, r := range catalogRules {
				got, err := r.check(ctx, admin)
				if err != nil {
					t.Fatal(err)
				}
				for _, v := range got {
					t.Errorf("%s: %s", r.name, v)
				}
			}
		})
	}
}

// El SQL del ADR 0001 («Caminos que no tienen un tenant fijado», 1; #187), tal cual; el de own_memberships de
// iam.membership_sites lo escribe #232 porque el ADR no lo tiene.
const adrPoliciesSQL = `
-- Las tablas y las políticas ya vienen de la migración de #232 (identidad, F2); aquí se reescriben con el texto del ADR
-- para que la prueba compare las constantes contra el ADR y no contra la migración.
DROP POLICY IF EXISTS own_memberships ON iam.membership_sites;
DROP POLICY IF EXISTS member_read ON org.sites;
DROP POLICY IF EXISTS member_read ON org.cash_registers;
DROP POLICY IF EXISTS own_memberships ON iam.memberships;
CREATE POLICY own_memberships ON iam.memberships FOR SELECT TO wanka_app USING (app.current_tenant() IS NULL AND user_id = app.current_user_id() AND status IN ('invitado','activo'));
CREATE POLICY own_memberships ON iam.membership_sites FOR SELECT TO wanka_app USING (
  app.current_tenant() IS NULL AND membership_id IN (SELECT id FROM iam.memberships
            WHERE user_id = app.current_user_id() AND status IN ('invitado','activo')));
DROP POLICY IF EXISTS member_read ON org.tenants;
CREATE POLICY member_read ON org.tenants FOR SELECT TO wanka_app USING (id = app.current_tenant() OR (app.current_tenant() IS NULL AND id IN (SELECT tenant_id FROM iam.memberships WHERE user_id = app.current_user_id() AND status IN ('invitado','activo'))));
DROP POLICY IF EXISTS platform_rw ON org.tenants;
CREATE POLICY platform_rw ON org.tenants TO wanka_platform USING (true) WITH CHECK (true);
DROP POLICY IF EXISTS member_read ON iam.roles;
CREATE POLICY member_read ON iam.roles FOR SELECT TO wanka_app USING (
  tenant_id = app.current_tenant()
  OR (app.current_tenant() IS NULL AND id IN (SELECT role_id FROM iam.memberships
            WHERE user_id = app.current_user_id() AND status IN ('invitado','activo'))));
CREATE POLICY member_read ON org.sites FOR SELECT TO wanka_app USING (
  tenant_id = app.current_tenant()
  OR (app.current_tenant() IS NULL AND EXISTS (SELECT 1 FROM iam.memberships m
             WHERE m.user_id = app.current_user_id() AND m.status = 'activo' AND m.tenant_id = sites.tenant_id
               AND (m.all_sites OR EXISTS (SELECT 1 FROM iam.membership_sites ms
                                           WHERE ms.membership_id = m.id AND ms.site_id = sites.id)))));
CREATE POLICY member_read ON org.cash_registers FOR SELECT TO wanka_app USING (
  tenant_id = app.current_tenant()
  OR (app.current_tenant() IS NULL AND site_id IN (SELECT id FROM org.sites)));
`

func adrPoliciesViolations(t *testing.T) []string {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	d.MigrateWith(ctx, t, fstest.MapFS{"99990101000098_adr_policies.sql": {Data: []byte("-- +goose Up\n" + adrPoliciesSQL)}})
	admin := d.Admin(ctx, t)
	var out []string
	for _, r := range catalogRules {
		got, err := r.check(ctx, admin)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range got {
			out = append(out, r.name+": "+v)
		}
	}
	return out
}

// Caso positivo: las constantes de texto exacto son lo que PostgreSQL deparsa del SQL del ADR.
func TestCatalogAcceptsTheADRPoliciesVerbatim(t *testing.T) {
	for _, v := range adrPoliciesViolations(t) {
		t.Error(v)
	}
}

// Si una constante se desvía (aquí, un paréntesis menos), el caso positivo se pone en rojo y nombra la política.
func TestCatalogADRPoliciesFailWhenAConstantDrifts(t *testing.T) {
	for _, key := range []string{"iam.memberships:own_memberships", "org.sites:member_read"} {
		t.Run(key, func(t *testing.T) {
			orig := policyAllow[key]
			drifted := orig
			drifted.qual = strings.Replace(orig.qual, "(", "", 1)
			policyAllow[key] = drifted
			t.Cleanup(func() { policyAllow[key] = orig })
			got := strings.Join(adrPoliciesViolations(t), "\n")
			if !strings.Contains(got, "policy "+strings.SplitN(key, ":", 2)[1]+" does not match the canonical expression") {
				t.Errorf("a drifted constant for %s was not reported:\n%s", key, got)
			}
		})
	}
}
