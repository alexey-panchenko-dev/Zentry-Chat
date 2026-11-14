package auth

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"image/png"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

var ErrMFARequired = errors.New("second factor required")
var ErrMFAInvalid = errors.New("invalid second factor")
var ErrMFAUnavailable = errors.New("second factor unavailable")
var ErrSecurityLimit = errors.New("security action limited")
var ErrMFAState = errors.New("invalid second factor state")

func mfaCipher() (cipher.AEAD, error) {
	key, err := base64.StdEncoding.DecodeString(os.Getenv("TOTP_ENCRYPTION_KEY"))
	if err != nil || len(key) != 32 {
		return nil, ErrMFAUnavailable
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrMFAUnavailable
	}
	return cipher.NewGCM(block)
}
func MFAConfigured() bool { _, err := mfaCipher(); return err == nil }
func sealTOTP(user, secret string) ([]byte, error) {
	a, err := mfaCipher()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return a.Seal(nonce, nonce, []byte(secret), []byte(user)), nil
}
func openTOTP(user string, value []byte) (string, error) {
	a, err := mfaCipher()
	if err != nil {
		return "", err
	}
	if len(value) < a.NonceSize() {
		return "", ErrMFAUnavailable
	}
	plain, err := a.Open(nil, value[:a.NonceSize()], value[a.NonceSize():], []byte(user))
	if err != nil {
		return "", ErrMFAUnavailable
	}
	return string(plain), nil
}
func randomToken() (string, []byte, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", nil, err
	}
	value := hex.EncodeToString(data)
	hash := sha256.Sum256([]byte(value))
	return value, hash[:], nil
}
func tokenDigest(value string) []byte { hash := sha256.Sum256([]byte(value)); return hash[:] }
func lockMFAUser(ctx context.Context, tx pgx.Tx, user string) error {
	var id string
	return tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id=$1 FOR UPDATE`, user).Scan(&id)
}
func activeMFASession(ctx context.Context, tx pgx.Tx, user, token string) error {
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE user_id=$1 AND token_hash=$2 AND expires_at>clock_timestamp())`, user, tokenDigest(token)).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return ErrSession
	}
	return nil
}

// Budget authenticated security actions before expensive password/TOTP work.
func (s *Service) securityBudget(ctx context.Context, user string) error {
	var attempts int
	err := s.db.QueryRow(ctx, `INSERT INTO security_action_limits(user_id,attempts,resets_at) VALUES($1,1,now()+interval '10 minutes') ON CONFLICT(user_id) DO UPDATE SET attempts=CASE WHEN security_action_limits.resets_at<=now() THEN 1 ELSE security_action_limits.attempts+1 END,resets_at=CASE WHEN security_action_limits.resets_at<=now() THEN now()+interval '10 minutes' ELSE security_action_limits.resets_at END RETURNING attempts`, user).Scan(&attempts)
	if err != nil {
		return err
	}
	if attempts > 10 {
		return ErrSecurityLimit
	}
	return nil
}
func (s *Service) confirmPassword(ctx context.Context, user, password string) error {
	if err := s.securityBudget(ctx, user); err != nil {
		return err
	}
	var hash string
	if err := s.db.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1`, user).Scan(&hash); err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return ErrCredentials
	}
	return nil
}

type MFASetup struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
	QR     string `json:"qr"`
}

func (s *Service) BeginMFA(ctx context.Context, user, email, password string) (MFASetup, error) {
	if !MFAConfigured() {
		return MFASetup{}, ErrMFAUnavailable
	}
	if err := s.confirmPassword(ctx, user, password); err != nil {
		return MFASetup{}, err
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Zently", AccountName: email, SecretSize: 20})
	if err != nil {
		return MFASetup{}, err
	}
	encrypted, err := sealTOTP(user, key.Secret())
	if err != nil {
		return MFASetup{}, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return MFASetup{}, err
	}
	defer tx.Rollback(context.Background())
	if err = lockMFAUser(ctx, tx, user); err != nil {
		return MFASetup{}, err
	}
	result, err := tx.Exec(ctx, `INSERT INTO user_totp(user_id,secret_cipher,pending_until) VALUES($1,$2,now()+interval '10 minutes') ON CONFLICT(user_id) DO UPDATE SET secret_cipher=EXCLUDED.secret_cipher,pending_until=EXCLUDED.pending_until,setup_verified=false,last_step=-1 WHERE NOT user_totp.enabled`, user, encrypted)
	if err != nil {
		return MFASetup{}, err
	}
	if result.RowsAffected() != 1 {
		return MFASetup{}, ErrMFAState
	}
	image, err := key.Image(220, 220)
	if err != nil {
		return MFASetup{}, err
	}
	var out bytes.Buffer
	if err = png.Encode(&out, image); err != nil {
		return MFASetup{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MFASetup{}, err
	}
	return MFASetup{key.Secret(), key.URL(), "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())}, nil
}

// Use the vetted library for each allowed time step and persist the consumed step.
func consumeFactor(ctx context.Context, tx pgx.Tx, user, code string, allowBackup bool) error {
	var encrypted []byte
	var last int64
	if err := tx.QueryRow(ctx, `SELECT secret_cipher,last_step FROM user_totp WHERE user_id=$1 FOR UPDATE`, user).Scan(&encrypted, &last); err != nil {
		return ErrMFAInvalid
	}
	secret, err := openTOTP(user, encrypted)
	if err != nil {
		return err
	}
	if len(code) == 6 {
		now := time.Now().Unix() / 30
		for _, step := range []int64{now, now - 1, now + 1} {
			if step <= last {
				continue
			}
			value, err := totp.GenerateCodeCustom(secret, time.Unix(step*30, 0), totp.ValidateOpts{Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
			if err != nil {
				return err
			}
			if subtle.ConstantTimeCompare([]byte(value), []byte(code)) == 1 {
				_, err = tx.Exec(ctx, `UPDATE user_totp SET last_step=$2 WHERE user_id=$1`, user, step)
				return err
			}
		}
	}
	if allowBackup {
		canonical := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
		if len(canonical) == 32 {
			hash := sha256.Sum256([]byte(canonical))
			result, err := tx.Exec(ctx, `DELETE FROM totp_backup_codes WHERE user_id=$1 AND code_hash=$2`, user, hash[:])
			if err != nil {
				return err
			}
			if result.RowsAffected() == 1 {
				return nil
			}
		}
	}
	return ErrMFAInvalid
}
func newBackupCodes(ctx context.Context, tx pgx.Tx, user string) ([]string, error) {
	if _, err := tx.Exec(ctx, `DELETE FROM totp_backup_codes WHERE user_id=$1`, user); err != nil {
		return nil, err
	}
	codes := make([]string, 10)
	for index := range codes {
		value := make([]byte, 16)
		if _, err := rand.Read(value); err != nil {
			return nil, err
		}
		plain := hex.EncodeToString(value)
		hash := sha256.Sum256([]byte(plain))
		if _, err := tx.Exec(ctx, `INSERT INTO totp_backup_codes(user_id,code_hash) VALUES($1,$2)`, user, hash[:]); err != nil {
			return nil, err
		}
		codes[index] = plain[:8] + "-" + plain[8:16] + "-" + plain[16:24] + "-" + plain[24:]
	}
	return codes, nil
}
func (s *Service) VerifyMFASetup(ctx context.Context, user, code string) ([]string, error) {
	if err := s.securityBudget(ctx, user); err != nil {
		return nil, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if err = lockMFAUser(ctx, tx, user); err != nil {
		return nil, err
	}
	var ready bool
	if err = tx.QueryRow(ctx, `SELECT NOT enabled AND pending_until>clock_timestamp() FROM user_totp WHERE user_id=$1 FOR UPDATE`, user).Scan(&ready); err != nil || !ready {
		return nil, ErrMFAState
	}
	if err = consumeFactor(ctx, tx, user, code, false); err != nil {
		return nil, err
	}
	codes, err := newBackupCodes(ctx, tx, user)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE user_totp SET setup_verified=true WHERE user_id=$1`, user); err != nil {
		return nil, err
	}
	return codes, tx.Commit(ctx)
}
func (s *Service) ActivateMFA(ctx context.Context, user, currentToken string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = lockMFAUser(ctx, tx, user); err != nil {
		return err
	}
	if err = activeMFASession(ctx, tx, user, currentToken); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE user_totp SET enabled=true WHERE user_id=$1 AND NOT enabled AND setup_verified AND pending_until>clock_timestamp()`, user)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrMFAState
	}
	if _, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1 AND token_hash<>$2`, user, tokenDigest(currentToken)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO security_events(user_id,kind) VALUES($1,'mfa_enabled')`, user); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Service) MFAEnabled(ctx context.Context, user string) (bool, error) {
	var enabled bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_totp WHERE user_id=$1 AND enabled)`, user).Scan(&enabled)
	return enabled, err
}

func (s *Service) ChangeMFA(ctx context.Context, user, currentToken, password, code string, disable bool) ([]string, error) {
	if err := s.confirmPassword(ctx, user, password); err != nil {
		return nil, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if err = lockMFAUser(ctx, tx, user); err != nil {
		return nil, err
	}
	if err = activeMFASession(ctx, tx, user, currentToken); err != nil {
		return nil, err
	}
	var enabled bool
	if err = tx.QueryRow(ctx, `SELECT enabled FROM user_totp WHERE user_id=$1 FOR UPDATE`, user).Scan(&enabled); err != nil || !enabled {
		return nil, ErrMFAState
	}
	if err = consumeFactor(ctx, tx, user, code, true); err != nil {
		return nil, err
	}
	var codes []string
	kind := "mfa_backup_regenerated"
	if disable {
		kind = "mfa_disabled"
		if _, err = tx.Exec(ctx, `DELETE FROM user_totp WHERE user_id=$1`, user); err != nil {
			return nil, err
		}
	} else {
		codes, err = newBackupCodes(ctx, tx, user)
		if err != nil {
			return nil, err
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM mfa_challenges WHERE user_id=$1`, user); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1 AND token_hash<>$2`, user, tokenDigest(currentToken)); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO security_events(user_id,kind) VALUES($1,$2)`, user, kind); err != nil {
		return nil, err
	}
	return codes, tx.Commit(ctx)
}

func (s *Service) createMFAChallenge(ctx context.Context, tx pgx.Tx, user, agent string) (string, time.Time, bool, error) {
	var enabled bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_totp WHERE user_id=$1 AND enabled)`, user).Scan(&enabled); err != nil {
		return "", time.Time{}, false, err
	}
	if !enabled {
		return "", time.Time{}, false, nil
	}
	if !MFAConfigured() {
		return "", time.Time{}, false, ErrMFAUnavailable
	}
	token, hash, err := randomToken()
	if err != nil {
		return "", time.Time{}, false, err
	}
	expiry := time.Now().UTC().Add(5 * time.Minute)
	if _, err = tx.Exec(ctx, `DELETE FROM mfa_challenges WHERE user_id=$1 AND (expires_at<=now() OR attempts>=5)`, user); err != nil {
		return "", time.Time{}, false, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM mfa_challenges WHERE user_id=$1 AND token_hash IN (SELECT token_hash FROM mfa_challenges WHERE user_id=$1 ORDER BY expires_at DESC OFFSET 4)`, user); err != nil {
		return "", time.Time{}, false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO mfa_challenges(token_hash,user_id,expires_at,user_agent) VALUES($1,$2,$3,$4)`, hash, user, expiry, agent)
	return token, expiry, true, err
}
func (s *Service) CompleteMFA(ctx context.Context, challenge, code string) (User, string, time.Time, error) {
	if !MFAConfigured() {
		return User{}, "", time.Time{}, ErrMFAUnavailable
	}
	if !validToken(challenge) {
		return User{}, "", time.Time{}, ErrMFAInvalid
	}
	hash := tokenDigest(challenge)
	var user string
	err := s.db.QueryRow(ctx, `SELECT user_id::text FROM mfa_challenges WHERE token_hash=$1 AND expires_at>clock_timestamp() AND attempts<5`, hash).Scan(&user)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", time.Time{}, ErrMFAInvalid
	}
	if err != nil {
		return User{}, "", time.Time{}, err
	}
	if err = s.mfaLoginBudget(ctx, user); err != nil {
		return User{}, "", time.Time{}, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return User{}, "", time.Time{}, err
	}
	defer tx.Rollback(context.Background())
	if err = lockMFAUser(ctx, tx, user); err != nil {
		return User{}, "", time.Time{}, err
	}
	var enabled bool
	if err = tx.QueryRow(ctx, `SELECT enabled FROM user_totp WHERE user_id=$1 FOR UPDATE`, user).Scan(&enabled); err != nil || !enabled {
		return User{}, "", time.Time{}, ErrMFAInvalid
	}
	var agent string
	if err = tx.QueryRow(ctx, `UPDATE mfa_challenges SET attempts=attempts+1 WHERE token_hash=$1 AND expires_at>clock_timestamp() AND attempts<5 RETURNING user_agent`, hash).Scan(&agent); err != nil {
		return User{}, "", time.Time{}, ErrMFAInvalid
	}
	if err = consumeFactor(ctx, tx, user, code, true); err != nil {
		if errors.Is(err, ErrMFAInvalid) {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return User{}, "", time.Time{}, commitErr
			}
		}
		return User{}, "", time.Time{}, err
	}
	token, digest, err := randomToken()
	if err != nil {
		return User{}, "", time.Time{}, err
	}
	expiry := time.Now().UTC().Add(SessionTTL)
	if _, err = tx.Exec(ctx, `INSERT INTO sessions(user_id,token_hash,expires_at,user_agent) VALUES($1,$2,$3,$4)`, user, digest, expiry, agent); err != nil {
		return User{}, "", time.Time{}, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM mfa_challenges WHERE token_hash=$1`, hash); err != nil {
		return User{}, "", time.Time{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO security_events(user_id,kind) VALUES($1,'login')`, user); err != nil {
		return User{}, "", time.Time{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, "", time.Time{}, err
	}
	value, err := s.CurrentUser(ctx, token)
	return value, token, expiry, err
}

func (s *Service) mfaLoginBudget(ctx context.Context, user string) error {
	var attempts int
	err := s.db.QueryRow(ctx, `INSERT INTO mfa_login_limits(user_id,attempts,resets_at) VALUES($1,1,now()+interval '10 minutes') ON CONFLICT(user_id) DO UPDATE SET attempts=CASE WHEN mfa_login_limits.resets_at<=now() THEN 1 ELSE mfa_login_limits.attempts+1 END,resets_at=CASE WHEN mfa_login_limits.resets_at<=now() THEN now()+interval '10 minutes' ELSE mfa_login_limits.resets_at END RETURNING attempts`, user).Scan(&attempts)
	if err != nil {
		return err
	}
	if attempts > 10 {
		return ErrSecurityLimit
	}
	return nil
}
