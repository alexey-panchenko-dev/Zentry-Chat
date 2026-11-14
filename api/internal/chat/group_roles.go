package chat

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

func lockManagedGroup(ctx context.Context, tx pgx.Tx, id, actor string, allowNotes ...bool) (string, error) {
	var owner, role string
	var notes bool
	err := tx.QueryRow(ctx, `SELECT g.owner_id::text,m.role,EXISTS(SELECT 1 FROM private_note_chats WHERE group_id=g.id) FROM group_chats g JOIN group_members m ON m.group_id=g.id AND m.user_id=$2 WHERE g.id=$1 FOR UPDATE OF g`, id, actor).Scan(&owner, &role, &notes)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	err = tx.QueryRow(ctx, `SELECT g.owner_id::text,m.role FROM group_chats g JOIN group_members m ON m.group_id=g.id AND m.user_id=$2 WHERE g.id=$1`, id, actor).Scan(&owner, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if notes && !(len(allowNotes) > 0 && allowNotes[0]) {
		return "", ErrForbidden
	}
	if owner == actor {
		return "owner", nil
	}
	return role, nil
}
func (s *Store) ManageGroup(ctx context.Context, id, actor, target, action, value string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	role, err := lockManagedGroup(ctx, tx, id, actor)
	if err != nil {
		return err
	}
	if action == "name" {
		if role != "owner" && role != "admin" {
			return ErrForbidden
		}
		_, err = tx.Exec(ctx, `UPDATE group_chats SET name=$2,updated_at=now() WHERE id=$1`, id, value)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if action == "add" {
		if role != "owner" && role != "admin" {
			return ErrForbidden
		}
		var eligible bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN conversations c ON c.user_low=LEAST($1::uuid,u.id) AND c.user_high=GREATEST($1::uuid,u.id) JOIN conversation_members cm ON cm.conversation_id=c.id AND cm.user_id=$1 WHERE u.id=$2 AND u.email_verified AND NOT cm.hidden AND NOT EXISTS(SELECT 1 FROM contact_blocks WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))) AND (SELECT count(*) FROM group_members WHERE group_id=$3)<200`, actor, target, id).Scan(&eligible)
		if err != nil {
			return err
		}
		if !eligible {
			return ErrForbidden
		}
		_, err = tx.Exec(ctx, `INSERT INTO group_members(group_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, target)
	} else {
		var owner string
		var targetRole string
		err = tx.QueryRow(ctx, `SELECT g.owner_id::text,m.role FROM group_chats g JOIN group_members m ON m.group_id=g.id AND m.user_id=$2 WHERE g.id=$1`, id, target).Scan(&owner, &targetRole)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		switch action {
		case "remove":
			if target == owner {
				return ErrForbidden
			}
			if target != actor && role != "owner" && (role != "admin" || targetRole != "member") {
				return ErrForbidden
			}
			_, err = tx.Exec(ctx, `DELETE FROM group_members WHERE group_id=$1 AND user_id=$2`, id, target)
		case "role":
			if role != "owner" || target == owner {
				return ErrForbidden
			}
			_, err = tx.Exec(ctx, `UPDATE group_members SET role=$3 WHERE group_id=$1 AND user_id=$2`, id, target, value)
		case "owner":
			if role != "owner" || target == actor {
				return ErrForbidden
			}
			_, err = tx.Exec(ctx, `UPDATE group_chats SET owner_id=$2,updated_at=now() WHERE id=$1`, id, target)
			if err == nil {
				_, err = tx.Exec(ctx, `UPDATE group_members SET role=CASE WHEN user_id=$2 THEN 'admin' ELSE 'member' END WHERE group_id=$1 AND user_id IN ($2,$3)`, id, actor, target)
			}
		default:
			return ErrForbidden
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func checkGroupMember(ctx context.Context, tx pgx.Tx, id, user string) error {
	var member bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id=$1 AND user_id=$2)`, id, user).Scan(&member); err != nil {
		return err
	}
	if !member {
		return ErrNotFound
	}
	return nil
}
