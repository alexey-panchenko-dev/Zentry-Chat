package contactguard

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

var ErrBlocked = errors.New("contact blocked")

// LockUsers is the common first lock for blocks, sends, discovery and calls.
// Callers must acquire it before call rows, and conversation rows last.
func LockUsers(ctx context.Context, tx pgx.Tx, a, b string) error {
	rows, err := tx.Query(ctx, `SELECT id FROM users WHERE id IN ($1,$2) ORDER BY id FOR UPDATE`, a, b)
	if err != nil {
		return err
	}
	for rows.Next() {
	}
	rows.Close()
	return rows.Err()
}
func CheckBlocked(ctx context.Context, tx pgx.Tx, a, b string) error {
	var blocked bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contact_blocks WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))`, a, b).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return ErrBlocked
	}
	return nil
}
