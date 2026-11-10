package chat

import (
	"context"
	"strconv"
	"time"
)

type Favorite struct {
	ID     string    `json:"id"`
	ChatID string    `json:"chat_id"`
	Scope  string    `json:"scope"`
	Title  string    `json:"title"`
	Body   string    `json:"body"`
	Kind   string    `json:"kind"`
	Saved  time.Time `json:"saved_at"`
}

func (s *Store) Favorites(ctx context.Context, user string) ([]Favorite, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, chat_id, scope, title, body, kind, saved_at FROM (
			SELECT m.id::text id, m.conversation_id::text chat_id, 'personal' scope,
				COALESCE(a.alias, peer.name) title, COALESCE(NULLIF(m.body,''), m.attachment_name, '') body, m.kind, f.saved_at
			FROM personal_message_favorites f
			JOIN messages m ON m.id=f.message_id
			JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=f.user_id
			JOIN conversations c ON c.id=m.conversation_id
			JOIN users peer ON peer.id=CASE WHEN c.user_low=f.user_id THEN c.user_high ELSE c.user_low END
			LEFT JOIN contact_aliases a ON a.owner_id=f.user_id AND a.peer_id=peer.id
			WHERE f.user_id=$1 AND NOT cm.hidden AND m.id>cm.cleared_through
			UNION ALL
			SELECT m.id::text, m.group_id::text, 'group', g.name,
				COALESCE(NULLIF(m.body,''), m.attachment_name, ''), m.kind, f.saved_at
			FROM personal_message_favorites f
			JOIN group_messages m ON m.id=f.group_message_id
			JOIN group_members gm ON gm.group_id=m.group_id AND gm.user_id=f.user_id
			JOIN group_chats g ON g.id=m.group_id
			WHERE f.user_id=$1
		) items ORDER BY saved_at DESC, scope, id LIMIT 200`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Favorite{}
	for rows.Next() {
		var f Favorite
		if err := rows.Scan(&f.ID, &f.ChatID, &f.Scope, &f.Title, &f.Body, &f.Kind, &f.Saved); err != nil {
			return nil, err
		}
		items = append(items, f)
	}
	return items, rows.Err()
}

func (s *Store) SetFavorite(ctx context.Context, user, scope string, id int64, save bool) error {
	if scope != "personal" && scope != "group" {
		return ErrNotFound
	}
	if !save {
		_, err := s.db.Exec(ctx, `DELETE FROM personal_message_favorites WHERE user_id=$1
			AND (($2='personal' AND message_id=$3) OR ($2='group' AND group_message_id=$3))`, user, scope, id)
		return err
	}
	var visible bool
	if scope == "group" {
		err := s.db.QueryRow(ctx, `WITH eligible AS (
			SELECT m.id FROM group_messages m JOIN group_members gm ON gm.group_id=m.group_id
			WHERE m.id=$2 AND gm.user_id=$1
		), added AS (
			INSERT INTO personal_message_favorites(user_id,group_message_id)
			SELECT $1,id FROM eligible ON CONFLICT DO NOTHING
		) SELECT EXISTS(SELECT 1 FROM eligible)`, user, id).Scan(&visible)
		if err != nil {
			return err
		}
	} else {
		err := s.db.QueryRow(ctx, `WITH eligible AS (
			SELECT m.id FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id
			WHERE m.id=$2 AND cm.user_id=$1 AND NOT cm.hidden AND m.id>cm.cleared_through
		), added AS (
			INSERT INTO personal_message_favorites(user_id,message_id)
			SELECT $1,id FROM eligible ON CONFLICT DO NOTHING
		) SELECT EXISTS(SELECT 1 FROM eligible)`, user, id).Scan(&visible)
		if err != nil {
			return err
		}
	}
	if !visible {
		return ErrNotFound
	}
	return nil
}

func (s *Store) FirstUnread(ctx context.Context, user, conversationID string) (string, error) {
	var id int64
	err := s.db.QueryRow(ctx, `SELECT COALESCE(min(m.id),0)
		FROM conversation_members cm JOIN messages m ON m.conversation_id=cm.conversation_id
		WHERE cm.user_id=$1 AND cm.conversation_id=$2 AND NOT cm.hidden
		  AND m.id>GREATEST(cm.last_read_id,cm.cleared_through)
		  AND m.sender_id IS DISTINCT FROM cm.user_id`, user, conversationID).Scan(&id)
	if id == 0 {
		return "", err
	}
	return strconv.FormatInt(id, 10), err
}
