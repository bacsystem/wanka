// Package idempotency hace que repetir un POST con la misma Idempotency-Key no
// repita su efecto (api/conventions.md §3).
//
// El middleware se monta solo en rutas Tenant (necesita la sesión con empresa).
// Con clave, abre él mismo la transacción de tenant de la petición, inserta
// primero la fila de ops.idempotency_keys (INSERT ... ON CONFLICT DO NOTHING) y
// deja la transacción en el contexto: el handler hace su efecto con InTx, en esa
// misma transacción, así que la clave y el efecto confirman o abortan juntos.
//
//   - Clave nueva: corre el handler. Si responde 2xx se guarda la respuesta en la
//     fila y se hace COMMIT; si responde cualquier otra cosa (4xx, 5xx) se hace
//     ROLLBACK, con lo que la clave no queda y un reintento se ejecuta de nuevo.
//     La respuesta solo se envía al cliente cuando la transacción terminó.
//   - Misma clave y mismo cuerpo, ya confirmada: repite el código, el cuerpo y las
//     cabeceras guardadas con Idempotent-Replayed: true, sin tocar el handler.
//   - Misma clave y otro cuerpo (o otra ruta): 422 idempotency_key_reused.
//   - Misma clave mientras la primera sigue en curso: en la misma réplica la
//     segunda espera en memoria, sin conexión de la base; entre réplicas espera
//     el INSERT el lock de la fila no confirmada. La espera total es de 3 s; si
//     vence, 409 request_in_progress con Retry-After: 1, el único caso de ese código.
//   - La clave es del principal (el usuario): otro usuario con la misma clave es
//     otra operación.
//
// Un 422 idempotency_key_reused, un 409 request_in_progress y toda respuesta no
// 2xx se escriben después del ROLLBACK y no se guardan.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/auth"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/httpx"
)

// Header es la cabecera de la petición con la clave.
const Header = "Idempotency-Key"

// ReplayedHeader la lleva la respuesta que se repite desde lo guardado.
const ReplayedHeader = "Idempotent-Replayed"

// DefaultLockTimeout es lo que espera una petición a que otra con la misma clave
// termine (conventions.md §3).
const DefaultLockTimeout = 3 * time.Second

// Contador de resultados; el label outcome es uno de los Outcome*.
const metricName = "idempotency_requests_total"

// Valores del label outcome del contador.
const (
	OutcomeNew        = "new"         // clave nueva, 2xx guardado
	OutcomeNotStored  = "not_stored"  // clave nueva, el handler no respondió 2xx: ROLLBACK
	OutcomeReplayed   = "replayed"    // respuesta repetida
	OutcomeReused     = "reused"      // otro cuerpo con la misma clave
	OutcomeInProgress = "in_progress" // venció el lock_timeout
)

// storedHeaders son las cabeceras de la respuesta que se guardan y se repiten.
var storedHeaders = []string{"Content-Type", "Location", "ETag"}

// Pool es lo que el middleware necesita del pool; *db.Pool lo cumple.
type Pool interface {
	InTenantTx(ctx context.Context, a db.Actor, fn func(pgx.Tx) error) error
}

// Config ajusta el middleware; los ceros son los valores del contrato.
type Config struct {
	// LockTimeout es lo que espera el INSERT a otra petición con la misma clave.
	LockTimeout time.Duration
}

// Middleware construye los pasos de ruta de idempotencia.
type Middleware struct {
	pool        Pool
	lockTimeout time.Duration
	locks       keyLocks
}

// New devuelve el middleware sobre pool. Entra en pánico si pool es nil: es un
// error de cableado y debe fallar al arrancar.
func New(pool Pool, c Config) *Middleware {
	if pool == nil {
		panic("idempotency: New needs a pool")
	}
	if c.LockTimeout <= 0 {
		c.LockTimeout = DefaultLockTimeout
	}
	return &Middleware{pool: pool, lockTimeout: c.LockTimeout, locks: keyLocks{held: map[string]chan struct{}{}}}
}

// Required es el paso de un POST marcado (I) en resources.md: sin Idempotency-Key
// responde 422 validation_failed y no ejecuta nada.
func (m *Middleware) Required() httpx.RouteOption { return m.step(true) }

// Optional es el paso de los demás POST: sin clave el handler corre como siempre
// (y abre su propia transacción); con clave, como Required.
func (m *Middleware) Optional() httpx.RouteOption { return m.step(false) }

func (m *Middleware) step(required bool) httpx.RouteOption {
	// Innermost: va después de la sesión, los limitadores y todo paso de ruta (Use), pegado al handler.
	return httpx.Innermost(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.serve(w, r, next, required)
		})
	})
}

type txKey struct{}

// Tx devuelve la transacción de la petición cuando la ruta corre con clave de
// idempotencia. Sin clave (ruta Optional) devuelve false.
func Tx(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}

// InTx corre fn en la transacción de la petición si hay una clave de
// idempotencia, y si no abre una InTenantTx con a. Es la forma en que un handler
// de una ruta con idempotencia hace su efecto: con clave, el efecto y la clave
// confirman juntos. Con la transacción de la petición, a y la confirmación son
// del middleware (a es el Actor de la misma sesión): fn no debe hacer COMMIT ni
// ROLLBACK, y un error de fn aborta la petición.
func InTx(ctx context.Context, pool Pool, a db.Actor, fn func(pgx.Tx) error) error {
	if tx, ok := Tx(ctx); ok {
		return fn(tx)
	}
	return pool.InTenantTx(ctx, a, fn)
}

// result es lo que decidió la transacción, para escribirlo cuando ya terminó.
type result struct {
	outcome string
	// stored: lo guardado que se repite (outcome replayed).
	stored storedResponse
	// rec: lo que respondió el handler (outcome new o not_stored).
	rec *recorder
}

func (m *Middleware) serve(w http.ResponseWriter, r *http.Request, next http.Handler, required bool) {
	values := r.Header.Values(Header)
	if len(values) == 0 {
		if required {
			httpx.WriteProblem(w, r, "validation_failed", "the Idempotency-Key header is required on this operation",
				httpx.FieldError{Field: Header, Code: "required"})
			return
		}
		next.ServeHTTP(w, r)
		return
	}
	key, ok := parseKey(values)
	if !ok {
		httpx.WriteProblem(w, r, "validation_failed", "the Idempotency-Key header must be a single UUID",
			httpx.FieldError{Field: Header, Code: "invalid"})
		return
	}
	s, ok := auth.FromContext(r.Context())
	if !ok || !s.HasTenant() {
		// El middleware solo se monta en rutas Tenant; llegar aquí es un error de cableado.
		httpx.Logger(r.Context()).ErrorContext(r.Context(), "idempotency middleware on a route without a tenant session", "route", httpx.Route(r))
		httpx.WriteProblem(w, r, "internal", "")
		return
	}
	if emptyUUID(s.Actor.UserID) {
		// La clave es del principal: sin usuario (sesión de sistema) no hay a quién ligarla.
		httpx.Logger(r.Context()).ErrorContext(r.Context(), "idempotency middleware on a session without a user", "route", httpx.Route(r))
		httpx.WriteProblem(w, r, "internal", "")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, r, bodyError(err))
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	hash := fingerprint(r, body)

	// Una sola petición por (tenant, usuario, clave) llega a la base en esta réplica; las demás esperan aquí, en
	// memoria y sin conexión, hasta lockTimeout en total. Entre réplicas espera el INSERT (insertKey).
	deadline := time.Now().Add(m.lockTimeout)
	wctx, cancel := context.WithDeadline(r.Context(), deadline)
	release, werr := m.locks.enter(wctx, s.Actor.TenantID.String()+"|"+s.Actor.UserID.String()+"|"+key)
	cancel()
	if werr != nil {
		if r.Context().Err() == nil {
			httpx.Metrics(r.Context()).Inc(metricName, "outcome", OutcomeInProgress)
			httpx.WriteProblem(w, r, "request_in_progress", "another request with the same Idempotency-Key is still running")
		}
		return // el cliente se fue o venció el plazo del handler: responde otro o nadie
	}
	defer release()

	var res result
	err = m.pool.InTenantTx(r.Context(), s.Actor, func(tx pgx.Tx) error {
		var err error
		res, err = m.inTx(tx, r, next, key, hash, time.Until(deadline))
		return err
	})
	var inProgress *inProgressError
	switch {
	case err == nil || errors.Is(err, errRollback):
		// La transacción ya terminó (COMMIT o ROLLBACK): recién ahora se responde.
		m.write(w, r, res)
	case errors.As(err, &inProgress):
		httpx.Metrics(r.Context()).Inc(metricName, "outcome", OutcomeInProgress)
		httpx.WriteProblem(w, r, "request_in_progress", "another request with the same Idempotency-Key is still running")
	default:
		// Incluye el COMMIT incierto (503 dependency_unavailable): la respuesta del handler no se envía.
		httpx.WriteError(w, r, err)
	}
}

// inProgressError marca el lock_timeout del INSERT de la clave.
type inProgressError struct{ err error }

func (e *inProgressError) Error() string { return "idempotency key is locked by another request" }
func (e *inProgressError) Unwrap() error { return e.err }

// errRollback aborta la transacción sin que sea un fallo: la petición termina
// con la respuesta que dejó el result. El ROLLBACK deshace la clave y el efecto.
var errRollback = errors.New("idempotency: roll back and answer")

func (m *Middleware) inTx(tx pgx.Tx, r *http.Request, next http.Handler, key string, hash []byte, budget time.Duration) (result, error) {
	ctx := r.Context()
	if budget < time.Millisecond {
		return result{}, &inProgressError{err: context.DeadlineExceeded} // la espera en memoria consumió los 3 s
	}
	inserted, err := m.insertKey(ctx, tx, key, hash, budget)
	if err != nil {
		return result{}, err
	}
	var existing *storedRow
	if !inserted {
		existing, err = readKey(ctx, tx, key)
		if err != nil {
			return result{}, err
		}
		if existing == nil {
			// La limpieza borró una clave de más de 24 h entre el conflicto y su lectura. El 500 se cura solo: el
			// reintento con la misma clave la inserta como nueva.
			return result{}, errors.New("idempotency key was purged between the conflict and its read")
		}
	}

	if existing != nil {
		return replayOrReuse(existing, hash)
	}

	rec := newRecorder()
	next.ServeHTTP(rec, r.WithContext(context.WithValue(ctx, txKey{}, tx)))
	if err := ctx.Err(); err != nil {
		// Venció el plazo o el cliente se fue: no se confirma nada; el middleware de plazo ya respondió.
		return result{}, err
	}
	if !rec.is2xx() {
		return result{outcome: OutcomeNotStored, rec: rec}, errRollback
	}
	if err := saveResponse(ctx, tx, key, rec); err != nil {
		return result{}, err
	}
	return result{outcome: OutcomeNew, rec: rec}, nil
}

// insertKey inserta la fila de la clave con lock_timeout acotado. Devuelve false
// si la clave ya existía confirmada (esperó a la otra petición si seguía en curso).
func (m *Middleware) insertKey(ctx context.Context, tx pgx.Tx, key string, hash []byte, budget time.Duration) (bool, error) {
	b := &pgx.Batch{}
	// set_config con is_local = true es el SET LOCAL con parámetro; RESET devuelve el valor del rol
	// para que el resto del efecto no herede el límite.
	b.Queue(`SELECT set_config('lock_timeout', $1, true)`, strconv.FormatInt(budget.Milliseconds(), 10)+"ms")
	b.Queue(`INSERT INTO ops.idempotency_keys (tenant_id, key, request_hash)
		VALUES (app.current_tenant(), $1, $2) ON CONFLICT DO NOTHING`, key, hash)
	b.Queue(`RESET lock_timeout`)
	br := tx.SendBatch(ctx, b)
	defer br.Close()
	if _, err := br.Exec(); err != nil {
		return false, err
	}
	tag, err := br.Exec()
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			return false, &inProgressError{err: err}
		}
		return false, err
	}
	if _, err := br.Exec(); err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

type storedRow struct {
	hash    []byte
	status  *int16
	body    []byte
	headers map[string][]string
}

type storedResponse struct {
	status  int
	body    []byte
	headers map[string][]string
}

func readKey(ctx context.Context, tx pgx.Tx, key string) (*storedRow, error) {
	var row storedRow
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT request_hash, status_code, response, response_headers
		FROM ops.idempotency_keys WHERE tenant_id = app.current_tenant() AND principal_id = app.current_user_id() AND key = $1`, key).
		Scan(&row.hash, &row.status, &row.body, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &row.headers); err != nil {
			return nil, fmt.Errorf("decoding stored response headers: %w", err)
		}
	}
	return &row, nil
}

func replayOrReuse(row *storedRow, hash []byte) (result, error) {
	if !bytes.Equal(row.hash, hash) {
		return result{outcome: OutcomeReused}, errRollback
	}
	if row.status == nil {
		// Una fila confirmada sin respuesta no debería existir: el INSERT y el UPDATE van en la misma transacción.
		return result{}, errors.New("stored idempotency key has no response")
	}
	return result{outcome: OutcomeReplayed, stored: storedResponse{status: int(*row.status), body: row.body, headers: row.headers}}, errRollback
}

func saveResponse(ctx context.Context, tx pgx.Tx, key string, rec *recorder) error {
	headers := map[string][]string{}
	for _, name := range storedHeaders {
		if v := rec.Header().Values(name); len(v) > 0 {
			headers[name] = v
		}
	}
	raw, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("encoding response headers: %w", err)
	}
	tag, err := tx.Exec(ctx, `UPDATE ops.idempotency_keys SET status_code = $2, response = $3, response_headers = $4
		WHERE tenant_id = app.current_tenant() AND principal_id = app.current_user_id() AND key = $1`, key, int16(rec.status()), rec.body.Bytes(), raw)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("saving the idempotency response updated %d rows", tag.RowsAffected())
	}
	return nil
}

// write envía lo decidido, ya con la transacción terminada.
func (m *Middleware) write(w http.ResponseWriter, r *http.Request, res result) {
	httpx.Metrics(r.Context()).Inc(metricName, "outcome", res.outcome)
	switch res.outcome {
	case OutcomeReplayed:
		h := w.Header()
		for name, vv := range res.stored.headers {
			h[http.CanonicalHeaderKey(name)] = vv
		}
		h.Set(ReplayedHeader, "true")
		w.WriteHeader(res.stored.status)
		_, _ = w.Write(res.stored.body)
	case OutcomeReused:
		httpx.WriteProblem(w, r, "idempotency_key_reused", "the Idempotency-Key was already used with a different request")
	default:
		res.rec.flush(w)
	}
}

// parseKey valida que haya una sola clave y que sea un UUID; la devuelve en
// minúsculas para que la misma clave en otra caja sea la misma clave.
func parseKey(values []string) (string, bool) {
	if len(values) != 1 {
		return "", false
	}
	k := values[0]
	if len(k) != 36 {
		return "", false
	}
	for i := 0; i < len(k); i++ {
		c := k[i]
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if c != '-' {
				return "", false
			}
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return "", false
		}
	}
	return strings.ToLower(k), true
}

// fingerprint es el sha256 de método, ruta con query y cuerpo canónico. La ruta
// va dentro para que la misma clave en otra operación no repita la respuesta de
// esta.
func fingerprint(r *http.Request, body []byte) []byte {
	h := sha256.New()
	h.Write([]byte(r.Method))
	h.Write([]byte{0})
	h.Write([]byte(r.URL.RequestURI()))
	h.Write([]byte{0})
	h.Write(canonicalBody(r.Header.Get("Content-Type"), body))
	return h.Sum(nil)
}

// canonicalBody normaliza un cuerpo JSON (claves de objeto ordenadas, sin
// espacios, mismas cifras) para que un reintento con otro orden de campos no sea
// «otro cuerpo». Cualquier otra cosa, o un JSON inválido, se usa tal cual: el
// handler lo rechazará y el ROLLBACK no deja la clave.
func canonicalBody(contentType string, body []byte) []byte {
	if mt, _, err := mime.ParseMediaType(contentType); err != nil || mt != "application/json" {
		return body
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return body
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return body
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return body
	}
	return bytes.TrimRight(out.Bytes(), "\n")
}

func bodyError(err error) error {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return httpx.NewError("payload_too_large", "request body exceeds "+strconv.Itoa(httpx.MaxBodyBytes)+" bytes")
	}
	return httpx.NewError("malformed_request", "the request body could not be read")
}

func emptyUUID(u pgtype.UUID) bool { return !u.Valid || u.Bytes == [16]byte{} }

// keyLocks deja pasar a la base una petición por id a la vez; las otras esperan
// en memoria, sin conexión del pool.
type keyLocks struct {
	mu   sync.Mutex
	held map[string]chan struct{}
}

// enter toma id o espera a que lo suelte quien lo tiene. Devuelve el error del
// contexto si este termina antes. El release es obligatorio.
func (l *keyLocks) enter(ctx context.Context, id string) (release func(), err error) {
	for {
		l.mu.Lock()
		ch, busy := l.held[id]
		if !busy {
			ch = make(chan struct{})
			l.held[id] = ch
			l.mu.Unlock()
			return func() {
				l.mu.Lock()
				delete(l.held, id)
				l.mu.Unlock()
				close(ch)
			}, nil
		}
		l.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
