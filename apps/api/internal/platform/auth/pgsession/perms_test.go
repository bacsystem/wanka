package pgsession_test

import (
	"context"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/permission"
)

// allowAll es el doble de auth.PermissionResolver de estas pruebas: concede el
// catálogo entero con la versión que ya tiene la membresía. El resolver real
// llega con identidad (#29).
type allowAll struct{}

func (allowAll) Resolve(_ context.Context, s auth.Session) (auth.Authz, error) {
	m := map[permission.Code]struct{}{}
	for _, c := range permission.All() {
		m[c] = struct{}{}
	}
	return auth.Authz{Version: s.CurrentAuthzVersion.Int64, Permissions: m}, nil
}
