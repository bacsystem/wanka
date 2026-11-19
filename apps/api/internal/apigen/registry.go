// Package apigen reúne los paquetes generados de openapi/ (go generate): el
// registro Modules es la única lista de ellos. La prueba de contrato
// (openapi/contract_test.go) y la de paridad con el router (cmd/wanka) leen de
// aquí; un archivo de paths con operaciones cuyo paquete falte en el registro
// hace fallar la prueba de contrato.
package apigen

import (
	"slices"
	"strings"

	facturacionelectronica "github.com/bacsystem/wanka/apps/api/internal/apigen/facturacion-electronica"
	"github.com/bacsystem/wanka/apps/api/internal/apigen/identidad"
	"github.com/bacsystem/wanka/apps/api/internal/apigen/ventas"
)

// Operation es una operación del contrato, con la forma del OperationMeta que
// genera internal/apigen/operations.tmpl en cada paquete.
type Operation struct {
	Method           string
	Path             string
	ID               string
	Tags             []string
	Ext              map[string]any
	SecurityDeclared bool
	Security         []string
}

// Pattern es "MÉTODO /ruta", el patrón con que se registra la ruta.
func (o Operation) Pattern() string { return o.Method + " " + o.Path }

// noTenantRoutes son las rutas NoTenant fuera de /v1/auth/* (auth.noTenantRoutes).
var noTenantRoutes = []string{"GET /v1/me", "GET /v1/me/memberships"}

// Class deduce la clase de la ruta como auth.Routes (conventions.md §2):
// "Public" si declara security vacía, "NoTenant" si cuelga de /v1/auth/ o es
// una de noTenantRoutes, y "Tenant" en los demás casos.
func (o Operation) Class() string {
	switch {
	case o.SecurityDeclared && len(o.Security) == 0:
		return "Public"
	case strings.HasPrefix(o.Path, "/v1/auth/") || slices.Contains(noTenantRoutes, o.Pattern()):
		return "NoTenant"
	default:
		return "Tenant"
	}
}

type generated interface {
	~struct {
		Method           string
		Path             string
		ID               string
		Tags             []string
		Ext              map[string]any
		SecurityDeclared bool
		Security         []string
	}
}

func convert[T generated](in []T) []Operation {
	out := make([]Operation, len(in))
	for i, o := range in {
		out[i] = Operation(o)
	}
	return out
}

// Modules devuelve las operaciones de cada paquete generado, por módulo (el
// nombre del archivo de paths y su tag).
func Modules() map[string][]Operation {
	return map[string][]Operation{
		"identidad":               convert(identidad.Operations),
		"ventas":                  convert(ventas.Operations),
		"facturacion-electronica": convert(facturacionelectronica.Operations),
	}
}
