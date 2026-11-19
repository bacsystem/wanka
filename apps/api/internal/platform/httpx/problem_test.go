package httpx_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// conventionsTable lee la tabla de §4 de conventions.md: es la fuente de verdad
// del catálogo, no una copia en el test.
func conventionsTable(t *testing.T) map[string]int {
	t.Helper()
	raw, err := os.ReadFile("../../../../../docs/architecture/api/conventions.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "## 4. Formato de error")
	end := strings.Index(text, "## 5.")
	if start < 0 || end < start {
		t.Fatal("conventions.md: section 4 not found")
	}
	row := regexp.MustCompile("(?m)^\\| (\\d{3}) \\| `([a-z_]+)` \\|")
	got := map[string]int{}
	for _, m := range row.FindAllStringSubmatch(text[start:end], -1) {
		status, _ := strconv.Atoi(m[1])
		if _, dup := got[m[2]]; dup {
			t.Fatalf("conventions.md lists %s twice", m[2])
		}
		got[m[2]] = status
	}
	return got
}

func TestCodeTableMatchesConventions(t *testing.T) {
	want := conventionsTable(t)
	if rows := countTableRows(t); rows != len(want) {
		t.Fatalf("conventions.md §4 has %d rows but %d were recognised as a code", rows, len(want))
	}
	got := httpx.Codes()
	for code, status := range want {
		if g, ok := got[code]; !ok {
			t.Errorf("code %q is in conventions.md but not in the kernel table", code)
		} else if g != status {
			t.Errorf("code %q: kernel HTTP %d, conventions.md %d", code, g, status)
		}
	}
	for code := range got {
		if _, ok := want[code]; !ok {
			t.Errorf("code %q is in the kernel table but not in conventions.md", code)
		}
	}
}

func TestCodesReturnsACopy(t *testing.T) {
	httpx.Codes()["internal"] = 200
	if httpx.Codes()["internal"] != 500 {
		t.Fatal("Codes must not expose the internal table")
	}
}

func TestDomainErrorUsesTheTableHTTP(t *testing.T) {
	h, _, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /stock", func(w http.ResponseWriter, r *http.Request) {
			err := fmt.Errorf("selling: %w", httpx.NewError("insufficient_stock", "Stock disponible 2 NIU",
				httpx.FieldError{Field: "lines[1].quantity", Code: "insufficient_stock", Meta: map[string]any{"available": "2"}}))
			httpx.WriteError(w, r, err)
		})
	})
	rec := do(h, httptest.NewRequest(http.MethodGet, "/stock", nil))
	p := mustProblem(t, rec, http.StatusConflict, "insufficient_stock")
	if p.Detail != "Stock disponible 2 NIU" || len(p.Errors) != 1 || p.Errors[0].Field != "lines[1].quantity" {
		t.Fatalf("problem = %+v", p)
	}
	if !strings.Contains(rec.Body.String(), `"available":"2"`) {
		t.Fatalf("meta missing: %s", rec.Body.String())
	}
}

func TestUnknownCodeIs500AndLogged(t *testing.T) {
	h, buf, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /bad", func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteError(w, r, httpx.NewError("made_up_code", "should not reach the client"))
		})
		rt.HandleFunc("GET /bad2", func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteProblem(w, r, "another_made_up_code", "nor this")
		})
	})
	for _, tc := range []struct{ path, code string }{{"/bad", "made_up_code"}, {"/bad2", "another_made_up_code"}} {
		rec := do(h, httptest.NewRequest(http.MethodGet, tc.path, nil))
		p := mustProblem(t, rec, http.StatusInternalServerError, "internal")
		if p.Detail != "" || strings.Contains(rec.Body.String(), "made_up") {
			t.Fatalf("500 leaks the rejected code: %s", rec.Body.String())
		}
		var found bool
		for _, l := range buf.lines(t, "problem code not in catalog") {
			if l["level"] == "ERROR" && l["code"] == tc.code {
				found = true
			}
		}
		if !found {
			t.Fatalf("no error log naming %q: %s", tc.code, buf.String())
		}
	}
}

func TestUntypedErrorIs500WithoutDetail(t *testing.T) {
	h, buf, _ := chain(t, func(rt *httpx.Router) {
		rt.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteError(w, r, errors.New("pq: relation users has secret"))
		})
	})
	rec := do(h, httptest.NewRequest(http.MethodGet, "/x", nil))
	p := mustProblem(t, rec, http.StatusInternalServerError, "internal")
	if p.Detail != "" || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("leak: %s", rec.Body.String())
	}
	if l := buf.lines(t, "unhandled error"); len(l) != 1 || l[0]["level"] != "ERROR" || !strings.Contains(fmt.Sprint(l[0]["err"]), "relation users") {
		t.Fatalf("log = %v", l)
	}
}

// Todo 429 y 503 que emite el kernel lleva Retry-After en segundos enteros >= 1.
func TestKernelThrottleAndUnavailableCodesCarryRetryAfter(t *testing.T) {
	codes := []string{"rate_limited", "overloaded", "dependency_unavailable", "request_in_progress"}
	h, _, _ := chain(t, func(rt *httpx.Router) {
		for _, code := range codes {
			rt.HandleFunc("GET /"+code, func(w http.ResponseWriter, r *http.Request) {
				httpx.WriteError(w, r, httpx.NewError(code, ""))
			})
		}
	})
	for _, code := range codes {
		rec := do(h, httptest.NewRequest(http.MethodGet, "/"+code, nil))
		mustProblem(t, rec, httpx.Codes()[code], code)
		if n, err := strconv.Atoi(rec.Header().Get("Retry-After")); err != nil || n < 1 {
			t.Fatalf("%s: Retry-After = %q", code, rec.Header().Get("Retry-After"))
		}
	}
}

// countTableRows cuenta las filas de la tabla de §4 sin interpretarlas, para que
// una fila con otro formato rompa el test en vez de saltarse.
func countTableRows(t *testing.T) int {
	t.Helper()
	raw, err := os.ReadFile("../../../../../docs/architecture/api/conventions.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	section := text[strings.Index(text, "## 4. Formato de error"):strings.Index(text, "## 5.")]
	return len(regexp.MustCompile(`(?m)^\| \d{3} \|`).FindAllString(section, -1))
}

// forbidden lleva el permiso que falta como string; sin él, el miembro no sale.
func TestForbiddenCarriesPermissionOnlyWhenSet(t *testing.T) {
	for _, tc := range []struct {
		name, perm string
		want       bool
	}{{"with permission", "ventas.pos:read", true}, {"without", "", false}} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, _ := chain(t, func(rt *httpx.Router) {
				rt.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {
					httpx.WriteProblemExt(w, r, "forbidden", "", httpx.Extensions{Permission: tc.perm})
				})
			})
			rec := do(h, httptest.NewRequest(http.MethodGet, "/x", nil))
			mustProblem(t, rec, http.StatusForbidden, "forbidden")
			var m map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
				t.Fatal(err)
			}
			got, has := m["permission"]
			if has != tc.want || (has && got != tc.perm) {
				t.Fatalf("permission = %v (present %v), body %s", got, has, rec.Body.String())
			}
		})
	}
}
