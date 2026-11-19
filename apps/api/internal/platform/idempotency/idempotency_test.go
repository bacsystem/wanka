package idempotency_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/platform/idempotency"
)

// Criterio 1: un POST (I) sin clave responde 422 validation_failed (campo Idempotency-Key) y no ejecuta
// nada; en los demás POST la clave es opcional.
func TestMissingOrInvalidKey(t *testing.T) {
	e := newEnv(t, envOpts{})
	t.Run("required route without a key", func(t *testing.T) {
		rec := e.post("/v1/items", "a", "", itemReq{Name: "x"})
		p := mustProblem(t, rec, http.StatusUnprocessableEntity, "validation_failed")
		if len(p.Errors) != 1 || p.Errors[0].Field != "Idempotency-Key" || p.Errors[0].Code != "required" {
			t.Fatalf("errors = %+v", p.Errors)
		}
		if e.calls.Load() != 0 || e.items("a") != 0 || e.keys() != 0 {
			t.Fatalf("something ran: calls %d items %d keys %d", e.calls.Load(), e.items("a"), e.keys())
		}
	})
	for name, key := range map[string]string{
		"not a uuid":       "abc",
		"uuid with braces": "{" + keyA + "}",
		"uuid too long":    keyA + "0",
		"hex out of range": strings.Replace(keyA, "0190", "01zz", 1),
		"misplaced dashes": strings.ReplaceAll(keyA, "-", "_"),
	} {
		t.Run("invalid: "+name, func(t *testing.T) {
			rec := e.post("/v1/items", "a", key, itemReq{Name: "x"})
			p := mustProblem(t, rec, http.StatusUnprocessableEntity, "validation_failed")
			if len(p.Errors) != 1 || p.Errors[0].Field != "Idempotency-Key" || p.Errors[0].Code != "invalid" {
				t.Fatalf("errors = %+v", p.Errors)
			}
			if e.calls.Load() != 0 {
				t.Fatal("the handler ran")
			}
		})
	}
	t.Run("two Idempotency-Key headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/items", strings.NewReader(`{"name":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Tenant", "a")
		req.Header.Add("Idempotency-Key", keyA)
		req.Header.Add("Idempotency-Key", keyB)
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		mustProblem(t, rec, http.StatusUnprocessableEntity, "validation_failed")
		if e.calls.Load() != 0 {
			t.Fatal("the handler ran")
		}
	})
	t.Run("optional route without a key runs the handler with its own transaction", func(t *testing.T) {
		rec := e.post("/v1/notes", "a", "", itemReq{Name: "free"})
		if rec.Code != http.StatusCreated || rec.Header().Get("Idempotent-Replayed") != "" {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		rec = e.post("/v1/notes", "a", "", itemReq{Name: "free2"})
		if rec.Code != http.StatusCreated || e.items("a") != 2 || e.keys() != 0 {
			t.Fatalf("status %d items %d keys %d", rec.Code, e.items("a"), e.keys())
		}
	})
	t.Run("optional route with a key behaves like a required one", func(t *testing.T) {
		first := e.post("/v1/notes", "a", keyB, itemReq{Name: "opt"})
		second := e.post("/v1/notes", "a", keyB, itemReq{Name: "opt"})
		if first.Code != http.StatusCreated || second.Code != http.StatusCreated || second.Header().Get("Idempotent-Replayed") != "true" {
			t.Fatalf("first %d, second %d replayed %q", first.Code, second.Code, second.Header().Get("Idempotent-Replayed"))
		}
		if e.count(`SELECT count(*) FROM rlstest.items WHERE name LIKE 'opt@%'`) != 1 {
			t.Fatal("the effect ran twice")
		}
	})
}

// Criterio 2: la clave se inserta primero, en la misma transacción que el efecto, y se guardan hash, código y respuesta.
func TestNewKeyIsInsertedFirstInTheEffectTransaction(t *testing.T) {
	e := newEnv(t, envOpts{})
	var inTxKeys, outsideKeys int
	e.onTx = func(ctx context.Context, tx pgx.Tx) {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM ops.idempotency_keys WHERE key = $1`, keyA).Scan(&inTxKeys); err != nil {
			t.Error(err)
		}
		// Otra conexión no ve la clave todavía: no confirmó.
		if err := e.admin.QueryRow(ctx, `SELECT count(*) FROM ops.idempotency_keys`).Scan(&outsideKeys); err != nil {
			t.Error(err)
		}
		if tx2, ok := idempotency.Tx(ctx); !ok || tx2 != tx {
			t.Error("idempotency.Tx(ctx) is not the transaction InTx passed")
		}
	}
	rec := e.post("/v1/items", "a", strings.ToUpper(keyA), itemReq{Name: "one"})
	if rec.Code != http.StatusCreated || rec.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("status %d replayed %q: %s", rec.Code, rec.Header().Get("Idempotent-Replayed"), rec.Body.String())
	}
	if inTxKeys != 1 {
		t.Fatalf("the handler's own transaction saw %d keys, want 1 (the key is inserted first)", inTxKeys)
	}
	if outsideKeys != 0 {
		t.Fatalf("another connection saw %d keys before the commit", outsideKeys)
	}
	var (
		status int
		hash   []byte
		body   []byte
		hdr    map[string][]string
		key    string
	)
	if err := e.admin.QueryRow(context.Background(), `SELECT key, status_code, request_hash, response, response_headers FROM ops.idempotency_keys`).
		Scan(&key, &status, &hash, &body, &hdr); err != nil {
		t.Fatal(err)
	}
	if key != keyA {
		t.Errorf("stored key %q, want the lowercase form %q", key, keyA)
	}
	if status != 201 || len(hash) != 32 || string(body) != rec.Body.String() {
		t.Errorf("stored status %d, hash %d bytes, body %q (sent %q)", status, len(hash), body, rec.Body.String())
	}
	if hdr["Location"][0] != "/v1/items/one" || hdr["Content-Type"][0] != "application/json" {
		t.Errorf("stored headers = %v", hdr)
	}
	if _, internal := hdr["X-Internal"]; internal {
		t.Errorf("an arbitrary handler header was stored: %v", hdr)
	}
	if e.items("a") != 1 {
		t.Fatalf("items = %d", e.items("a"))
	}
}

// Criterio 3 y 11: la misma clave con el mismo cuerpo repite código, cuerpo y cabeceras guardadas con
// Idempotent-Replayed: true y no repite el efecto; una clave de más de 24 h que el job no borró se sigue repitiendo.
func TestSameKeySameBodyReplays(t *testing.T) {
	e := newEnv(t, envOpts{})
	first := e.post("/v1/items", "a", keyA, `{"name":"one","sleep_ms":0}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first: %d %s", first.Code, first.Body.String())
	}
	cases := []struct{ name, key, body string }{
		{"identical", keyA, `{"name":"one","sleep_ms":0}`},
		{"other field order and spacing", keyA, "{ \"sleep_ms\": 0,\n  \"name\": \"one\" }"},
		{"key in upper case", strings.ToUpper(keyA), `{"name":"one","sleep_ms":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := e.post("/v1/items", "a", tc.key, tc.body)
			if rec.Code != first.Code || rec.Body.String() != first.Body.String() {
				t.Fatalf("replay %d %q, first %d %q", rec.Code, rec.Body.String(), first.Code, first.Body.String())
			}
			if rec.Header().Get("Idempotent-Replayed") != "true" || rec.Header().Get("Location") != "/v1/items/one" ||
				rec.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("replay headers = %v", rec.Header())
			}
			if rec.Header().Get("X-Internal") != "" {
				t.Fatal("a header that was not stored came back")
			}
			if rec.Header().Get("X-Request-Id") == "" {
				t.Fatal("the replay lost the request id")
			}
		})
	}
	if e.calls.Load() != 1 || e.items("a") != 1 || e.keys() != 1 {
		t.Fatalf("handler ran %d times, items %d, keys %d: the effect must happen once", e.calls.Load(), e.items("a"), e.keys())
	}

	// Criterio 11: más de 24 h y todavía sin purgar se trata como completada.
	if _, err := e.admin.Exec(context.Background(), `UPDATE ops.idempotency_keys SET created_at = now() - interval '30 hours'`); err != nil {
		t.Fatal(err)
	}
	rec := e.post("/v1/items", "a", keyA, `{"name":"one","sleep_ms":0}`)
	if rec.Header().Get("Idempotent-Replayed") != "true" || rec.Body.String() != first.Body.String() || e.calls.Load() != 1 {
		t.Fatalf("a 30 h old key was not replayed: %d %v calls %d", rec.Code, rec.Header(), e.calls.Load())
	}
}

// Una respuesta 2xx sin cuerpo (201 vacío) también se repite.
func TestEmptyBodyResponseReplays(t *testing.T) {
	e := newEnv(t, envOpts{})
	first := e.post("/v1/items", "a", keyA, itemReq{Name: "e", Fail: "created"})
	second := e.post("/v1/items", "a", keyA, itemReq{Name: "e", Fail: "created"})
	if first.Code != 201 || second.Code != 201 || second.Body.Len() != 0 || second.Header().Get("Idempotent-Replayed") != "true" || e.items("a") != 1 {
		t.Fatalf("first %d, second %d %q replayed %q items %d", first.Code, second.Code, second.Body.String(), second.Header().Get("Idempotent-Replayed"), e.items("a"))
	}
}

// Criterio 4: la misma clave con otro cuerpo (u otra ruta) es 422 idempotency_key_reused, nunca 409,
// no se guarda y no toca lo guardado.
func TestSameKeyDifferentRequestIsReused(t *testing.T) {
	e := newEnv(t, envOpts{})
	first := e.post("/v1/items", "a", keyA, itemReq{Name: "one"})
	for name, call := range map[string]func() *httptest.ResponseRecorder{
		"other body": func() *httptest.ResponseRecorder { return e.post("/v1/items", "a", keyA, itemReq{Name: "two"}) },
		"extra field": func() *httptest.ResponseRecorder {
			return e.post("/v1/items", "a", keyA, itemReq{Name: "one", SleepMS: 1})
		},
		"other route": func() *httptest.ResponseRecorder { return e.post("/v1/notes", "a", keyA, itemReq{Name: "one"}) },
	} {
		t.Run(name, func(t *testing.T) {
			rec := call()
			mustProblem(t, rec, http.StatusUnprocessableEntity, "idempotency_key_reused")
			if rec.Header().Get("Idempotent-Replayed") != "" {
				t.Fatal("a reused key must not be marked as replayed")
			}
		})
	}
	if e.calls.Load() != 1 || e.items("a") != 1 || e.keys() != 1 {
		t.Fatalf("calls %d items %d keys %d", e.calls.Load(), e.items("a"), e.keys())
	}
	// Lo guardado sigue siendo la respuesta original.
	again := e.post("/v1/items", "a", keyA, itemReq{Name: "one"})
	if again.Body.String() != first.Body.String() || again.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("stored response changed: %q", again.Body.String())
	}
}

// Criterio 5: dos peticiones simultáneas con la misma clave. La segunda espera; si la primera confirma, devuelve la
// respuesta guardada y el efecto ocurre una vez.
func TestConcurrentSameKeyWaitsAndReplays(t *testing.T) {
	e := newEnv(t, envOpts{})
	started := make(chan struct{})
	var once sync.Once
	e.onTx = func(ctx context.Context, tx pgx.Tx) { once.Do(func() { close(started) }) }
	var wg sync.WaitGroup
	var first, second *httptest.ResponseRecorder
	wg.Add(1)
	go func() {
		defer wg.Done()
		first = e.post("/v1/items", "a", keyA, itemReq{Name: "slow", SleepMS: 700})
	}()
	<-started
	t0 := time.Now()
	second = e.post("/v1/items", "a", keyA, itemReq{Name: "slow", SleepMS: 700})
	waited := time.Since(t0)
	wg.Wait()
	if first.Code != http.StatusCreated || first.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("first: %d %v", first.Code, first.Header())
	}
	if second.Code != http.StatusCreated || second.Header().Get("Idempotent-Replayed") != "true" || second.Body.String() != first.Body.String() {
		t.Fatalf("second: %d %v %q", second.Code, second.Header(), second.Body.String())
	}
	if waited < 300*time.Millisecond {
		t.Fatalf("the second request returned after %v: it did not wait for the first", waited)
	}
	if e.calls.Load() != 1 || e.items("a") != 1 {
		t.Fatalf("calls %d items %d", e.calls.Load(), e.items("a"))
	}
}

// Criterio 5 (rama de aborto): si la primera aborta, la segunda sigue como nueva y su efecto ocurre una vez.
func TestConcurrentSameKeyContinuesWhenTheFirstAborts(t *testing.T) {
	e := newEnv(t, envOpts{})
	started := make(chan struct{})
	var once sync.Once
	e.onTx = func(ctx context.Context, tx pgx.Tx) { once.Do(func() { close(started) }) }
	var wg sync.WaitGroup
	var first *httptest.ResponseRecorder
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Mismo cuerpo salvo que falla: el hash es el de esta petición, así que el cuerpo de la segunda debe ser idéntico.
		first = e.post("/v1/items", "a", keyA, itemReq{Name: "x", SleepMS: 500, Fail: "shift"})
	}()
	<-started
	second := e.post("/v1/items", "a", keyA, itemReq{Name: "x", SleepMS: 500, Fail: "shift"})
	wg.Wait()
	// Las dos piden lo mismo y el turno sigue cerrado: ambas ejecutaron el handler, ninguna guardó nada.
	mustProblem(t, first, http.StatusConflict, "shift_not_open")
	mustProblem(t, second, http.StatusConflict, "shift_not_open")
	if e.calls.Load() != 2 {
		t.Fatalf("the handler ran %d times: the second must continue as new after the first aborted", e.calls.Load())
	}
	if e.items("a") != 0 || e.keys() != 0 {
		t.Fatalf("items %d keys %d after two rollbacks", e.items("a"), e.keys())
	}
}

// Criterio 6: la espera supera lock_timeout: 409 request_in_progress con Retry-After: 1, el único caso de ese código.
// El 409 no se guarda y la primera conserva su clave.
func TestLockTimeoutIsRequestInProgress(t *testing.T) {
	e := newEnv(t, envOpts{lockTimeout: 300 * time.Millisecond})
	started := make(chan struct{})
	var once sync.Once
	e.onTx = func(ctx context.Context, tx pgx.Tx) { once.Do(func() { close(started) }) }
	var wg sync.WaitGroup
	var first *httptest.ResponseRecorder
	wg.Add(1)
	go func() {
		defer wg.Done()
		first = e.post("/v1/items", "a", keyA, itemReq{Name: "slow", SleepMS: 1500})
	}()
	<-started
	t0 := time.Now()
	second := e.post("/v1/items", "a", keyA, itemReq{Name: "slow", SleepMS: 1500})
	waited := time.Since(t0)
	p := mustProblem(t, second, http.StatusConflict, "request_in_progress")
	_ = p
	if second.Header().Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q", second.Header().Get("Retry-After"))
	}
	if waited < 250*time.Millisecond || waited > 1200*time.Millisecond {
		t.Fatalf("the 409 came after %v, want about the 300 ms lock_timeout", waited)
	}
	wg.Wait()
	if first.Code != http.StatusCreated {
		t.Fatalf("first: %d %s", first.Code, first.Body.String())
	}
	if e.calls.Load() != 1 || e.items("a") != 1 || e.keys() != 1 {
		t.Fatalf("calls %d items %d keys %d: the 409 must not run the handler nor store anything", e.calls.Load(), e.items("a"), e.keys())
	}
	// Terminada la primera, el reintento recibe la respuesta guardada.
	again := e.post("/v1/items", "a", keyA, itemReq{Name: "slow", SleepMS: 1500})
	if again.Header().Get("Idempotent-Replayed") != "true" || again.Body.String() != first.Body.String() {
		t.Fatalf("retry after the 409: %d %v", again.Code, again.Header())
	}
}

// El lock_timeout de la clave no se queda en la transacción: el resto del efecto vuelve al valor del rol.
func TestLockTimeoutIsLimitedToTheKeyInsert(t *testing.T) {
	e := newEnv(t, envOpts{})
	var during string
	e.onTx = func(ctx context.Context, tx pgx.Tx) {
		if err := tx.QueryRow(ctx, `SHOW lock_timeout`).Scan(&during); err != nil {
			t.Error(err)
		}
	}
	rec := e.post("/v1/items", "a", keyA, itemReq{Name: "x"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if during != "0" {
		t.Fatalf("lock_timeout inside the effect = %q, want the role's default (0)", during)
	}
}

// Criterios 7 y 10: un 4xx de negocio, un 5xx, un panic o un fallo del efecto hacen ROLLBACK: ni efecto ni clave, y el
// reintento con la misma clave se ejecuta de nuevo; arreglada la causa, responde 201 y el efecto ocurre una vez.
func TestNon2xxRollsBackKeyAndEffect(t *testing.T) {
	e := newEnv(t, envOpts{})
	t.Run("409 shift_not_open, then the shift opens and the retry succeeds", func(t *testing.T) {
		rec := e.post("/v1/items", "a", keyA, itemReq{Name: "sale", Fail: "shift"})
		mustProblem(t, rec, http.StatusConflict, "shift_not_open")
		if e.items("a") != 0 || e.keys() != 0 {
			t.Fatalf("after the 409: items %d keys %d (the handler wrote its effect, the kernel must roll it back)", e.items("a"), e.keys())
		}
		e.shiftOpen.Store(true)
		rec = e.post("/v1/items", "a", keyA, itemReq{Name: "sale", Fail: "shift"})
		if rec.Code != http.StatusCreated || rec.Header().Get("Idempotent-Replayed") != "" {
			t.Fatalf("retry: %d %v %s", rec.Code, rec.Header(), rec.Body.String())
		}
		if e.items("a") != 1 || e.keys() != 1 {
			t.Fatalf("after the retry: items %d keys %d", e.items("a"), e.keys())
		}
		third := e.post("/v1/items", "a", keyA, itemReq{Name: "sale", Fail: "shift"})
		if third.Header().Get("Idempotent-Replayed") != "true" || e.items("a") != 1 {
			t.Fatal("the effect must happen once")
		}
	})
	e.shiftOpen.Store(false)
	for _, tc := range []struct {
		name   string
		key    string
		body   itemReq
		status int
		code   string
	}{
		{"422 from the handler", "0190c1a2-3333-7000-8000-000000000001", itemReq{Name: "v1", Fail: "validation"}, 422, "validation_failed"},
		{"500 from the handler", "0190c1a2-3333-7000-8000-000000000002", itemReq{Name: "v2", Fail: "internal"}, 500, "internal"},
		{"panic in the handler", "0190c1a2-3333-7000-8000-000000000003", itemReq{Name: "v3", Fail: "panic"}, 500, "internal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := e.items("a")
			rec := e.post("/v1/items", "a", tc.key, tc.body)
			mustProblem(t, rec, tc.status, tc.code)
			if e.items("a") != before || e.count(`SELECT count(*) FROM ops.idempotency_keys WHERE key = $1`, tc.key) != 0 {
				t.Fatalf("the key or the effect survived a %d", tc.status)
			}
		})
	}
	t.Run("a 3xx is not a success: nothing is stored", func(t *testing.T) {
		before := e.items("a")
		rec := e.post("/v1/items", "a", "0190c1a2-3333-7000-8000-000000000004", itemReq{Name: "r", Fail: "redirect"})
		if rec.Code != http.StatusFound || e.items("a") != before || e.keys() != 1 {
			t.Fatalf("status %d items %d keys %d", rec.Code, e.items("a"), e.keys())
		}
	})
}

// Si vence el plazo del handler, el kernel responde 503 overloaded, la transacción se deshace y la clave no queda:
// el reintento se ejecuta de nuevo.
func TestHandlerDeadlineRollsBackKeyAndEffect(t *testing.T) {
	e := newEnv(t, envOpts{handlerTimeout: 400 * time.Millisecond})
	rec := e.post("/v1/items", "a", keyA, itemReq{Name: "late", SleepMS: 3000})
	mustProblem(t, rec, http.StatusServiceUnavailable, "overloaded")
	// El ROLLBACK corre en segundo plano al cancelarse el contexto; se espera a que suelte la fila.
	deadline := time.Now().Add(5 * time.Second)
	for e.keys() != 0 || e.items("a") != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("after the deadline: keys %d items %d", e.keys(), e.items("a"))
		}
		time.Sleep(20 * time.Millisecond)
	}
	rec = e.post("/v1/items", "a", keyA, itemReq{Name: "late"})
	if rec.Code != http.StatusCreated || rec.Header().Get("Idempotent-Replayed") != "" || e.items("a") != 1 {
		t.Fatalf("retry: %d %v items %d", rec.Code, rec.Header(), e.items("a"))
	}
}

// Si el COMMIT falla después de un 2xx del handler, el cliente no recibe el 2xx y la clave no queda.
func TestCommitFailureDoesNotSendTheHandlerResponse(t *testing.T) {
	e := newEnv(t, envOpts{})
	if rec := e.post("/v1/notes", "a", "", itemReq{Name: "dup"}); rec.Code != http.StatusCreated {
		t.Fatalf("seed: %d", rec.Code)
	}
	// UNIQUE (tenant_id, name) es diferida: el duplicado falla recién al COMMIT.
	rec := e.post("/v1/items", "a", keyA, itemReq{Name: "dup"})
	mustProblem(t, rec, http.StatusInternalServerError, "internal")
	if e.items("a") != 1 || e.keys() != 0 {
		t.Fatalf("items %d keys %d after a failed commit", e.items("a"), e.keys())
	}
}

// Criterio 8: la misma clave en dos tenants no interfiere, y un tenant nunca recibe la respuesta del otro.
func TestSameKeyInTwoTenantsDoesNotInterfere(t *testing.T) {
	e := newEnv(t, envOpts{})
	a := e.post("/v1/items", "a", keyA, itemReq{Name: "same"})
	b := e.post("/v1/items", "b", keyA, itemReq{Name: "same"})
	if a.Code != 201 || b.Code != 201 || b.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("a %d, b %d replayed %q", a.Code, b.Code, b.Header().Get("Idempotent-Replayed"))
	}
	if e.items("a") != 1 || e.items("b") != 1 || e.keys() != 2 || e.calls.Load() != 2 {
		t.Fatalf("items a %d b %d keys %d calls %d", e.items("a"), e.items("b"), e.keys(), e.calls.Load())
	}
	// Otro cuerpo en B con la clave de A: B no ve la fila de A, así que no es «reused» ni repite la respuesta de A.
	b2 := e.post("/v1/items", "b", keyB, itemReq{Name: "only-a"})
	a2 := e.post("/v1/items", "a", keyB, itemReq{Name: "different"})
	if b2.Code != 201 || a2.Code != 201 || a2.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("b2 %d a2 %d replayed %q: %s", b2.Code, a2.Code, a2.Header().Get("Idempotent-Replayed"), a2.Body.String())
	}
	if !strings.Contains(a2.Body.String(), "different") || strings.Contains(a2.Body.String(), "only-a") {
		t.Fatalf("tenant A received %q", a2.Body.String())
	}
	// Cada uno repite lo suyo.
	if ra := e.post("/v1/items", "a", keyA, itemReq{Name: "same"}); ra.Body.String() != a.Body.String() || ra.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("a replay: %q", ra.Body.String())
	}
	if rb := e.post("/v1/items", "b", keyA, itemReq{Name: "same"}); rb.Body.String() != b.Body.String() || rb.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("b replay: %q", rb.Body.String())
	}
}

// Una ruta con idempotencia montada sin sesión de tenant es un error de cableado: 500, no ejecuta el handler.
func TestRouteWithoutTenantSessionIsAnInternalError(t *testing.T) {
	e := newEnv(t, envOpts{})
	rec := e.post("/v1/nosession", "a", keyA, itemReq{Name: "x"})
	mustProblem(t, rec, http.StatusInternalServerError, "internal")
	if e.calls.Load() != 0 || e.keys() != 0 {
		t.Fatal("the handler ran or a key was stored")
	}
	if !strings.Contains(e.log.String(), "without a tenant session") {
		t.Fatalf("no log line for the wiring error: %s", e.log.String())
	}
}

// El cuerpo más grande que acepta la API también se corta aquí (413), aunque llegue sin Content-Length.
func TestBodyOverTheLimitIs413(t *testing.T) {
	e := newEnv(t, envOpts{})
	big := `{"name":"` + strings.Repeat("x", 1<<20+10) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/items", strings.NewReader(big))
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Tenant", "a")
	req.Header.Set("Idempotency-Key", keyA)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	mustProblem(t, rec, http.StatusRequestEntityTooLarge, "payload_too_large")
	if e.calls.Load() != 0 || e.keys() != 0 {
		t.Fatal("something ran")
	}
}

// Criterio no funcional: con 50 peticiones simultáneas con la misma clave, exactamente 1 ejecuta el efecto y las 49
// restantes responden con la respuesta guardada o 409 request_in_progress.
func TestFiftyConcurrentRequestsRunTheEffectOnce(t *testing.T) {
	e := newEnv(t, envOpts{lockTimeout: 2 * time.Second, maxConns: 60})
	const n = 50
	start := make(chan struct{})
	recs := make([]*httptest.ResponseRecorder, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			recs[i] = e.post("/v1/items", "a", keyA, itemReq{Name: "burst", SleepMS: 300})
		}()
	}
	close(start)
	wg.Wait()
	var created, replayed, inProgress int
	for _, rec := range recs {
		switch {
		case rec.Code == 201 && rec.Header().Get("Idempotent-Replayed") == "true":
			replayed++
		case rec.Code == 201:
			created++
		case rec.Code == 409 && strings.Contains(rec.Body.String(), "request_in_progress"):
			inProgress++
		default:
			t.Fatalf("unexpected response %d: %s", rec.Code, rec.Body.String())
		}
	}
	if created != 1 || created+replayed+inProgress != n {
		t.Fatalf("created %d replayed %d in_progress %d", created, replayed, inProgress)
	}
	if e.calls.Load() != 1 || e.items("a") != 1 || e.keys() != 1 {
		t.Fatalf("calls %d items %d keys %d", e.calls.Load(), e.items("a"), e.keys())
	}
	t.Logf("50 concurrent: 1 created, %d replayed, %d request_in_progress", replayed, inProgress)
}

// Criterio no funcional: p95 de la ruta de repetición < 50 ms.
func TestReplayLatencyP95(t *testing.T) {
	e := newEnv(t, envOpts{})
	if rec := e.post("/v1/items", "a", keyA, itemReq{Name: "warm"}); rec.Code != 201 {
		t.Fatal(rec.Body.String())
	}
	const n = 200
	lat := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		t0 := time.Now()
		rec := e.post("/v1/items", "a", keyA, itemReq{Name: "warm"})
		lat = append(lat, time.Since(t0))
		if rec.Header().Get("Idempotent-Replayed") != "true" {
			t.Fatal("not a replay")
		}
	}
	sortDurations(lat)
	p95 := lat[n*95/100]
	t.Logf("replay p50 %v p95 %v max %v", lat[n/2], p95, lat[n-1])
	if p95 >= 50*time.Millisecond {
		t.Fatalf("p95 = %v, want < 50 ms", p95)
	}
}

func sortDurations(d []time.Duration) {
	for i := 1; i < len(d); i++ {
		for j := i; j > 0 && d[j] < d[j-1]; j-- {
			d[j], d[j-1] = d[j-1], d[j]
		}
	}
}

// La clave es del principal: otro usuario de la misma empresa con la misma clave y el mismo cuerpo ejecuta su propia
// operación (sin 422) y nunca recibe la respuesta guardada del primero.
func TestSameKeyFromAnotherUserIsANewOperation(t *testing.T) {
	e := newEnv(t, envOpts{})
	u1 := e.post("/v1/items", "a", keyA, itemReq{Name: "same"})
	u2 := e.post("/v1/items", "a2", keyA, itemReq{Name: "same"})
	if u1.Code != 201 || u2.Code != 201 || u2.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("u1 %d, u2 %d replayed %q: %s", u1.Code, u2.Code, u2.Header().Get("Idempotent-Replayed"), u2.Body.String())
	}
	if u1.Body.String() == u2.Body.String() {
		t.Fatalf("u2 got u1's response: %s", u2.Body.String())
	}
	if e.calls.Load() != 2 || e.items("a") != 2 || e.keys() != 2 {
		t.Fatalf("calls %d items %d keys %d: each user must run the effect once", e.calls.Load(), e.items("a"), e.keys())
	}
	// Otro cuerpo de u2 con esa clave: tampoco es «reused» (no revela la clave de u1).
	if rec := e.post("/v1/items", "a2", keyB, itemReq{Name: "other"}); rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if rec := e.post("/v1/items", "a2", keyA, itemReq{Name: "other"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("u2 changing the body of its own key must be reused, got %d", rec.Code)
	}
	// Cada uno repite lo suyo.
	r1 := e.post("/v1/items", "a", keyA, itemReq{Name: "same"})
	r2 := e.post("/v1/items", "a2", keyA, itemReq{Name: "same"})
	if r1.Body.String() != u1.Body.String() || r2.Body.String() != u2.Body.String() ||
		r1.Header().Get("Idempotent-Replayed") != "true" || r2.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replays: %q %q", r1.Body.String(), r2.Body.String())
	}
}

// Una sesión sin usuario no puede tener clave: error de cableado (500), sin ejecutar nada.
func TestSessionWithoutUserIsAnInternalError(t *testing.T) {
	e := newEnv(t, envOpts{})
	rec := e.post("/v1/items", "nouser", keyA, itemReq{Name: "x"})
	mustProblem(t, rec, http.StatusInternalServerError, "internal")
	if e.calls.Load() != 0 || e.keys() != 0 {
		t.Fatal("something ran")
	}
	if !strings.Contains(e.log.String(), "without a user") {
		t.Fatalf("no log line for the wiring error: %s", e.log.String())
	}
}

// La idempotencia es lo último antes del handler: un paso de permiso puesto con Use, aunque se declare después,
// corre antes. Quien perdió el permiso recibe 403, no la respuesta guardada, y no se abre transacción ni se toca la clave.
func TestIdempotencyRunsAfterThePermissionSteps(t *testing.T) {
	e := newEnv(t, envOpts{})
	first := e.post("/v1/items", "a", keyA, itemReq{Name: "once"})
	if first.Code != 201 {
		t.Fatalf("%d %s", first.Code, first.Body.String())
	}
	e.denied.Store(true)
	rec := e.post("/v1/items", "a", keyA, itemReq{Name: "once"})
	mustProblem(t, rec, http.StatusForbidden, "forbidden")
	if rec.Header().Get("Idempotent-Replayed") != "" || strings.Contains(rec.Body.String(), "once") {
		t.Fatalf("a denied request got the stored response: %s", rec.Body.String())
	}
	// Una clave nueva rechazada tampoco deja fila.
	rec = e.post("/v1/items", "a", keyB, itemReq{Name: "new"})
	mustProblem(t, rec, http.StatusForbidden, "forbidden")
	if e.keys() != 1 || e.calls.Load() != 1 {
		t.Fatalf("keys %d calls %d: a rejected request must not touch the key", e.keys(), e.calls.Load())
	}
}

// Las peticiones que esperan por una clave en la misma réplica esperan en memoria: con 50 simultáneas con la misma
// clave, la réplica usa como máximo 1 conexión y otra sesión sigue atendida sin 503, aunque el pool tenga 2 conexiones.
func TestWaitingRequestsHoldNoConnection(t *testing.T) {
	e := newEnv(t, envOpts{maxConns: 2, acquireTimeout: 500 * time.Millisecond})
	ctx := context.Background()
	var maxBusy int
	stop := make(chan struct{})
	sampled := make(chan struct{})
	go func() {
		defer close(sampled)
		for {
			select {
			case <-stop:
				return
			default:
			}
			var n int
			if err := e.admin.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name = 'idempotency-test' AND state <> 'idle'`).Scan(&n); err == nil && n > maxBusy {
				maxBusy = n
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	started := make(chan struct{})
	var once sync.Once
	e.onTx = func(ctx context.Context, tx pgx.Tx) { once.Do(func() { close(started) }) }
	const n = 50
	recs := make([]*httptest.ResponseRecorder, n)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		recs[0] = e.post("/v1/items", "a", keyA, itemReq{Name: "burst", SleepMS: 1500})
	}()
	<-started
	for i := 1; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recs[i] = e.post("/v1/items", "a", keyA, itemReq{Name: "burst", SleepMS: 1500})
		}()
	}
	time.Sleep(300 * time.Millisecond) // las 49 ya están esperando
	other := e.post("/v1/notes", "b", "", itemReq{Name: "other session"})
	if other.Code != http.StatusCreated {
		t.Fatalf("another session got %d while 49 requests waited on a key: %s", other.Code, other.Body.String())
	}
	wg.Wait()
	close(stop)
	<-sampled
	replayed := 0
	for i, rec := range recs {
		if rec.Code != 201 {
			t.Fatalf("request %d: %d %s", i, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Idempotent-Replayed") == "true" {
			replayed++
		}
	}
	if replayed != n-1 || e.calls.Load() != 2 { // 1 de la ráfaga + la otra sesión
		t.Fatalf("replayed %d, handler calls %d", replayed, e.calls.Load())
	}
	if maxBusy > 2 { // la ráfaga (1) + la otra sesión (1); con espera en la base serían 2 de la ráfaga y la otra sesión fallaría
		t.Fatalf("up to %d busy connections", maxBusy)
	}
	t.Logf("max busy connections: %d (pool of 2)", maxBusy)
}

// Entre réplicas la espera ocurre en el INSERT (lock del índice único): la segunda réplica espera a la primera y repite
// su respuesta; si pasa de lock_timeout, 409 request_in_progress.
func TestTwoReplicasWaitInTheDatabase(t *testing.T) {
	e := newEnv(t, envOpts{lockTimeout: 500 * time.Millisecond})
	other := e.replica()
	started := make(chan struct{})
	var once sync.Once
	e.onTx = func(ctx context.Context, tx pgx.Tx) { once.Do(func() { close(started) }) }
	var wg sync.WaitGroup
	var first *httptest.ResponseRecorder
	wg.Add(1)
	go func() {
		defer wg.Done()
		first = e.post("/v1/items", "a", keyA, itemReq{Name: "slow", SleepMS: 250})
	}()
	<-started
	second := e.postTo(other, "/v1/items", "a", keyA, itemReq{Name: "slow", SleepMS: 250})
	wg.Wait()
	if first.Code != 201 || second.Code != 201 || second.Header().Get("Idempotent-Replayed") != "true" || second.Body.String() != first.Body.String() {
		t.Fatalf("first %d, second %d %v", first.Code, second.Code, second.Header())
	}
	if e.calls.Load() != 1 || e.items("a") != 1 {
		t.Fatalf("calls %d items %d", e.calls.Load(), e.items("a"))
	}

	// Ahora la primera tarda más que lock_timeout: la otra réplica recibe 409 de la base (55P03).
	once = sync.Once{}
	started = make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		first = e.post("/v1/items", "a", keyB, itemReq{Name: "slower", SleepMS: 1500})
	}()
	<-started
	t0 := time.Now()
	second = e.postTo(other, "/v1/items", "a", keyB, itemReq{Name: "slower", SleepMS: 1500})
	waited := time.Since(t0)
	mustProblem(t, second, http.StatusConflict, "request_in_progress")
	if second.Header().Get("Retry-After") != "1" || waited < 400*time.Millisecond || waited > 1200*time.Millisecond {
		t.Fatalf("Retry-After %q after %v", second.Header().Get("Retry-After"), waited)
	}
	wg.Wait()
}
