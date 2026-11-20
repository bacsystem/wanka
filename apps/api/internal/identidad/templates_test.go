package identidad_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

// Las listas de abajo están escritas a mano desde la tabla «Plantillas de los
// roles base» de resources.md (decisión del dueño, 2026-11-19), sin leer la
// migración ni el parser: si la migración o el documento cambian un permiso, el
// test se pone en rojo en los dos sentidos.
var (
	accountantPerms = []string{
		"ventas.pos:read", "ventas.quotes:read", "compras.suppliers:read", "compras.orders:read", "compras.documents:read",
		"compras.documents:export", "facturacion.cpe:read", "facturacion.cpe:export", "facturacion.gre:read",
		"facturacion.credit_notes:create", "reportes.dashboard:read", "reportes.management:read", "reportes.management:export",
		"reportes.tax:read", "reportes.tax:export", "finanzas.cash:read", "finanzas.cash:read_all", "finanzas.cash:export",
		"finanzas.receivables:read", "finanzas.payables:read", "auditoria.events:read", "auditoria.events:export",
		"clientes.customers:read", "inventario.stock:read", "empresas.config:read", "catalogo.costs:read",
	}
	cashierPerms = []string{
		"ventas.pos:read", "ventas.pos:create", "ventas.discounts:create", "ventas.quotes:read", "clientes.customers:read",
		"clientes.customers:create", "catalogo.items:read", "catalogo.prices:read", "inventario.stock:read", "finanzas.cash:read",
		"finanzas.cash:create", "finanzas.cash:update", "facturacion.cpe:read", "facturacion.cpe:send",
		"agenda.appointments:read", "agenda.appointments:update", "restaurante.tables:read", "restaurante.tables:update",
		"restaurante.orders:read", "restaurante.orders:create", "restaurante.orders:update",
	}
	sellerPerms = []string{
		"ventas.pos:read", "ventas.pos:create", "ventas.discounts:create", "ventas.quotes:read", "ventas.quotes:create",
		"ventas.quotes:update", "ventas.quotes:send", "clientes.customers:read", "clientes.customers:create",
		"clientes.customers:update", "catalogo.items:read", "catalogo.prices:read", "inventario.stock:read",
		"facturacion.cpe:read", "facturacion.cpe:send", "finanzas.cash:read", "finanzas.cash:create", "finanzas.cash:update",
	}
	warehousePerms = []string{
		"inventario.warehouses:read", "inventario.stock:read", "inventario.stock:update", "inventario.transfers:read",
		"inventario.transfers:create", "inventario.transfers:update", "inventario.counts:read", "inventario.counts:create",
		"inventario.counts:update", "inventario.consignments:read", "inventario.supplier_returns:read",
		"inventario.supplier_returns:create", "catalogo.items:read", "compras.orders:read", "compras.documents:read",
		"facturacion.gre:read", "facturacion.gre:create",
	}
	// Lo que el administrador no recibe del catálogo (el resto sale de `*`).
	adminExceptions = []string{"identidad.support_access:approve", "empresas.einvoicing:update"}
)

type wantTemplate struct {
	code, name string
	mfa        bool
}

// wantTemplates son las seis plantillas, con nombre y 2FA, escritas a mano.
var wantTemplates = []wantTemplate{
	{"dueno", "Dueño", true},
	{"administrador", "Administrador", true},
	{"contador", "Contador", true},
	{"cajero", "Cajero", false},
	{"vendedor", "Vendedor", false},
	{"almacenero", "Almacenero", false},
}

// dbTemplates lee de la base migrada las plantillas y sus permisos ordenados.
func dbTemplates(ctx context.Context, t *testing.T, conn *pgx.Conn) (map[wantTemplate][]string, map[string][]string) {
	t.Helper()
	rows, err := conn.Query(ctx, `SELECT code, name, requires_mfa FROM iam.role_templates ORDER BY code`)
	if err != nil {
		t.Fatal(err)
	}
	meta := map[wantTemplate][]string{}
	for rows.Next() {
		var w wantTemplate
		if err := rows.Scan(&w.code, &w.name, &w.mfa); err != nil {
			t.Fatal(err)
		}
		meta[w] = nil
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	perms := map[string][]string{}
	prows, err := conn.Query(ctx, `SELECT template_code, permission_code FROM iam.role_template_permissions ORDER BY 1, 2`)
	if err != nil {
		t.Fatal(err)
	}
	for prows.Next() {
		var code, p string
		if err := prows.Scan(&code, &p); err != nil {
			t.Fatal(err)
		}
		perms[code] = append(perms[code], p)
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		t.Fatal(err)
	}
	return meta, perms
}

func migratedAdmin(t *testing.T) (*dbtest.DB, *pgx.Conn) {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	d.Migrate(ctx, t)
	return d, d.Admin(ctx, t)
}

func catalog() []string {
	var out []string
	for _, c := range permission.All() {
		out = append(out, string(c))
	}
	slices.Sort(out)
	return out
}

func without(in []string, drop ...string) []string {
	var out []string
	for _, c := range in {
		if !slices.Contains(drop, c) {
			out = append(out, c)
		}
	}
	return out
}

func diff(got, want []string) string {
	var extra, missing []string
	for _, c := range got {
		if !slices.Contains(want, c) {
			extra = append(extra, c)
		}
	}
	for _, c := range want {
		if !slices.Contains(got, c) {
			missing = append(missing, c)
		}
	}
	if len(extra) == 0 && len(missing) == 0 && len(got) != len(want) {
		return fmt.Sprintf("duplicates: got %d codes, want %d", len(got), len(want))
	}
	if len(extra) == 0 && len(missing) == 0 {
		return ""
	}
	return fmt.Sprintf("unexpected %v, missing %v", extra, missing)
}

// La migración siembra exactamente las seis plantillas con su nombre y su 2FA,
// y cada una con la lista escrita a mano (criterios de la siembra, dueno,
// administrador, cajero y 2FA).
func TestSeededTemplatesMatchTheHandWrittenMatrix(t *testing.T) {
	ctx := context.Background()
	_, admin := migratedAdmin(t)
	meta, perms := dbTemplates(ctx, t, admin)
	if len(meta) != len(wantTemplates) {
		t.Fatalf("templates = %d, want %d", len(meta), len(wantTemplates))
	}
	for _, w := range wantTemplates {
		if _, ok := meta[w]; !ok {
			t.Errorf("template %+v is not seeded exactly like that (name or requires_mfa differ)", w)
		}
	}
	all := catalog()
	want := map[string][]string{
		"dueno":         all,
		"administrador": without(all, adminExceptions...),
		"contador":      accountantPerms,
		"cajero":        cashierPerms,
		"vendedor":      sellerPerms,
		"almacenero":    warehousePerms,
	}
	for code, w := range want {
		if d := diff(perms[code], w); d != "" {
			t.Errorf("%s: %s", code, d)
		}
	}
	if len(perms) != len(want) {
		t.Errorf("permissions seeded for %d templates, want %d", len(perms), len(want))
	}
}

// Criterio: `dueno` tiene todos los códigos del catálogo; un permiso nuevo sin
// agregarlo a `dueno` rompe este test. Se compara contra el catálogo Go, que no
// sale de la migración.
func TestOwnerTemplateHasTheWholeCatalog(t *testing.T) {
	ctx := context.Background()
	_, admin := migratedAdmin(t)
	var inDB []string
	rows, err := admin.Query(ctx, `SELECT code FROM iam.permissions`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		inDB = append(inDB, c)
	}
	rows.Close()
	_, perms := dbTemplates(ctx, t, admin)
	if d := diff(perms["dueno"], catalog()); d != "" {
		t.Errorf("dueno vs the Go catalog: %s", d)
	}
	if d := diff(perms["dueno"], inDB); d != "" {
		t.Errorf("dueno vs iam.permissions: %s", d)
	}
	if !slices.Contains(perms["dueno"], "catalogo.costs:read") {
		t.Error("dueno lacks catalogo.costs:read")
	}
}

// El administrador no recibe lo que solo tiene el dueño, y recibe todo lo demás.
func TestAdministratorLacksOnlyTheTwoOwnerPermissions(t *testing.T) {
	ctx := context.Background()
	_, admin := migratedAdmin(t)
	_, perms := dbTemplates(ctx, t, admin)
	for _, c := range []string{"identidad.support_access:approve", "empresas.einvoicing:update"} {
		if slices.Contains(perms["administrador"], c) {
			t.Errorf("administrador has %s", c)
		}
	}
	if got, want := len(perms["administrador"]), len(catalog())-2; got != want {
		t.Errorf("administrador has %d permissions, want %d", got, want)
	}
}

// El cajero abre caja y hace arqueo, pero no aprueba caja ni anula comprobantes.
func TestCashierCashPermissions(t *testing.T) {
	ctx := context.Background()
	_, admin := migratedAdmin(t)
	_, perms := dbTemplates(ctx, t, admin)
	for _, c := range []string{"finanzas.cash:create", "finanzas.cash:update"} {
		if !slices.Contains(perms["cajero"], c) {
			t.Errorf("cajero lacks %s", c)
		}
	}
	for _, c := range []string{"finanzas.cash:approve", "facturacion.cpe:void"} {
		if slices.Contains(perms["cajero"], c) {
			t.Errorf("cajero has %s", c)
		}
	}
}

// La migración es idempotente: volver a correr su sección Up no duplica ni falla.
func TestTemplateMigrationIsIdempotent(t *testing.T) {
	ctx := context.Background()
	_, admin := migratedAdmin(t)
	before, bp := dbTemplates(ctx, t, admin)
	sqlText, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261120120000_identity_role_templates.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, string(sqlText)); err != nil {
		t.Fatalf("second run: %v", err)
	}
	after, ap := dbTemplates(ctx, t, admin)
	if len(before) != len(after) {
		t.Errorf("templates %d -> %d", len(before), len(after))
	}
	for code, p := range bp {
		if d := diff(ap[code], p); d != "" {
			t.Errorf("%s changed: %s", code, d)
		}
	}
}

// ---- paridad con la tabla de resources.md ----

var (
	tickRe   = regexp.MustCompile("`([^`]+)`")
	rowRe    = regexp.MustCompile("^\\| `([a-z_]+)` \\| ([^|]+) \\| (obligatoria|opcional) \\| (.+) \\| (.+) \\|$")
	headRe   = "## Plantillas de los roles base"
	nextHead = "\n## "
)

type docTemplate struct {
	name        string
	mfa         bool
	grants, not string
}

// docTemplates lee las columnas Código, Nombre, 2FA, Otorga y No otorga de la
// tabla de plantillas, y falla si una fila no tiene la forma esperada.
func docTemplates(t *testing.T) map[string]docTemplate {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "architecture", "api", "resources.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	i := strings.Index(doc, headRe)
	if i < 0 {
		t.Fatalf("heading %q not found", headRe)
	}
	sec := doc[i+len(headRe):]
	if j := strings.Index(sec, nextHead); j >= 0 {
		sec = sec[:j]
	}
	out := map[string]docTemplate{}
	for _, line := range strings.Split(sec, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		m := rowRe.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("template row does not parse: %s", line)
		}
		out[m[1]] = docTemplate{name: strings.TrimSpace(m[2]), mfa: m[3] == "obligatoria", grants: m[4], not: m[5]}
	}
	return out
}

// codes devuelve los códigos entre comillas inversas de una celda.
func codes(cell string) []string {
	var out []string
	for _, m := range tickRe.FindAllStringSubmatch(cell, -1) {
		out = append(out, m[1])
	}
	return out
}

// expand devuelve los permisos de la fila: `*` es todo el catálogo menos «No otorga».
func (d docTemplate) expand(t *testing.T) []string {
	t.Helper()
	g := codes(d.grants)
	if len(g) == 1 && g[0] == "*" {
		return without(catalog(), codes(d.not)...)
	}
	if strings.Contains(d.grants, "*") {
		t.Fatalf("a grant list mixes * with codes: %s", d.grants)
	}
	if strings.TrimSpace(d.not) != "—" {
		t.Fatalf("«No otorga» only qualifies a *, got %q next to a list", d.not)
	}
	return g
}

// Criterio de paridad: la tabla de resources.md y la siembra no difieren en un
// solo permiso, nombre ni valor de 2FA.
func TestSeededTemplatesMatchResourcesMd(t *testing.T) {
	ctx := context.Background()
	_, admin := migratedAdmin(t)
	meta, perms := dbTemplates(ctx, t, admin)
	doc := docTemplates(t)
	if len(doc) != len(wantTemplates) {
		t.Fatalf("resources.md lists %d templates, want %d", len(doc), len(wantTemplates))
	}
	for w := range meta {
		d, ok := doc[w.code]
		if !ok {
			t.Errorf("template %s is seeded but not in resources.md", w.code)
			continue
		}
		if d.name != w.name {
			t.Errorf("%s: name %q in resources.md, %q seeded", w.code, d.name, w.name)
		}
		if d.mfa != w.mfa {
			t.Errorf("%s: 2FA required = %v in resources.md, %v seeded", w.code, d.mfa, w.mfa)
		}
		if s := diff(perms[w.code], d.expand(t)); s != "" {
			t.Errorf("%s: seeded vs resources.md: %s", w.code, s)
		}
	}
}

// Los códigos de la tabla existen todos en el catálogo: una errata no pasa por `*`.
func TestDocumentTemplateCodesAreInTheCatalog(t *testing.T) {
	cat := catalog()
	for code, d := range docTemplates(t) {
		for _, c := range append(codes(d.grants), codes(d.not)...) {
			if c != "*" && !slices.Contains(cat, c) {
				t.Errorf("%s: %q is not in the catalog", code, c)
			}
		}
	}
}

// Solo el dueño tiene lo que un administrador no puede asignar: ninguna otra plantilla, hoy ni mañana, trae
// identidad.support_access:approve ni empresas.einvoicing:update (lista escrita a mano).
func TestOnlyTheOwnerTemplateHasTheTwoOwnerOnlyPermissions(t *testing.T) {
	ctx := context.Background()
	_, admin := migratedAdmin(t)
	meta, perms := dbTemplates(ctx, t, admin)
	ownerOnly := []string{"identidad.support_access:approve", "empresas.einvoicing:update"}
	checked := 0
	for w := range meta {
		if w.code == "dueno" {
			continue
		}
		checked++
		for _, c := range ownerOnly {
			if slices.Contains(perms[w.code], c) {
				t.Errorf("template %s has %s", w.code, c)
			}
		}
	}
	if checked != 5 {
		t.Errorf("checked %d templates besides dueno, want 5", checked)
	}
	for _, c := range ownerOnly {
		if !slices.Contains(perms["dueno"], c) {
			t.Errorf("dueno lacks %s", c)
		}
	}
}
