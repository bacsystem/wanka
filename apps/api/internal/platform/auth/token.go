// Package auth define el puerto de la sesión del plano tenant: el formato del
// token de acceso, la sesión que devuelve la búsqueda y el contrato
// SessionResolver que implementa el adaptador de PostgreSQL (iam.session_lookup).
// El middleware que lo usa vive en httpx. Ver api/conventions.md §2 y ADR 0004.
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

// tokenBodyLen es la longitud de 32 bytes en base64url sin relleno.
const tokenBodyLen = 43

// ErrMalformedToken es todo lo que se rechaza sin consultar la base:
// 401 unauthenticated. No repite el valor recibido.
var ErrMalformedToken = errors.New("auth: malformed access token")

// TokenHash es el sha256 del token completo, con su prefijo: lo único que se
// busca y se guarda. El token en claro no sale de ParseAuthorization.
type TokenHash [sha256.Size]byte

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
	if !ok || len(body) != tokenBodyLen {
		return false
	}
	// Strict rechaza el relleno, los saltos de línea y los bits sobrantes del
	// último carácter: dos textos distintos no pueden ser el mismo token.
	raw, err := base64.RawURLEncoding.Strict().DecodeString(body)
	return err == nil && len(raw) == 32
}
