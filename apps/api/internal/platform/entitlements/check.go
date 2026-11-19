package entitlements

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
)

// NoModule es el valor reservado de «esta ruta o job no exige módulo» (ADR 0009,
// «Cómo se declara ninguno»). Es un literal: un string vacío no lo es y quien
// registra la ruta falla al arrancar.
const NoModule = "none"

// Códigos de problema que produce la comprobación (conventions.md §4).
const (
	CodeReadOnly           = "tenant_read_only"
	CodeSubscriptionIdle   = "subscription_inactive"
	CodeModuleNotEnabled   = "module_not_enabled"
	CodePlanLimitReached   = "plan_limit_reached"
	detailReadOnly         = "the company is closed and only reads are allowed"
	detailSubscriptionIdle = "the subscription is not active"
)

// Request es lo que una petición o un job pide a los entitlements.
type Request struct {
	// Method es el método HTTP. Vacío para un job: un job nunca es un GET, así
	// que una empresa cerrada no lo corre.
	Method string
	// Module es el módulo que exige; NoModule si ninguno.
	Module string
	// Limit es la clave del límite que consume (LimitUsers...); vacío si ninguno.
	Limit string
	// KeepsRunningWhenInactive deja pasar la suscripción morosa, suspendida o
	// cancelada: las rutas de la lista fija (exportar, ver el motivo) y los jobs
	// que terminan lo ya emitido (ADR 0005, «Suscripción inactiva»).
	KeepsRunningWhenInactive bool
}

// Denial es el motivo por el que se rechaza una petición o un job. Es un error
// para que un job lo devuelva tal cual.
type Denial struct {
	// Code es uno de CodeReadOnly, CodeSubscriptionIdle, CodeModuleNotEnabled o
	// CodePlanLimitReached.
	Code   string
	Detail string
	// Module va en module_not_enabled.
	Module string
	// Limit y Used van en plan_limit_reached.
	Limit, Used *int64
}

func (d *Denial) Error() string { return d.Code + ": " + d.Detail }

// Check aplica las reglas en el orden de la secuencia 08: empresa cerrada,
// suscripción, módulo, límite; gana la primera que falla. Devuelve nil si pasa.
// A una empresa cerrada no se le evalúa la suscripción (su único bloqueo es
// tenant_read_only). Falla cerrado: un módulo que no está en la lista, o un
// límite sin clave o con used >= limit, se rechaza.
func (e Entitlements) Check(r Request) *Denial {
	if e.ReadOnly {
		if r.Method != http.MethodGet {
			return &Denial{Code: CodeReadOnly, Detail: detailReadOnly}
		}
	} else if e.Status.Inactive() && !r.KeepsRunningWhenInactive {
		return &Denial{Code: CodeSubscriptionIdle, Detail: detailSubscriptionIdle}
	}
	if r.Module != NoModule && !e.HasModule(r.Module) {
		return &Denial{Code: CodeModuleNotEnabled, Detail: "the module is not enabled for this company", Module: r.Module}
	}
	if r.Limit != "" {
		limit, used := e.Limit(r.Limit), e.Usage[r.Limit]
		if used >= limit {
			return &Denial{Code: CodePlanLimitReached, Detail: "the plan limit for " + r.Limit + " has been reached", Limit: &limit, Used: &used}
		}
	}
	return nil
}

// Job es lo que un job de un tenant declara antes de correr su lógica.
type Job struct {
	// Module es el módulo del job; NoModule si ninguno.
	Module string
	// FinishesIssued marca el envío a Khipu de un CPE ya emitido y la recepción
	// de sus webhooks: siguen con la suscripción inactiva porque el comprobante
	// ya se entregó y tiene que llegar a SUNAT. Los demás jobs no inician
	// operaciones nuevas con la suscripción inactiva.
	FinishesIssued bool
}

// CheckJob es la misma comprobación que el middleware, para el job de una
// empresa: no corre con la empresa cerrada ni con el módulo deshabilitado, y no
// inicia operaciones con la suscripción inactiva. Devuelve nil si el job puede
// correr, un *Denial si no, y el error de la lectura si no se pudieron leer los
// entitlements (el job no corre: el que lo lanza lo reintenta).
func (s *Service) CheckJob(ctx context.Context, tenant pgtype.UUID, j Job) error {
	if j.Module == "" {
		return errors.New("entitlements: a job must declare its module or NoModule")
	}
	e, err := s.For(ctx, tenant)
	if err != nil {
		return fmt.Errorf("reading entitlements for a job: %w", err)
	}
	if d := e.Check(Request{Module: j.Module, KeepsRunningWhenInactive: j.FinishesIssued}); d != nil {
		return d
	}
	return nil
}
