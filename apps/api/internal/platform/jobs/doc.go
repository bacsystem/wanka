// Package jobs es la cola de trabajos diferidos del kernel (ADR 0006): encolar
// un job dentro de la misma transacción del cambio de datos y ejecutarlo en el
// comando "wanka worker", con reintentos. Está hecho sobre River, pero sus tipos
// no salen de este paquete: los módulos usan Enqueuer, Args, Job, Registry,
// Cancel y Snooze, de modo que el outbox propio de la opción 2 del ADR 0006
// pueda sustituir a River sin tocarlos.
//
// # Encolar
//
// Un módulo recibe un Enqueuer y llama a InsertTx con el pgx.Tx de su
// InTenantTx: el job existe si y solo si esa transacción hace COMMIT. Los
// argumentos llevan el tenant_id (campo `json:"tenant_id"`) e implementan
// TenantArgs. Una cola se elige con WithQueue (ver las constantes Queue*).
//
// # Ejecutar
//
// Cada módulo registra sus handlers con Register en el Registry que recibe el
// worker. Un handler:
//
//   - Es idempotente. La entrega es al menos una vez: un job puede correr más de
//     una vez (reintento, caída del worker, plazo vencido), así que toda
//     transición va condicionada por estado, toda clave es única y toda llamada a
//     un tercero lleva su idem_key.
//   - Accede a datos de tenant solo con Job.InTenantTx, que abre la transacción
//     como Actor{TenantID del job, Kind: system} sin UserID (ADR 0001, «Workers»).
//     Un job sin tenant_id recibe db.ErrNoTenant y no se reintenta.
//   - No mantiene una conexión durante una llamada de red: abre una
//     InTenantTx corta para leer, la cierra, hace la llamada y abre otra para
//     guardar el resultado.
//   - Corre con un plazo (JOBS_JOB_TIMEOUT, 1 min por defecto; WithJobTimeout al
//     registrarlo): vencido, se cancela su contexto y el intento falla.
//   - Devuelve nil (éxito), un error cualquiera (transitorio: se reintenta con
//     backoff exponencial y jitter hasta el máximo de intentos y queda
//     "discarded"), Cancel(err) (definitivo: no se reintenta) o Snooze(d) (volver
//     a correr pasado d, sin gastar un intento).
package jobs
