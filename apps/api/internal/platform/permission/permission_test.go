package permission_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

type fakeModule []permission.Code

func (m fakeModule) Permissions() []permission.Code { return m }

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		codes   []permission.Code
		wantErr []string
	}{
		{"empty", nil, nil},
		{"known codes", []permission.Code{permission.ClientesCustomersRead, permission.VentasPosCreate}, nil},
		{"unknown code", []permission.Code{"clientes.customers:delete"}, []string{`unknown permission code "clientes.customers:delete"`}},
		{"unknown resource", []permission.Code{"clientes.nada:read"}, []string{"unknown permission code"}},
		{"empty code", []permission.Code{""}, []string{"unknown permission code"}},
		{"platform code in the tenant catalog", []permission.Code{"plataforma.tenants:read"}, []string{"unknown permission code"}},
		{"duplicate", []permission.Code{permission.VentasPosCreate, permission.VentasPosCreate}, []string{"declared twice"}},
		{"reports every problem", []permission.Code{"a.b:read", permission.VentasPosCreate, permission.VentasPosCreate}, []string{"unknown permission code", "declared twice"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := permission.Validate(tt.codes)
			if len(tt.wantErr) == 0 {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("err = nil, want an error")
			}
			for _, w := range tt.wantErr {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("err = %q, want it to contain %q", err, w)
				}
			}
		})
	}
}

// Criterio 3: el chequeo de arranque de un módulo rechaza un código inexistente y nombra al módulo.
func TestCheck(t *testing.T) {
	if err := permission.Check("clientes", fakeModule{permission.ClientesCustomersRead}); err != nil {
		t.Fatalf("valid module: %v", err)
	}
	err := permission.Check("clientes", fakeModule{permission.ClientesCustomersRead, "clientes.customers:borrar"})
	if err == nil || !strings.Contains(err.Error(), "module clientes") || !strings.Contains(err.Error(), "clientes.customers:borrar") {
		t.Fatalf("err = %v, want it to name the module and the code", err)
	}
}

func TestAllReturnsACopy(t *testing.T) {
	a := permission.All()
	a[0] = "x.y:read"
	if permission.All()[0] == "x.y:read" || !permission.All()[0].Valid() {
		t.Fatal("changing the result of All changed the catalog")
	}
	if len(permission.All()) < 100 {
		t.Fatalf("len(All) = %d, the catalog looks truncated", len(permission.All()))
	}
}

func TestActionsMatchTheDocument(t *testing.T) {
	doc := readRepoFile(t, "docs/architecture/api/resources.md")
	sec := section(t, doc, "## Catálogo de permisos (fuente de verdad)", "| Módulo |")
	for _, a := range permission.Actions() {
		if !strings.Contains(sec, "`"+a+"`") {
			t.Errorf("action %s is not named in the catalog's intro", a)
		}
	}
	if len(permission.Actions()) != 10 {
		t.Errorf("len(Actions) = %d, want 10", len(permission.Actions()))
	}
}

// Criterio 6 e invariante: todo código que resources.md usa en la correspondencia de historias a
// permisos (de donde salen las plantillas de rol) y que el ADR 0004 nombra (con comodín o sin él)
// existe en el catálogo. Las plantillas mismas las siembra identidad (#29); aquí solo se vigila
// que no apunten a un permiso que no existe.
func TestReferencedCodesExist(t *testing.T) {
	catalog := goCatalog()
	resources := map[string]bool{}
	for _, c := range catalog {
		resources[c[:strings.Index(c, ":")]] = true
	}

	doc := readRepoFile(t, "docs/architecture/api/resources.md")
	mapping := section(t, doc, "## Correspondencia: código de historia", "## Catálogo de permisos de plataforma")
	codes, _ := referencedCodes(mapping)
	if len(codes) < 50 {
		t.Fatalf("resources.md mapping: only %d codes found, the parser lost the tables", len(codes))
	}
	for _, c := range codes {
		if !slices.Contains(catalog, c) {
			t.Errorf("resources.md mapping uses %s, which is not in the catalog", c)
		}
	}

	adr := readRepoFile(t, "docs/architecture/adr/0004-in-house-auth.md")
	adrCodes, wildcards := referencedCodes(adr)
	if len(adrCodes)+len(wildcards) == 0 {
		t.Fatal("ADR 0004: no permission codes found")
	}
	for _, c := range adrCodes {
		if !slices.Contains(catalog, c) {
			t.Errorf("ADR 0004 uses %s, which is not in the catalog", c)
		}
	}
	for _, r := range wildcards {
		if !resources[r] {
			t.Errorf("ADR 0004 uses %s:*, but that resource is not in the catalog", r)
		}
	}
}
