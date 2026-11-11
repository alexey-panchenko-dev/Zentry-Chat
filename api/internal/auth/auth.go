package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var ErrCredentials = errors.New("invalid credentials")
var ErrEmailExists = errors.New("email already registered")
var ErrUsernameExists = errors.New("username already registered")
var ErrSession = errors.New("invalid session")
var ErrEmailNotVerified = errors.New("email not verified")
var ErrLoginLocked = errors.New("too many failed login attempts")
var ErrCodeInvalid = errors.New("invalid verification code")
var ErrCodeExpired = errors.New("verification code expired")
var ErrCodeAttempts = errors.New("verification attempts exceeded")
var ErrCodeCooldown = errors.New("verification code resend cooldown")

const SessionTTL = 7 * 24 * time.Hour
const LoginMaxAttempts = 3
const LoginLockout = 10 * time.Minute

type User struct {
	AvatarURL string    `json:"avatar_url,omitempty"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	db        *pgxpool.Pool
	dummyHash []byte
}

func New(db *pgxpool.Pool) *Service {
	// Used for a missing account so wrong-password checks take similar time.
	hash, err := bcrypt.GenerateFromPassword([]byte("dummy-password-not-a-user-secret"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return &Service{db: db, dummyHash: hash}
}

func (s *Service) Register(ctx context.Context, name, email, password string) (User, error) {
	return s.RegisterWithBirthDate(ctx, name, email, password, nil)
}
func (s *Service) RegisterWithBirthDate(ctx context.Context, name, email, password string, birthDate *string, handle ...string) (User, error) {
	return s.register(ctx, name, email, password, birthDate, true, handle...)
}
func (s *Service) RegisterPendingWithBirthDate(ctx context.Context, name, email, password string, birthDate *string, handle ...string) (User, error) {
	return s.register(ctx, name, email, password, birthDate, false, handle...)
}
func (s *Service) register(ctx context.Context, name, email, password string, birthDate *string, verified bool, handle ...string) (User, error) {
	var date any
	if birthDate != nil {
		d, err := ParseBirthDate(*birthDate, time.Now())
		if err != nil {
			return User{}, err
		}
		date = d
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(context.Background())
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	var u User
	if len(handle) > 0 {
		err = tx.QueryRow(ctx, `INSERT INTO users (name,username,email,password_hash,email_verified) VALUES ($1,$1,$2,$3,$4)
			RETURNING id::text,name,username,email,created_at`, handle[0], email, string(hash), verified).Scan(&u.ID, &u.Name, &u.Username, &u.Email, &u.CreatedAt)
	} else {
		err = tx.QueryRow(ctx, `INSERT INTO users (name,email,password_hash,email_verified) VALUES ($1,$2,$3,$4)
			RETURNING id::text,name,username,email,created_at`, name, email, string(hash), verified).Scan(&u.ID, &u.Name, &u.Username, &u.Email, &u.CreatedAt)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if pgErr.ConstraintName == "users_username_unique" {
			return User{}, ErrUsernameExists
		}
		return User{}, ErrEmailExists
	}
	if err != nil {
		return User{}, err
	}
	if birthDate != nil {
		if _, err = tx.Exec(ctx, `INSERT INTO user_profiles(user_id,birth_date) VALUES($1,$2)`, u.ID, date); err != nil {
			return User{}, err
		}
	}
	return u, tx.Commit(ctx)
}

func (s *Service) Login(ctx context.Context, email, password string) (User, string, time.Time, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return User{}, "", time.Time{}, err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, email); err != nil {
		return User{}, "", time.Time{}, err
	}
	var locked bool
	err = tx.QueryRow(ctx, `SELECT locked_until IS NOT NULL AND locked_until > now() FROM auth_login_limits WHERE email=$1 FOR UPDATE`, email).Scan(&locked)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", time.Time{}, err
	}
	if err == nil && locked {
		return User{}, "", time.Time{}, ErrLoginLocked
	}
	if err == nil {
		if _, err := tx.Exec(ctx, `DELETE FROM auth_login_limits WHERE email=$1`, email); err != nil {
			return User{}, "", time.Time{}, err
		}
	}
	var u User
	var hash string
	var verified bool
	err = tx.QueryRow(ctx, `SELECT id::text,name,username,email,created_at,password_hash,COALESCE((SELECT '/api/profiles/'||users.id::text||'/photo?v='||p.revision::text FROM profile_photos p WHERE p.user_id=users.id),''),email_verified FROM users WHERE email=$1`, email).
		Scan(&u.ID, &u.Name, &u.Username, &u.Email, &u.CreatedAt, &hash, &u.AvatarURL, &verified)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		locked, err := recordLoginFailure(ctx, tx, email)
		if err != nil {
			return User{}, "", time.Time{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return User{}, "", time.Time{}, err
		}
		if locked { return User{}, "", time.Time{}, ErrLoginLocked }
		return User{}, "", time.Time{}, ErrCredentials
	}
	if err != nil {
		return User{}, "", time.Time{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		locked, err := recordLoginFailure(ctx, tx, email)
		if err != nil {
			return User{}, "", time.Time{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return User{}, "", time.Time{}, err
		}
		if locked { return User{}, "", time.Time{}, ErrLoginLocked }
		return User{}, "", time.Time{}, ErrCredentials
	}
	if _, err := tx.Exec(ctx, `DELETE FROM auth_login_limits WHERE email=$1`, email); err != nil {
		return User{}, "", time.Time{}, err
	}
	if !verified {
		return User{}, "", time.Time{}, ErrEmailNotVerified
	}
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return User{}, "", time.Time{}, err
	}
	token := hex.EncodeToString(bytes[:])
	digest := sha256.Sum256([]byte(token))
	expires := time.Now().UTC().Add(SessionTTL)
	if _, err = tx.Exec(ctx, `INSERT INTO sessions (user_id,token_hash,expires_at) VALUES ($1,$2,$3)`, u.ID, digest[:], expires); err != nil {
		return User{}, "", time.Time{}, err
	}
	return u, token, expires, tx.Commit(ctx)
}

func recordLoginFailure(ctx context.Context, tx pgx.Tx, email string) (bool, error) {
	var locked bool
	err := tx.QueryRow(ctx, `INSERT INTO auth_login_limits(email,attempts,locked_until,updated_at)
		VALUES($1,1,NULL,now())
		ON CONFLICT(email) DO UPDATE SET
		attempts=LEAST(auth_login_limits.attempts+1,$2),
		locked_until=CASE WHEN auth_login_limits.attempts+1 >= $2 THEN now()+($3 * interval '1 minute') ELSE NULL END,
		updated_at=now()
		RETURNING locked_until IS NOT NULL AND locked_until > now()`, email, LoginMaxAttempts, int(LoginLockout/time.Minute)).Scan(&locked)
	return locked, err
}

func (s *Service) CurrentUser(ctx context.Context, token string) (User, error) {
	if !validToken(token) {
		return User{}, ErrSession
	}
	digest := sha256.Sum256([]byte(token))
	var u User
	err := s.db.QueryRow(ctx, `SELECT u.id::text,u.name,u.username,u.email,u.created_at,COALESCE((SELECT '/api/profiles/'||u.id::text||'/photo?v='||p.revision::text FROM profile_photos p WHERE p.user_id=u.id),'')
		FROM sessions s JOIN users u ON u.id=s.user_id
		WHERE s.token_hash=$1 AND s.expires_at>now()`, digest[:]).Scan(&u.ID, &u.Name, &u.Username, &u.Email, &u.CreatedAt, &u.AvatarURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrSession
	}
	return u, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if !validToken(token) {
		return nil
	}
	digest := sha256.Sum256([]byte(token))
	_, err := s.db.Exec(ctx, "DELETE FROM sessions WHERE token_hash=$1", digest[:])
	return err
}

func validToken(token string) bool {
	if len(token) != 64 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}
