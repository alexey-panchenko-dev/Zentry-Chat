package chat

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"newsChat/api/internal/calls"
	"newsChat/api/internal/contactguard"
)

var ErrBlocked = contactguard.ErrBlocked

type Controls struct {
	Hidden         bool   `json:"hidden"`
	BlockedByMe    bool   `json:"blocked_by_me"`
	BlockedMe      bool   `json:"blocked_me"`
	ClearedThrough string `json:"cleared_through"`
}
type ControlChange struct {
	State Controls
	Users []string
	Calls []calls.Call
}
type rowQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func controls(ctx context.Context, q rowQuery, id, user string) (Controls, error) {
	var v Controls
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contact_blocks b WHERE b.blocker_id=$2 AND b.blocked_id=CASE WHEN c.user_low=$2 THEN c.user_high ELSE c.user_low END),EXISTS(SELECT 1 FROM contact_blocks b WHERE b.blocked_id=$2 AND b.blocker_id=CASE WHEN c.user_low=$2 THEN c.user_high ELSE c.user_low END),m.cleared_through::text,m.hidden FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id WHERE c.id=$1 AND m.user_id=$2`, id, user).Scan(&v.BlockedByMe, &v.BlockedMe, &v.ClearedThrough, &v.Hidden)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return v, err
}
func (s *Store) Controls(ctx context.Context, id, user string) (Controls, error) {
	return controls(ctx, s.db, id, user)
}
func conversationPeer(ctx context.Context, tx pgx.Tx, id, user string) (string, error) {
	var peer string
	err := tx.QueryRow(ctx, `SELECT CASE WHEN c.user_low=$2 THEN c.user_high ELSE c.user_low END::text FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id WHERE c.id=$1 AND m.user_id=$2`, id, user).Scan(&peer)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return peer, err
}
func lockConversation(ctx context.Context, tx pgx.Tx, id, user string) error {
	var found bool
	err := tx.QueryRow(ctx, `SELECT true FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id WHERE c.id=$1 AND m.user_id=$2 FOR UPDATE OF c`, id, user).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return err
}
func lockSending(ctx context.Context, tx pgx.Tx, id, user, clientID string) error {
	peer, err := conversationPeer(ctx, tx, id, user)
	if err != nil {
		return err
	}
	if err = contactguard.LockUsers(ctx, tx, user, peer); err != nil {
		return err
	}
	if err = contactguard.CheckBlocked(ctx, tx, user, peer); err != nil {
		return err
	}
	if err = contactguard.CheckFirstMessage(ctx, tx, user, peer, id, clientID); err != nil {
		return err
	}
	return lockConversation(ctx, tx, id, user)
}
func (s *Store) ChangeControls(ctx context.Context, id, user, action string) (ControlChange, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return ControlChange{}, err
	}
	defer tx.Rollback(context.Background())
	peer, err := conversationPeer(ctx, tx, id, user)
	if err != nil {
		return ControlChange{}, err
	}
	change := ControlChange{Users: []string{user}}
	if action == "block" || action == "unblock" {
		if err = contactguard.LockUsers(ctx, tx, user, peer); err != nil {
			return change, err
		}
		if action == "block" {
			change.Calls, err = calls.TerminatePair(ctx, tx, user, peer)
			if err != nil {
				return change, err
			}
		}
		change.Users = append(change.Users, peer)
	}
	if action == "delete_everyone" {
		change.Users = append(change.Users, peer)
	}
	if err = lockConversation(ctx, tx, id, user); err != nil {
		return change, err
	}
	switch action {
	case "clear", "delete_chat":
		_, err = tx.Exec(ctx, `UPDATE conversation_members SET cleared_through=GREATEST(cleared_through,COALESCE((SELECT max(id) FROM messages WHERE conversation_id=$1),0)),last_read_id=GREATEST(last_read_id,COALESCE((SELECT max(id) FROM messages WHERE conversation_id=$1),0)),hidden=CASE WHEN $3 THEN true ELSE hidden END WHERE conversation_id=$1 AND user_id=$2`, id, user, action == "delete_chat")
	case "delete_everyone":
		_, err = tx.Exec(ctx, `UPDATE conversation_members SET cleared_through=GREATEST(cleared_through,COALESCE((SELECT max(id) FROM messages WHERE conversation_id=$1),0)),last_read_id=GREATEST(last_read_id,COALESCE((SELECT max(id) FROM messages WHERE conversation_id=$1),0)),hidden=true WHERE conversation_id=$1`, id)
	case "remove_contact":
		_, err = tx.Exec(ctx, `UPDATE conversation_members SET hidden=true WHERE conversation_id=$1 AND user_id=$2`, id, user)
		if err == nil {
			_, err = tx.Exec(ctx, `DELETE FROM contact_aliases WHERE owner_id=$1 AND peer_id=$2`, user, peer)
		}
	case "block":
		_, err = tx.Exec(ctx, `INSERT INTO contact_blocks(blocker_id,blocked_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, user, peer)
	case "unblock":
		_, err = tx.Exec(ctx, `DELETE FROM contact_blocks WHERE blocker_id=$1 AND blocked_id=$2`, user, peer)
	default:
		return change, errors.New("invalid control action")
	}
	if err != nil {
		return change, err
	}
	change.State, err = controls(ctx, tx, id, user)
	if err != nil {
		return change, err
	}
	return change, tx.Commit(ctx)
}
func checkMessageVisible(ctx context.Context, tx pgx.Tx, id, user, messageID string) error {
	var visible bool
	err := tx.QueryRow(ctx, `SELECT $3::bigint>cleared_through FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`, id, user, messageID).Scan(&visible)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !visible {
		return ErrNotFound
	}
	return err
}
