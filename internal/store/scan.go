package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"ssl-expire-checker/internal/ssl"
)

// SaveScanResult persists SSL scan output for a domain row.
func SaveScanResult(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, res ssl.ScanResult) error {
	_, err := pool.Exec(ctx, `
		update public.domains
		set issuer = $2,
		    expiry_date = $3,
		    status = $4,
		    last_checked = $5,
		    last_error = $6
		where id = $1
	`, id, res.Issuer, res.Expiry, res.Status, res.LastChecked, res.LastError)
	return err
}
