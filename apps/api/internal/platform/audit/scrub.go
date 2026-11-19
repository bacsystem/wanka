package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Redacted reemplaza el valor de un campo prohibido: la clave se conserva para
// que se sepa que el campo cambió, pero no su contenido.
const Redacted = "[REDACTED]"

const (
	maxDiffBytes = 64 << 10
	maxDiffDepth = 32
)

// forbiddenParts son fragmentos que, dentro del nombre de un campo (en minúsculas
// y sin guiones ni guiones bajos), lo marcan como secreto: contraseñas, tokens,
// api keys, certificados y claves. Es una lista de prohibidos por nombre, a
// propósito amplia: enmascarar de más cuesta un campo ilegible en la bitácora,
// y de menos cuesta una filtración. "hash" cubre los de contraseña y de sesión.
var forbiddenParts = []string{
	"password", "passwd", "contrasena", "clave", "secret", "token", "apikey", "privatekey",
	"certificate", "certificado", "credential", "authorization", "cookie", "totp", "recoverycode", "hash", "cvv",
}

// secretValuePrefixes son los prefijos de los tokens del sistema (wk_at_, wk_rt_)
// y de las API keys de Khipu (fk_): un valor que empieza así se enmascara
// aunque venga bajo un nombre inocente.
var secretValuePrefixes = []string{"wk_at_", "wk_rt_", "wka_", "fk_"}

func forbiddenKey(key string) bool {
	k := strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return r + 'a' - 'A'
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r == 'ñ' || r == 'Ñ':
			return 'n'
		}
		return -1
	}, key)
	for _, p := range forbiddenParts {
		if strings.Contains(k, p) {
			return true
		}
	}
	return false
}

// scrubDiff serializa v a JSON y enmascara, a cualquier profundidad, los campos
// prohibidos y los valores con forma de token. Devuelve nil para un diff vacío
// (se guarda NULL). Un diff que no se puede serializar, pasa de 64 KiB o está
// anidado en exceso es ErrInvalidEvent: no se guarda a medias.
func scrubDiff(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%w: diff is not JSON: %v", ErrInvalidEvent, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber() // no pierde precisión en enteros grandes
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, fmt.Errorf("%w: diff: %v", ErrInvalidEvent, err)
	}
	if tree == nil {
		return nil, nil
	}
	clean, err := scrubValue(tree, 0)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(clean)
	if err != nil {
		return nil, fmt.Errorf("%w: diff: %v", ErrInvalidEvent, err)
	}
	if len(out) > maxDiffBytes {
		return nil, fmt.Errorf("%w: diff is %d bytes, the limit is %d", ErrInvalidEvent, len(out), maxDiffBytes)
	}
	return out, nil
}

func scrubValue(v any, depth int) (any, error) {
	if depth > maxDiffDepth {
		return nil, fmt.Errorf("%w: diff nested deeper than %d levels", ErrInvalidEvent, maxDiffDepth)
	}
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			if forbiddenKey(k) {
				t[k] = Redacted
				continue
			}
			c, err := scrubValue(child, depth+1)
			if err != nil {
				return nil, err
			}
			t[k] = c
		}
		return t, nil
	case []any:
		for i, child := range t {
			c, err := scrubValue(child, depth+1)
			if err != nil {
				return nil, err
			}
			t[i] = c
		}
		return t, nil
	case string:
		for _, p := range secretValuePrefixes {
			if strings.HasPrefix(t, p) {
				return Redacted, nil
			}
		}
	}
	return v, nil
}
