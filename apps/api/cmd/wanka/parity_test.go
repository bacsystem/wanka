package main

import (
	"fmt"
	"slices"
	"sort"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/apigen/facturacion-electronica"
	"github.com/bacsystem/wanka/apps/api/internal/apigen/identidad"
	"github.com/bacsystem/wanka/apps/api/internal/apigen/ventas"
	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// Paridad entre el router y el contrato (#176, criterio 10): cada ruta que
// registra el router real de cmd/wanka tiene su operación en apps/api/openapi
// con el mismo método, ruta, x-module y x-permission, y cada operación del
// contrato está registrada o figura en pendingOperations.

// specOp es una operación del contrato, leída de las tablas Operations que
// genera oapi-codegen (las mismas que usa openapi/contract_test.go).
type specOp struct{ Module, Permission string }

// specOperations devuelve las operaciones del contrato por "MÉTODO /ruta".
func specOperations(t *testing.T) map[string]specOp {
	t.Helper()
	out := map[string]specOp{}
	add := func(method, path string, ext map[string]any) {
		m, _ := ext["x-module"].(string)
		p, _ := ext["x-permission"].(string)
		key := method + " " + path
		if _, dup := out[key]; dup {
			t.Fatalf("%s is declared twice in the generated tables", key)
		}
		out[key] = specOp{Module: m, Permission: p}
	}
	for _, o := range identidad.Operations {
		add(o.Method, o.Path, o.Ext)
	}
	for _, o := range ventas.Operations {
		add(o.Method, o.Path, o.Ext)
	}
	for _, o := range facturacionelectronica.Operations {
		add(o.Method, o.Path, o.Ext)
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
	sw, _, err := mountPlane("tenant", rt, nil, wiring{})
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
		"GET /v1/a":  {Module: "ventas", Permission: "ventas.pos:read"},
		"POST /v1/b": {Module: "none", Permission: "none"},
	}
	route := func(pattern, module, perm string) auth.RouteInfo {
		return auth.RouteInfo{Pattern: pattern, Spec: auth.Spec{Module: module, Permission: perm}}
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
