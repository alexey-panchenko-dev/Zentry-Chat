package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	VerificationPurpose = "verify_email"
	RecoveryPurpose     = "reset_password"
	EmailCodeTTL        = 10 * time.Minute
	EmailCodeCooldown   = time.Minute
	EmailCodeMaxAttempts = 5
)

// IssueEmailCode stores only a bcrypt hash; the raw code is returned once for delivery.
func (s *Service) IssueEmailCode(ctx context.Context, email, purpose string) (string, error) {
	if purpose != VerificationPurpose && purpose != RecoveryPurpose {
		return "", errors.New("invalid email code purpose")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil { return "", err }
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, email+":"+purpose); err != nil { return "", err }
	var resendAfter time.Time
	err = tx.QueryRow(ctx, `SELECT resend_after FROM auth_email_codes WHERE email=$1 AND purpose=$2 FOR UPDATE`, email, purpose).Scan(&resendAfter)
	if err == nil && time.Now().Before(resendAfter) { return "", ErrCodeCooldown }
	if err != nil && !errors.Is(err, pgx.ErrNoRows) { return "", err }
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil { return "", err }
	code := fmt.Sprintf("%06d", value.Int64())
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil { return "", err }
	_, err = tx.Exec(ctx, `INSERT INTO auth_email_codes(email,purpose,code_hash,expires_at,resend_after,attempts)
		VALUES($1,$2,$3,now()+($4 * interval '1 second'),now()+($5 * interval '1 second'),0)
		ON CONFLICT(email,purpose) DO UPDATE SET code_hash=EXCLUDED.code_hash,expires_at=EXCLUDED.expires_at,resend_after=EXCLUDED.resend_after,attempts=0,created_at=now()`, email, purpose, string(hash), int(EmailCodeTTL/time.Second), int(EmailCodeCooldown/time.Second))
	if err != nil { return "", err }
	return code, tx.Commit(ctx)
}

func (s *Service) IssueRecoveryCode(ctx context.Context, email string) (string, bool, error) {
	var exists bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email=$1 AND email_verified=true)`, email).Scan(&exists); err != nil || !exists { return "", exists, err }
	code, err := s.IssueEmailCode(ctx, email, RecoveryPurpose)
	return code, true, err
}

func (s *Service) IssueVerificationCode(ctx context.Context, email string) (string, bool, error) {
	var exists bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email=$1 AND email_verified=false)`, email).Scan(&exists); err != nil || !exists { return "", exists, err }
	code, err := s.IssueEmailCode(ctx, email, VerificationPurpose)
	return code, true, err
}

func (s *Service) VerifyEmailCode(ctx context.Context, email, code string) error {
	return s.consumeEmailCode(ctx, email, VerificationPurpose, code, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `UPDATE users SET email_verified=true WHERE email=$1 AND email_verified=false RETURNING true`, email).Scan(&exists); errors.Is(err, pgx.ErrNoRows) { return ErrCodeInvalid } else if err != nil { return err }
		return nil
	})
}

func (s *Service) ResetPasswordWithCode(ctx context.Context, email, code, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil { return err }
	return s.consumeEmailCode(ctx, email, RecoveryPurpose, code, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE email=$1 AND email_verified=true`, email, string(hash)); err != nil { return err }
		_, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=(SELECT id FROM users WHERE email=$1)`, email)
		if err != nil { return err }
		_, err = tx.Exec(ctx, `DELETE FROM auth_login_limits WHERE email=$1`, email)
		return err
	})
}

func (s *Service) consumeEmailCode(ctx context.Context, email, purpose, code string, complete func(pgx.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, email+":"+purpose); err != nil { return err }
	var hash string
	var expires time.Time
	var attempts int
	err = tx.QueryRow(ctx, `SELECT code_hash,expires_at,attempts FROM auth_email_codes WHERE email=$1 AND purpose=$2 FOR UPDATE`, email, purpose).Scan(&hash, &expires, &attempts)
	if errors.Is(err, pgx.ErrNoRows) { return ErrCodeInvalid }
	if err != nil { return err }
	if time.Now().After(expires) { _, _ = tx.Exec(ctx, `DELETE FROM auth_email_codes WHERE email=$1 AND purpose=$2`, email, purpose); _ = tx.Commit(ctx); return ErrCodeExpired }
	if attempts >= EmailCodeMaxAttempts { return ErrCodeAttempts }
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) != nil {
		_, err = tx.Exec(ctx, `UPDATE auth_email_codes SET attempts=attempts+1 WHERE email=$1 AND purpose=$2`, email, purpose)
		if err != nil { return err }
		if attempts+1 >= EmailCodeMaxAttempts {
			if err := tx.Commit(ctx); err != nil { return err }
			return ErrCodeAttempts
		}
		if err := tx.Commit(ctx); err != nil { return err }
		return ErrCodeInvalid
	}
	if err := complete(tx); err != nil { return err }
	if _, err := tx.Exec(ctx, `DELETE FROM auth_email_codes WHERE email=$1 AND purpose=$2`, email, purpose); err != nil { return err }
	return tx.Commit(ctx)
}
