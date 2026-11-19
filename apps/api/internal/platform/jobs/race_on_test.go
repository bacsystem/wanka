//go:build race

package jobs

// raceEnabled indica si los tests corren con -race: el detector multiplica el
// tiempo de cada operación, así que las cotas de tiempo no significan nada.
const raceEnabled = true
