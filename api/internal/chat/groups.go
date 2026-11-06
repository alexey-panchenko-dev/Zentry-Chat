package chat

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

var ErrGroupMembers = errors.New("select at least three contacts")

type Group struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	OwnerID     string    `json:"owner_id"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type GroupMessage struct {
	ID              string    `json:"id"`
	GroupID         string    `json:"group_id"`
	SenderID        string    `json:"sender_id"`
	SenderName      string    `json:"sender_name"`
	ClientMessageID string    `json:"client_message_id"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"created_at"`
}

func (s *Store) GroupContacts(ctx context.Context, user string) ([]PublicUser, error) {
	rows, err := s.db.Query(ctx, `SELECT u.id::text,u.name,COALESCE((SELECT '/api/profiles/'||u.id::text||'/photo?v='||p.revision::text FROM profile_photos p WHERE p.user_id=u.id),'') FROM conversations c JOIN conversation_members me ON me.conversation_id=c.id AND me.user_id=$1 JOIN users u ON u.id=CASE WHEN c.user_low=$1 THEN c.user_high ELSE c.user_low END WHERE NOT me.hidden AND NOT EXISTS(SELECT 1 FROM contact_blocks b WHERE (b.blocker_id=$1 AND b.blocked_id=u.id) OR (b.blocked_id=$1 AND b.blocker_id=u.id)) ORDER BY u.name,u.id LIMIT 500`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PublicUser{}
	for rows.Next() {
		var v PublicUser
		if err = rows.Scan(&v.ID, &v.Name, &v.AvatarURL); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) CreateGroup(ctx context.Context, owner, name string, peers []string) (Group, []string, error) {
	if len(peers) < 3 || len(peers) > 49 {
		return Group{}, nil, ErrGroupMembers
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Group{}, nil, err
	}
	defer tx.Rollback(context.Background())
	var count int
	err = tx.QueryRow(ctx, `SELECT count(DISTINCT u.id) FROM users u JOIN conversations c ON c.user_low=LEAST($1::uuid,u.id) AND c.user_high=GREATEST($1::uuid,u.id) JOIN conversation_members cm ON cm.conversation_id=c.id AND cm.user_id=$1 WHERE u.id=ANY($2::uuid[]) AND u.id<>$1 AND NOT cm.hidden AND NOT EXISTS(SELECT 1 FROM contact_blocks b WHERE (b.blocker_id=$1 AND b.blocked_id=u.id) OR (b.blocked_id=$1 AND b.blocker_id=u.id))`, owner, peers).Scan(&count)
	if err != nil {
		return Group{}, nil, err
	}
	if count != len(peers) {
		return Group{}, nil, ErrGroupMembers
	}
	var g Group
	err = tx.QueryRow(ctx, `INSERT INTO group_chats(name,owner_id) VALUES($1,$2) RETURNING id::text,name,owner_id::text,created_at,updated_at`, name, owner).Scan(&g.ID, &g.Name, &g.OwnerID, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return g, nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO group_members(group_id,user_id) SELECT $1,id FROM users WHERE id=$2 OR id=ANY($3::uuid[])`, g.ID, owner, peers)
	if err != nil {
		return g, nil, err
	}
	g.MemberCount = len(peers) + 1
	return g, append([]string{owner}, peers...), tx.Commit(ctx)
}
func (s *Store) Groups(ctx context.Context, user string) ([]Group, error) {
	rows, err := s.db.Query(ctx, `SELECT g.id::text,g.name,g.owner_id::text,g.created_at,g.updated_at,(SELECT count(*) FROM group_members WHERE group_id=g.id) FROM group_chats g JOIN group_members m ON m.group_id=g.id WHERE m.user_id=$1 ORDER BY g.updated_at DESC LIMIT 200`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Group{}
	for rows.Next() {
		var g Group
		if err = rows.Scan(&g.ID, &g.Name, &g.OwnerID, &g.CreatedAt, &g.UpdatedAt, &g.MemberCount); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
func (s *Store) Group(ctx context.Context, id, user string) (Group, error) {
	var g Group
	err := s.db.QueryRow(ctx, `SELECT g.id::text,g.name,g.owner_id::text,g.created_at,g.updated_at,(SELECT count(*) FROM group_members WHERE group_id=g.id) FROM group_chats g JOIN group_members m ON m.group_id=g.id WHERE g.id=$1 AND m.user_id=$2`, id, user).Scan(&g.ID, &g.Name, &g.OwnerID, &g.CreatedAt, &g.UpdatedAt, &g.MemberCount)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return g, err
}
func (s *Store) GroupMembers(ctx context.Context, id, user string) ([]PublicUser, error) {
	if _, err := s.Group(ctx, id, user); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT u.id::text,u.name,COALESCE((SELECT '/api/profiles/'||u.id::text||'/photo?v='||p.revision::text FROM profile_photos p WHERE p.user_id=u.id),'') FROM group_members m JOIN users u ON u.id=m.user_id WHERE m.group_id=$1 ORDER BY m.joined_at,u.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PublicUser{}
	for rows.Next() {
		var v PublicUser
		if err = rows.Scan(&v.ID, &v.Name, &v.AvatarURL); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) GroupMessages(ctx context.Context, id, user string, before int64, limit int) ([]GroupMessage, error) {
	if _, err := s.Group(ctx, id, user); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT m.id::text,m.group_id::text,m.sender_id::text,u.name,m.client_message_id::text,m.body,m.created_at FROM group_messages m JOIN users u ON u.id=m.sender_id WHERE m.group_id=$1 AND ($2::bigint=0 OR m.id<$2) ORDER BY m.id DESC LIMIT $3`, id, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GroupMessage{}
	for rows.Next() {
		var m GroupMessage
		if err = rows.Scan(&m.ID, &m.GroupID, &m.SenderID, &m.SenderName, &m.ClientMessageID, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) SendGroupMessage(ctx context.Context, id, user, clientID, body string) (GroupMessage, bool, []string, error) {
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
	var blocked bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contact_blocks b JOIN group_members gm ON gm.group_id=$1 AND gm.user_id=CASE WHEN b.blocker_id=$2 THEN b.blocked_id ELSE b.blocker_id END WHERE b.blocker_id=$2 OR b.blocked_id=$2)`, id, user).Scan(&blocked)
	if err != nil {
		return GroupMessage{}, false, nil, err
	}
	if blocked {
		return GroupMessage{}, false, nil, ErrBlocked
	}
	var m GroupMessage
	err = tx.QueryRow(ctx, `INSERT INTO group_messages(group_id,sender_id,client_message_id,body) VALUES($1,$2,$3,$4) ON CONFLICT(sender_id,client_message_id) DO NOTHING RETURNING id::text,group_id::text,sender_id::text,client_message_id::text,body,created_at`, id, user, clientID, body).Scan(&m.ID, &m.GroupID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt)
	created := err == nil
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id::text,group_id::text,sender_id::text,client_message_id::text,body,created_at FROM group_messages WHERE sender_id=$1 AND client_message_id=$2`, user, clientID).Scan(&m.ID, &m.GroupID, &m.SenderID, &m.ClientMessageID, &m.Body, &m.CreatedAt)
		if err == nil && (m.GroupID != id || m.Body != body) {
			return GroupMessage{}, false, nil, ErrConflict
		}
	}
	if err != nil {
		return m, false, nil, err
	}
	err = tx.QueryRow(ctx, `SELECT name FROM users WHERE id=$1`, user).Scan(&m.SenderName)
	if err != nil {
		return m, false, nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE group_chats SET updated_at=now() WHERE id=$1`, id)
	if err != nil {
		return m, false, nil, err
	}
	rows, err := tx.Query(ctx, `SELECT user_id::text FROM group_members WHERE group_id=$1`, id)
	if err != nil {
		return m, false, nil, err
	}
	members := []string{}
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			rows.Close()
			return m, false, nil, err
		}
		members = append(members, v)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return m, false, nil, err
	}
	return m, created, members, tx.Commit(ctx)
}
