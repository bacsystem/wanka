package db

import "github.com/riverqueue/river/riverdriver/riverpgxv5"

// RiverDriver devuelve el driver de River sobre este pool, para
// internal/platform/jobs. Existe aquí y no en jobs porque este paquete es el
// único dueño de pgxpool (go-arch-lint): jobs recibe el driver, que no entrega
// conexiones crudas, y no el *pgxpool.Pool.
//
// River usa el pool por su cuenta (LISTEN, búsqueda de jobs, marcar como
// terminado, elección de líder): esas conexiones se suman a las de InTenantTx
// dentro de DB_POOL_MAX_CONNS (ADR 0006, «Conexiones frente a concurrencia»).
func (p *Pool) RiverDriver() *riverpgxv5.Driver { return riverpgxv5.New(p.pool) }
