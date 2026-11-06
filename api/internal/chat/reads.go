package chat

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

type ReadState struct {
	PeerLastDeliveredID string `json:"peer_last_delivered_id"`
	UnreadCount         int    `json:"unread_count"`
	LastMessageID       string `json:"last_message_id"`
	PeerLastReadID      string `json:"peer_last_read_id"`
}

func (s *Store) UnreadChats(ctx context.Context, user string) (int, error) {
	var count int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM conversation_members cm WHERE cm.user_id=$1 AND NOT cm.hidden AND EXISTS(SELECT 1 FROM messages m WHERE m.conversation_id=cm.conversation_id AND m.id>GREATEST(cm.last_read_id,cm.cleared_through) AND (m.sender_id IS NULL OR m.sender_id<>$1))`, user).Scan(&count)
	return count, err
}
func (s *Store) ReadState(ctx context.Context, id, user string) (ReadState, error) {
	var state ReadState
	err := s.db.QueryRow(ctx, `SELECT (SELECT count(*) FROM messages m WHERE m.conversation_id=cm.conversation_id AND m.id>GREATEST(cm.last_read_id,cm.cleared_through) AND (m.sender_id IS NULL OR m.sender_id<>$2)),COALESCE((SELECT max(id) FROM messages WHERE conversation_id=cm.conversation_id AND id>cm.cleared_through),0)::text,COALESCE((SELECT CASE WHEN viewed_through>cm.cleared_through THEN viewed_through ELSE 0 END FROM conversation_members WHERE conversation_id=cm.conversation_id AND user_id<>$2),0)::text,COALESCE((SELECT received_through FROM conversation_members WHERE conversation_id=cm.conversation_id AND user_id<>$2),0)::text FROM conversation_members cm WHERE cm.conversation_id=$1 AND cm.user_id=$2`, id, user).Scan(&state.UnreadCount, &state.LastMessageID, &state.PeerLastReadID, &state.PeerLastDeliveredID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return state, err
}
func (s *Store) MarkRead(ctx context.Context, id, user string, through int64) (string, []string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", nil, err
	}
	defer tx.Rollback(context.Background())
	// Sends and reads use the same conversation lock, so a lower message ID
	// cannot commit after a newer read cursor and disappear from unread counts.
	var exists bool
	err = tx.QueryRow(ctx, `SELECT true FROM conversations c JOIN conversation_members cm ON cm.conversation_id=c.id WHERE c.id=$1 AND cm.user_id=$2 FOR UPDATE OF c`, id, user).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, ErrNotFound
	}
	if err != nil {
		return "", nil, err
	}
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE conversation_id=$1 AND id=$2 AND id>(SELECT cleared_through FROM conversation_members WHERE conversation_id=$1 AND user_id=$3))`, id, through, user).Scan(&exists)
	if err != nil {
		return "", nil, err
	}
	if !exists {
		return "", nil, ErrNotFound
	}
	var cursor string
	err = tx.QueryRow(ctx, `UPDATE conversation_members SET last_read_id=GREATEST(last_read_id,$3),viewed_through=GREATEST(viewed_through,$3),received_through=GREATEST(received_through,$3) WHERE conversation_id=$1 AND user_id=$2 RETURNING last_read_id::text`, id, user, through).Scan(&cursor)
	if err != nil {
		return "", nil, err
	}
	rows, err := tx.Query(ctx, `SELECT user_id::text FROM conversation_members WHERE conversation_id=$1`, id)
	if err != nil {
		return "", nil, err
	}
	var users []string
	for rows.Next() {
		var u string
		if err = rows.Scan(&u); err != nil {
			rows.Close()
			return "", nil, err
		}
		users = append(users, u)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return "", nil, err
	}
	return cursor, users, tx.Commit(ctx)
}
func (s *Store) Members(ctx context.Context, id string) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT user_id::text FROM conversation_members WHERE conversation_id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]string, 0, 2)
	for rows.Next() {
		var u string
		if err = rows.Scan(&u); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
