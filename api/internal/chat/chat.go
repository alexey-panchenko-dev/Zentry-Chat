package chat

import (
	"context"
	"errors"
	"fmt"
	"newsChat/api/internal/contactguard"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")
var ErrConflict = errors.New("client message id already used")

type PublicUser struct {
	DisplayName string `json:"display_name,omitempty"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Username    string `json:"username"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type Conversation struct {
	LastActivity        time.Time  `json:"last_activity"`
	LastMessageID       string     `json:"last_message_id,omitempty"`
	FirstUnreadID       string     `json:"first_unread_id,omitempty"`
	LastMessageIncoming bool       `json:"last_message_incoming,omitempty"`
	LastMessage         string     `json:"last_message"`
	ID                  string     `json:"id"`
	Participant         PublicUser `json:"participant"`
	DisplayName         string     `json:"display_name,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	Position            string     `json:"-"`
	UnreadCount         int        `json:"unread_count"`
}

type Message struct {
	Reactions           []ReactionCount `json:"reactions"`
	ImageURL            string          `json:"image_url,omitempty"`
	Kind                string          `json:"kind,omitempty"`
	AudioURL            string          `json:"audio_url,omitempty"`
	AttachmentURL       string          `json:"attachment_url,omitempty"`
	AttachmentName      string          `json:"attachment_name,omitempty"`
	AttachmentMIME      string          `json:"attachment_mime,omitempty"`
	ReplyToID           string          `json:"reply_to_id,omitempty"`
	ReplyBody           string          `json:"reply_body,omitempty"`
	ReplySenderName     string          `json:"reply_sender_name,omitempty"`
	ForwardedSenderName string          `json:"forwarded_sender_name,omitempty"`
	ForwardedFromID     string          `json:"forwarded_from_id,omitempty"`
	EditedAt            *time.Time      `json:"edited_at,omitempty"`
	DurationMS          int             `json:"duration_ms,omitempty"`
	ID                  string          `json:"id"`
	ConversationID      string          `json:"conversation_id"`
	SenderID            string          `json:"sender_id"`
	ClientMessageID     string          `json:"client_message_id"`
	Body                string          `json:"body"`
	CreatedAt           time.Time       `json:"created_at"`
}

type Store struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Store { return &Store{db: db} }

func (s *Store) SearchUsers(ctx context.Context, userID, query string) ([]PublicUser, error) {
	rows, err := s.db.Query(ctx, `SELECT id::text,name,username,COALESCE((SELECT '/api/profiles/'||users.id::text||'/photo?v='||p.revision::text FROM profile_photos p WHERE p.user_id=users.id),'') FROM users
		WHERE email_verified=true AND id<>$1 AND (COALESCE((SELECT search FROM user_privacy WHERE user_id=users.id),'everyone')='everyone' OR (EXISTS(SELECT 1 FROM user_privacy WHERE user_id=users.id AND search='contacts') AND EXISTS(SELECT 1 FROM contact_aliases WHERE owner_id=users.id AND peer_id=$1))) AND (strpos(lower(name),lower(trim(leading '@' from $2)))>0 OR email=lower($2) OR strpos(lower(username),lower(trim(leading '@' from $2)))>0 OR EXISTS(SELECT 1 FROM contact_aliases a WHERE a.owner_id=$1 AND a.peer_id=users.id AND strpos(lower(a.alias),lower($2))>0))
        ORDER BY lower(name),id LIMIT 20`, userID, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]PublicUser, 0)
	for rows.Next() {
		var u PublicUser
		if err := rows.Scan(&u.ID, &u.Name, &u.Username, &u.AvatarURL); err != nil {
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
	if err = contactguard.LockUsers(ctx, tx, userID, peerID); err != nil {
		return Conversation{}, err
	}
	var peer PublicUser
	err = tx.QueryRow(ctx, "SELECT id::text,name,username,COALESCE((SELECT '/api/profiles/'||users.id::text||'/photo?v='||p.revision::text FROM profile_photos p WHERE p.user_id=users.id),'') FROM users WHERE id=$1 AND email_verified=true FOR KEY SHARE", peerID).Scan(&peer.ID, &peer.Name, &peer.Username, &peer.AvatarURL)
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
	if _, err = tx.Exec(ctx, `UPDATE conversation_members SET hidden=false WHERE conversation_id=$1 AND user_id=$2`, c.ID, userID); err != nil {
		return c, err
	}
	c.Participant = peer
	return c, tx.Commit(ctx)
}

func (s *Store) Conversations(ctx context.Context, userID string, before int64, limit int) ([]Conversation, error) {
	return s.ConversationsScope(ctx, userID, before, limit, false)
}
func (s *Store) ConversationsScope(ctx context.Context, userID string, before int64, limit int, requests bool) ([]Conversation, error) {
	rows, err := s.db.Query(ctx, `SELECT c.id::text,c.created_at,COALESCE(recent.created_at,c.created_at),c.position::text,u.id::text,u.name,u.username,COALESCE((SELECT '/api/profiles/'||u.id::text||'/photo?v='||p.revision::text FROM profile_photos p WHERE p.user_id=u.id),''),COALESCE(a.alias,''),COALESCE(CASE recent.kind WHEN 'image' THEN 'Фото' WHEN 'voice' THEN 'Голосовое сообщение' WHEN 'file' THEN COALESCE(recent.attachment_name,'Файл') ELSE left(recent.body,160) END,''),COALESCE(recent.id::text,''),COALESCE(recent.sender_id<>$1,false), unread.total, COALESCE(unread.first_id::text,'')
        FROM conversation_members m JOIN conversations c ON c.id=m.conversation_id
        JOIN users u ON u.id=CASE WHEN c.user_low=$1 THEN c.user_high ELSE c.user_low END
		LEFT JOIN contact_aliases a ON a.owner_id=$1 AND a.peer_id=u.id
        LEFT JOIN LATERAL (SELECT id, sender_id, kind, body, attachment_name, created_at FROM messages
            WHERE conversation_id=c.id AND id>m.cleared_through ORDER BY id DESC LIMIT 1) recent ON true
        LEFT JOIN LATERAL (SELECT count(*) total, min(id) first_id FROM messages
            WHERE conversation_id=c.id AND id>GREATEST(m.last_read_id,m.cleared_through) AND sender_id IS DISTINCT FROM m.user_id) unread ON true
        WHERE m.user_id=$1 AND NOT m.hidden AND (EXISTS(SELECT 1 FROM message_requests req WHERE req.conversation_id=c.id AND req.recipient_id=$1 AND req.status='pending'))=$4 AND ($2::bigint=0 OR c.position<$2)
        ORDER BY c.position DESC LIMIT $3`, userID, before, limit, requests)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Conversation, 0)
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.CreatedAt, &c.LastActivity, &c.Position, &c.Participant.ID, &c.Participant.Name, &c.Participant.Username, &c.Participant.AvatarURL, &c.DisplayName, &c.LastMessage, &c.LastMessageID, &c.LastMessageIncoming, &c.UnreadCount, &c.FirstUnreadID); err != nil {
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

func (s *Store) ConversationParticipants(ctx context.Context, conversationID, userID string) ([]string, error) {
	if err := s.CheckMember(ctx, conversationID, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT user_id::text FROM conversation_members WHERE conversation_id=$1`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	participants := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		participants = append(participants, id)
	}
	return participants, rows.Err()
}

func (s *Store) Messages(ctx context.Context, conversationID, userID string, before int64, limit int) ([]Message, error) {
	if err := s.CheckMember(ctx, conversationID, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT m.id::text,m.conversation_id::text,COALESCE(m.sender_id::text,''),m.client_message_id::text,m.body,m.created_at,CASE WHEN m.kind<>'text' THEN m.kind ELSE '' END,COALESCE(m.duration_ms,0),COALESCE(m.reply_to_id::text,''),COALESCE(parent.body,''),COALESCE(reply_author.name,''),COALESCE(m.forwarded_from_id::text,''),COALESCE((SELECT u.name FROM messages original JOIN users u ON u.id=original.sender_id WHERE original.id=m.forwarded_from_id),''),m.edited_at,COALESCE(m.attachment_name,''),COALESCE(m.attachment_mime,''),
        COALESCE((SELECT jsonb_agg(to_jsonb(t) ORDER BY t.emoji) FROM (
            SELECT emoji,count(*)::int count,bool_or(user_id=$4) mine FROM message_reactions WHERE message_id=m.id GROUP BY emoji
        ) t),'[]'::jsonb)
	    FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$4 LEFT JOIN messages parent ON parent.id=m.reply_to_id AND parent.conversation_id=m.conversation_id AND parent.id>cm.cleared_through LEFT JOIN users reply_author ON reply_author.id=parent.sender_id WHERE m.conversation_id=$1 AND m.id>cm.cleared_through AND ($2::bigint=0 OR m.id<$2) ORDER BY m.id DESC LIMIT $3`, conversationID, before, limit, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Message, 0)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt, &m.Kind, &m.DurationMS, &m.ReplyToID, &m.ReplyBody, &m.ReplySenderName, &m.ForwardedFromID, &m.ForwardedSenderName, &m.EditedAt, &m.AttachmentName, &m.AttachmentMIME, &m.Reactions); err != nil {
			return nil, err
		}
		if m.Kind == "voice" {
			m.AudioURL = "/api/messages/" + m.ID + "/audio"
		}
		if m.Kind == "image" {
			m.ImageURL = "/api/messages/" + m.ID + "/image"
		}
		if m.Kind == "file" {
			m.AttachmentURL = "/api/messages/" + m.ID + "/file"
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (s *Store) Send(ctx context.Context, conversationID, userID, clientID, body string) (Message, bool, error) {
	return s.SendWithReply(ctx, conversationID, userID, clientID, body, "")
}

func (s *Store) SendWithReply(ctx context.Context, conversationID, userID, clientID, body, replyID string) (Message, bool, error) {
	if err := s.CheckMember(ctx, conversationID, userID); err != nil {
		return Message{}, false, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Message{}, false, err
	}
	defer tx.Rollback(context.Background())
	if err = lockSending(ctx, tx, conversationID, userID, clientID); err != nil {
		return Message{}, false, err
	}
	var reply any
	var replyBody, replySender string
	if replyID != "" {
		var replyInt int64
		if _, err = fmt.Sscan(replyID, &replyInt); err != nil || replyInt < 1 {
			return Message{}, false, ErrNotFound
		}
		err = tx.QueryRow(ctx, `SELECT m.body,COALESCE(u.name,'') FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$3 LEFT JOIN users u ON u.id=m.sender_id WHERE m.id=$2 AND m.conversation_id=$1 AND m.id>cm.cleared_through`, conversationID, replyInt, userID).Scan(&replyBody, &replySender)
		if errors.Is(err, pgx.ErrNoRows) {
			return Message{}, false, ErrNotFound
		}
		if err != nil {
			return Message{}, false, err
		}
		reply = replyInt
	}
	var m Message
	err = tx.QueryRow(ctx, `INSERT INTO messages(conversation_id,sender_id,client_message_id,body,reply_to_id)
	    VALUES($1,$2,$3,$4,$5) ON CONFLICT(sender_id,client_message_id) DO NOTHING
	    RETURNING id::text,conversation_id::text,sender_id::text,client_message_id::text,body,created_at,COALESCE(reply_to_id::text,'')`, conversationID, userID, clientID, body, reply).
		Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt, &m.ReplyToID)
	if err == nil {
		m.ReplyBody = replyBody
		m.ReplySenderName = replySender
		return m, true, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return m, false, err
	}
	err = tx.QueryRow(ctx, `SELECT m.id::text,m.conversation_id::text,m.sender_id::text,m.client_message_id::text,m.body,m.created_at,COALESCE(m.reply_to_id::text,''),COALESCE(parent.body,''),COALESCE(reply_author.name,'')
        FROM messages m LEFT JOIN messages parent ON parent.id=m.reply_to_id LEFT JOIN users reply_author ON reply_author.id=parent.sender_id WHERE m.sender_id=$1 AND m.client_message_id=$2`, userID, clientID).
		Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt, &m.ReplyToID, &m.ReplyBody, &m.ReplySenderName)
	if err == nil && (m.ConversationID != conversationID || m.Body != body || m.ReplyToID != replyID) {
		return Message{}, false, ErrConflict
	}
	if err != nil {
		return m, false, err
	}
	if err = checkMessageVisible(ctx, tx, conversationID, userID, m.ID); err != nil {
		return Message{}, false, err
	}
	return m, false, tx.Commit(ctx)
}
