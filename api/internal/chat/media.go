package chat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

func (s *Store) SendVoice(ctx context.Context, c, u, id, mime string, data []byte, duration int) (Message, bool, error) {
	if err := s.CheckMember(ctx, c, u); err != nil {
		return Message{}, false, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Message{}, false, err
	}
	defer tx.Rollback(context.Background())
	if err = lockSending(ctx, tx, c, u, id); err != nil {
		return Message{}, false, err
	}
	var m Message
	err = tx.QueryRow(ctx, `INSERT INTO messages(conversation_id,sender_id,client_message_id,body,kind,audio,audio_mime,duration_ms) VALUES($1,$2,$3,'','voice',$4,$5,$6) ON CONFLICT(sender_id,client_message_id) DO NOTHING RETURNING id::text,conversation_id::text,sender_id::text,client_message_id::text,body,created_at,kind,duration_ms`, c, u, id, data, mime, duration).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt, &m.Kind, &m.DurationMS)
	created := err == nil
	if errors.Is(err, pgx.ErrNoRows) {
		var original []byte
		var originalMime string
		err = tx.QueryRow(ctx, `SELECT id::text,conversation_id::text,sender_id::text,client_message_id::text,body,created_at,kind,COALESCE(duration_ms,0),COALESCE(audio,''::bytea),COALESCE(audio_mime,'') FROM messages WHERE sender_id=$1 AND client_message_id=$2`, u, id).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt, &m.Kind, &m.DurationMS, &original, &originalMime)
		if err == nil && (m.Kind != "voice" || m.ConversationID != c || m.DurationMS != duration || originalMime != mime || !bytes.Equal(original, data)) {
			return Message{}, false, ErrConflict
		}
	}
	if err == nil {
		m.AudioURL = fmt.Sprintf("/api/messages/%s/audio", m.ID)
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
func (s *Store) Audio(ctx context.Context, id, u string) ([]byte, string, error) {
	var b []byte
	var mime string
	err := s.db.QueryRow(ctx, `SELECT m.audio,m.audio_mime FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$2 WHERE m.id=$1 AND m.id>cm.cleared_through AND m.kind='voice'`, id, u).Scan(&b, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return b, mime, err
}
func (s *Store) SetPhoto(ctx context.Context, u string, b []byte, mime string, profile, thumbnail []byte) (string, error) {
	var revision string
	err := s.db.QueryRow(ctx, `INSERT INTO profile_photos(user_id,data,mime,profile_data,thumbnail_data) VALUES($1,$2,$3,$4,$5) ON CONFLICT(user_id) DO UPDATE SET data=EXCLUDED.data,mime=EXCLUDED.mime,profile_data=EXCLUDED.profile_data,thumbnail_data=EXCLUDED.thumbnail_data,revision=gen_random_uuid() RETURNING revision::text`, u, b, mime, profile, thumbnail).Scan(&revision)
	return "/api/profiles/" + u + "/photo?v=" + revision, err
}
func (s *Store) DeletePhoto(ctx context.Context, u string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM profile_photos WHERE user_id=$1`, u)
	return err
}
func (s *Store) Photo(ctx context.Context, u, size string) ([]byte, string, string, error) {
	var b []byte
	var mime string
	var revision string
	query := `SELECT data,mime,revision::text FROM profile_photos WHERE user_id=$1`
	switch size {
	case "128":
		query = `SELECT COALESCE(thumbnail_data,profile_data,data),mime,revision::text FROM profile_photos WHERE user_id=$1`
	case "512":
		query = `SELECT COALESCE(profile_data,data),mime,revision::text FROM profile_photos WHERE user_id=$1`
	}
	err := s.db.QueryRow(ctx, query, u).Scan(&b, &mime, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return b, mime, revision, err
}
