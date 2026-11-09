package chat

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s *Store) SetGroupPhoto(ctx context.Context, groupID, userID string, data []byte, mime string) (string, error) {
	group, err := s.Group(ctx, groupID, userID)
	if err != nil {
		return "", err
	}
	if group.OwnerID != userID {
		return "", ErrForbidden
	}
	var revision string
	err = s.db.QueryRow(ctx, `UPDATE group_chats SET avatar=$3,avatar_mime=$4,avatar_revision=gen_random_uuid(),updated_at=now() WHERE id=$1 AND owner_id=$2 RETURNING avatar_revision::text`, groupID, userID, data, mime).Scan(&revision)
	return "/api/groups/" + groupID + "/photo?v=" + revision, err
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
