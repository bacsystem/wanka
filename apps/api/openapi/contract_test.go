package openapi_test

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/apigen"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

// Prueba de contrato del spec (ADR 0009, #166). No usa una librería YAML (no hay ninguna
// aprobada): las operaciones las lee de las tablas Operations que genera oapi-codegen y la
// raíz y los archivos de texto los recorre por líneas. Redocly (scripts/openapi-check.sh)
// valida la sintaxis y los $ref; esto valida las reglas del proyecto.

// opsByModule son las operaciones de los paquetes generados (apigen.Modules, el registro
// compartido con la prueba de paridad de cmd/wanka). Un archivo de paths con operaciones
// que no esté en el registro falla la prueba: hay que generar su paquete y registrarlo.
var opsByModule = func() map[string][]opMeta {
	out := map[string][]opMeta{}
	for m, ops := range apigen.Modules() {
		for _, o := range ops {
			out[m] = append(out[m], opMeta(o))
		}
	}
	return out
}()

// opMeta es la forma de apigen.Operation que usa esta prueba (los tests la mutan).
type opMeta struct {
	Method           string
	Path             string
	ID               string
	Tags             []string
	Ext              map[string]any
	SecurityDeclared bool
	Security         []string
}

// none es el literal reservado de ADR 0009 para «ningún módulo» o «ningún permiso».
const none = "none"

// moduleNoneTenantRoutes son las únicas rutas Tenant con x-module none (ADR 0009, la lista fija
// que pasa con la suscripción inactiva; la misma de auth.inactivePass).
var moduleNoneTenantRoutes = []string{"GET /v1/me/entitlements", "GET /v1/exports"}

type spec struct {
	rootTags []string          // tags de openapi.yaml
	rootRefs map[string]string // camino -> "<módulo>|<puntero>" del $ref de openapi.yaml
	files    map[string]bool   // paths/<módulo>.yaml -> tiene operaciones (paths no es {})
	ops      map[string][]opMeta
	texts    map[string]string // ruta relativa -> contenido, de todos los YAML
}

var (
	tagRe  = regexp.MustCompile(`^  - name: (\S+)\s*$`)
	pathRe = regexp.MustCompile(`^  (/\S*):\s*$`)
	refRe  = regexp.MustCompile(`^    \$ref: '?paths/(.+)\.yaml#/paths/(\S+?)'?\s*$`)
)

func load(t *testing.T) spec {
	t.Helper()
	s := spec{rootRefs: map[string]string{}, files: map[string]bool{}, texts: map[string]string{}, ops: map[string][]opMeta{}}
	for m, ops := range opsByModule {
		s.ops[m] = slices.Clone(ops)
	}
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".yaml") {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		s.texts[filepath.ToSlash(p)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatalf("reading the spec files: %v", err)
	}
	inTags, inPaths, lastPath := false, false, ""
	for _, line := range strings.Split(s.texts["openapi.yaml"], "\n") {
		switch {
		case line == "tags:":
			inTags, inPaths = true, false
		case line == "paths:":
			inTags, inPaths = false, true
		case line != "" && line[0] != ' ' && line[0] != '#':
			inTags, inPaths = false, false
		case inTags:
			if m := tagRe.FindStringSubmatch(line); m != nil {
				s.rootTags = append(s.rootTags, m[1])
			}
		case inPaths:
			if m := pathRe.FindStringSubmatch(line); m != nil {
				lastPath = m[1]
			} else if m := refRe.FindStringSubmatch(line); m != nil {
				s.rootRefs[lastPath] = m[1] + "|" + m[2]
			}
		}
	}
	for p, text := range s.texts {
		if mod, ok := strings.CutPrefix(p, "paths/"); ok {
			s.files[strings.TrimSuffix(mod, ".yaml")] = !strings.Contains(text, "\npaths: {}\n")
		}
	}
	return s
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// escapePointer codifica un camino como segmento de JSON Pointer (RFC 6901).
func escapePointer(p string) string {
	return strings.ReplaceAll(strings.ReplaceAll(p, "~", "~0"), "/", "~1")
}

var (
	tenantIDRe = regexp.MustCompile(`\btenant_?[iI]d\b`)
	numberRe   = regexp.MustCompile(`type:\s*(number\b|\[[^\]]*\bnumber\b)`)
	// Funciones de registro de rutas que genera oapi-codegen (std-http-server): Handler,
	// HandlerFromMux, HandlerFromMuxWithBaseURL y HandlerWithOptions.
	handlerRe    = regexp.MustCompile(`\b(HandlerWithOptions|HandlerFromMux\w*)\b`)
	handlerSelRe = regexp.MustCompile(`\b\w+\.Handler\b`) // solo cuenta en un archivo que importa internal/apigen
	patternRe    = regexp.MustCompile(`\bpattern:\s*(?:'((?:[^']|'')*)'|"((?:[^"\\]|\\.)*)"|([^'"\s{},][^{},]*))`)
	blockNumRe   = regexp.MustCompile(`^\s*-\s*['"]?number['"]?\s*$`)
)

// check devuelve las violaciones de las reglas del contrato; vacío = cumple.
func check(s spec) []string {
	var v []string
	add := func(format string, a ...any) { v = append(v, fmt.Sprintf(format, a...)) }

	// 1. tags = archivos de paths (el tag es el módulo, ADR 0009). Un tag sin archivo, un archivo
	// sin tag y un tag repetido fallan.
	seen := map[string]bool{}
	for _, tag := range s.rootTags {
		if seen[tag] {
			add("tag %q is declared twice in openapi.yaml", tag)
		}
		seen[tag] = true
		if tag == none {
			add("%q is reserved and cannot be a tag", none)
		}
		if _, ok := s.files[tag]; !ok {
			add("tag %q has no paths/%s.yaml", tag, tag)
		}
	}
	for m := range s.files {
		if !seen[m] {
			add("paths/%s.yaml has no tag in openapi.yaml", m)
		}
	}

	// 2. un archivo con operaciones tiene su paquete generado, y uno vacío no.
	for m, has := range s.files {
		switch n := len(s.ops[m]); {
		case has && n == 0:
			add("paths/%s.yaml has operations but no generated package is registered for it (internal/apigen/%s)", m, m)
		case !has && n > 0:
			add("paths/%s.yaml is empty but its generated package still has %d operations: go generate is stale", m, n)
		}
	}
	for m := range s.ops {
		if _, ok := s.files[m]; !ok {
			add("generated package %q has no paths/%s.yaml", m, m)
		}
	}

	// 3. cada operación: archivo de su tag, extensiones válidas y reglas de none por clase.
	patterns := map[string]bool{}
	for _, m := range keys(s.ops) {
		for _, op := range s.ops[m] {
			pat := op.Method + " " + op.Path
			if patterns[pat] {
				add("%s is declared twice", pat)
			}
			patterns[pat] = true
			id := fmt.Sprintf("%s (%s)", pat, m)
			if !slices.Equal(op.Tags, []string{m}) {
				add("%s: tags = %v, want exactly [%s] (the tag is the module's file)", id, op.Tags, m)
			}
			if got := s.rootRefs[op.Path]; got != m+"|"+escapePointer(op.Path) {
				add("%s: openapi.yaml must reference it as paths/%s.yaml#/paths/%s, has %q", id, m, escapePointer(op.Path), got)
			}
			mod, modErr := extension(op, "x-module")
			perm, permErr := extension(op, "x-permission")
			if modErr != "" {
				add("%s: x-module %s", id, modErr)
			}
			if permErr != "" {
				add("%s: x-permission %s", id, permErr)
			}
			if permErr == "" && perm != none && !permission.Code(perm).Valid() {
				add("%s: x-permission %q is not in the permission catalog (or %q)", id, perm, none)
			}
			if modErr != "" {
				continue
			}
			switch class := classOf(op); {
			case class != "Tenant" && mod != none:
				add("%s: a %s route does not go through entitlements: x-module must be %q, has %q", id, class, none, mod)
			case class == "Tenant" && mod == none && !slices.Contains(moduleNoneTenantRoutes, pat):
				add("%s: only %v are Tenant routes with x-module %q", id, moduleNoneTenantRoutes, none)
			case class == "Tenant" && mod != none && mod != m:
				add("%s: x-module %q is not the tag %q", id, mod, m)
			}
		}
	}
	for p, ref := range s.rootRefs {
		m, _, _ := strings.Cut(ref, "|")
		if _, ok := s.files[m]; !ok {
			add("openapi.yaml: %s references paths/%s.yaml, which does not exist", p, m)
		}
		if !slices.ContainsFunc(s.ops[m], func(o opMeta) bool { return o.Path == p }) {
			add("openapi.yaml: %s references paths/%s.yaml, which has no operation for it", p, m)
		}
	}

	// 4. pertenencia, 5. representación del dinero: reglas de texto sobre todos los YAML.
	for _, p := range keys(s.texts) {
		inType := false // dentro de un `type:` en forma de bloque (lista con guiones)
		for i, line := range strings.Split(s.texts[p], "\n") {
			at := fmt.Sprintf("%s:%d", p, i+1)
			if t := strings.TrimSpace(line); t == "type:" {
				inType = true
			} else if inType && !strings.HasPrefix(t, "- ") {
				inType = false
			}
			if inType && blockNumRe.MatchString(line) {
				add("%s: type number (block list) is forbidden (money is a string, ADR 0008)", at)
			}
			for _, m := range patternRe.FindAllStringSubmatch(line, -1) {
				pat := m[1]
				switch {
				case m[1] != "":
					pat = strings.ReplaceAll(m[1], "''", "'")
				case m[2] != "":
					pat = strings.ReplaceAll(m[2], `\\`, `\`)
				default:
					pat = m[3]
				}
				if _, err := regexp.Compile(pat); err != nil {
					add("%s: pattern %q does not compile in Go (RE2, no lookahead): %v", at, pat, err)
				}
			}
			if tenantIDRe.MatchString(line) {
				add("%s: tenantId must not appear in the contract: the tenant comes from the session (ADR 0001)", at)
			}
			if strings.Contains(line, "x-go-type") {
				add("%s: x-go-type is forbidden: money is a string with its pattern and converts in the handler (ADR 0008)", at)
			}
			if numberRe.MatchString(line) && (!strings.Contains(line, "x-wanka-not-money: true") || p == "components/money.yaml") {
				add("%s: type number is forbidden unless the schema is marked x-wanka-not-money: true (money is a string, ADR 0008)", at)
			}
		}
	}
	return v
}

// extension lee x-module o x-permission de una operación: solo vale un string no vacío.
func extension(op opMeta, name string) (string, string) {
	raw, ok := op.Ext[name]
	switch s, isString := raw.(string); {
	case !ok:
		return "", "is missing"
	case raw == nil:
		return "", "is null"
	case !isString:
		return "", fmt.Sprintf("is %T, not a string", raw)
	case s == "":
		return "", "is empty"
	default:
		return s, ""
	}
}

// classOf deduce la clase de la ruta como auth.Routes (conventions.md §2).
func classOf(op opMeta) string { return apigen.Operation(op).Class() }

func TestContractHolds(t *testing.T) {
	s := load(t)
	if len(s.rootTags) != 21 || len(s.files) != 21 {
		t.Fatalf("tags = %d, files = %d, want 21 each (one per platform.modules code)", len(s.rootTags), len(s.files))
	}
	for _, v := range check(s) {
		t.Error(v)
	}
}

func TestSkeletonOperationsAreStillThere(t *testing.T) {
	s := load(t)
	var got []string
	for _, ops := range s.ops {
		for _, op := range ops {
			got = append(got, op.Method+" "+op.Path)
		}
	}
	for _, want := range []string{"POST /v1/sales", "GET /v1/einvoicing/documents", "GET /v1/me/entitlements"} {
		if !slices.Contains(got, want) {
			t.Errorf("%s is missing from the spec", want)
		}
	}
}

// Cada protección de check tiene un caso que la rompe a propósito: si la regla se debilita, su caso
// deja de fallar y la prueba lo avisa (review-checklist, «Tests que discriminan»).
func TestContractRejects(t *testing.T) {
	opOf := func(s *spec, pat string) *opMeta {
		for m := range s.ops {
			s.ops[m] = deepCopy(s.ops[m])
			for i := range s.ops[m] {
				if s.ops[m][i].Method+" "+s.ops[m][i].Path == pat {
					return &s.ops[m][i]
				}
			}
		}
		t.Fatalf("%s not in the spec", pat)
		return nil
	}
	setExt := func(pat, name string, val any, absent bool) func(*spec) {
		return func(s *spec) {
			op := opOf(s, pat)
			if absent {
				delete(op.Ext, name)
			} else {
				op.Ext[name] = val
			}
		}
	}
	const sales, list, ent, totp = "POST /v1/sales", "GET /v1/einvoicing/documents", "GET /v1/me/entitlements", "POST /v1/auth/mfa/enrollment/totp"
	cases := []struct {
		name   string
		mutate func(*spec)
		want   string
	}{
		{"x-module missing", setExt(sales, "x-module", nil, true), "x-module is missing"},
		{"x-module null", setExt(sales, "x-module", nil, false), "x-module is null"},
		{"x-module empty", setExt(sales, "x-module", "", false), "x-module is empty"},
		{"x-module not a string", setExt(sales, "x-module", 3, false), "not a string"},
		{"x-module of another module", setExt(sales, "x-module", "compras", false), "is not the tag"},
		{"x-module none on a Tenant route", setExt(sales, "x-module", "none", false), "only [GET /v1/me/entitlements"},
		{"x-module of a Public route", setExt(totp, "x-module", "identidad", false), "x-module must be \"none\""},
		{"x-permission missing", setExt(sales, "x-permission", nil, true), "x-permission is missing"},
		{"x-permission null", setExt(sales, "x-permission", nil, false), "x-permission is null"},
		{"x-permission empty", setExt(list, "x-permission", "", false), "x-permission is empty"},
		{"x-permission not a string", setExt(list, "x-permission", true, false), "not a string"},
		{"x-permission outside the catalog", setExt(sales, "x-permission", "ventas.pos:fly", false), "not in the permission catalog"},
		{"x-permission uppercase none", setExt(ent, "x-permission", "None", false), "not in the permission catalog"},
		{"wrong tag", func(s *spec) { opOf(s, sales).Tags = []string{"compras"} }, "tags = [compras]"},
		{"two tags", func(s *spec) { opOf(s, sales).Tags = []string{"ventas", "compras"} }, "tags = [ventas compras]"},
		{"tag without file", func(s *spec) { s.rootTags = append(s.rootTags, "zeta") }, `tag "zeta" has no paths/zeta.yaml`},
		{"file without tag", func(s *spec) {
			s.rootTags = slices.DeleteFunc(s.rootTags, func(t string) bool { return t == "agenda" })
		}, "paths/agenda.yaml has no tag"},
		{"duplicated tag", func(s *spec) { s.rootTags = append(s.rootTags, "ventas") }, `tag "ventas" is declared twice`},
		{"none as tag", func(s *spec) { s.rootTags = append(s.rootTags, "none"); s.files["none"] = false }, "reserved"},
		{"file with operations and no package", func(s *spec) { s.files["agenda"] = true }, "no generated package is registered"},
		{"stale generated package", func(s *spec) { s.files["ventas"] = false }, "go generate is stale"},
		{"package without file", func(s *spec) { s.ops["fantasma"] = []opMeta{{}} }, `generated package "fantasma"`},
		{"path not referenced from the root", func(s *spec) { delete(s.rootRefs, "/v1/sales") }, "openapi.yaml must reference it"},
		{"path referenced from another file", func(s *spec) { s.rootRefs["/v1/sales"] = "compras|~1v1~1sales" }, "openapi.yaml must reference it"},
		{"root references a missing file", func(s *spec) { s.rootRefs["/v1/zzz"] = "zeta|~1v1~1zzz" }, "does not exist"},
		{"tenantId parameter", func(s *spec) { s.texts["paths/ventas.yaml"] += "        - { name: tenantId, in: query }\n" }, "tenantId must not appear"},
		{"tenant_id property", func(s *spec) { s.texts["paths/compras.yaml"] += "    tenant_id: { type: string }\n" }, "tenantId must not appear"},
		{"x-go-type decimal", func(s *spec) { s.texts["components/money.yaml"] += "      x-go-type: decimal.Decimal\n" }, "x-go-type is forbidden"},
		{"number in money", func(s *spec) {
			s.texts["components/money.yaml"] += "    Foo: { type: number, x-wanka-not-money: true }\n"
		}, "type number is forbidden"},
		{"number in a module", func(s *spec) { s.texts["paths/ventas.yaml"] += "    total: { type: number }\n" }, "type number is forbidden"},
		{"number as block list", func(s *spec) {
			s.texts["paths/ventas.yaml"] += "    total:\n      type:\n        - number\n        - 'null'\n"
		}, "type number (block list)"},
		{"pattern with lookahead", func(s *spec) { s.texts["components/money.yaml"] += "      pattern: '^(?!0)[0-9]+$'\n" }, "does not compile"},
		{"pattern with lookahead in double quotes", func(s *spec) { s.texts["paths/ventas.yaml"] += "      pattern: \"^(?=a)a\"\n" }, "does not compile"},
		{"nullable number in a module", func(s *spec) { s.texts["paths/ventas.yaml"] += "    total: { type: [number, 'null'] }\n" }, "type number is forbidden"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := load(t)
			tc.mutate(&s)
			got := check(s)
			if !slices.ContainsFunc(got, func(v string) bool { return strings.Contains(v, tc.want) }) {
				t.Errorf("violations = %q, want one containing %q", got, tc.want)
			}
		})
	}
	t.Run("number marked as not money passes", func(t *testing.T) {
		s := load(t)
		s.texts["paths/ventas.yaml"] += "    ratio: { type: number, x-wanka-not-money: true }\n"
		if got := check(s); len(got) != 0 {
			t.Errorf("violations = %q, want none", got)
		}
	})
}

func deepCopy(ops []opMeta) []opMeta {
	out := slices.Clone(ops)
	for i := range out {
		out[i].Tags = slices.Clone(out[i].Tags)
		ext := map[string]any{}
		for k, v := range out[i].Ext {
			ext[k] = v
		}
		out[i].Ext = ext
	}
	return out
}

// Los códigos de platform.modules (migración del kernel) son los mismos que los archivos paths/.
func TestFilesMatchPlatformModules(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	d.Migrate(ctx, t)
	rows, err := d.Admin(ctx, t).Query(ctx, `SELECT code FROM platform.modules ORDER BY code`)
	if err != nil {
		t.Fatalf("selecting platform.modules: %v", err)
	}
	defer rows.Close()
	var db []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		db = append(db, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	s := load(t)
	if files := keys(s.files); !slices.Equal(files, db) {
		t.Errorf("paths/*.yaml = %v\nplatform.modules = %v\nthey must be the same set", files, db)
	}
	if tags := slices.Sorted(slices.Values(s.rootTags)); !slices.Equal(tags, db) {
		t.Errorf("tags = %v\nplatform.modules = %v\nthey must be the same set", tags, db)
	}
}

// Las rutas /v1 solo las registra auth.Routes, con su clase y su Spec: el código generado se usa
// por su ServerInterfaceWrapper, nunca por HandlerWithOptions (que haría mux.HandleFunc a secas).
func TestGeneratedRouteRegistrationIsNotUsed(t *testing.T) {
	if bad := handlerUses(t, ".."); len(bad) != 0 {
		t.Errorf("code outside the generated files uses the generated route registration: %v", bad)
	}
}

func handlerUses(t *testing.T, root string) []string {
	t.Helper()
	var bad []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, ".gen.go") || strings.HasSuffix(p, "contract_test.go") {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if handlerRe.Match(b) || (bytes.Contains(b, []byte("internal/apigen/")) && handlerSelRe.Match(b)) {
			bad = append(bad, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return bad
}

func TestHandlerUseIsDetected(t *testing.T) {
	imp := "package x\n\nimport \"github.com/bacsystem/wanka/apps/api/internal/apigen/ventas\"\n\n"
	for name, src := range map[string]string{
		"HandlerWithOptions": imp + "var _ = ventas.HandlerWithOptions\n",
		"Handler":            imp + "var _ = ventas.Handler\n",
		"HandlerFromMux":     imp + "var _ = ventas.HandlerFromMux\n",
		"WithBaseURL":        imp + "var _ = ventas.HandlerFromMuxWithBaseURL\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(src), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := handlerUses(t, dir); len(got) != 1 {
				t.Errorf("uses = %v, want the file using %s", got, name)
			}
		})
	}
	t.Run("a Handler field of something else is fine", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n\nvar _ = w.Handler\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := handlerUses(t, dir); len(got) != 0 {
			t.Errorf("uses = %v, want none (no apigen import)", got)
		}
	})
}
