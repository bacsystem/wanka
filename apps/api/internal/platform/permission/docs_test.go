package permission_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

// Raíz del repositorio vista desde este paquete (apps/api/internal/platform/permission).
const repoRoot = "../../../../.."

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, rel))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(b)
}

// section devuelve el texto desde el encabezado from hasta el encabezado to (excluido).
func section(t *testing.T, doc, from, to string) string {
	t.Helper()
	i := strings.Index(doc, from)
	if i < 0 {
		t.Fatalf("heading %q not found", from)
	}
	rest := doc[i:]
	j := strings.Index(rest, to)
	if j < 0 {
		t.Fatalf("heading %q not found after %q", to, from)
	}
	return rest[:j]
}

var (
	tokenRe     = regexp.MustCompile("`([^`]+)`")
	fullCodeRe  = regexp.MustCompile(`^([a-z]+\.[a-z_]+):([a-z_]+(?:/[a-z_]+)*)$`)
	shortCodeRe = regexp.MustCompile(`^:([a-z_]+(?:/[a-z_]+)*)$`)
	bareCodeRe  = regexp.MustCompile(`[a-z]+\.[a-z_]+:[a-z_]+`)
)

// parseCatalogTable lee la tabla del catálogo de resources.md: cada código entre comillas inversas
// de una fila debe ser modulo.recurso:accion[/accion...] con acciones permitidas y sin repetirse.
func parseCatalogTable(section string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if bare := bareCodeRe.FindString(tokenRe.ReplaceAllString(line, "")); bare != "" {
			return nil, fmt.Errorf("code %q is written without backticks", bare)
		}
		for _, m := range tokenRe.FindAllStringSubmatch(line, -1) {
			parts := fullCodeRe.FindStringSubmatch(m[1])
			if parts == nil {
				return nil, fmt.Errorf("token %q is not modulo.recurso:accion", m[1])
			}
			for _, act := range strings.Split(parts[2], "/") {
				if !slices.Contains(permission.Actions(), act) {
					return nil, fmt.Errorf("%s:%s: action %q is not in the catalog's list", parts[1], act, act)
				}
				code := parts[1] + ":" + act
				if seen[code] {
					return nil, fmt.Errorf("code %s is listed twice", code)
				}
				seen[code] = true
				out = append(out, code)
			}
		}
	}
	return out, nil
}

// referencedCodes lee los códigos que un texto usa entre comillas inversas: completos
// (modulo.recurso:accion), abreviados (`:update` hereda el recurso del código anterior de la
// misma línea, como en las tablas de correspondencia) y comodines (modulo.recurso:*). Lo que no
// lleva «:» (un código de historia como `caja.operar`) se ignora.
func referencedCodes(text string) (codes, wildcards []string) {
	for _, line := range strings.Split(text, "\n") {
		resource := ""
		for _, m := range tokenRe.FindAllStringSubmatch(line, -1) {
			tok := m[1]
			if tok == "modulo.recurso:accion" { // el ejemplo del formato
				continue
			}
			if res, ok := strings.CutSuffix(tok, ":*"); ok {
				wildcards = append(wildcards, res)
				continue
			}
			if parts := fullCodeRe.FindStringSubmatch(tok); parts != nil {
				resource = parts[1]
				for _, act := range strings.Split(parts[2], "/") {
					codes = append(codes, resource+":"+act)
				}
				continue
			}
			if parts := shortCodeRe.FindStringSubmatch(tok); parts != nil && resource != "" {
				for _, act := range strings.Split(parts[1], "/") {
					codes = append(codes, resource+":"+act)
				}
			}
		}
	}
	return codes, wildcards
}

// docCatalog es el catálogo de tenant de resources.md, leído del repositorio.
func docCatalog(t *testing.T) []string {
	t.Helper()
	doc := readRepoFile(t, "docs/architecture/api/resources.md")
	sec := section(t, doc, "## Catálogo de permisos (fuente de verdad)", "## Correspondencia: código de historia")
	codes, err := parseCatalogTable(sec)
	if err != nil {
		t.Fatalf("resources.md: %v", err)
	}
	if len(codes) == 0 {
		t.Fatal("resources.md: no permission codes found")
	}
	return codes
}

func goCatalog() []string {
	all := permission.All()
	out := make([]string, len(all))
	for i, c := range all {
		out[i] = string(c)
	}
	return out
}

// missingFrom devuelve los códigos de a que no están en b.
func missingFrom(a, b []string) []string {
	var out []string
	for _, c := range a {
		if !slices.Contains(b, c) {
			out = append(out, c)
		}
	}
	return out
}
