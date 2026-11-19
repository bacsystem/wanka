// Package entitlements lee qué módulos, flags y límites tiene una empresa
// (ADR 0005, «Entitlements de un tenant»): los módulos de su plan más o menos
// los overrides, los feature flags con rollout determinista y los límites del
// plan frente al uso del mes.
//
// La lectura es una sola consulta agregada con la RLS del tenant (InTenantReadTx)
// y se guarda en memoria por tenant 30 s. Un NOTIFY entitlements_changed, que
// escuchan todas las réplicas, la invalida antes. Este paquete solo lee: el
// middleware que aplica las reglas (402, 403) es de la historia #175 y las
// escrituras son de plataforma y empresas.
//
// La falta de datos cierra el acceso: sin suscripción, o con una suscripción
// que no está activa ni en prueba, no hay ningún módulo ni flag habilitado.
package entitlements

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"slices"
)

// Status es el estado de la suscripción de la empresa, o StatusNoPlan si no
// tiene ninguna.
type Status string

// Estados. Los de suscripción son los valores de platform.subscriptions.status.
const (
	StatusTrial     Status = "trial"
	StatusActive    Status = "activa"
	StatusPastDue   Status = "morosa"
	StatusSuspended Status = "suspendida"
	StatusCancelled Status = "cancelada"
	// StatusNoPlan: la empresa no tiene suscripción. No es el plan semilla: no
	// habilita ningún módulo.
	StatusNoPlan Status = "sin_plan"
)

// Usable dice si la suscripción permite usar la API: activa o en prueba.
func (s Status) Usable() bool { return s == StatusActive || s == StatusTrial }

// Inactive dice si la suscripción está en mora, suspendida o cancelada: el
// middleware responde 402 subscription_inactive.
func (s Status) Inactive() bool {
	return s == StatusPastDue || s == StatusSuspended || s == StatusCancelled
}

// Claves de los límites y del uso que hoy define el producto.
const (
	LimitUsers       = "users"
	LimitSites       = "sites"
	LimitCPEPerMonth = "cpe_per_month"
)

// Entitlements es lo que una empresa tiene habilitado ahora. El valor que
// devuelve For es una copia: el llamador puede modificarla sin afectar la caché.
type Entitlements struct {
	// Modules son los códigos de módulo habilitados, ordenados. Vacío si la
	// suscripción no es Usable.
	Modules []string `json:"modules"`
	// Flags son las claves de los feature flags encendidos para la empresa,
	// ordenadas. Vacío si la suscripción no es Usable.
	Flags []string `json:"flags"`
	// Limits son los topes del plan por clave; todo plan declara las tres (CHECK
	// de la base). Usar Limit, que cierra ante una clave ausente.
	Limits map[string]int64 `json:"limits"`
	// Usage es el uso del mes en curso (hora de Lima), con las claves users y
	// cpe_per_month. sites no se mide (no hay dónde contarlo todavía) y no está en
	// Usage; quien aplique LimitSites debe tratarlo como error al arrancar (#175).
	// Sin fila del mes, todo está en 0.
	Usage  map[string]int64 `json:"usage"`
	Status Status           `json:"status"`
	// ReadOnly: la empresa está cerrada (org.tenants.status = cerrada); el
	// middleware rechaza todo método distinto de GET con 403 tenant_read_only.
	ReadOnly bool `json:"readOnly"`
	// Version identifica el contenido (módulos, flags, límites, estado y
	// readOnly; no el uso): cambia solo cuando cambia lo que la empresa puede
	// hacer, así que sirve de ETag para la UI.
	Version string `json:"version"`
}

// Limit es el tope del plan para key. Una clave ausente es 0: cierra, nunca "sin tope".
func (e Entitlements) Limit(key string) int64 { return e.Limits[key] }

// HasModule dice si el módulo está habilitado.
func (e Entitlements) HasModule(code string) bool {
	_, ok := slices.BinarySearch(e.Modules, code)
	return ok
}

// HasFlag dice si el feature flag está encendido.
func (e Entitlements) HasFlag(key string) bool {
	_, ok := slices.BinarySearch(e.Flags, key)
	return ok
}

// clone copia los campos mutables para no compartir la caché con el llamador.
func (e Entitlements) clone() Entitlements {
	e.Modules = slices.Clone(e.Modules)
	e.Flags = slices.Clone(e.Flags)
	e.Limits = maps.Clone(e.Limits)
	e.Usage = maps.Clone(e.Usage)
	return e
}

// computeVersion es el hash del contenido de e sin Usage ni Version. encoding/json
// ordena las claves de los mapas, y Modules y Flags ya van ordenados: el mismo
// contenido da el mismo hash en cualquier réplica.
func computeVersion(e Entitlements) string {
	e.Usage, e.Version = nil, ""
	b, err := json.Marshal(e)
	if err != nil { // no puede ocurrir con estos tipos
		panic("entitlements: marshaling the version: " + err.Error())
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}
