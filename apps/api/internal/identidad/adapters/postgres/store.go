// Package postgres es el adaptador de persistencia del login sobre iam.users, iam.sessions y la vista
// iam.list_my_memberships. Todo acceso es sin tenant, por db.InNoTenantTx (ADR 0001).
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/app"
	"github.com/bacsystem/wanka/apps/api/internal/identidad/domain"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/jobs"
)

// maxUserAgent es el tope de iam.sessions.user_agent (conventions.md §3 y ADR 0004).
const maxUserAgent = 512

// Store implementa app.Store.
type Store struct {
	pool *db.Pool
	enq  jobs.Enqueuer
}

// New arma el Store. enq encola el aviso del bloqueo en la transacción del cambio.
func New(pool *db.Pool, enq jobs.Enqueuer) *Store { return &Store{pool: pool, enq: enq} }

// chargeSQL cobra un intento: ADR 0004, nota del 2026-11-19. Un solo UPDATE, así que N peticiones
// simultáneas se serializan en el candado de la fila y cada una ve el contador de la anterior. La fila
// solo se cobra si la cuenta puede intentar: activa, con contraseña, sin bloqueo vigente, bajo el techo de
// bloqueos y con cupo en la ventana (o con la ventana vencida, que empieza otra). $1 correo, $2 ventana en
// segundos, $3 intentos por ventana, $4 techo de bloqueos.
const chargeSQL = `
UPDATE iam.users SET
  failed_logins = CASE WHEN login_window_started_at IS NULL OR login_window_started_at <= now() - make_interval(secs => $2)
                       THEN 1 ELSE failed_logins + 1 END,
  login_window_started_at = CASE WHEN login_window_started_at IS NULL OR login_window_started_at <= now() - make_interval(secs => $2)
                                 THEN now() ELSE login_window_started_at END,
  login_failure_streak = login_failure_streak + 1
WHERE email = $1 AND status = 'activo' AND password_hash IS NOT NULL
  AND (locked_until IS NULL OR locked_until <= now())
  AND login_failure_streak < $4
  AND (failed_logins < $3 OR login_window_started_at <= now() - make_interval(secs => $2))
RETURNING id::text, password_hash, failed_logins, login_failure_streak`

// ChargeAttempt implementa app.Store.
func (s *Store) ChargeAttempt(ctx context.Context, email string) (app.Account, bool, error) {
	var acct app.Account
	found := false
	err := s.pool.InNoTenantTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, chargeSQL, email, domain.Window.Seconds(), domain.MaxAttempts, domain.MaxStreak).
			Scan(&acct.ID, &acct.PasswordHash, &acct.FailedLogins, &acct.FailureStreak)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	if err != nil {
		return app.Account{}, false, err
	}
	return acct, found, nil
}

// InUserTx implementa app.Store.
func (s *Store) InUserTx(ctx context.Context, userID string, fn func(app.UserTx) error) error {
	var id pgtype.UUID
	if err := id.Scan(userID); err != nil || !id.Valid {
		return fmt.Errorf("invalid user id: %w", err)
	}
	return s.pool.InNoTenantTx(ctx, db.NoTenantScope{UserID: id}, func(tx pgx.Tx) error {
		return fn(&userTx{tx: tx, id: userID, enq: s.enq})
	})
}

type userTx struct {
	tx  pgx.Tx
	id  string
	enq jobs.Enqueuer
}

func (u *userTx) Lock(ctx context.Context) (app.UserState, error) {
	var (
		st          app.UserState
		status      string
		hash        *string
		failed      int
		mfaLockouts int
		mfaFor      float64
		isLocked    bool
	)
	err := u.tx.QueryRow(ctx, `
		SELECT status, password_hash, failed_logins, login_failure_streak, coalesce(locked_until > now(), false), mfa_lockouts,
		       coalesce(ceil(extract(epoch FROM mfa_locked_until - now())), 0)::float8
		  FROM iam.users WHERE id = $1::uuid FOR UPDATE`, u.id).
		Scan(&status, &hash, &failed, &st.FailureStreak, &isLocked, &mfaLockouts, &mfaFor)
	if err != nil {
		return app.UserState{}, fmt.Errorf("locking the user row: %w", err)
	}
	st.Active = status == "activo" && hash != nil
	if hash != nil {
		st.PasswordHash = *hash
	}
	st.FailedLogins, st.Locked, st.MFALockouts = failed, isLocked, mfaLockouts
	if mfaFor > 0 {
		st.MFALockedFor = time.Duration(mfaFor * float64(time.Second))
	}
	return st, nil
}

func (u *userTx) ClearLoginCounters(ctx context.Context) error {
	_, err := u.tx.Exec(ctx, `UPDATE iam.users SET failed_logins = 0, login_lockouts = 0, login_failure_streak = 0, login_window_started_at = NULL WHERE id = $1::uuid`, u.id)
	if err != nil {
		return fmt.Errorf("clearing the login counters: %w", err)
	}
	return nil
}

func (u *userTx) LockAccount(ctx context.Context, streak int) (bool, error) {
	lo := domain.LockoutFor(streak)
	if lo.None {
		return false, nil
	}
	// El cierre (racha 100) no fija locked_until: el cobro ya no cobra con la racha en 100. Los bloqueos sí.
	var until, now pgtype.Timestamptz
	err := u.tx.QueryRow(ctx, `
		UPDATE iam.users SET failed_logins = 0, login_lockouts = $3,
		       locked_until = CASE WHEN $5 THEN locked_until ELSE now() + make_interval(secs => $2) END
		 WHERE id = $1::uuid AND status = 'activo' AND login_failure_streak >= $4
		   AND (locked_until IS NULL OR locked_until <= now())
		RETURNING coalesce(locked_until, 'epoch'::timestamptz), now()`,
		u.id, lo.For.Seconds(), lo.Counted, streak, lo.Closed).Scan(&until, &now)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("locking the account: %w", err)
	}
	// El aviso va en la misma transacción: si el bloqueo se revierte, no hay aviso. Único por usuario y fin del bloqueo.
	// El cierre no tiene fin de bloqueo: lleva el instante del cierre (now() de esta transacción) en vez de locked_until,
	// así un segundo cierre, meses después y tras un restablecimiento, es otro job y otro correo.
	args := app.LockoutNoticeArgs{UserID: u.id, Closed: lo.Closed}
	if lo.Closed {
		t := now.Time.UTC()
		args.ClosedAt = &t
	} else {
		t := until.Time.UTC()
		args.LockedUntil = &t
	}
	if err := u.enq.InsertTx(ctx, u.tx, args, jobs.WithQueue(jobs.QueueNotifications), jobs.WithUnique()); err != nil {
		return false, fmt.Errorf("enqueueing the lockout notice: %w", err)
	}
	return true, nil
}

func (u *userTx) ActiveMemberships(ctx context.Context) ([]app.Membership, error) {
	rows, err := u.tx.Query(ctx, `
		SELECT membership_id::text, tenant_id::text, coalesce(tenant_name, ''), coalesce(industry, ''), tenant_status,
		       role_id::text, role_name, requires_mfa, sites, cash_registers
		  FROM iam.list_my_memberships WHERE membership_status = 'activo'
		 ORDER BY coalesce(tenant_name, ''), membership_id`)
	if err != nil {
		return nil, fmt.Errorf("listing the memberships: %w", err)
	}
	defer rows.Close()
	var out []app.Membership
	for rows.Next() {
		var (
			m           app.Membership
			sites, cash []byte
		)
		if err := rows.Scan(&m.ID, &m.TenantID, &m.TenantName, &m.Industry, &m.TenantStatus, &m.RoleID, &m.RoleName, &m.RequiresMFA, &sites, &cash); err != nil {
			return nil, fmt.Errorf("reading a membership: %w", err)
		}
		if m.Sites, err = decodeSites(sites, cash); err != nil {
			return nil, err
		}
		m.Selectable = m.TenantStatus != "suspendida"
		if !m.Selectable {
			m.BlockedReason = "tenant_suspended"
		}
		m.ReadOnly = m.TenantStatus == "cerrada"
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing the memberships: %w", err)
	}
	return out, nil
}

func decodeSites(sitesJSON, cashJSON []byte) ([]app.Site, error) {
	var sites []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var cash []struct {
		ID     string `json:"id"`
		SiteID string `json:"site_id"`
		Code   string `json:"code"`
	}
	if err := json.Unmarshal(sitesJSON, &sites); err != nil {
		return nil, fmt.Errorf("decoding the sites of a membership: %w", err)
	}
	if err := json.Unmarshal(cashJSON, &cash); err != nil {
		return nil, fmt.Errorf("decoding the cash registers of a membership: %w", err)
	}
	out := make([]app.Site, 0, len(sites))
	for _, s := range sites {
		site := app.Site{ID: s.ID, Name: s.Name, CashRegisters: []app.CashRegister{}}
		for _, c := range cash {
			if c.SiteID == s.ID {
				site.CashRegisters = append(site.CashRegisters, app.CashRegister{ID: c.ID, Code: c.Code})
			}
		}
		out = append(out, site)
	}
	return out, nil
}

func (u *userTx) HasConfirmedTOTP(ctx context.Context) (bool, error) {
	var ok bool
	err := u.tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM iam.user_totp WHERE user_id = $1::uuid AND confirmed_at IS NOT NULL)`, u.id).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("reading the second factor: %w", err)
	}
	return ok, nil
}

func (u *userTx) CreateSession(ctx context.Context, s app.NewSession) (accessExpiresAt time.Time, err error) {
	var ip, refreshSecs any
	if s.IP.IsValid() {
		ip = s.IP.String()
	}
	var refreshHash any
	if s.RefreshHash != nil {
		refreshHash = s.RefreshHash[:]
		refreshSecs = s.RefreshTTL.Seconds()
	}
	err = u.tx.QueryRow(ctx, `
		INSERT INTO iam.sessions (id, user_id, access_hash, access_expires_at, refresh_hash, refresh_family, refresh_expires_at,
		                          mfa_satisfied, mfa_method, ip, user_agent)
		VALUES (gen_random_uuid(), $1::uuid, $2, now() + make_interval(secs => $3), $4::bytea, gen_random_uuid(),
		        CASE WHEN $5::float8 IS NULL THEN NULL ELSE now() + make_interval(secs => $5::float8) END, $6, 'none', $7::inet, $8)
		RETURNING access_expires_at`,
		u.id, s.AccessHash[:], s.AccessTTL.Seconds(), refreshHash, refreshSecs, s.MFASatisfied, ip, truncateUserAgent(s.UserAgent)).Scan(&accessExpiresAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("creating the session: %w", err)
	}
	return accessExpiresAt, nil
}

// truncateUserAgent deja el user agent en UTF-8 válido, sin NUL (PostgreSQL no los admite en text) y en
// 512 bytes como máximo, sin cortar un carácter por la mitad.
func truncateUserAgent(ua string) string {
	ua = strings.ToValidUTF8(strings.ReplaceAll(ua, "\x00", ""), "")
	if len(ua) <= maxUserAgent {
		return ua
	}
	ua = ua[:maxUserAgent]
	for !utf8.ValidString(ua) {
		ua = ua[:len(ua)-1]
	}
	return ua
}
