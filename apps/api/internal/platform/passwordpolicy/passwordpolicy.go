// Package passwordpolicy rechaza las contraseñas que ya aparecieron en filtraciones públicas
// (ADR 0004, #238). La comprobación es offline: la lista va embebida en el binario como los
// primeros 8 bytes del sha256 de cada contraseña (sin texto claro), ordenados, y se busca con
// búsqueda binaria. La comparación distingue mayúsculas, igual que la fuente. El origen y la
// licencia están en docs/architecture/data/breached-passwords.md.
package passwordpolicy

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"

	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// Longitud permitida de una contraseña, en caracteres (ADR 0004). La lista solo guarda las de este rango.
const (
	MinLength = 10
	MaxLength = 128
)

const (
	magic      = "WKBP"
	version    = 1
	headerSize = 4 + 4 + 8 + sha256.Size // magic, versión, cantidad, sha256 del cuerpo
)

// ErrBreached es lo que devuelve Check cuando la contraseña está en la lista.
var ErrBreached = errors.New("password appears in a public breach list")

// ErrCorrupt envuelve todo defecto del archivo de la lista.
var ErrCorrupt = errors.New("breached password list is corrupt")

//go:embed breached.bin
var embedded []byte

// Policy es la lista cargada. Es de solo lectura: se comparte entre goroutines sin cerrojo.
type Policy struct {
	hashes []uint64
}

// NewEmbedded carga la lista que va dentro del binario. Falla si falta, está vacía o dañada.
func NewEmbedded() (*Policy, error) { return Parse(embedded) }

// Parse valida y carga un archivo de la lista. Falla cerrado: encabezado, tamaño, cantidad,
// sha256 del cuerpo y orden estricto tienen que cuadrar, y la lista no puede estar vacía.
func Parse(data []byte) (*Policy, error) {
	if len(data) < headerSize {
		return nil, fmt.Errorf("%w: %d bytes, header needs %d", ErrCorrupt, len(data), headerSize)
	}
	if string(data[:4]) != magic {
		return nil, fmt.Errorf("%w: bad magic", ErrCorrupt)
	}
	if v := binary.LittleEndian.Uint32(data[4:8]); v != version {
		return nil, fmt.Errorf("%w: unsupported version %d", ErrCorrupt, v)
	}
	count := binary.LittleEndian.Uint64(data[8:16])
	body := data[headerSize:]
	if len(body)%8 != 0 {
		return nil, fmt.Errorf("%w: body of %d bytes is not a multiple of 8", ErrCorrupt, len(body))
	}
	if count == 0 || count != uint64(len(body)/8) {
		return nil, fmt.Errorf("%w: header says %d entries, body has %d", ErrCorrupt, count, len(body)/8)
	}
	if sum := sha256.Sum256(body); !bytes.Equal(sum[:], data[16:headerSize]) {
		return nil, fmt.Errorf("%w: body sha256 does not match the header", ErrCorrupt)
	}
	hashes := make([]uint64, len(body)/8)
	for i := range hashes {
		hashes[i] = binary.LittleEndian.Uint64(body[i*8:])
		if i > 0 && hashes[i] <= hashes[i-1] {
			return nil, fmt.Errorf("%w: entry %d is not in strictly ascending order", ErrCorrupt, i)
		}
	}
	return &Policy{hashes: hashes}, nil
}

// Encode arma el archivo a partir de hashes ya ordenados y sin repetir. Lo usa el generador.
func Encode(hashes []uint64) ([]byte, error) {
	if len(hashes) == 0 {
		return nil, errors.New("empty list")
	}
	if !slices.IsSorted(hashes) || len(slices.Compact(slices.Clone(hashes))) != len(hashes) {
		return nil, errors.New("hashes must be strictly ascending")
	}
	body := make([]byte, 8*len(hashes))
	for i, h := range hashes {
		binary.LittleEndian.PutUint64(body[i*8:], h)
	}
	sum := sha256.Sum256(body)
	out := make([]byte, 0, headerSize+len(body))
	out = append(out, magic...)
	out = binary.LittleEndian.AppendUint32(out, version)
	out = binary.LittleEndian.AppendUint64(out, uint64(len(hashes)))
	out = append(out, sum[:]...)
	return append(out, body...), nil
}

// Hash es la clave de una contraseña en la lista: los primeros 8 bytes del sha256 de sus bytes exactos.
func Hash(password string) uint64 {
	sum := sha256.Sum256([]byte(password))
	return binary.BigEndian.Uint64(sum[:8])
}

// Len es la cantidad de entradas.
func (p *Policy) Len() int { return len(p.hashes) }

// Contains dice si la contraseña está en la lista.
func (p *Policy) Contains(password string) bool {
	_, found := slices.BinarySearch(p.hashes, Hash(password))
	return found
}

// Check devuelve ErrBreached si la contraseña está en la lista y nil si no. No registra ni
// guarda la contraseña. La longitud la valida quien llama (la lista solo trae de 10 a 128).
func (p *Policy) Check(password string) error {
	if p.Contains(password) {
		return ErrBreached
	}
	return nil
}

// BreachedError es el 422 validation_failed con errors[{field, code: validation_failed,
// meta.rule: breached}] que las rutas de contraseña devuelven cuando Check falla.
func BreachedError(field string) *httpx.Error {
	return httpx.NewError("validation_failed", "password appears in a public breach list",
		httpx.FieldError{Field: field, Code: "validation_failed", Meta: map[string]any{"rule": "breached"}})
}
