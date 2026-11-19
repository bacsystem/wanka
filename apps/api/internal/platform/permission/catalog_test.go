package permission_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
	"github.com/bacsystem/wanka/apps/api/migrations"
)

func TestParseCatalogTable(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr string
	}{
		{"expands slashes", "| m | `a.b:read/create` |", []string{"a.b:read", "a.b:create"}, ""},
		{"ignores prose and the format example outside rows", "Formato `modulo.recurso:accion`.\n| m | `a.b:read` |", []string{"a.b:read"}, ""},
		{"rejects an action outside the list", "| m | `a.b:read/delete` |", nil, `action "delete"`},
		{"rejects a duplicate", "| m | `a.b:read` |\n| n | `a.b:read/update` |", nil, "listed twice"},
		{"rejects a duplicate inside one cell", "| m | `a.b:read/read` |", nil, "listed twice"},
		{"rejects a token that is not a code", "| m | `a.b` |", nil, "not modulo.recurso:accion"},
		{"rejects an uppercase code", "| m | `A.b:read` |", nil, "not modulo.recurso:accion"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCatalogTable(tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || !slices.Equal(got, tt.want) {
				t.Fatalf("got %v, %v; want %v", got, err, tt.want)
			}
		})
	}
}

func TestReferencedCodes(t *testing.T) {
	codes, wild := referencedCodes("| `caja.operar` | #66 | `finanzas.cash:create` (abrir), `:update` y `a.b:read/void`, `x.y:*` |")
	if want := []string{"finanzas.cash:create", "finanzas.cash:update", "a.b:read", "a.b:void"}; !slices.Equal(codes, want) {
		t.Errorf("codes = %v, want %v", codes, want)
	}
	if want := []string{"x.y"}; !slices.Equal(wild, want) {
		t.Errorf("wildcards = %v, want %v", wild, want)
	}
}

// Criterio 2: resources.md y el código Go tienen los mismos códigos.
func TestCatalogMatchesDocument(t *testing.T) {
	doc, code := docCatalog(t), goCatalog()
	if m := missingFrom(doc, code); len(m) > 0 {
		t.Errorf("in resources.md but not in the Go catalog: %v", m)
	}
	if m := missingFrom(code, doc); len(m) > 0 {
		t.Errorf("in the Go catalog but not in resources.md: %v", m)
	}
}

func TestGoCatalogHasNoDuplicatesAndOnlyAllowedActions(t *testing.T) {
	seen := map[string]bool{}
	format := regexp.MustCompile(`^[a-z]+\.[a-z_]+:([a-z_]+)$`)
	for _, c := range goCatalog() {
		if seen[c] {
			t.Errorf("duplicate code %s", c)
		}
		seen[c] = true
		m := format.FindStringSubmatch(c)
		if m == nil {
			t.Errorf("%q is not modulo.recurso:accion", c)
			continue
		}
		if !slices.Contains(permission.Actions(), m[1]) {
			t.Errorf("%s: action %q is not allowed", c, m[1])
		}
	}
}

// Todas las constantes de catalog.go están en All(): si alguien agrega una y olvida la lista, el
// módulo que la use pasaría Validate con un código que el catálogo no reporta.
func TestConstantsAreListed(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "catalog.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var consts []string
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, s := range gd.Specs {
			vs := s.(*ast.ValueSpec)
			for _, v := range vs.Values {
				if lit, ok := v.(*ast.BasicLit); ok {
					consts = append(consts, strings.Trim(lit.Value, `"`))
				}
			}
		}
	}
	if len(consts) == 0 {
		t.Fatal("no constants found in catalog.go")
	}
	if m := missingFrom(consts, goCatalog()); len(m) > 0 {
		t.Errorf("constants missing from all: %v", m)
	}
	if m := missingFrom(goCatalog(), consts); len(m) > 0 {
		t.Errorf("entries of all without a constant: %v", m)
	}
}

// seedRe toma cada INSERT en iam.permissions de las migraciones; un código sembrado por cualquier
// migración (la actual o una futura) debe estar en resources.md.
var (
	insertRe  = regexp.MustCompile(`(?s)INSERT INTO iam\.permissions\b.*?;`)
	literalRe = regexp.MustCompile(`'([a-z]+\.[a-z_]+:[a-z_]+)'`)
)

// seededCodes devuelve los códigos que siembran los archivos de migración de fsys.
func seededCodes(t *testing.T, fsys fs.FS) []string {
	t.Helper()
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, name := range files {
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Fatal(err)
		}
		for _, stmt := range insertRe.FindAllString(string(b), -1) {
			for _, m := range literalRe.FindAllStringSubmatch(stmt, -1) {
				out = append(out, m[1])
			}
		}
	}
	return out
}

// Criterios 1, 2 y 5: la migración siembra exactamente el catálogo de resources.md, sin repetir, y
// una migración que agrega un código no listado en resources.md hace fallar este test (la CI).
func TestMigrationsSeedExactlyTheDocument(t *testing.T) {
	doc, seeded := docCatalog(t), seededCodes(t, migrations.FS)
	if m := missingFrom(doc, seeded); len(m) > 0 {
		t.Errorf("in resources.md but not seeded by a migration: %v", m)
	}
	if m := missingFrom(seeded, doc); len(m) > 0 {
		t.Errorf("seeded by a migration but not in resources.md (a new code needs its PR to resources.md first): %v", m)
	}
	seen := map[string]bool{}
	for _, c := range seeded {
		if seen[c] {
			t.Errorf("code %s is seeded twice", c)
		}
		seen[c] = true
	}
}
