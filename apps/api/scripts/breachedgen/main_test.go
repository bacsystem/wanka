package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/passwordpolicy"
)

func TestExtract(t *testing.T) {
	in := strings.Join([]string{
		"123456789",                    // 9: fuera
		"1234567890",                   // 10: entra
		strings.Repeat("a", 128),       // 128: entra
		strings.Repeat("b", 129),       // 129: fuera
		"contraseña1",                  // 11 caracteres (12 bytes): entra
		"áéíóúáéíóú",                   // 10 caracteres (20 bytes): entra
		"1234567890",                   // repetida: una sola vez
		"with-crlf-ending\r",           // el \r no cuenta ni se hashea
		" leading and trailing space ", // los espacios cuentan
	}, "\n")
	hashes, total, err := extract(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if total != 9 {
		t.Errorf("total = %d, want 9", total)
	}
	want := []string{"1234567890", strings.Repeat("a", 128), "contraseña1", "áéíóúáéíóú", "with-crlf-ending", " leading and trailing space "}
	var wantHashes []uint64
	for _, w := range want {
		wantHashes = append(wantHashes, passwordpolicy.Hash(w))
	}
	slices.Sort(wantHashes)
	if !slices.Equal(hashes, wantHashes) {
		t.Fatalf("hashes = %v, want %v", hashes, wantHashes)
	}
}
