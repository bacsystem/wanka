package argon2_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/adapters/argon2"
)

// fast son parámetros baratos solo para los tests; los de producción están en argon2.Default.
var fast = argon2.Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1}

func newHasher(t *testing.T) *argon2.Hasher {
	t.Helper()
	h, err := argon2.New(fast)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestHashVerifyRoundTrip(t *testing.T) {
	h := newHasher(t)
	enc, err := h.Hash("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "$argon2id$v=19$m=64,t=1,p=1$") {
		t.Fatalf("hash = %q, want PHC argon2id with the parameters used", enc)
	}
	for _, tc := range []struct {
		name, pw string
		want     bool
	}{{"same", "correct horse", true}, {"other", "correct horsf", false}, {"empty", "", false}, {"prefix", "correct hors", false}} {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := h.Verify(tc.pw, enc)
			if err != nil || ok != tc.want {
				t.Fatalf("Verify = %v, %v, want %v, nil", ok, err, tc.want)
			}
		})
	}
	enc2, _ := h.Hash("correct horse")
	if enc == enc2 {
		t.Fatal("two hashes of the same password share the salt")
	}
}

// Un hash guardado con otros parámetros se verifica con los suyos: cambiar argon2.Default no encierra a nadie.
func TestVerifyUsesTheParametersOfTheStoredHash(t *testing.T) {
	old := newHasher(t)
	enc, _ := old.Hash("pw")
	other, err := argon2.New(argon2.Params{MemoryKiB: 128, Iterations: 2, Parallelism: 2})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := other.Verify("pw", enc); err != nil || !ok {
		t.Fatalf("Verify with other params = %v, %v", ok, err)
	}
}

func TestMalformedStoredHashesNeverMatch(t *testing.T) {
	h := newHasher(t)
	good, _ := h.Hash("pw")
	parts := strings.Split(good, "$")
	join := func(p []string) string { return strings.Join(p, "$") }
	with := func(i int, v string) string { c := append([]string(nil), parts...); c[i] = v; return join(c) }
	for name, enc := range map[string]string{
		"empty":            "",
		"plain text":       "pw",
		"bcrypt":           "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012345",
		"argon2i":          with(1, "argon2i"),
		"old version":      with(2, "v=16"),
		"missing part":     join(parts[:5]),
		"extra part":       good + "$x",
		"no params":        with(3, ""),
		"param order":      with(3, "t=1,m=64,p=1"),
		"zero iterations":  with(3, "m=64,t=0,p=1"),
		"zero parallelism": with(3, "m=64,t=1,p=0"),
		"huge memory":      with(3, "m=4294967295,t=1,p=1"),
		"huge iterations":  with(3, "m=64,t=1000,p=1"),
		"huge lanes":       with(3, "m=64,t=1,p=255"),
		"trailing junk":    with(3, "m=64,t=1,p=1,x=2"),
		"short salt":       with(4, "AAAA"),
		"padded salt":      with(4, "AAAAAAAAAAAAAAAAAAAAAA=="),
		"bad key":          with(5, "!!!"),
		"short key":        with(5, "AAAA"),
	} {
		t.Run(name, func(t *testing.T) {
			ok, err := h.Verify("pw", enc)
			if ok || !errors.Is(err, argon2.ErrMalformedHash) {
				t.Fatalf("Verify = %v, %v, want false and ErrMalformedHash", ok, err)
			}
		})
	}
}

func TestInvalidParams(t *testing.T) {
	for name, p := range map[string]argon2.Params{
		"zero":            {},
		"no iterations":   {MemoryKiB: 64, Parallelism: 1},
		"no lanes":        {MemoryKiB: 64, Iterations: 1},
		"too little mem":  {MemoryKiB: 7, Iterations: 1, Parallelism: 1},
		"mem under lanes": {MemoryKiB: 15, Iterations: 1, Parallelism: 2},
		"huge memory":     {MemoryKiB: 1<<20 + 1, Iterations: 1, Parallelism: 1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := argon2.New(p); err == nil {
				t.Fatal("New accepted the parameters")
			}
		})
	}
	if _, err := argon2.New(argon2.Default); err != nil {
		t.Fatalf("the default parameters are invalid: %v", err)
	}
}
