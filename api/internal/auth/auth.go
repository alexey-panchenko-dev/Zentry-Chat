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
var ErrSession = errors.New("invalid session")

const SessionTTL = 7 * 24 * time.Hour

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
func (s *Service) RegisterWithBirthDate(ctx context.Context, name, email, password string, birthDate *string) (User, error) {
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
	err = tx.QueryRow(ctx, `INSERT INTO users (name,email,password_hash) VALUES ($1,$2,$3)
		RETURNING id::text,name,username,email,created_at`, name, email, string(hash)).Scan(&u.ID, &u.Name, &u.Username, &u.Email, &u.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
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
	var u User
	var hash string
	err := s.db.QueryRow(ctx, `SELECT id::text,name,username,email,created_at,password_hash,COALESCE((SELECT '/api/profiles/'||users.id::text||'/photo?v='||p.revision::text FROM profile_photos p WHERE p.user_id=users.id),'') FROM users WHERE email=$1`, email).
		Scan(&u.ID, &u.Name, &u.Username, &u.Email, &u.CreatedAt, &hash, &u.AvatarURL)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return User{}, "", time.Time{}, ErrCredentials
	}
	if err != nil {
		return User{}, "", time.Time{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, "", time.Time{}, ErrCredentials
	}
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return User{}, "", time.Time{}, err
	}
	token := hex.EncodeToString(bytes[:])
	digest := sha256.Sum256([]byte(token))
	expires := time.Now().UTC().Add(SessionTTL)
	_, err = s.db.Exec(ctx, `INSERT INTO sessions (user_id,token_hash,expires_at) VALUES ($1,$2,$3)`, u.ID, digest[:], expires)
	return u, token, expires, err
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
