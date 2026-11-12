package contactguard

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

var ErrPrivacy = errors.New("contact privacy restriction")

type PrivacyQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func Allowed(ctx context.Context, q PrivacyQuery, owner, actor, field string) (bool, error) {
	if owner == actor {
		return true, nil
	}
	switch field {
	case "messages", "calls", "presence", "search", "birthday":
	default:
		return false, ErrPrivacy
	}
	var allowed bool
	err := q.QueryRow(ctx, `SELECT COALESCE(p.`+field+`,'everyone')='everyone' OR (p.`+field+`='contacts' AND EXISTS(SELECT 1 FROM contact_aliases WHERE owner_id=$1 AND peer_id=$2)) FROM users u LEFT JOIN user_privacy p ON p.user_id=u.id WHERE u.id=$1`, owner, actor).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return allowed, err
}
func CheckFirstMessage(ctx context.Context, tx pgx.Tx, actor, peer, conversation, clientID string) error {
	if _, err := tx.Exec(ctx, `UPDATE message_requests SET status='accepted',updated_at=now() WHERE conversation_id=$1 AND recipient_id=$2 AND status='pending'`, conversation, actor); err != nil {
		return err
	}
	var requestState string
	err := tx.QueryRow(ctx, `SELECT status FROM message_requests WHERE conversation_id=$1 AND recipient_id=$2`, conversation, peer).Scan(&requestState)
	if err == nil {
		if requestState == "pending" {
			var retry bool
			if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE conversation_id=$1 AND sender_id=$2 AND client_message_id=$3)`, conversation, actor, clientID).Scan(&retry); e != nil {
				return e
			}
			if retry {
				return nil
			}
		}
		if requestState == "accepted" {
			return nil
		}
		return ErrPrivacy
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	// Existing conversations remain usable: this setting controls who writes first.
	var started bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE conversation_id=$1 AND (sender_id=$2 OR created_at<COALESCE((SELECT updated_at FROM user_privacy WHERE user_id=$2),'-infinity'::timestamptz)))`, conversation, peer).Scan(&started); err != nil {
		return err
	}
	if started {
		return nil
	}
	var requests bool
	if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_privacy WHERE user_id=$1 AND messages='requests')`, peer).Scan(&requests); e != nil {
		return e
	}
	if requests {
		_, e := tx.Exec(ctx, `INSERT INTO message_requests(conversation_id,sender_id,recipient_id) VALUES($1,$2,$3)`, conversation, actor, peer)
		return e
	}
	allowed, err := Allowed(ctx, tx, peer, actor, "messages")
	if err != nil {
		return err
	}
	if !allowed {
		return ErrPrivacy
	}
	return nil
}
