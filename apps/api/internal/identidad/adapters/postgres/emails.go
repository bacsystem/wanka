package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/bacsystem/wanka/apps/api/internal/identidad/app"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

// Emails implementa app.EmailDirectory para el worker, que solo puede leer id y email de iam.users.
type Emails struct{ pool *db.Pool }

// NewEmails arma el directorio de correos.
func NewEmails(pool *db.Pool) *Emails { return &Emails{pool: pool} }

// EmailOf implementa app.EmailDirectory.
func (e *Emails) EmailOf(ctx context.Context, userID string) (string, error) {
	var email string
	err := e.pool.InNoTenantReadTx(ctx, db.NoTenantScope{}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT email FROM iam.users WHERE id = $1::uuid`, userID).Scan(&email)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", app.ErrUserNotFound
	}
	if err != nil {
		return "", fmt.Errorf("reading the user email: %w", err)
	}
	return email, nil
}
