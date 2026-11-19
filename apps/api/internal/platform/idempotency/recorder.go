package idempotency

import (
	"bytes"
	"net/http"
)

// recorder guarda la respuesta del handler hasta saber si la transacción
// confirmó: solo entonces se envía al cliente.
type recorder struct {
	h    http.Header
	code int
	body bytes.Buffer
}

func newRecorder() *recorder { return &recorder{h: make(http.Header)} }

func (r *recorder) Header() http.Header { return r.h }

func (r *recorder) WriteHeader(code int) {
	if r.code == 0 {
		r.code = code
	}
}

func (r *recorder) Write(p []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	return r.body.Write(p)
}

// status es el código que recibiría el cliente: 200 si el handler no escribió nada.
func (r *recorder) status() int {
	if r.code == 0 {
		return http.StatusOK
	}
	return r.code
}

func (r *recorder) is2xx() bool { return r.status() >= 200 && r.status() <= 299 }

// flush envía lo grabado a w.
func (r *recorder) flush(w http.ResponseWriter) {
	dst := w.Header()
	for k, vv := range r.h {
		dst[k] = vv
	}
	w.WriteHeader(r.status())
	_, _ = w.Write(r.body.Bytes())
}
