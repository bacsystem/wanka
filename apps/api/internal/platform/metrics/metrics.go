// Package metrics es el puerto de métricas del kernel: el código de la
// aplicación cuenta eventos sin conocer el backend. El adaptador OpenTelemetry
// llega con su propia historia; mientras tanto Nop descarta todo.
package metrics

// Recorder cuenta eventos. labels son pares clave, valor; deben tener pocos
// valores distintos (la ruta es un patrón, nunca la URL con sus parámetros).
type Recorder interface {
	Inc(name string, labels ...string)
}

// Nop es el Recorder que descarta todo.
type Nop struct{}

// Inc no hace nada.
func (Nop) Inc(string, ...string) {}

// Nombres de los contadores que emite el kernel HTTP.
const (
	StatementTimeouts = "db_statement_timeouts_total" // labels: phase, route
	HandlerTimeouts   = "http_handler_timeouts_total" // labels: route
)
