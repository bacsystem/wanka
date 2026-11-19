package main

import (
	"fmt"
	"slices"
	"sort"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/apigen"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// Paridad entre el router y el contrato (#176, criterio 10): cada ruta que
// registra el router real de cmd/wanka tiene su operación en apps/api/openapi
// con el mismo método, ruta, x-module y x-permission, y cada operación del
// contrato está registrada o figura en pendingOperations.

// specOp es una operación del contrato, leída del registro apigen.Modules (el mismo
// que usa openapi/contract_test.go). Class sale de su security como en conventions.md §2.
type specOp struct{ Module, Permission, Class string }

// specOperations devuelve las operaciones del contrato por "MÉTODO /ruta".
func specOperations(t *testing.T) map[string]specOp {
	t.Helper()
	out := map[string]specOp{}
	for _, ops := range apigen.Modules() {
		for _, o := range ops {
			m, _ := o.Ext["x-module"].(string)
			p, _ := o.Ext["x-permission"].(string)
			if _, dup := out[o.Pattern()]; dup {
				t.Fatalf("%s is declared twice in the generated tables", o.Pattern())
			}
			out[o.Pattern()] = specOp{Module: m, Permission: p, Class: o.Class()}
		}
	}
	return out
}

// pendingOperations son las operaciones del contrato que ningún módulo
// implementa todavía. La prueba exige que la lista esté al día: una operación
// ya registrada o que ya no existe en el contrato la hace fallar. Al registrar
// una ruta, se quita de aquí en el mismo PR.
var pendingOperations = []string{
	"POST /v1/auth/mfa/enrollment/email-code/resend", // identidad, #25
	"POST /v1/auth/mfa/enrollment/totp",              // identidad, #25
	"POST /v1/auth/mfa/enrollment/totp/confirm",      // identidad, #25
	"GET /v1/me/entitlements",                        // identidad, #27
	"POST /v1/sales",                                 // ventas, ola 3
	"GET /v1/einvoicing/documents",                   // facturación electrónica, ola 3
}

// parityProblems compara lo registrado con el contrato y devuelve las
// diferencias, ordenadas; vacío = paridad.
func parityProblems(registered []auth.RouteInfo, spec map[string]specOp, pending []string) []string {
	var out []string
	reg := map[string]auth.RouteInfo{}
	for _, ri := range registered {
		reg[ri.Pattern] = ri
		op, ok := spec[ri.Pattern]
		switch {
		case !ok:
			out = append(out, fmt.Sprintf("route %q is registered but the contract has no such operation", ri.Pattern))
		case op.Class != ri.Class.String():
			out = append(out, fmt.Sprintf("route %q: the contract's security makes it %s and it is registered as %s", ri.Pattern, op.Class, ri.Class))
		case op.Module != ri.Spec.Module:
			out = append(out, fmt.Sprintf("route %q: x-module is %q in the contract and Spec.Module is %q", ri.Pattern, op.Module, ri.Spec.Module))
		case op.Permission != ri.Spec.Permission:
			out = append(out, fmt.Sprintf("route %q: x-permission is %q in the contract and Spec.Permission is %q", ri.Pattern, op.Permission, ri.Spec.Permission))
		}
	}
	for key := range spec {
		if _, ok := reg[key]; !ok && !slices.Contains(pending, key) {
			out = append(out, fmt.Sprintf("operation %q is in the contract but no route is registered and it is not in pendingOperations", key))
		}
	}
	for _, key := range pending {
		if _, ok := spec[key]; !ok {
			out = append(out, fmt.Sprintf("pendingOperations lists %q, which is not in the contract", key))
		}
		if _, ok := reg[key]; ok {
			out = append(out, fmt.Sprintf("pendingOperations lists %q, which is already registered: remove it", key))
		}
	}
	sort.Strings(out)
	return out
}

// El router que arma serve (mountPlane) y el contrato dicen lo mismo.
func TestRegisteredRoutesMatchTheContract(t *testing.T) {
	rt := httpx.NewRouter(httpx.RouterOptions{})
	sw, _, err := mountPlane("tenant", rt, nil, defaultWiring) // el cableado que usa serve, con su mount
	if err != nil {
		t.Fatal(err)
	}
	routes, ok := sw.(*auth.Routes)
	if !ok {
		t.Fatalf("mountPlane returned %T, want *auth.Routes", sw)
	}
	for _, p := range parityProblems(routes.Registered(), specOperations(t), pendingOperations) {
		t.Error(p)
	}
}

// La comparación misma: cada diferencia se detecta, en las dos direcciones.
func TestParityProblems(t *testing.T) {
	spec := map[string]specOp{
		"GET /v1/a":  {Module: "ventas", Permission: "ventas.pos:read", Class: "Tenant"},
		"POST /v1/b": {Module: "none", Permission: "none", Class: "Public"},
	}
	route := func(pattern, module, perm string) auth.RouteInfo {
		class := auth.Tenant
		if pattern == "POST /v1/b" {
			class = auth.Public
		}
		return auth.RouteInfo{Pattern: pattern, Class: class, Spec: auth.Spec{Module: module, Permission: perm}}
	}
	for _, tc := range []struct {
		name       string
		registered []auth.RouteInfo
		pending    []string
		want       int
	}{
		{"all registered", []auth.RouteInfo{route("GET /v1/a", "ventas", "ventas.pos:read"), route("POST /v1/b", "none", "none")}, nil, 0},
		{"one registered, one pending", []auth.RouteInfo{route("GET /v1/a", "ventas", "ventas.pos:read")}, []string{"POST /v1/b"}, 0},
		{"an extra route", []auth.RouteInfo{route("GET /v1/a", "ventas", "ventas.pos:read"), route("GET /v1/c", "ventas", "ventas.pos:read")}, []string{"POST /v1/b"}, 1},
		{"an operation without route nor pending", []auth.RouteInfo{route("GET /v1/a", "ventas", "ventas.pos:read")}, nil, 1},
		{"another permission", []auth.RouteInfo{route("GET /v1/a", "ventas", "ventas.pos:create")}, []string{"POST /v1/b"}, 1},
		{"another module", []auth.RouteInfo{route("GET /v1/a", "compras", "ventas.pos:read")}, []string{"POST /v1/b"}, 1},
		{"another method", []auth.RouteInfo{route("POST /v1/a", "ventas", "ventas.pos:read")}, []string{"GET /v1/a", "POST /v1/b"}, 1},
		{"another class", []auth.RouteInfo{route("GET /v1/a", "ventas", "ventas.pos:read"), {Pattern: "POST /v1/b", Class: auth.NoTenant, Spec: auth.Spec{Module: "none", Permission: "none"}}}, nil, 1},
		{"a stale pending: already registered", []auth.RouteInfo{route("GET /v1/a", "ventas", "ventas.pos:read")}, []string{"GET /v1/a", "POST /v1/b"}, 1},
		{"a pending not in the contract", []auth.RouteInfo{route("GET /v1/a", "ventas", "ventas.pos:read")}, []string{"POST /v1/b", "GET /v1/gone"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parityProblems(tc.registered, spec, tc.pending); len(got) != tc.want {
				t.Fatalf("problems = %v, want %d", got, tc.want)
			}
		})
	}
}
