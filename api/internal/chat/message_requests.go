package chat

import (
	"context"
	"newsChat/api/internal/contactguard"
)

func (s *Store) AnswerMessageRequest(ctx context.Context, id, user, action string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var sender string
	if err = tx.QueryRow(ctx, `SELECT sender_id::text FROM message_requests WHERE conversation_id=$1 AND recipient_id=$2`, id, user).Scan(&sender); err != nil {
		return ErrNotFound
	}
	if err = contactguard.LockUsers(ctx, tx, user, sender); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE message_requests SET status=$3,updated_at=now() WHERE conversation_id=$1 AND recipient_id=$2 AND status='pending'`, id, user, action)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if action == "declined" {
		_, err = tx.Exec(ctx, `UPDATE conversation_members SET hidden=true WHERE conversation_id=$1 AND user_id=$2`, id, user)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
