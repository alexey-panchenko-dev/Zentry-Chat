package chat

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"newsChat/api/internal/realtime"
	"time"
)

var ErrScheduleState = errors.New("scheduled message cannot be changed")
var ErrScheduleLimit = errors.New("scheduled message limit")

type ScheduledMessage struct {
	ID       string    `json:"id"`
	Scope    string    `json:"scope"`
	ChatID   string    `json:"chat_id"`
	ClientID string    `json:"client_message_id"`
	Body     string    `json:"body"`
	SendAt   time.Time `json:"send_at"`
	Status   string    `json:"status"`
	Failure  string    `json:"failure"`
}

const scheduledColumns = `id::text,CASE WHEN group_id IS NULL THEN 'personal' ELSE 'group' END,COALESCE(conversation_id,group_id)::text,client_message_id::text,body,send_at,status,failure`

func (s *Store) Schedule(ctx context.Context, user, scope, id, client, body string, at time.Time) (ScheduledMessage, error) {
	var out ScheduledMessage
	at = at.UTC().Truncate(time.Microsecond)
	if scope == "group" {
		if _, err := s.Group(ctx, id, user); err != nil {
			return out, err
		}
	} else {
		if err := s.CheckMember(ctx, id, user); err != nil {
			return out, err
		}
		state, err := s.Controls(ctx, id, user)
		if err != nil {
			return out, err
		}
		if state.BlockedByMe || state.BlockedMe {
			return out, ErrBlocked
		}
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(context.Background())
	var actor string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id=$1 FOR UPDATE`, user).Scan(&actor); err != nil {
		return out, err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM scheduled_messages WHERE sender_id=$1 AND status='pending' AND client_message_id<>$2`, user, client).Scan(&count); err != nil {
		return out, err
	}
	if count >= 500 {
		return out, ErrScheduleLimit
	}
	var conversation, group any
	if scope == "group" {
		group = id
	} else {
		conversation = id
	}
	err = tx.QueryRow(ctx, `INSERT INTO scheduled_messages(sender_id,conversation_id,group_id,client_message_id,body,send_at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(sender_id,client_message_id) DO UPDATE SET sender_id=EXCLUDED.sender_id RETURNING `+scheduledColumns, user, conversation, group, client, body, at).Scan(&out.ID, &out.Scope, &out.ChatID, &out.ClientID, &out.Body, &out.SendAt, &out.Status, &out.Failure)
	if err != nil {
		return out, err
	}
	if out.Scope != scope || out.ChatID != id || out.Body != body || !out.SendAt.Equal(at) {
		return out, ErrConflict
	}
	return out, tx.Commit(ctx)
}
func (s *Store) Scheduled(ctx context.Context, user, scope, id string) ([]ScheduledMessage, error) {
	rows, err := s.db.Query(ctx, `SELECT `+scheduledColumns+` FROM scheduled_messages WHERE sender_id=$1 AND COALESCE(conversation_id,group_id)=$2 AND ((group_id IS NULL AND $3='personal') OR (group_id IS NOT NULL AND $3='group')) AND (status='pending' OR created_at>now()-interval '7 days') ORDER BY (status='pending') DESC,send_at LIMIT 500`, user, id, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ScheduledMessage{}
	for rows.Next() {
		var item ScheduledMessage
		if err = rows.Scan(&item.ID, &item.Scope, &item.ChatID, &item.ClientID, &item.Body, &item.SendAt, &item.Status, &item.Failure); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) ChangeScheduled(ctx context.Context, user, id, body string, at time.Time, cancel bool) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM scheduled_messages WHERE id=$1 AND sender_id=$2 FOR UPDATE`, id, user).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if status != "pending" {
		return ErrScheduleState
	}
	if cancel {
		_, err = tx.Exec(ctx, `UPDATE scheduled_messages SET status='cancelled',updated_at=now() WHERE id=$1`, id)
	} else {
		_, err = tx.Exec(ctx, `UPDATE scheduled_messages SET body=$2,send_at=$3,retry_at=now(),attempts=0,updated_at=now() WHERE id=$1`, id, body, at)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Dispatch and message insertion share a transaction: cancellation and retries cannot duplicate delivery.
func (s *Store) DispatchScheduled(ctx context.Context) (ScheduledMessage, bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return ScheduledMessage{}, false, err
	}
	defer tx.Rollback(context.Background())
	var item ScheduledMessage
	var user string
	err = tx.QueryRow(ctx, `SELECT `+scheduledColumns+`,sender_id::text FROM scheduled_messages WHERE status='pending' AND send_at<=now() AND retry_at<=now() ORDER BY send_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&item.ID, &item.Scope, &item.ChatID, &item.ClientID, &item.Body, &item.SendAt, &item.Status, &item.Failure, &user)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, false, nil
	}
	if err != nil {
		return item, false, err
	}
	if item.Scope == "personal" {
		err = lockSending(ctx, tx, item.ChatID, user, item.ClientID)
	} else {
		_, err = lockManagedGroup(ctx, tx, item.ChatID, user, true)
		if err == nil {
			var blocked bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contact_blocks b JOIN group_members gm ON gm.group_id=$1 AND gm.user_id=CASE WHEN b.blocker_id=$2 THEN b.blocked_id ELSE b.blocker_id END WHERE b.blocker_id=$2 OR b.blocked_id=$2)`, item.ChatID, user).Scan(&blocked)
			if err == nil && blocked {
				err = ErrBlocked
			}
		}
	}
	if err != nil {
		if errors.Is(err, ErrBlocked) || errors.Is(err, ErrPrivacy) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrForbidden) {
			if _, e := tx.Exec(ctx, `UPDATE scheduled_messages SET status='failed',failure='delivery_restricted',updated_at=now() WHERE id=$1`, item.ID); e != nil {
				return item, false, e
			}
			return item, true, tx.Commit(ctx)
		}
		return s.scheduleRetry(ctx, tx, item, err)
	}
	table, column := "messages", "conversation_id"
	if item.Scope == "group" {
		table, column = "group_messages", "group_id"
	}
	var messageID string
	// Table names are fixed server constants, never request values.
	err = tx.QueryRow(ctx, `INSERT INTO `+table+`(`+column+`,sender_id,client_message_id,body) VALUES($1,$2,$3,$4) ON CONFLICT(sender_id,client_message_id) DO NOTHING RETURNING id::text`, item.ChatID, user, item.ClientID, item.Body).Scan(&messageID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id::text FROM `+table+` WHERE `+column+`=$1 AND sender_id=$2 AND client_message_id=$3 AND body=$4`, item.ChatID, user, item.ClientID, item.Body).Scan(&messageID)
	}
	if err != nil {
		return s.scheduleRetry(ctx, tx, item, err)
	}
	if item.Scope == "group" {
		if _, err = tx.Exec(ctx, `UPDATE group_chats SET updated_at=now() WHERE id=$1`, item.ChatID); err != nil {
			return s.scheduleRetry(ctx, tx, item, err)
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE scheduled_messages SET status='sent',message_id=$2,updated_at=now() WHERE id=$1`, item.ID, messageID); err != nil {
		return s.scheduleRetry(ctx, tx, item, err)
	}
	if err = tx.Commit(ctx); err != nil {
		return item, false, err
	}
	return item, true, nil
}
func (s *Store) scheduleRetry(ctx context.Context, tx pgx.Tx, item ScheduledMessage, cause error) (ScheduledMessage, bool, error) {
	_ = tx.Rollback(context.Background())
	if ctx.Err() != nil {
		return item, false, cause
	}
	_, err := s.db.Exec(ctx, `UPDATE scheduled_messages SET attempts=attempts+1,retry_at=now()+interval '30 seconds',status=CASE WHEN attempts>=19 THEN 'failed' ELSE 'pending' END,failure=CASE WHEN attempts>=19 THEN 'delivery_failed' ELSE '' END WHERE id=$1 AND status='pending'`, item.ID)
	if err != nil {
		return item, false, err
	}
	return item, true, nil
}
func (s *Store) RunScheduled(ctx context.Context, publisher realtime.Publisher) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		for index := 0; index < 20; index++ {
			task, cancel := context.WithTimeout(ctx, 15*time.Second)
			item, found, err := s.DispatchScheduled(task)
			cancel()
			if err != nil || !found {
				break
			}
			if publisher.Enabled() {
				var users []string
				if item.Scope == "group" {
					rows, e := s.db.Query(ctx, `SELECT user_id::text FROM group_members WHERE group_id=$1`, item.ChatID)
					if e == nil {
						for rows.Next() {
							var id string
							if rows.Scan(&id) == nil {
								users = append(users, id)
							}
						}
						rows.Close()
					}
				} else {
					users, _ = s.Members(ctx, item.ChatID)
				}
				for _, id := range users {
					event := "conversation.message"
					key := "conversation_id"
					if item.Scope == "group" {
						event = "group.updated"
						key = "group_id"
					}
					_ = publisher.Publish("private-user-"+id, event, map[string]string{key: item.ChatID})
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
