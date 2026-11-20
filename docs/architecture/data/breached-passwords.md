# Lista de contraseñas filtradas

Fuente de la regla `breached` de la política de contraseñas (ADR 0004; historia #238). La decisión de fuente es del dueño (2026-11-19, opción A de la propuesta del architect en la issue #238).

## Fuente y licencia

| Dato | Valor |
|---|---|
| Repositorio | `danielmiessler/SecLists` |
| Archivo | `Passwords/Common-Credentials/xato-net-10-million-passwords-1000000.txt` (el top 1M de la lista de 10 millones; el nombre `10-million-password-list-top-1000000.txt` de la propuesta ya no existe en el repo: se movió y renombró) |
| Commit fijado | `c205c36a445bff37f8e58a9ec829105cd4975c58` (2025-05-08) |
| `sha256` del archivo | `424a3e03a17df0a2bc2b3ca749d81b04e79d59cb7aeec8876a5a3f308d0caf51` (1 000 000 líneas, 8 557 632 bytes) |
| Licencia | MIT, Copyright (c) 2018 Daniel Miessler. El aviso está en `apps/api/internal/platform/passwordpolicy/LICENSE-SecLists` |

El repo es público y **no contiene la lista en texto plano**: solo `breached.bin` (hashes truncados). El script descarga el archivo del commit fijado, comprueba su `sha256` y escribe solo el binario.

## Formato de `breached.bin`

Encabezado de 48 bytes: `WKBP`, versión (`uint32`, 1), cantidad de entradas (`uint64`) y `sha256` del cuerpo (32 bytes). Cuerpo: cada entrada es un `uint64` little-endian con los primeros 8 bytes (big-endian) del `sha256` de los bytes UTF-8 exactos de la contraseña; ordenadas de menor a mayor y sin repetir. Solo entran las de 10 a 128 caracteres (la longitud ya rechaza el resto). La comparación distingue mayúsculas y no normaliza nada.

Al arrancar, `serve` valida magia, versión, tamaño múltiplo de 8, cantidad, `sha256` del cuerpo, orden estricto y que no esté vacía. Si algo falla, el proceso termina con error (no se degrada a «sin lista»).

## Cómo se actualiza

1. Elegir el commit de SecLists y el archivo; calcular el `sha256` del archivo descargado.
2. Cambiar `sourceCommit`, `sourcePath` y `sourceSHA256` en `apps/api/scripts/breachedgen/main.go`.
3. Desde `apps/api`: `go run ./scripts/breachedgen` (o `-source <archivo>` con una copia local, que se verifica igual contra el `sha256` fijado).
4. El PR muestra el cambio de cantidad y de tamaño; se actualizan las cifras de este documento. La fuente se mueve poco (filtraciones de 2016 y anteriores): es su límite conocido.

## Tamaños medidos

| Medida | Valor |
|---|---|
| Entradas de la fuente | 1 000 000 |
| Entradas guardadas (10 a 128 caracteres, sin repetir) | 118 544 |
| `breached.bin` (y aporte al binario) | 948 400 bytes |
| Montón extra al cargar | 947 888 bytes (el arreglo `[]uint64`) |
| Tope fijado | 16 MB por réplica (lo vigila `TestMemoryCap`) |
| p95 de una consulta | 1,0 µs (`TestLookupLatency`, 20 000 consultas; cota 5 ms) |
