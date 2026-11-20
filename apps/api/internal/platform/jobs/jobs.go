package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

// Colas del worker (ADR 0006): cada una con su concurrencia por réplica, para
// que una saturada no deje sin hilos a las demás.
const (
	QueueCPE           = "cpe"
	QueueWebhooks      = "webhooks"
	QueueNotifications = "notifications"
	QueueReports       = "reports"
	QueueMaintenance   = "maintenance"
	QueueEinvConfig    = "einv_config"
)

// DefaultConcurrency es la concurrencia por réplica de cada cola; las variables
// JOBS_CONCURRENCY_<COLA> la ajustan. Es la fuente de la lista de colas.
var DefaultConcurrency = map[string]int{
	QueueCPE:           20,
	QueueWebhooks:      10,
	QueueNotifications: 10,
	QueueReports:       2,
	QueueMaintenance:   2,
	QueueEinvConfig:    5,
}

// DefaultMaxAttempts es el máximo de intentos de un job (el primero cuenta).
// Con el backoff por defecto (10 s, duplicando hasta 1 h) los diez intentos
// cubren unos 85 minutos; un job que necesite más lo pide con WithMaxAttempts.
const DefaultMaxAttempts = 10

var (
	// ErrUnknownQueue: el job pide una cola que el worker no atiende o no pide ninguna.
	ErrUnknownQueue = errors.New("jobs: unknown queue")
	// ErrInvalidOption: una opción de encolado fuera de rango.
	ErrInvalidOption = errors.New("jobs: invalid option")
)

// Args son los argumentos de un job. Kind identifica al handler y es estable:
// está guardado en cada fila. Los argumentos se serializan a JSON; no llevan
// secretos ni datos personales, porque la tabla de jobs se ve desde la consola.
type Args interface {
	Kind() string
}

// TenantArgs son los argumentos de un job de tenant: llevan el tenant_id.
type TenantArgs interface {
	Args
	// Tenant devuelve el tenant dueño del job; vacío si falta.
	Tenant() pgtype.UUID
}

// Enqueuer encola jobs. Es el único tipo que ven los módulos para encolar.
type Enqueuer interface {
	// InsertTx inserta el job en tx, que debe ser la transacción del cambio de
	// datos: si tx hace ROLLBACK no hay job. Con WithUnique, un job igual en
	// curso hace que la llamada no inserte nada y no falle.
	InsertTx(ctx context.Context, tx pgx.Tx, args Args, opts ...Option) error
}

// Option ajusta un encolado.
type Option func(*insertOptions)

type insertOptions struct {
	queue       string
	maxAttempts int
	scheduledAt time.Time
	unique      bool
	runOnStart  bool
}

// WithQueue elige la cola (una de las constantes Queue*). Es obligatoria.
func WithQueue(name string) Option { return func(o *insertOptions) { o.queue = name } }

// WithMaxAttempts fija el máximo de intentos, en lugar de DefaultMaxAttempts.
func WithMaxAttempts(n int) Option { return func(o *insertOptions) { o.maxAttempts = n } }

// WithScheduledAt difiere la primera ejecución hasta t.
func WithScheduledAt(t time.Time) Option { return func(o *insertOptions) { o.scheduledAt = t } }

// WithUnique hace al job único mientras haya uno igual en curso (disponible,
// programado, en ejecución o por reintentar; uno ya terminado no cuenta). La
// clave son todos los argumentos serializados: dos jobs del mismo Kind con algún
// campo distinto no son el mismo job.
func WithUnique() Option { return func(o *insertOptions) { o.unique = true } }

// WithRunOnStart hace que un job periódico corra también al arrancar el worker. River no guarda el próximo turno:
// sin esto, un intervalo largo (24 h) se reinicia con cada despliegue y el job puede no correr nunca. Solo para jobs
// idempotentes (usar con WithUnique si hay varias réplicas).
func WithRunOnStart() Option { return func(o *insertOptions) { o.runOnStart = true } }

func resolveOptions(opts []Option) (insertOptions, error) {
	o := insertOptions{maxAttempts: DefaultMaxAttempts}
	for _, opt := range opts {
		opt(&o)
	}
	if _, ok := DefaultConcurrency[o.queue]; !ok {
		return o, fmt.Errorf("%w: %q", ErrUnknownQueue, o.queue)
	}
	if o.maxAttempts < 1 || o.maxAttempts > 100 {
		return o, fmt.Errorf("%w: max attempts must be between 1 and 100", ErrInvalidOption)
	}
	return o, nil
}

// cancelError marca un error definitivo: el job no se reintenta.
type cancelError struct{ err error }

func (e *cancelError) Error() string { return "job cancelled: " + e.err.Error() }
func (e *cancelError) Unwrap() error { return e.err }

// Cancel marca err como definitivo: el job queda cancelado, sin reintentos. El
// handler debe haber dejado antes el estado de negocio explícito (p. ej. el CPE
// «rechazado»). errors.Is y errors.As siguen viendo a err.
func Cancel(err error) error {
	if err == nil {
		err = errors.New("cancelled")
	}
	return &cancelError{err: err}
}

// snoozeError pide volver a correr el job más tarde.
type snoozeError struct{ d time.Duration }

func (e *snoozeError) Error() string { return fmt.Sprintf("job snoozed for %s", e.d) }

// Snooze pide que el job vuelva a correr pasado d, sin gastar un intento (p. ej.
// la cabeza de una serie de CPE está en backoff).
func Snooze(d time.Duration) error { return &snoozeError{d: d} }

// txRunner es lo que el Job necesita del pool; *db.Pool lo cumple.
type txRunner interface {
	InTenantTx(ctx context.Context, a db.Actor, fn func(pgx.Tx) error) error
}

// Job es lo que recibe un handler.
type Job[A Args] struct {
	ID          int64
	Attempt     int // 1 en la primera ejecución
	MaxAttempts int
	Args        A

	db txRunner
}

// InTenantTx abre una transacción de tenant corta como Actor{TenantID del job,
// Kind: system}, sin UserID (ADR 0001, «Workers»), y llama a fn. Si los
// argumentos no son TenantArgs o no traen tenant devuelve un error que cumple
// errors.Is(err, db.ErrNoTenant) y es definitivo (Cancel): reintentar no lo
// arregla. Es la única puerta del handler a las tablas de tenant.
func (j Job[A]) InTenantTx(ctx context.Context, fn func(pgx.Tx) error) error {
	ta, ok := any(j.Args).(TenantArgs)
	if !ok {
		return Cancel(fmt.Errorf("%w: %s arguments carry no tenant", db.ErrNoTenant, j.Args.Kind()))
	}
	err := j.db.InTenantTx(ctx, db.Actor{TenantID: ta.Tenant(), Kind: db.KindSystem}, fn)
	if errors.Is(err, db.ErrNoTenant) {
		return Cancel(err)
	}
	return err
}

// CompanySyncArgs son los argumentos del job de sincronización de datos de la empresa con Khipu (ADR 0013).
type CompanySyncArgs struct {
	TenantID pgtype.UUID `json:"tenant_id"`
}

func (CompanySyncArgs) Kind() string            { return "company_sync" }
func (a CompanySyncArgs) Tenant() pgtype.UUID   { return a.TenantID }

// SeriesReconcileArgs son los argumentos del job de conciliación de series con Khipu.
type SeriesReconcileArgs struct {
	TenantID pgtype.UUID `json:"tenant_id"`
}

func (SeriesReconcileArgs) Kind() string          { return "series_reconcile" }
func (a SeriesReconcileArgs) Tenant() pgtype.UUID { return a.TenantID }

// EmpresasActivationSLACheckArgs son los argumentos del job de verificación de SLA de activación (#31).
type EmpresasActivationSLACheckArgs struct {
	TenantID pgtype.UUID `json:"tenant_id"`
}

func (EmpresasActivationSLACheckArgs) Kind() string          { return "empresas_activation_sla_check" }
func (a EmpresasActivationSLACheckArgs) Tenant() pgtype.UUID { return a.TenantID }
