package chat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ReactionCount struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
	Mine  bool   `json:"mine"`
}

type FeatureMessage struct {
	ImageURL        string     `json:"image_url,omitempty"`
	AttachmentSize  int        `json:"attachment_size,omitempty"`
	ID              string     `json:"id"`
	ConversationID  string     `json:"conversation_id"`
	SenderID        string     `json:"sender_id"`
	SenderName      string     `json:"sender_name"`
	Body            string     `json:"body"`
	Kind            string     `json:"kind"`
	CreatedAt       time.Time  `json:"created_at"`
	EditedAt        *time.Time `json:"edited_at,omitempty"`
	ReplyToID       string     `json:"reply_to_id,omitempty"`
	ReplyBody       string     `json:"reply_body,omitempty"`
	ReplySenderName string     `json:"reply_sender_name,omitempty"`
	ForwardedFromID string     `json:"forwarded_from_id,omitempty"`
	AttachmentName  string     `json:"attachment_name,omitempty"`
	AttachmentMIME  string     `json:"attachment_mime,omitempty"`
	AttachmentURL   string     `json:"attachment_url,omitempty"`
}

var ErrMessageUnavailable = errors.New("message unavailable")

func (s *Store) MessageConversation(ctx context.Context, messageID, userID string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `SELECT m.conversation_id::text FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$2 WHERE m.id=$1 AND m.id>cm.cleared_through`, messageID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

func (s *Store) SetReaction(ctx context.Context, messageID, userID, emoji string) ([]ReactionCount, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	var conversationID string
	err = tx.QueryRow(ctx, `SELECT m.conversation_id::text FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$2 WHERE m.id=$1 AND m.id>cm.cleared_through`, messageID, userID).Scan(&conversationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if emoji == "" {
		_, err = tx.Exec(ctx, `DELETE FROM message_reactions WHERE message_id=$1 AND user_id=$2`, messageID, userID)
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO message_reactions(message_id,user_id,emoji) VALUES($1,$2,$3) ON CONFLICT(message_id,user_id) DO UPDATE SET emoji=EXCLUDED.emoji`, messageID, userID, emoji)
	}
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT emoji,count(*)::int,bool_or(user_id=$2) FROM message_reactions WHERE message_id=$1 GROUP BY emoji ORDER BY emoji`, messageID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ReactionCount, 0)
	for rows.Next() {
		var v ReactionCount
		if err = rows.Scan(&v.Emoji, &v.Count, &v.Mine); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, tx.Commit(ctx)
}

func (s *Store) Reactions(ctx context.Context, messageID, userID string) ([]ReactionCount, error) {
	var visible bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$2 WHERE m.id=$1 AND m.id>cm.cleared_through)`, messageID, userID).Scan(&visible); err != nil {
		return nil, err
	}
	if !visible {
		return nil, ErrNotFound
	}
	rows, err := s.db.Query(ctx, `SELECT emoji,count(*)::int,bool_or(user_id=$2) FROM message_reactions WHERE message_id=$1 GROUP BY emoji ORDER BY emoji`, messageID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReactionCount{}
	for rows.Next() {
		var v ReactionCount
		if err = rows.Scan(&v.Emoji, &v.Count, &v.Mine); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) EditMessage(ctx context.Context, messageID, userID, body string) (FeatureMessage, error) {
	var m FeatureMessage
	err := s.db.QueryRow(ctx, `UPDATE messages m SET body=$3,edited_at=now() FROM conversation_members cm WHERE m.id=$1 AND m.sender_id=$2 AND m.kind='text' AND m.created_at>now()-interval '15 minutes' AND cm.conversation_id=m.conversation_id AND cm.user_id=$2 AND m.id>cm.cleared_through RETURNING m.id::text,m.conversation_id::text,m.sender_id::text,m.body,m.kind,m.created_at,m.edited_at`, messageID, userID, body).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Body, &m.Kind, &m.CreatedAt, &m.EditedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, ErrMessageUnavailable
	}
	return m, err
}

func (s *Store) SetPinned(ctx context.Context, conversationID, messageID, userID string, pinned bool) error {
	if pinned {
		var exists bool
		err := s.db.QueryRow(ctx, `WITH added AS (INSERT INTO pinned_messages(conversation_id,message_id,pinned_by) SELECT m.conversation_id,m.id,$3 FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$3 WHERE m.id=$2 AND m.conversation_id=$1 AND m.id>cm.cleared_through ON CONFLICT DO NOTHING RETURNING 1) SELECT EXISTS(SELECT 1 FROM added) OR EXISTS(SELECT 1 FROM pinned_messages p JOIN conversation_members cm ON cm.conversation_id=p.conversation_id AND cm.user_id=$3 WHERE p.conversation_id=$1 AND p.message_id=$2 AND p.message_id>cm.cleared_through)`, conversationID, messageID, userID).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		return nil
	}
	result, err := s.db.Exec(ctx, `DELETE FROM pinned_messages p USING conversation_members cm WHERE p.conversation_id=$1 AND p.message_id=$2 AND cm.conversation_id=p.conversation_id AND cm.user_id=$3`, conversationID, messageID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Pinned(ctx context.Context, conversationID, userID string) ([]FeatureMessage, error) {
	rows, err := s.db.Query(ctx, `SELECT m.id::text,m.conversation_id::text,COALESCE(m.sender_id::text,''),COALESCE(u.name,''),m.body,m.kind,m.created_at,m.edited_at,COALESCE(m.reply_to_id::text,''),COALESCE(r.body,''),COALESCE(ru.name,''),COALESCE(m.forwarded_from_id::text,''),COALESCE(m.attachment_name,''),COALESCE(m.attachment_mime,''),COALESCE(octet_length(m.attachment),0) FROM pinned_messages p JOIN conversation_members cm ON cm.conversation_id=p.conversation_id AND cm.user_id=$2 JOIN messages m ON m.id=p.message_id AND m.conversation_id=p.conversation_id AND m.id>cm.cleared_through LEFT JOIN users u ON u.id=m.sender_id LEFT JOIN messages r ON r.id=m.reply_to_id AND r.conversation_id=m.conversation_id AND r.id>cm.cleared_through LEFT JOIN users ru ON ru.id=r.sender_id WHERE p.conversation_id=$1 ORDER BY p.created_at DESC LIMIT 100`, conversationID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FeatureMessage{}
	for rows.Next() {
		var m FeatureMessage
		if err = rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.SenderName, &m.Body, &m.Kind, &m.CreatedAt, &m.EditedAt, &m.ReplyToID, &m.ReplyBody, &m.ReplySenderName, &m.ForwardedFromID, &m.AttachmentName, &m.AttachmentMIME, &m.AttachmentSize); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) SearchMessages(ctx context.Context, conversationID, userID, query, kind string, before int64, limit int) ([]FeatureMessage, error) {
	needle := strings.ToLower(query)
	rows, err := s.db.Query(ctx, `SELECT m.id::text,m.conversation_id::text,COALESCE(m.sender_id::text,''),COALESCE(u.name,''),m.body,m.kind,m.created_at,m.edited_at,COALESCE(m.reply_to_id::text,''),COALESCE(r.body,''),COALESCE(ru.name,''),COALESCE(m.forwarded_from_id::text,''),COALESCE(m.attachment_name,''),COALESCE(m.attachment_mime,''),COALESCE(octet_length(m.attachment),0) FROM conversation_members cm JOIN messages m ON m.conversation_id=cm.conversation_id AND m.id>cm.cleared_through LEFT JOIN users u ON u.id=m.sender_id LEFT JOIN messages r ON r.id=m.reply_to_id AND r.conversation_id=m.conversation_id AND r.id>cm.cleared_through LEFT JOIN users ru ON ru.id=r.sender_id WHERE cm.conversation_id=$1 AND cm.user_id=$2 AND ($3::bigint=0 OR m.id<$3) AND ($4='' OR strpos(lower(m.body),$4)>0 OR strpos(lower(COALESCE(m.attachment_name,'')),$4)>0) AND ($5='' OR ($5='messages' AND m.kind IN ('text','system')) OR ($5='photos' AND m.kind='image') OR ($5='voice' AND m.kind='voice') OR ($5='videos' AND m.kind='file' AND m.attachment_mime LIKE 'video/%') OR ($5='files' AND m.kind='file') OR ($5='links' AND m.body LIKE '%http%')) ORDER BY m.id DESC LIMIT $6`, conversationID, userID, before, needle, kind, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FeatureMessage{}
	for rows.Next() {
		var m FeatureMessage
		if err = rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.SenderName, &m.Body, &m.Kind, &m.CreatedAt, &m.EditedAt, &m.ReplyToID, &m.ReplyBody, &m.ReplySenderName, &m.ForwardedFromID, &m.AttachmentName, &m.AttachmentMIME, &m.AttachmentSize); err != nil {
			return nil, err
		}
		if m.Kind == "file" {
			m.AttachmentURL = "/api/messages/" + m.ID + "/file"
		}
		if m.Kind == "image" {
			m.ImageURL = "/api/messages/" + m.ID + "/image"
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) ForwardMessage(ctx context.Context, sourceID, destinationID, userID, newID string) (Message, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback(context.Background())
	var src Message
	var audio, image, attachment []byte
	var audioMIME, imageMIME, attachmentMIME, attachmentName string
	err = tx.QueryRow(ctx, `SELECT m.id::text,m.conversation_id::text,m.sender_id::text,m.client_message_id::text,m.body,m.created_at,m.kind,m.audio,COALESCE(m.audio_mime,''),COALESCE(m.duration_ms,0),m.image,COALESCE(m.image_mime,''),m.attachment,COALESCE(m.attachment_mime,''),COALESCE(m.attachment_name,'') FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$2 WHERE m.id=$1 AND m.id>cm.cleared_through AND m.kind IN ('text','image','voice','file')`, sourceID, userID).Scan(&src.ID, &src.ConversationID, &src.SenderID, &src.ClientMessageID, &src.Body, &src.CreatedAt, &src.Kind, &audio, &audioMIME, &src.DurationMS, &image, &imageMIME, &attachment, &attachmentMIME, &attachmentName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, ErrNotFound
	}
	if err != nil {
		return Message{}, err
	}
	if err = lockSending(ctx, tx, destinationID, userID, newID); err != nil {
		return Message{}, err
	}
	var visible bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 AND NOT hidden)`, destinationID, userID).Scan(&visible); err != nil {
		return Message{}, err
	}
	if !visible {
		return Message{}, ErrNotFound
	}
	var m Message
	err = tx.QueryRow(ctx, `INSERT INTO messages(conversation_id,sender_id,client_message_id,body,kind,audio,audio_mime,duration_ms,image,image_mime,attachment,attachment_mime,attachment_name,forwarded_from_id) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,0),$9,NULLIF($10,''),$11,NULLIF($12,''),NULLIF($13,''),$14) RETURNING id::text,conversation_id::text,sender_id::text,client_message_id::text,body,created_at,kind,COALESCE(duration_ms,0),COALESCE(attachment_name,''),COALESCE(attachment_mime,'')`, destinationID, userID, newID, src.Body, src.Kind, audio, audioMIME, src.DurationMS, image, imageMIME, attachment, attachmentMIME, attachmentName, sourceID).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt, &m.Kind, &m.DurationMS, &m.AttachmentName, &m.AttachmentMIME)
	if err != nil {
		return Message{}, err
	}
	m.ForwardedFromID = sourceID
	if m.Kind == "image" {
		m.ImageURL = "/api/messages/" + m.ID + "/image"
	}
	if m.Kind == "voice" {
		m.AudioURL = "/api/messages/" + m.ID + "/audio"
	}
	if m.Kind == "file" {
		m.AttachmentURL = "/api/messages/" + m.ID + "/file"
	}
	return m, tx.Commit(ctx)
}

func (s *Store) SendFile(ctx context.Context, conversationID, userID, clientID, caption, name, mimeType string, data []byte) (Message, bool, error) {
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
	var m Message
	err = tx.QueryRow(ctx, `INSERT INTO messages(conversation_id,sender_id,client_message_id,body,kind,attachment,attachment_mime,attachment_name) VALUES($1,$2,$3,$4,'file',$5,$6,$7) ON CONFLICT(sender_id,client_message_id) DO NOTHING RETURNING id::text,conversation_id::text,sender_id::text,client_message_id::text,body,created_at,kind`, conversationID, userID, clientID, caption, data, mimeType, name).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt, &m.Kind)
	if err == nil {
		m.AttachmentName = name
		m.AttachmentMIME = mimeType
		return m, true, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return m, false, err
	}
	var existing []byte
	var originalName, originalMIME, originalCaption string
	err = tx.QueryRow(ctx, `SELECT id::text,conversation_id::text,sender_id::text,client_message_id::text,body,created_at,kind,attachment,attachment_name,attachment_mime FROM messages WHERE sender_id=$1 AND client_message_id=$2`, userID, clientID).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt, &m.Kind, &existing, &originalName, &originalMIME)
	originalCaption = m.Body
	if err != nil {
		return m, false, err
	}
	if m.ConversationID != conversationID || originalName != name || originalMIME != mimeType || originalCaption != caption || !bytes.Equal(existing, data) {
		return Message{}, false, ErrConflict
	}
	if err = checkMessageVisible(ctx, tx, conversationID, userID, m.ID); err != nil {
		return Message{}, false, err
	}
	m.AttachmentName = name
	m.AttachmentMIME = mimeType
	return m, false, tx.Commit(ctx)
}

func mustMessageID(v string) int64 { var n int64; _, _ = fmt.Sscan(v, &n); return n }

func (s *Store) File(ctx context.Context, messageID, userID string) ([]byte, string, string, error) {
	var body []byte
	var name, mimeType string
	err := s.db.QueryRow(ctx, `SELECT m.attachment,m.attachment_name,m.attachment_mime FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$2 WHERE m.id=$1 AND m.kind='file' AND m.id>cm.cleared_through`, messageID, userID).Scan(&body, &name, &mimeType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", "", ErrNotFound
	}
	return body, name, mimeType, err
}
