// Package migrations embebe las migraciones goose (data-model §6): archivos SQL
// llamados <yyyymmddhhmmss>_<nombre>.sql, solo hacia adelante. Corren como
// wanka_migrator con "wanka migrate up".
package migrations

import "embed"

// FS contiene los archivos de migración en su raíz.
//
//go:embed *.sql
var FS embed.FS
