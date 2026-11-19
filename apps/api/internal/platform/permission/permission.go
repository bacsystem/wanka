// Package permission es el catálogo de permisos de tenant (formato modulo.recurso:accion) en Go.
//
// La fuente de verdad es docs/architecture/api/resources.md; se siembra en iam.permissions por
// migración y este paquete es su copia en código. Los tres se mantienen iguales con un test de
// paridad. El catálogo de plataforma (plataforma.*) es otro y no está aquí.
package permission

import (
	"errors"
	"fmt"
	"slices"
)

// Code es un permiso de tenant: modulo.recurso:accion.
type Code string

// Actions son las acciones que define el catálogo, las mismas que el CHECK de iam.permissions.
func Actions() []string {
	return []string{"read", "read_all", "create", "update", "void", "export", "import", "approve", "send", "retry"}
}

// All devuelve todos los códigos del catálogo, en el orden de resources.md. Es una copia.
func All() []Code { return slices.Clone(all[:]) }

// Valid dice si c es un código del catálogo.
func (c Code) Valid() bool { return slices.Contains(all[:], c) }

// Provider es lo que implementa cada módulo para declarar los permisos que usa.
type Provider interface {
	Permissions() []Code
}

// Validate comprueba que cada código exista en el catálogo y no se repita. Devuelve todos los
// problemas juntos (errors.Join), no solo el primero.
func Validate(codes []Code) error {
	var errs []error
	seen := make(map[Code]struct{}, len(codes))
	for _, c := range codes {
		if !c.Valid() {
			errs = append(errs, fmt.Errorf("unknown permission code %q", string(c)))
			continue
		}
		if _, dup := seen[c]; dup {
			errs = append(errs, fmt.Errorf("permission code %q declared twice", string(c)))
		}
		seen[c] = struct{}{}
	}
	return errors.Join(errs...)
}

// Check valida los permisos que declara un módulo. Se llama al arrancar, una vez por módulo, y un
// error aborta el arranque: así un código inventado no llega a producción.
func Check(module string, p Provider) error {
	if err := Validate(p.Permissions()); err != nil {
		return fmt.Errorf("module %s: %w", module, err)
	}
	return nil
}
