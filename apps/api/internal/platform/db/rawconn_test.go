package db_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// forbiddenConn lista, por ruta de importación, el prefijo de los nombres que
// abren una conexión o un pool por cuenta propia. Solo internal/platform/db puede
// usarlos; los demás reciben un *db.Pool y pasan por InTenantTx.
var forbiddenConn = map[string]string{
	"github.com/jackc/pgx/v5":         "Connect",
	"github.com/jackc/pgx/v5/pgconn":  "Connect",
	"github.com/jackc/pgx/v5/pgxpool": "New",
	"github.com/jackc/pgx/v5/stdlib":  "Open",
}

// defaultName es el nombre con que se usa un paquete sin alias: el último
// elemento de la ruta, salvo el sufijo de versión mayor (v5 -> pgx).
func defaultName(importPath string) string {
	parts := strings.Split(importPath, "/")
	last := parts[len(parts)-1]
	if len(last) > 1 && last[0] == 'v' && strings.Trim(last[1:], "0123456789") == "" && len(parts) > 1 {
		return parts[len(parts)-2]
	}
	return last
}

// rawConnUses recorre los .go que no son de test bajo root, sin las carpetas
// skip (rutas relativas a root) ni testdata, y devuelve "archivo:línea uso" por
// cada referencia a una función prohibida, resolviendo el nombre local de la
// importación (alias incluidos). Un import con "." se marca entero, porque sus
// funciones se usarían sin calificar.
func rawConnUses(t *testing.T, root string, skip ...string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			for _, s := range skip {
				if rel == s {
					return filepath.SkipDir
				}
			}
			if d.Name() == "testdata" && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return err
		}
		local := map[string]string{} // nombre local -> ruta de importación vigilada
		for _, imp := range f.Imports {
			ip := strings.Trim(imp.Path.Value, `"`)
			if _, watched := forbiddenConn[ip]; !watched {
				continue
			}
			name := defaultName(ip)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			switch name {
			case "_":
			case ".":
				found = append(found, fset.Position(imp.Pos()).String()+" dot import of "+ip)
			default:
				local[name] = ip
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if ip, ok := local[id.Name]; ok && strings.HasPrefix(sel.Sel.Name, forbiddenConn[ip]) {
				found = append(found, fset.Position(sel.Pos()).String()+" "+defaultName(ip)+"."+sel.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(found)
	return found
}

// Nadie fuera de internal/platform/db abre conexiones o pools por su cuenta: es
// lo que go-arch-lint no ve, porque pgx y pgconn sí se pueden importar.
func TestNoRawConnectionCallsOutsideDBPackage(t *testing.T) {
	if got := rawConnUses(t, "../../..", "internal/platform/db"); len(got) != 0 {
		t.Fatalf("code outside internal/platform/db opens connections by itself:\n%s", strings.Join(got, "\n"))
	}
}

// El detector funciona: el fixture usa los nombres prohibidos con alias y el
// detector debe verlos todos; los usos permitidos (pgx.Tx, pgconn.PgError) no.
func TestRawConnDetectorSeesAliasedCalls(t *testing.T) {
	got := rawConnUses(t, "testdata/rawconn")
	var bad []string
	for _, g := range got {
		if strings.Contains(g, "bad_alias.go") {
			bad = append(bad, g)
		}
		if strings.Contains(g, "good.go") {
			t.Fatalf("an allowed use was flagged: %s", g)
		}
	}
	want := []string{"pgx.Connect", "pgx.ConnectConfig", "pgconn.Connect", "pgconn.ConnectConfig",
		"pgxpool.New", "pgxpool.NewWithConfig", "stdlib.OpenDB", "stdlib.OpenDBFromPool"}
	if len(bad) != len(want) {
		t.Fatalf("detected %d uses, want %d:\n%s", len(bad), len(want), strings.Join(bad, "\n"))
	}
	for _, w := range want {
		ok := false
		for _, b := range bad {
			if strings.HasSuffix(b, " "+w) {
				ok = true
			}
		}
		if !ok {
			t.Errorf("missed %s in:\n%s", w, strings.Join(bad, "\n"))
		}
	}
}
