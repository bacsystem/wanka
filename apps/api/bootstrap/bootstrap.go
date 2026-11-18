// Package bootstrap embebe el script de bootstrap de roles. Lo corre el
// superusuario de la base con psql, fuera de goose (ADR 0001, "Creación de los
// roles"); el binario nunca lo ejecuta. Se embebe para que los tests apliquen
// el mismo texto que corren los operadores.
package bootstrap

import _ "embed"

// RolesSQL es bootstrap/roles.sql: idempotente, sin contraseñas ni metacomandos de psql.
//
//go:embed roles.sql
var RolesSQL string
