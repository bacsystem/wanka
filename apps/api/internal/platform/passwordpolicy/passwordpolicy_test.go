package passwordpolicy

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// known está en la fuente (SecLists, xato-net top 1M) con 11 caracteres; known en mayúsculas no.
const (
	known      = "password123"
	knownUpper = "PASSWORD123"
)

func newEmbedded(t testing.TB) *Policy {
	t.Helper()
	p, err := NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	return p
}

func TestHashVector(t *testing.T) {
	// Oráculo externo: printf qwertyuiop | shasum -a 256 -> 9a900403ac313ba2...
	want, _ := hex.DecodeString("9a900403ac313ba2")
	if got := Hash("qwertyuiop"); got != binary.BigEndian.Uint64(want) {
		t.Fatalf("Hash = %016x, want 9a900403ac313ba2", got)
	}
}

func TestCheckEmbedded(t *testing.T) {
	p := newEmbedded(t)
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		pw   string
		want error
	}{
		{"known from the source is rejected", known, ErrBreached},
		{"another known one", "1qaz2wsx3edc", ErrBreached},
		{"comparison is case sensitive", knownUpper, nil},
		{"random 32 chars is accepted", hex.EncodeToString(random), nil},
		{"trailing space is a different password", known + " ", nil},
		{"empty is accepted here (length is the caller's rule)", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.Check(tt.pw); !errors.Is(got, tt.want) {
				t.Fatalf("Check = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmbeddedSize(t *testing.T) {
	p := newEmbedded(t)
	if p.Len() < 100_000 {
		t.Fatalf("embedded list has %d entries, expected the filtered top 1M (over 100000)", p.Len())
	}
}

func TestBreachedError(t *testing.T) {
	err := BreachedError("newPassword")
	if err.Code != "validation_failed" || len(err.Fields) != 1 {
		t.Fatalf("unexpected error %+v", err)
	}
	f := err.Fields[0]
	if f.Field != "newPassword" || f.Code != "validation_failed" || f.Meta["rule"] != "breached" {
		t.Fatalf("unexpected field error %+v", f)
	}
	if got := httpx.Codes()[err.Code]; got != 422 {
		t.Fatalf("validation_failed maps to %d, want 422", got)
	}
}

func valid(t *testing.T) []byte {
	t.Helper()
	data, err := Encode([]uint64{1, 5, 9})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseRoundTrip(t *testing.T) {
	p, err := Parse(valid(t))
	if err != nil {
		t.Fatal(err)
	}
	if p.Len() != 3 || !slices.Equal(p.hashes, []uint64{1, 5, 9}) {
		t.Fatalf("round trip lost data: %v", p.hashes)
	}
}

func TestParseRejectsCorruptFiles(t *testing.T) {
	// Un cuerpo malo con encabezado y sha256 coherentes: solo el orden lo delata.
	unsorted := valid(t)
	binary.LittleEndian.PutUint64(unsorted[headerSize:], 7) // 7, 5, 9
	fixSum(unsorted)
	dup := valid(t)
	binary.LittleEndian.PutUint64(dup[headerSize+8:], 1) // 1, 1, 9
	fixSum(dup)

	tests := []struct {
		name string
		data func() []byte
	}{
		{"nil (missing file)", func() []byte { return nil }},
		{"shorter than the header", func() []byte { return valid(t)[:headerSize-1] }},
		{"bad magic", func() []byte { d := valid(t); d[0] = 'X'; return d }},
		{"unknown version", func() []byte { d := valid(t); d[4] = 9; return d }},
		{"size not a multiple of 8", func() []byte { d := valid(t); return d[:len(d)-3] }},
		{"trailing bytes with a coherent count and sha256", func() []byte {
			d := append(valid(t), 1, 2, 3, 4)
			fixSum(d)
			return d
		}},
		{"count does not match the body", func() []byte { d := valid(t); d[8] = 4; return d }},
		{"truncated by whole entries", func() []byte { d := valid(t); return d[:len(d)-8] }},
		{"body altered, sha256 does not match", func() []byte { d := valid(t); d[headerSize] ^= 0xff; return d }},
		{"not in ascending order", func() []byte { return unsorted }},
		{"duplicate entry", func() []byte { return dup }},
		{"zero entries", func() []byte {
			d := valid(t)[:headerSize]
			binary.LittleEndian.PutUint64(d[8:], 0)
			fixSum(d)
			return d
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse(tt.data())
			if !errors.Is(err, ErrCorrupt) || p != nil {
				t.Fatalf("Parse = %v, %v; want ErrCorrupt", p, err)
			}
		})
	}
}

// fixSum recalcula el sha256 del encabezado para el cuerpo actual.
func fixSum(data []byte) {
	sum := sha256.Sum256(data[headerSize:])
	copy(data[16:headerSize], sum[:])
}

func TestEncodeRejectsBadInput(t *testing.T) {
	for name, in := range map[string][]uint64{"empty": nil, "unsorted": {2, 1}, "duplicates": {1, 1}} {
		if _, err := Encode(in); err == nil {
			t.Errorf("%s: Encode accepted it", name)
		}
	}
}

// TestLookupLatency mide la magnitud del criterio: el p95 de una consulta con la lista real cargada.
func TestLookupLatency(t *testing.T) {
	p := newEmbedded(t)
	const n = 20000
	pws := make([]string, n)
	for i := range pws {
		pws[i] = "not-in-the-list-" + strings.Repeat("x", i%20) + string(rune('a'+i%26))
	}
	pws[0] = known
	durs := make([]time.Duration, n)
	for i, pw := range pws {
		start := time.Now()
		_ = p.Check(pw)
		durs[i] = time.Since(start)
	}
	slices.Sort(durs)
	p95 := durs[n*95/100]
	t.Logf("p95 = %v, max = %v over %d lookups", p95, durs[n-1], n)
	if p95 >= 5*time.Millisecond {
		t.Fatalf("p95 = %v, want < 5ms", p95)
	}
}

// TestMemoryCap mide el montón extra al cargar la lista real; el tope del PR es 16 MB.
func TestMemoryCap(t *testing.T) {
	const capBytes = 16 << 20
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	p := newEmbedded(t)
	runtime.GC()
	runtime.ReadMemStats(&after)
	extra := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("extra heap = %d bytes for %d entries", extra, p.Len())
	if extra > capBytes {
		t.Fatalf("extra heap %d bytes exceeds the %d cap", extra, capBytes)
	}
	runtime.KeepAlive(p)
}

func TestConcurrentChecks(t *testing.T) {
	p := newEmbedded(t)
	done := make(chan error, 8)
	for range 8 {
		go func() {
			for range 500 {
				if !errors.Is(p.Check(known), ErrBreached) || p.Check(knownUpper) != nil {
					done <- errors.New("wrong answer under concurrency")
					return
				}
			}
			done <- nil
		}()
	}
	for range 8 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func BenchmarkCheck(b *testing.B) {
	p := newEmbedded(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = p.Check("a-password-that-is-not-listed")
	}
}
