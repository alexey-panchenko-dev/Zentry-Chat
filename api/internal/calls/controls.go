package calls

import (
	"context"
	"github.com/jackc/pgx/v5"
	"newsChat/api/internal/contactguard"
)

var ErrBlocked = contactguard.ErrBlocked

// TerminatePair requires both users already locked via contactguard.LockUsers.
// Call rows precede conversation rows in the common lock order.
func TerminatePair(ctx context.Context, tx pgx.Tx, a, b string) ([]Call, error) {
	rows, err := tx.Query(ctx, `SELECT `+columns+` FROM calls WHERE ((caller_id=$1 AND callee_id=$2) OR (caller_id=$2 AND callee_id=$1)) AND status IN ('ringing','accepted') ORDER BY id FOR UPDATE`, a, b)
	if err != nil {
		return nil, err
	}
	var list []Call
	for rows.Next() {
		c, e := scanCall(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		list = append(list, c)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for i, c := range list {
		status := "cancelled"
		if c.Status == "accepted" {
			status = "ended"
		}
		updated, e := scanCall(tx.QueryRow(ctx, `UPDATE calls SET status=$2,version=version+1,updated_at=now() WHERE id=$1 RETURNING `+columns, c.ID, status))
		if e != nil {
			return nil, e
		}
		list[i] = updated
		if _, e = tx.Exec(ctx, `DELETE FROM active_call_participants WHERE call_id=$1`, c.ID); e != nil {
			return nil, e
		}
		if _, e = tx.Exec(ctx, `DELETE FROM call_signals WHERE call_id=$1`, c.ID); e != nil {
			return nil, e
		}
	}
	return list, nil
}
