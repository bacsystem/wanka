package httpx_test

import (
	"io"
	"net/http"
)

func readAll(r *http.Request) ([]byte, error) { return io.ReadAll(r.Body) }
