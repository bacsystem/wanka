// Package auth define el puerto de la sesión del plano tenant: el formato del
// token de acceso, la sesión que devuelve la búsqueda y el contrato
// SessionResolver que implementa el adaptador de PostgreSQL (iam.session_lookup).
// El middleware que lo usa vive en auth.Routes (routes.go). Ver api/conventions.md §2 y ADR 0004.
package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

// accessPrefix es el prefijo del token de acceso del plano tenant. Un refresh
// (wk_rt_) o un token de plataforma (wka_*) no sirven como acceso aquí.
const accessPrefix = "wk_at_"

// tokenBytes y tokenBodyLen: 32 bytes aleatorios son 43 caracteres en base64url sin relleno.
const (
	tokenBytes   = 32
	tokenBodyLen = 43
)

// ErrMalformedToken es todo lo que se rechaza sin consultar la base:
// 401 unauthenticated. No repite el valor recibido.
var ErrMalformedToken = errors.New("auth: malformed access token")

// TokenHash es el sha256 del token completo, con su prefijo: lo único que se
// busca y se guarda. El token en claro no sale de ParseAuthorization.
type TokenHash = [sha256.Size]byte

// ParseAuthorization valida los valores de la cabecera Authorization y devuelve
// el hash del token. Exige una sola cabecera con el esquema Bearer (sin
// distinguir mayúsculas), un espacio y wk_at_ seguido de 43 caracteres base64url
// que decodifiquen de forma canónica a 32 bytes, sin nada más. Cualquier otra
// forma es ErrMalformedToken.
func ParseAuthorization(values []string) (TokenHash, error) {
	if len(values) != 1 {
		return TokenHash{}, ErrMalformedToken
	}
	scheme, tok, ok := strings.Cut(values[0], " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || !validAccessToken(tok) {
		return TokenHash{}, ErrMalformedToken
	}
	return sha256.Sum256([]byte(tok)), nil
}

func validAccessToken(tok string) bool {
	body, ok := strings.CutPrefix(tok, accessPrefix)
	if !ok || !validBody(body) {
		return false
	}
	// Strict rechaza el relleno y los bits sobrantes del último carácter (no
	// canónico): dos textos distintos no pueden ser el mismo token.
	raw, err := base64.RawURLEncoding.Strict().DecodeString(body)
	return err == nil && len(raw) == tokenBytes
}

// validBody exige exactamente 43 caracteres de [A-Za-z0-9_-], uno por uno y
// antes de decodificar. El decodificador de Go ignora \r y \n (también con
// Strict), así que la validación de forma no debe depender de él. Hoy esta
// comprobación, la longitud y el len(raw) == 32 posterior se cubren entre sí
// (un salto dentro de 43 bytes deja 42 caracteres útiles, que son 31 bytes):
// es defensa en profundidad, no la única barrera. No se quita ninguna sin
// revisar las otras dos.
func validBody(body string) bool {
	if len(body) != tokenBodyLen {
		return false
	}
	for i := 0; i < len(body); i++ {
		c := body[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
