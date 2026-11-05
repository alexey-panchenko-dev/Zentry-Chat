package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrReset = errors.New("invalid or expired reset")

// CreateReset reserves one link per account per minute. The raw token is returned
// only to the mail transport, never an HTTP response. Missing users are a no-op.
func (s *Service) CreateReset(ctx context.Context, email string) (string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.Background())
	var id string
	err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE email=$1 FOR UPDATE`, email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var recent bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM password_resets WHERE user_id=$1 AND created_at>now()-interval '60 seconds')`, id).Scan(&recent)
	if err != nil {
		return "", err
	}
	if recent {
		return "", nil
	}
	var raw [32]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw[:])
	hash := sha256.Sum256([]byte(token))
	_, err = tx.Exec(ctx, `INSERT INTO password_resets(user_id,token_hash,expires_at) VALUES($1,$2,now()+interval '15 minutes')
 ON CONFLICT(user_id) DO UPDATE SET token_hash=EXCLUDED.token_hash,created_at=now(),expires_at=EXCLUDED.expires_at`, id, hash[:])
	if err != nil {
		return "", err
	}
	return token, tx.Commit(ctx)
}

func (s *Service) CancelReset(ctx context.Context, token string) {
	hash := sha256.Sum256([]byte(token))
	_, _ = s.db.Exec(ctx, `DELETE FROM password_resets WHERE token_hash=$1`, hash[:])
}

func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if !validToken(token) {
		return ErrReset
	}
	hash := sha256.Sum256([]byte(token))
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var id string
	// Lock the account first, matching CreateReset's order. Recheck the token
	// after acquiring the lock so concurrent redemption can succeed only once.
	err = tx.QueryRow(ctx, `SELECT u.id::text FROM users u JOIN password_resets p ON p.user_id=u.id WHERE p.token_hash=$1 FOR UPDATE OF u`, hash[:]).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrReset
	}
	if err != nil {
		return err
	}
	var consumed string
	err = tx.QueryRow(ctx, `DELETE FROM password_resets WHERE user_id=$1 AND token_hash=$2 AND expires_at>now() RETURNING user_id::text`, id, hash[:]).Scan(&consumed)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrReset
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$1 WHERE id=$2`, string(passwordHash), id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
