package chat

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

var ErrPollInput = errors.New("invalid poll")
var ErrPollClosed = errors.New("poll closed")

func pollInteractionAllowed(ctx context.Context, tx pgx.Tx, group, user string) error {
	var blocked bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contact_blocks b JOIN group_members gm ON gm.group_id=$1 AND gm.user_id=CASE WHEN b.blocker_id=$2 THEN b.blocked_id ELSE b.blocker_id END WHERE b.blocker_id=$2 OR b.blocked_id=$2)`, group, user).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return ErrBlocked
	}
	return nil
}

type PollOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Votes int    `json:"votes"`
	Mine  bool   `json:"mine"`
}
type Poll struct {
	ID       string       `json:"id"`
	Question string       `json:"question"`
	Multiple bool         `json:"multiple"`
	ClosesAt *time.Time   `json:"closes_at"`
	Closed   bool         `json:"closed"`
	CanClose bool         `json:"can_close"`
	Total    int          `json:"total"`
	Options  []PollOption `json:"options"`
}

func (s *Store) CreatePoll(ctx context.Context, group, user, client, question string, labels []string, multiple bool, until *time.Time) (Poll, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Poll{}, err
	}
	defer tx.Rollback(context.Background())
	if _, err = lockManagedGroup(ctx, tx, group, user); err != nil {
		return Poll{}, err
	}
	if err = pollInteractionAllowed(ctx, tx, group, user); err != nil {
		return Poll{}, err
	}
	var message string
	err = tx.QueryRow(ctx, `INSERT INTO group_messages(group_id,sender_id,client_message_id,body) VALUES($1,$2,$3,$4) ON CONFLICT(sender_id,client_message_id) DO NOTHING RETURNING id::text`, group, user, client, "Опрос: "+question).Scan(&message)
	var id string
	if errors.Is(err, pgx.ErrNoRows) {
		var oldQuestion string
		var oldMultiple bool
		var oldUntil *time.Time
		var oldLabels []string
		err = tx.QueryRow(ctx, `SELECT p.id::text,p.question,p.multiple,p.closes_at,ARRAY(SELECT label FROM group_poll_options o WHERE o.poll_id=p.id ORDER BY position) FROM group_polls p JOIN group_messages m ON m.id=p.message_id WHERE p.group_id=$1 AND m.sender_id=$2 AND m.client_message_id=$3`, group, user, client).Scan(&id, &oldQuestion, &oldMultiple, &oldUntil, &oldLabels)
		if err != nil {
			return Poll{}, ErrConflict
		}
		sameTime := oldUntil == nil && until == nil || (oldUntil != nil && until != nil && oldUntil.Equal(*until))
		if oldQuestion != question || oldMultiple != multiple || !sameTime || len(oldLabels) != len(labels) {
			return Poll{}, ErrConflict
		}
		for i := range labels {
			if labels[i] != oldLabels[i] {
				return Poll{}, ErrConflict
			}
		}
	} else {
		if err != nil {
			return Poll{}, err
		}
		if err = tx.QueryRow(ctx, `INSERT INTO group_polls(group_id,message_id,creator_id,question,multiple,closes_at) VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text`, group, message, user, question, multiple, until).Scan(&id); err != nil {
			return Poll{}, err
		}
		for position, label := range labels {
			if _, err = tx.Exec(ctx, `INSERT INTO group_poll_options(poll_id,label,position) VALUES($1,$2,$3)`, id, label, position); err != nil {
				return Poll{}, err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE group_chats SET updated_at=now() WHERE id=$1`, group); err != nil {
			return Poll{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Poll{}, err
	}
	return s.Poll(ctx, group, user, id)
}
func (s *Store) Poll(ctx context.Context, group, user, id string) (Poll, error) {
	var out Poll
	err := s.db.QueryRow(ctx, `SELECT p.id::text,p.question,p.multiple,p.closes_at,(p.closed_at IS NOT NULL OR COALESCE(p.closes_at<=clock_timestamp(),false)),(p.creator_id=$2 OR g.owner_id=$2 OR gm.role='admin'),(SELECT count(DISTINCT user_id)::int FROM group_poll_votes WHERE poll_id=p.id) FROM group_polls p JOIN group_chats g ON g.id=p.group_id JOIN group_members gm ON gm.group_id=p.group_id AND gm.user_id=$2 WHERE p.group_id=$1 AND p.id=$3`, group, user, id).Scan(&out.ID, &out.Question, &out.Multiple, &out.ClosesAt, &out.Closed, &out.CanClose, &out.Total)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	rows, err := s.db.Query(ctx, `SELECT o.id::text,o.label,count(v.user_id)::int,COALESCE(bool_or(v.user_id=$2),false) FROM group_poll_options o LEFT JOIN group_poll_votes v ON v.poll_id=o.poll_id AND v.option_id=o.id WHERE o.poll_id=$1 AND EXISTS(SELECT 1 FROM group_polls p JOIN group_members gm ON gm.group_id=p.group_id AND gm.user_id=$2 WHERE p.id=o.poll_id) GROUP BY o.id,o.label,o.position ORDER BY o.position`, id, user)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	out.Options = []PollOption{}
	for rows.Next() {
		var option PollOption
		if err = rows.Scan(&option.ID, &option.Label, &option.Votes, &option.Mine); err != nil {
			return out, err
		}
		out.Options = append(out.Options, option)
	}
	return out, rows.Err()
}
func (s *Store) VotePoll(ctx context.Context, group, user, id string, options []string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = lockManagedGroup(ctx, tx, group, user); err != nil {
		return err
	}
	if err = pollInteractionAllowed(ctx, tx, group, user); err != nil {
		return err
	}
	var multiple, closed bool
	if err = tx.QueryRow(ctx, `SELECT multiple,closed_at IS NOT NULL OR COALESCE(closes_at<=clock_timestamp(),false) FROM group_polls WHERE group_id=$1 AND id=$2 FOR UPDATE`, group, id).Scan(&multiple, &closed); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if closed {
		return ErrPollClosed
	}
	if len(options) > 10 || (!multiple && len(options) > 1) {
		return ErrPollInput
	}
	seen := map[string]bool{}
	for _, option := range options {
		if seen[option] {
			return ErrPollInput
		}
		seen[option] = true
	}
	if len(options) > 0 {
		var valid int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM group_poll_options WHERE poll_id=$1 AND id=ANY($2::uuid[])`, id, options).Scan(&valid); err != nil {
			return err
		}
		if valid != len(options) {
			return ErrPollInput
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM group_poll_votes WHERE poll_id=$1 AND user_id=$2`, id, user); err != nil {
		return err
	}
	for _, option := range options {
		if _, err = tx.Exec(ctx, `INSERT INTO group_poll_votes(poll_id,option_id,user_id) VALUES($1,$2,$3)`, id, option, user); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) ClosePoll(ctx context.Context, group, user, id string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	role, err := lockManagedGroup(ctx, tx, group, user)
	if err != nil {
		return err
	}
	var creator string
	if err = tx.QueryRow(ctx, `SELECT creator_id::text FROM group_polls WHERE id=$1 AND group_id=$2 FOR UPDATE`, id, group).Scan(&creator); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if role != "owner" && role != "admin" && creator != user {
		return ErrForbidden
	}
	if _, err = tx.Exec(ctx, `UPDATE group_polls SET closed_at=COALESCE(closed_at,now()) WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
