// Command breachedgen genera internal/platform/passwordpolicy/breached.bin a partir de SecLists
// (licencia MIT). Se corre a mano, nunca en serve: go run ./scripts/breachedgen. Descarga el
// archivo del commit fijado, comprueba su sha256, se queda con las contraseñas de 10 a 128
// caracteres y escribe solo los hashes truncados; el texto claro no se guarda en el repo.
// Para actualizar la fuente se cambian las tres constantes y se vuelve a correr.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/bacsystem/wanka/apps/api/internal/platform/passwordpolicy"
)

// Fuente fijada: danielmiessler/SecLists, top 1M de la lista xato-net (el archivo
// 10-million-password-list-top-1000000.txt de la propuesta ya no existe en el repo).
const (
	sourceCommit = "c205c36a445bff37f8e58a9ec829105cd4975c58"
	sourcePath   = "Passwords/Common-Credentials/xato-net-10-million-passwords-1000000.txt"
	sourceSHA256 = "424a3e03a17df0a2bc2b3ca749d81b04e79d59cb7aeec8876a5a3f308d0caf51"
	maxSource    = 32 << 20 // tope de lectura: el archivo real pesa unos 8,5 MB
)

func main() {
	out := flag.String("out", "internal/platform/passwordpolicy/breached.bin", "archivo de salida")
	local := flag.String("source", "", "archivo ya descargado (se verifica igual contra el sha256 fijado)")
	flag.Parse()
	if err := run(context.Background(), *out, *local); err != nil {
		fmt.Fprintln(os.Stderr, "breachedgen:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, out, local string) error {
	var raw []byte
	var err error
	if local != "" {
		raw, err = os.ReadFile(local)
	} else {
		raw, err = download(ctx, "https://raw.githubusercontent.com/danielmiessler/SecLists/"+sourceCommit+"/"+sourcePath)
	}
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != sourceSHA256 {
		return fmt.Errorf("source sha256 is %s, pinned %s", got, sourceSHA256)
	}
	hashes, total, err := extract(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	data, err := passwordpolicy.Encode(hashes)
	if err != nil {
		return err
	}
	if err := writeAtomic(out, data); err != nil {
		return err
	}
	fmt.Printf("source lines: %d; kept (10 to 128 characters, unique): %d; file: %d bytes\n", total, len(hashes), len(data))
	return nil
}

func download(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading source: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading source: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxSource))
}

// extract lee una contraseña por línea (sin recortar espacios: se comparan los bytes exactos),
// filtra de 10 a 128 caracteres y devuelve los hashes ordenados y sin repetir, y las líneas leídas.
func extract(r io.Reader) ([]uint64, int, error) {
	var hashes []uint64
	total := 0
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := bytes.TrimSuffix(sc.Bytes(), []byte("\r"))
		total++
		n := utf8.RuneCount(line)
		if n < passwordpolicy.MinLength || n > passwordpolicy.MaxLength {
			continue
		}
		hashes = append(hashes, passwordpolicy.Hash(string(line)))
	}
	if err := sc.Err(); err != nil {
		return nil, 0, fmt.Errorf("reading source: %w", err)
	}
	slices.Sort(hashes)
	return slices.Compact(hashes), total, nil
}

// writeAtomic escribe a un temporal en la misma carpeta, lo sincroniza y lo renombra.
func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".breached-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(f.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
