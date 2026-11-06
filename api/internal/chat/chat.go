package chat

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("client message id already used")

type PublicUser struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type Conversation struct {
	ID          string     `json:"id"`
	Participant PublicUser `json:"participant"`
	CreatedAt   time.Time  `json:"created_at"`
	Position    string     `json:"-"`
}

type Message struct {
	Kind            string    `json:"kind,omitempty"`
	AudioURL        string    `json:"audio_url,omitempty"`
	DurationMS      int       `json:"duration_ms,omitempty"`
	ID              string    `json:"id"`
	ConversationID  string    `json:"conversation_id"`
	SenderID        string    `json:"sender_id"`
	ClientMessageID string    `json:"client_message_id"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"created_at"`
}

type Store struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Store { return &Store{db: db} }

func (s *Store) SearchUsers(ctx context.Context, userID, query string) ([]PublicUser, error) {
	rows, err := s.db.Query(ctx, `SELECT id::text,name,COALESCE((SELECT '/api/profiles/'||users.id::text||'/photo?v='||p.revision::text FROM profile_photos p WHERE p.user_id=users.id),'') FROM users
        WHERE id<>$1 AND (strpos(lower(name),lower($2))>0 OR email=lower($2))
        ORDER BY lower(name),id LIMIT 20`, userID, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]PublicUser, 0)
	for rows.Next() {
		var u PublicUser
		if err := rows.Scan(&u.ID, &u.Name, &u.AvatarURL); err != nil {
			return nil, err
		}
		result = append(result, u)
	}
	return result, rows.Err()
}

func (s *Store) CreateConversation(ctx context.Context, userID, peerID string) (Conversation, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Conversation{}, err
	}
	defer tx.Rollback(context.Background())
	var peer PublicUser
	err = tx.QueryRow(ctx, "SELECT id::text,name,COALESCE((SELECT '/api/profiles/'||users.id::text||'/photo?v='||p.revision::text FROM profile_photos p WHERE p.user_id=users.id),'') FROM users WHERE id=$1 FOR KEY SHARE", peerID).Scan(&peer.ID, &peer.Name, &peer.AvatarURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, ErrNotFound
	}
	if err != nil {
		return Conversation{}, err
	}
	var c Conversation
	// The unique pair makes simultaneous A->B and B->A requests return one dialog.
	err = tx.QueryRow(ctx, `INSERT INTO conversations(user_low,user_high)
        VALUES (LEAST($1::uuid,$2::uuid),GREATEST($1::uuid,$2::uuid))
        ON CONFLICT (user_low,user_high) DO UPDATE SET user_low=EXCLUDED.user_low
        RETURNING id::text,created_at`, userID, peerID).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return c, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id) VALUES ($1,$2),($1,$3)
        ON CONFLICT DO NOTHING`, c.ID, userID, peerID)
	if err != nil {
		return c, err
	}
	c.Participant = peer
	return c, tx.Commit(ctx)
}

func (s *Store) Conversations(ctx context.Context, userID string, before int64, limit int) ([]Conversation, error) {
	rows, err := s.db.Query(ctx, `SELECT c.id::text,c.created_at,c.position::text,u.id::text,u.name,COALESCE((SELECT '/api/profiles/'||u.id::text||'/photo?v='||p.revision::text FROM profile_photos p WHERE p.user_id=u.id),'')
        FROM conversation_members m JOIN conversations c ON c.id=m.conversation_id
        JOIN users u ON u.id=CASE WHEN c.user_low=$1 THEN c.user_high ELSE c.user_low END
        WHERE m.user_id=$1 AND ($2::bigint=0 OR c.position<$2)
        ORDER BY c.position DESC LIMIT $3`, userID, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Conversation, 0)
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.CreatedAt, &c.Position, &c.Participant.ID, &c.Participant.Name, &c.Participant.AvatarURL); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (s *Store) CheckMember(ctx context.Context, conversationID, userID string) error {
	var exists bool
	err := s.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2)", conversationID, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Messages(ctx context.Context, conversationID, userID string, before int64, limit int) ([]Message, error) {
	if err := s.CheckMember(ctx, conversationID, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT id::text,conversation_id::text,sender_id::text,client_message_id::text,body,created_at,CASE WHEN kind='voice' THEN kind ELSE '' END,COALESCE(duration_ms,0)
        FROM messages WHERE conversation_id=$1 AND ($2::bigint=0 OR id<$2) ORDER BY id DESC LIMIT $3`, conversationID, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Message, 0)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt, &m.Kind, &m.DurationMS); err != nil {
			return nil, err
		}
		if m.Kind == "voice" {
			m.AudioURL = "/api/messages/" + m.ID + "/audio"
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (s *Store) Send(ctx context.Context, conversationID, userID, clientID, body string) (Message, bool, error) {
	if err := s.CheckMember(ctx, conversationID, userID); err != nil {
		return Message{}, false, err
	}
	var m Message
	err := s.db.QueryRow(ctx, `INSERT INTO messages(conversation_id,sender_id,client_message_id,body)
        VALUES($1,$2,$3,$4) ON CONFLICT(sender_id,client_message_id) DO NOTHING
        RETURNING id::text,conversation_id::text,sender_id::text,client_message_id::text,body,created_at`, conversationID, userID, clientID, body).
		Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt)
	if err == nil {
		return m, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return m, false, err
	}
	err = s.db.QueryRow(ctx, `SELECT id::text,conversation_id::text,sender_id::text,client_message_id::text,body,created_at
        FROM messages WHERE sender_id=$1 AND client_message_id=$2`, userID, clientID).
		Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt)
	if err == nil && (m.ConversationID != conversationID || m.Body != body) {
		return Message{}, false, ErrConflict
	}
	return m, false, err
}
