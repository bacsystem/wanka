package entitlements

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

// ErrTenantNotFound: la empresa no existe (o se purgó). No es «sin plan»: una
// sesión con un tenant que no existe es un dato roto, y no se cachea.
var ErrTenantNotFound = errors.New("entitlements: tenant not found")

// limaZone es la hora de Lima (UTC-5, sin horario de verano): el mes de uso se
// cuenta con ella.
var limaZone = time.FixedZone("America/Lima", -5*60*60)

// usagePeriod es el primer día del mes de now en Lima.
func usagePeriod(now time.Time) time.Time {
	n := now.In(limaZone)
	return time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// loadSQL es la lectura completa en UNA consulta (sin N+1). Todo se filtra por
// app.current_tenant(), que fija InTenantReadTx: las tablas de platform no tienen
// RLS, así que ese filtro es lo único que separa a una empresa de otra y no
// depende de un parámetro que un llamador pueda equivocar. La suscripción que
// cuenta es la viva (no cancelada); si solo hay canceladas, la más reciente.
// Los módulos salen del plan (plan_modules) con el override del tenant encima.
const loadSQL = `
WITH sub AS (
  SELECT s.plan_id, s.status
    FROM platform.subscriptions s
   WHERE s.tenant_id = app.current_tenant()
   ORDER BY (s.status <> 'cancelada') DESC, s.created_at DESC
   LIMIT 1
)
SELECT t.status,
       sub.status,
       COALESCE(p.limits, '{}'::jsonb),
       COALESCE((SELECT jsonb_agg(m.code ORDER BY m.code)
                   FROM platform.modules m
                   LEFT JOIN platform.plan_modules pm ON pm.plan_id = sub.plan_id AND pm.module_code = m.code
                   LEFT JOIN platform.tenant_module_overrides o
                          ON o.tenant_id = app.current_tenant() AND o.module_code = m.code
                  WHERE COALESCE(o.enabled, pm.module_code IS NOT NULL)), '[]'::jsonb),
       COALESCE((SELECT jsonb_agg(jsonb_build_object('key', f.key, 'rolloutPct', f.rollout_pct,
                                                     'defaultOn', f.default_on, 'override', fo.enabled))
                   FROM platform.feature_flags f
                   LEFT JOIN platform.tenant_flag_overrides fo
                          ON fo.tenant_id = app.current_tenant() AND fo.flag_key = f.key), '[]'::jsonb),
       (SELECT jsonb_build_object('users', u.active_users, 'cpe_per_month', u.cpe_issued)
          FROM platform.tenant_usage u
         WHERE u.tenant_id = app.current_tenant() AND u.period = $1)
  FROM org.tenants t
  LEFT JOIN sub ON true
  LEFT JOIN platform.plans p ON p.id = sub.plan_id
 WHERE t.id = app.current_tenant()`

// flagRow es un feature flag tal como sale de la consulta.
type flagRow struct {
	Key        string `json:"key"`
	RolloutPct int    `json:"rolloutPct"`
	DefaultOn  bool   `json:"defaultOn"`
	Override   *bool  `json:"override"`
}

// rolloutBucket reparte las empresas en 0..99 por hash(tenant_id, flag): el mismo
// par da el mismo número en cualquier réplica y en cualquier arranque.
func rolloutBucket(tenant pgtype.UUID, key string) int {
	h := sha256.New()
	h.Write([]byte(tenant.String()))
	h.Write([]byte{0})
	h.Write([]byte(key))
	return int(binary.BigEndian.Uint64(h.Sum(nil)[:8]) % 100)
}

// flagEnabled: un override del tenant gana; si no, default_on lo enciende para
// todos; si no, entra el porcentaje de empresas que fija rollout_pct.
func flagEnabled(tenant pgtype.UUID, f flagRow) bool {
	switch {
	case f.Override != nil:
		return *f.Override
	case f.DefaultOn:
		return true
	default:
		return rolloutBucket(tenant, f.Key) < f.RolloutPct
	}
}

// load lee y arma los entitlements de la empresa. El error de la base sale
// envuelto (errors.Is con db.ErrOverloaded, db.ErrUnavailable,
// db.ErrStatementTimeout); nunca se devuelve un Entitlements parcial junto a un error.
func load(ctx context.Context, d DB, tenant pgtype.UUID, now time.Time) (Entitlements, error) {
	var (
		tenantStatus            string
		subStatus               *string
		limitsJSON, modulesJSON []byte
		flagsJSON, usageJSON    []byte
	)
	// KindSystem: no hay usuario que pasar, solo la empresa cuyos datos se leen.
	err := d.InTenantReadTx(ctx, db.Actor{TenantID: tenant, Kind: db.KindSystem}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, loadSQL, usagePeriod(now)).
			Scan(&tenantStatus, &subStatus, &limitsJSON, &modulesJSON, &flagsJSON, &usageJSON)
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Entitlements{}, ErrTenantNotFound
	case err != nil:
		return Entitlements{}, fmt.Errorf("reading entitlements: %w", err)
	}
	return build(tenant, tenantStatus, subStatus, limitsJSON, modulesJSON, flagsJSON, usageJSON)
}

// build convierte la fila en Entitlements. Aquí, y solo aquí, se decide que sin
// suscripción usable no hay módulos ni flags (falla cerrado), salvo en la
// empresa cerrada, que conserva los de su último plan.
func build(tenant pgtype.UUID, tenantStatus string, subStatus *string, limitsJSON, modulesJSON, flagsJSON, usageJSON []byte) (Entitlements, error) {
	e := Entitlements{
		Modules:  []string{},
		Flags:    []string{},
		Limits:   map[string]int64{},
		Usage:    map[string]int64{LimitUsers: 0, LimitCPEPerMonth: 0},
		Status:   StatusNoPlan,
		ReadOnly: tenantStatus == "cerrada",
	}
	if subStatus != nil {
		e.Status = Status(*subStatus)
	}
	if err := json.Unmarshal(limitsJSON, &e.Limits); err != nil {
		return Entitlements{}, fmt.Errorf("plan limits are not numbers: %w", err)
	}
	if len(usageJSON) > 0 {
		var u map[string]int64
		if err := json.Unmarshal(usageJSON, &u); err != nil {
			return Entitlements{}, fmt.Errorf("decoding usage: %w", err)
		}
		maps.Copy(e.Usage, u)
	}
	// Una empresa cerrada conserva los módulos y flags de su último plan aunque la suscripción esté caída: su lectura no depende del pago (ADR 0005, «Empresa cerrada»).
	if e.Status.Usable() || e.ReadOnly {
		if err := json.Unmarshal(modulesJSON, &e.Modules); err != nil {
			return Entitlements{}, fmt.Errorf("decoding modules: %w", err)
		}
		var flags []flagRow
		if err := json.Unmarshal(flagsJSON, &flags); err != nil {
			return Entitlements{}, fmt.Errorf("decoding flags: %w", err)
		}
		for _, f := range flags {
			if flagEnabled(tenant, f) {
				e.Flags = append(e.Flags, f.Key)
			}
		}
		slices.Sort(e.Modules)
		slices.Sort(e.Flags)
	}
	e.Version = computeVersion(e)
	return e, nil
}
