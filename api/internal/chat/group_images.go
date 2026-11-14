package chat

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (s *Store) SendGroupImage(ctx context.Context, id, user, clientID, caption string, data []byte) (GroupMessage, bool, []string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return GroupMessage{}, false, nil, err
	}
	defer tx.Rollback(context.Background())
	var exists bool
	err = tx.QueryRow(ctx, `SELECT true FROM group_chats g JOIN group_members gm ON gm.group_id=g.id WHERE g.id=$1 AND gm.user_id=$2 FOR UPDATE OF g`, id, user).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return GroupMessage{}, false, nil, ErrNotFound
	}
	if err != nil {
		return GroupMessage{}, false, nil, err
	}
	if err = checkGroupMember(ctx, tx, id, user); err != nil {
		return GroupMessage{}, false, nil, err
	}
	var blocked bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contact_blocks b JOIN group_members gm ON gm.group_id=$1 AND gm.user_id=CASE WHEN b.blocker_id=$2 THEN b.blocked_id ELSE b.blocker_id END WHERE b.blocker_id=$2 OR b.blocked_id=$2)`, id, user).Scan(&blocked)
	if err != nil {
		return GroupMessage{}, false, nil, err
	}
	if blocked {
		return GroupMessage{}, false, nil, ErrBlocked
	}
	var message GroupMessage
	err = tx.QueryRow(ctx, `INSERT INTO group_messages(group_id,sender_id,client_message_id,body,kind,image,image_mime) VALUES($1,$2,$3,$4,'image',$5,'image/jpeg') ON CONFLICT(sender_id,client_message_id) DO NOTHING RETURNING id::text,group_id::text,sender_id::text,client_message_id::text,body,created_at,kind`, id, user, clientID, caption, data).Scan(&message.ID, &message.GroupID, &message.SenderID, &message.ClientMessageID, &message.Body, &message.CreatedAt, &message.Kind)
	created := err == nil
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id::text,group_id::text,sender_id::text,client_message_id::text,body,created_at,kind FROM group_messages WHERE sender_id=$1 AND client_message_id=$2`, user, clientID).Scan(&message.ID, &message.GroupID, &message.SenderID, &message.ClientMessageID, &message.Body, &message.CreatedAt, &message.Kind)
		if err == nil && (message.GroupID != id || message.Body != caption || message.Kind != "image") {
			return GroupMessage{}, false, nil, ErrConflict
		}
	}
	if err != nil {
		return GroupMessage{}, false, nil, err
	}
	if !created {
		var original []byte
		if err = tx.QueryRow(ctx, `SELECT image FROM group_messages WHERE id=$1`, message.ID).Scan(&original); err != nil {
			return GroupMessage{}, false, nil, err
		}
		if !bytes.Equal(original, data) {
			return GroupMessage{}, false, nil, ErrConflict
		}
	}
	if err = tx.QueryRow(ctx, `SELECT name FROM users WHERE id=$1`, user).Scan(&message.SenderName); err != nil {
		return GroupMessage{}, false, nil, err
	}
	message.ImageURL = fmt.Sprintf("/api/groups/%s/messages/%s/image", id, message.ID)
	if _, err = tx.Exec(ctx, `UPDATE group_chats SET updated_at=now() WHERE id=$1`, id); err != nil {
		return GroupMessage{}, false, nil, err
	}
	rows, err := tx.Query(ctx, `SELECT user_id::text FROM group_members WHERE group_id=$1`, id)
	if err != nil {
		return GroupMessage{}, false, nil, err
	}
	members := []string{}
	for rows.Next() {
		var member string
		if err = rows.Scan(&member); err != nil {
			rows.Close()
			return GroupMessage{}, false, nil, err
		}
		members = append(members, member)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return GroupMessage{}, false, nil, err
	}
	return message, created, members, tx.Commit(ctx)
}

func (s *Store) GroupImages(ctx context.Context, id, user string, before int64, limit int) ([]GroupMessage, error) {
	if _, err := s.Group(ctx, id, user); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT m.id::text,m.group_id::text,m.sender_id::text,u.name,m.client_message_id::text,m.body,m.created_at,m.kind FROM group_messages m JOIN users u ON u.id=m.sender_id WHERE m.group_id=$1 AND m.kind='image' AND ($2::bigint=0 OR m.id<$2) ORDER BY m.id DESC LIMIT $3`, id, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GroupMessage{}
	for rows.Next() {
		var m GroupMessage
		if err = rows.Scan(&m.ID, &m.GroupID, &m.SenderID, &m.SenderName, &m.ClientMessageID, &m.Body, &m.CreatedAt, &m.Kind); err != nil {
			return nil, err
		}
		m.ImageURL = fmt.Sprintf("/api/groups/%s/messages/%s/image", id, m.ID)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GroupMessageImage(ctx context.Context, groupID, messageID, userID string) ([]byte, string, error) {
	var data []byte
	var mime string
	err := s.db.QueryRow(ctx, `SELECT gm.image,gm.image_mime FROM group_messages gm JOIN group_members m ON m.group_id=gm.group_id AND m.user_id=$3 WHERE gm.group_id=$1 AND gm.id=$2 AND gm.kind='image'`, groupID, messageID, userID).Scan(&data, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return data, mime, err
}
