package chat

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

var ErrReportLimit = errors.New("report limit reached")

type Report struct {
	UserID    string `json:"user_id"`
	Scope     string `json:"scope"`
	MessageID int64  `json:"message_id,string"`
	Category  string `json:"category"`
	Details   string `json:"details"`
}

func (s *Store) Report(ctx context.Context, actor string, v Report) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,711))`, actor); err != nil {
		return err
	}
	var valid bool
	if v.Scope == "message" {
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$1 WHERE m.id=$2 AND m.sender_id=$3 AND m.id>cm.cleared_through)`, actor, v.MessageID, v.UserID).Scan(&valid)
	} else if v.Scope == "group_message" {
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM group_messages m JOIN group_members gm ON gm.group_id=m.group_id AND gm.user_id=$1 WHERE m.id=$2 AND m.sender_id=$3)`, actor, v.MessageID, v.UserID).Scan(&valid)
	} else {
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND email_verified=true AND id<>$2)`, v.UserID, actor).Scan(&valid)
	}
	if err != nil {
		return err
	}
	if !valid {
		return ErrNotFound
	}
	var duplicate bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM abuse_reports WHERE reporter_id=$1 AND subject_id=$2 AND scope=$3 AND COALESCE(message_id,0)=$4)`, actor, v.UserID, v.Scope, v.MessageID).Scan(&duplicate); err != nil {
		return err
	}
	if duplicate {
		return tx.Commit(ctx)
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM abuse_reports WHERE reporter_id=$1 AND created_at>now()-interval '24 hours'`, actor).Scan(&count); err != nil {
		return err
	}
	if count >= 5 {
		return ErrReportLimit
	}
	var message any
	if v.MessageID > 0 {
		message = v.MessageID
	}
	_, err = tx.Exec(ctx, `INSERT INTO abuse_reports(reporter_id,subject_id,scope,message_id,category,details) VALUES($1,$2,$3,$4,$5,$6)`, actor, v.UserID, v.Scope, message, v.Category, v.Details)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
