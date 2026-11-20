// Package argon2 es el adaptador de hash de contraseñas: argon2id en el formato
// PHC (ADR 0002 y 0004).
package argon2

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params son los parámetros de argon2id. Los que se guardan en cada hash son los
// que usó ese hash: cambiar estos no invalida las contraseñas existentes.
type Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
}

// Default son los parámetros de R1, fijados con la prueba de carga de #24 (TestLoginLoad): m = 64 MiB y p = 2 del ADR 0004,
// con t = 2 en vez de 3. Con t = 3 una réplica de 2 vCPU atiende unos 13 ingresos/s; con t = 2, unos 22, que es lo que
// pide la carga de R1 (20/s). Quedan por encima del mínimo de OWASP (m = 46 MiB, t = 1, p = 1).
var Default = Params{MemoryKiB: 64 * 1024, Iterations: 2, Parallelism: 2}

const (
	saltLen = 16
	keyLen  = 32
	// Topes de lo que se acepta leer de un hash guardado: un dato corrupto no puede pedir gigas de memoria.
	maxMemoryKiB   = 1 << 20 // 1 GiB
	maxIterations  = 16
	maxParallelism = 16
)

// ErrMalformedHash es un hash guardado que no es PHC de argon2id con parámetros razonables.
var ErrMalformedHash = errors.New("argon2: malformed password hash")

// Hasher calcula y verifica hashes con unos parámetros fijos.
type Hasher struct {
	p         Params
	dummySalt []byte
}

// New valida p y devuelve el Hasher.
func New(p Params) (*Hasher, error) {
	if p.MemoryKiB < 8*uint32(p.Parallelism) || p.Iterations < 1 || p.Parallelism < 1 || p.MemoryKiB > maxMemoryKiB || p.Iterations > maxIterations || p.Parallelism > maxParallelism {
		return nil, errors.New("argon2: invalid parameters")
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("reading random bytes for the dummy salt: %w", err)
	}
	return &Hasher{p: p, dummySalt: salt}, nil
}

// Params devuelve los parámetros con que hashea.
func (h *Hasher) Params() Params { return h.p }

// Hash devuelve el hash PHC de password con una sal nueva.
func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("reading random bytes for the salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, h.p.Iterations, h.p.MemoryKiB, h.p.Parallelism, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, h.p.MemoryKiB, h.p.Iterations, h.p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify dice si password es la contraseña de encoded, en tiempo constante. Un
// hash que no se puede leer es ErrMalformedHash y nunca "coincide".
func (h *Hasher) Verify(password, encoded string) (bool, error) {
	p, salt, want, err := parse(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// Dummy hace el mismo trabajo que Verify con los parámetros del Hasher y descarta
// el resultado: lo usa el login cuando no hay hash que comprobar, para que la
// respuesta tarde lo mismo y no diga si la cuenta existe.
func (h *Hasher) Dummy(password string) {
	_ = argon2.IDKey([]byte(password), h.dummySalt, h.p.Iterations, h.p.MemoryKiB, h.p.Parallelism, keyLen)
}

// parse lee "$argon2id$v=19$m=65536,t=3,p=2$<sal>$<clave>" (base64 estándar sin relleno).
func parse(encoded string) (p Params, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v="+strconv.Itoa(argon2.Version) {
		return p, nil, nil, ErrMalformedHash
	}
	var m, t, par uint64
	n, scanErr := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &par)
	if scanErr != nil || n != 3 || parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", m, t, par) {
		return p, nil, nil, ErrMalformedHash
	}
	if m < 8 || m > maxMemoryKiB || t < 1 || t > maxIterations || par < 1 || par > maxParallelism || m < 8*par {
		return p, nil, nil, ErrMalformedHash
	}
	salt, err = base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return p, nil, nil, ErrMalformedHash
	}
	key, err = base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(key) < 16 || len(key) > 64 {
		return p, nil, nil, ErrMalformedHash
	}
	return Params{MemoryKiB: uint32(m), Iterations: uint32(t), Parallelism: uint8(par)}, salt, key, nil
}
