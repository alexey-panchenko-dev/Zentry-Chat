package chat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

func (s *Store) SendImage(ctx context.Context, c, u, id, caption string, data []byte) (Message, bool, error) {
	if err := s.CheckMember(ctx, c, u); err != nil {
		return Message{}, false, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Message{}, false, err
	}
	defer tx.Rollback(context.Background())
	if err = lockSending(ctx, tx, c, u); err != nil {
		return Message{}, false, err
	}
	var m Message
	err = tx.QueryRow(ctx, `INSERT INTO messages(conversation_id,sender_id,client_message_id,body,kind,image,image_mime) VALUES($1,$2,$3,$4,'image',$5,'image/jpeg') ON CONFLICT(sender_id,client_message_id) DO NOTHING RETURNING id::text,conversation_id::text,sender_id::text,client_message_id::text,body,created_at,kind`, c, u, id, caption, data).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt, &m.Kind)
	created := err == nil
	if errors.Is(err, pgx.ErrNoRows) {
		var original []byte
		var originalMime string
		err = tx.QueryRow(ctx, `SELECT id::text,conversation_id::text,sender_id::text,client_message_id::text,body,created_at,kind,COALESCE(image,''::bytea),COALESCE(image_mime,'') FROM messages WHERE sender_id=$1 AND client_message_id=$2`, u, id).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt, &m.Kind, &original, &originalMime)
		if err == nil && (m.Kind != "image" || m.ConversationID != c || m.Body != caption || originalMime != "image/jpeg" || !bytes.Equal(original, data)) {
			return Message{}, false, ErrConflict
		}
	}
	if err == nil {
		m.ImageURL = fmt.Sprintf("/api/messages/%s/image", m.ID)
	}
	if err != nil {
		return m, created, err
	}
	if !created {
		if err = checkMessageVisible(ctx, tx, c, u, m.ID); err != nil {
			return Message{}, false, err
		}
	}
	return m, created, tx.Commit(ctx)
}
func (s *Store) Image(ctx context.Context, id, u string) ([]byte, string, error) {
	var b []byte
	var mime string
	err := s.db.QueryRow(ctx, `SELECT m.image,m.image_mime FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$2 WHERE m.id=$1 AND m.id>cm.cleared_through AND m.kind='image'`, id, u).Scan(&b, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return b, mime, err
}
