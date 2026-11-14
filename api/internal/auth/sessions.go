package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"
)

var ErrDeviceNotFound = errors.New("session not found")

type DeviceSession struct {
	ID         string    `json:"id"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Current    bool      `json:"current"`
}

// The caller's identity must come from a validated server session.
func (s *Service) DeviceSessions(ctx context.Context, userID, currentToken string) ([]DeviceSession, error) {
	digest := sha256.Sum256([]byte(currentToken))
	rows, err := s.db.Query(ctx, `SELECT id::text,user_agent,created_at,last_seen_at,expires_at,token_hash=$2 FROM sessions WHERE user_id=$1 AND expires_at>now() ORDER BY last_seen_at DESC LIMIT 100`, userID, digest[:])
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []DeviceSession{}
	for rows.Next() {
		var value DeviceSession
		if err = rows.Scan(&value.ID, &value.UserAgent, &value.CreatedAt, &value.LastSeenAt, &value.ExpiresAt, &value.Current); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *Service) RevokeDeviceSession(ctx context.Context, userID, id string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var actor string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&actor); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1 AND id=$2`, userID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrDeviceNotFound
	}
	if _, err = tx.Exec(ctx, `INSERT INTO security_events(user_id,kind) VALUES($1,'session_revoked')`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) RevokeOtherSessions(ctx context.Context, userID, currentToken string) error {
	digest := sha256.Sum256([]byte(currentToken))
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var actor string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&actor); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1 AND token_hash<>$2`, userID, digest[:]); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO security_events(user_id,kind) VALUES($1,'other_sessions_revoked')`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
