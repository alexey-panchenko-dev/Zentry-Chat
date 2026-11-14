package chat

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s *Store) SetGroupPhoto(ctx context.Context, groupID, userID string, data []byte, mime string) (string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.Background())
	role, err := lockManagedGroup(ctx, tx, groupID, userID)
	if err != nil {
		return "", err
	}
	if role != "owner" && role != "admin" {
		return "", ErrForbidden
	}
	var revision string
	err = tx.QueryRow(ctx, `UPDATE group_chats SET avatar=$2,avatar_mime=$3,avatar_revision=gen_random_uuid(),updated_at=now() WHERE id=$1 RETURNING avatar_revision::text`, groupID, data, mime).Scan(&revision)
	if err != nil {
		return "", err
	}
	return "/api/groups/" + groupID + "/photo?v=" + revision, tx.Commit(ctx)
}

func (s *Store) GroupPhoto(ctx context.Context, groupID, userID string) ([]byte, string, error) {
	var data []byte
	var mime string
	err := s.db.QueryRow(ctx, `SELECT g.avatar,g.avatar_mime FROM group_chats g JOIN group_members m ON m.group_id=g.id AND m.user_id=$2 WHERE g.id=$1 AND g.avatar IS NOT NULL`, groupID, userID).Scan(&data, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return data, mime, err
}
