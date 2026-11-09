package calls

import (
	"context"
	"encoding/json"
	"errors"
	"newsChat/api/internal/contactguard"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("call not found")
var ErrState = errors.New("invalid call state or action")
var ErrBusy = errors.New("participant busy")
var ErrConflict = errors.New("idempotency key conflict")

type Call struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	CallerID       string    `json:"caller_id"`
	CalleeID       string    `json:"callee_id"`
	ClientCallID   string    `json:"client_call_id"`
	Status         string    `json:"status"`
	Version        int       `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	AcceptedAt     *time.Time `json:"accepted_at,omitempty"`
}

type HistoryEntry struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversation_id"`
	PeerID         string     `json:"peer_id"`
	PeerName       string     `json:"peer_name"`
	Direction      string     `json:"direction"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	AcceptedAt     *time.Time `json:"accepted_at,omitempty"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	DurationMS     int64      `json:"duration_ms"`
}

type Signal struct {
	ID             string          `json:"id"`
	CallID         string          `json:"call_id"`
	SenderID       string          `json:"sender_id"`
	RecipientID    string          `json:"recipient_id"`
	ClientSignalID string          `json:"client_signal_id"`
	Type           string          `json:"type"`
	Payload        json.RawMessage `json:"payload"`
	CreatedAt      time.Time       `json:"created_at"`
}

const columns = `id::text,conversation_id::text,caller_id::text,callee_id::text,client_call_id::text,status,version,created_at,updated_at,expires_at,accepted_at`

type scanner interface{ Scan(...any) error }

func scanCall(row scanner) (Call, error) {
	var c Call
	err := row.Scan(&c.ID, &c.ConversationID, &c.CallerID, &c.CalleeID, &c.ClientCallID, &c.Status, &c.Version, &c.CreatedAt, &c.UpdatedAt, &c.ExpiresAt, &c.AcceptedAt)
	return c, err
}
func scanSignal(row scanner) (Signal, error) {
	var s Signal
	err := row.Scan(&s.ID, &s.CallID, &s.SenderID, &s.RecipientID, &s.ClientSignalID, &s.Type, &s.Payload, &s.CreatedAt)
	return s, err
}

const signalColumns = `id::text,call_id::text,sender_id::text,recipient_id::text,client_signal_id::text,type,payload,created_at`

type Store struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Store { return &Store{db: db} }

// Expiry is persisted lazily when reading/changing calls or making a new call.
func expire(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `WITH expired AS (SELECT id FROM calls WHERE status IN ('ringing','accepted') AND expires_at<=now() ORDER BY id FOR UPDATE SKIP LOCKED)
 UPDATE calls SET status='expired',version=version+1,updated_at=now() WHERE id IN (SELECT id FROM expired)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM active_call_participants a USING calls c
        WHERE a.call_id=c.id AND c.status NOT IN ('ringing','accepted')`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM call_signals s USING calls c WHERE s.call_id=c.id AND c.status NOT IN ('ringing','accepted')`)
	return err
}

func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}

func (s *Store) Create(ctx context.Context, convID, userID, clientID string) (Call, bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Call{}, false, err
	}
	defer rollback(tx)
	var peer string
	err = tx.QueryRow(ctx, `SELECT CASE WHEN c.user_low=$2 THEN c.user_high ELSE c.user_low END::text
        FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id
        WHERE c.id=$1 AND m.user_id=$2`, convID, userID).Scan(&peer)
	if errors.Is(err, pgx.ErrNoRows) {
		return Call{}, false, ErrNotFound
	}
	if err != nil {
		return Call{}, false, err
	}
	// Users first, then call rows, then conversations (shared with block/send).
	if err = contactguard.LockUsers(ctx, tx, userID, peer); err != nil {
		return Call{}, false, err
	}
	if err = contactguard.CheckBlocked(ctx, tx, userID, peer); err != nil {
		return Call{}, false, err
	}
	if err := expire(ctx, tx); err != nil {
		return Call{}, false, err
	}
	old, err := scanCall(tx.QueryRow(ctx, "SELECT "+columns+" FROM calls WHERE caller_id=$1 AND client_call_id=$2", userID, clientID))
	if err == nil {
		if old.ConversationID != convID {
			return Call{}, false, ErrConflict
		}
		return old, false, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Call{}, false, err
	}
	var busy bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM active_call_participants WHERE user_id IN ($1,$2))", userID, peer).Scan(&busy); err != nil {
		return Call{}, false, err
	}
	if busy {
		return Call{}, false, ErrBusy
	}
	call, err := scanCall(tx.QueryRow(ctx, `INSERT INTO calls(conversation_id,caller_id,callee_id,client_call_id)
        VALUES($1,$2,$3,$4) RETURNING `+columns, convID, userID, peer, clientID))
	if err != nil {
		return Call{}, false, err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO active_call_participants(user_id,call_id) VALUES($1,$3),($2,$3)", userID, peer, call.ID); err != nil {
		return Call{}, false, err
	}
	return call, true, tx.Commit(ctx)
}

func (s *Store) Get(ctx context.Context, id, userID string) (Call, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Call{}, err
	}
	defer rollback(tx)
	if err := expire(ctx, tx); err != nil {
		return Call{}, err
	}
	c, err := scanCall(tx.QueryRow(ctx, "SELECT "+columns+" FROM calls WHERE id=$1 AND (caller_id=$2 OR callee_id=$2)", id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Call{}, ErrNotFound
	}
	if err != nil {
		return Call{}, err
	}
	return c, tx.Commit(ctx)
}

func (s *Store) Active(ctx context.Context, userID string) ([]Call, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if err := expire(ctx, tx); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, "SELECT "+columns+" FROM calls WHERE (caller_id=$1 OR callee_id=$1) AND status IN ('ringing','accepted') ORDER BY created_at DESC", userID)
	if err != nil {
		return nil, err
	}
	list := make([]Call, 0)
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		list = append(list, c)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return list, tx.Commit(ctx)
}

func (s *Store) History(ctx context.Context, userID string) ([]HistoryEntry, error) {
	rows, err := s.db.Query(ctx, `SELECT c.id::text,c.conversation_id::text,peer.id::text,COALESCE(a.alias,peer.name),
		CASE WHEN c.caller_id=$1 THEN 'outgoing' ELSE 'incoming' END,c.status,c.created_at,c.accepted_at,
        CASE WHEN c.status IN ('ended','expired','rejected','cancelled') THEN c.updated_at ELSE NULL END,
        CASE WHEN c.accepted_at IS NOT NULL AND c.status IN ('ended','expired') THEN GREATEST(0,FLOOR(EXTRACT(EPOCH FROM (c.updated_at-c.accepted_at))*1000))::bigint ELSE 0 END
        FROM calls c JOIN users peer ON peer.id=CASE WHEN c.caller_id=$1 THEN c.callee_id ELSE c.caller_id END
        LEFT JOIN contact_aliases a ON a.owner_id=$1 AND a.peer_id=peer.id
        WHERE (c.caller_id=$1 OR c.callee_id=$1) AND c.status NOT IN ('ringing','accepted')
        ORDER BY c.created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]HistoryEntry, 0)
	for rows.Next() {
		var item HistoryEntry
		if err := rows.Scan(&item.ID, &item.ConversationID, &item.PeerID, &item.PeerName, &item.Direction, &item.Status, &item.CreatedAt, &item.AcceptedAt, &item.EndedAt, &item.DurationMS); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s *Store) Transition(ctx context.Context, id, userID, action string) (Call, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Call{}, err
	}
	defer rollback(tx)
	// Read participant IDs without a row lock, then take deterministic user
	// locks before touching call rows. Re-read the call after acquiring locks.
	initial, err := scanCall(tx.QueryRow(ctx, "SELECT "+columns+" FROM calls WHERE id=$1 AND (caller_id=$2 OR callee_id=$2)", id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Call{}, ErrNotFound
	}
	if err != nil {
		return Call{}, err
	}
	if err = contactguard.LockUsers(ctx, tx, initial.CallerID, initial.CalleeID); err != nil {
		return Call{}, err
	}
	if action == "accept" {
		if err = contactguard.CheckBlocked(ctx, tx, initial.CallerID, initial.CalleeID); err != nil {
			return Call{}, err
		}
	}
	if err := expire(ctx, tx); err != nil {
		return Call{}, err
	}
	c, err := scanCall(tx.QueryRow(ctx, "SELECT "+columns+" FROM calls WHERE id=$1 AND (caller_id=$2 OR callee_id=$2) FOR UPDATE", id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Call{}, ErrNotFound
	}
	if err != nil {
		return Call{}, err
	}
	target := ""
	switch action {
	case "accept":
		if userID == c.CalleeID && (c.Status == "ringing" || c.Status == "accepted") {
			target = "accepted"
		}
	case "reject":
		if userID == c.CalleeID && (c.Status == "ringing" || c.Status == "rejected") {
			target = "rejected"
		}
	case "cancel":
		if userID == c.CallerID && (c.Status == "ringing" || c.Status == "cancelled") {
			target = "cancelled"
		}
	case "end":
		if c.Status == "accepted" || c.Status == "ended" {
			target = "ended"
		}
	}
	if target == "" {
		return Call{}, ErrState
	}
	if c.Status != target {
		c, err = scanCall(tx.QueryRow(ctx, `UPDATE calls SET status=$2,version=version+1,updated_at=now(),
            accepted_at=CASE WHEN $2='accepted' THEN COALESCE(accepted_at,now()) ELSE accepted_at END,
            expires_at=CASE WHEN $2='accepted' THEN now()+interval '2 hours' ELSE expires_at END
            WHERE id=$1 RETURNING `+columns, id, target))
		if err != nil {
			return Call{}, err
		}
		if target != "accepted" {
			if _, err := tx.Exec(ctx, "DELETE FROM active_call_participants WHERE call_id=$1", id); err != nil {
				return Call{}, err
			}
			if _, err := tx.Exec(ctx, "DELETE FROM call_signals WHERE call_id=$1", id); err != nil {
				return Call{}, err
			}
		}
	}
	return c, tx.Commit(ctx)
}

func (s *Store) SendSignal(ctx context.Context, id, userID, clientID, kind string, payload json.RawMessage) (Signal, bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Signal{}, false, err
	}
	defer rollback(tx)
	initial, err := scanCall(tx.QueryRow(ctx, "SELECT "+columns+" FROM calls WHERE id=$1 AND (caller_id=$2 OR callee_id=$2)", id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Signal{}, false, ErrNotFound
	}
	if err != nil {
		return Signal{}, false, err
	}
	if err = contactguard.LockUsers(ctx, tx, initial.CallerID, initial.CalleeID); err != nil {
		return Signal{}, false, err
	}
	if err = contactguard.CheckBlocked(ctx, tx, initial.CallerID, initial.CalleeID); err != nil {
		return Signal{}, false, err
	}
	if err := expire(ctx, tx); err != nil {
		return Signal{}, false, err
	}
	c, err := scanCall(tx.QueryRow(ctx, "SELECT "+columns+" FROM calls WHERE id=$1 AND (caller_id=$2 OR callee_id=$2) FOR UPDATE", id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Signal{}, false, ErrNotFound
	}
	if err != nil {
		return Signal{}, false, err
	}
	if c.Status != "accepted" || (kind == "offer" && userID != c.CallerID) || (kind == "answer" && userID != c.CalleeID) {
		return Signal{}, false, ErrState
	}
	recipient := c.CalleeID
	if userID == c.CalleeID {
		recipient = c.CallerID
	}
	signal, err := scanSignal(tx.QueryRow(ctx, `INSERT INTO call_signals(call_id,sender_id,recipient_id,client_signal_id,type,payload)
        VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(sender_id,client_signal_id) DO NOTHING RETURNING `+signalColumns, id, userID, recipient, clientID, kind, []byte(payload)))
	created := err == nil
	if errors.Is(err, pgx.ErrNoRows) {
		var same bool
		err = tx.QueryRow(ctx, "SELECT call_id=$3 AND type=$4 AND payload=$5::jsonb FROM call_signals WHERE sender_id=$1 AND client_signal_id=$2", userID, clientID, id, kind, []byte(payload)).Scan(&same)
		if err != nil {
			return Signal{}, false, err
		}
		if !same {
			return Signal{}, false, ErrConflict
		}
		signal, err = scanSignal(tx.QueryRow(ctx, "SELECT "+signalColumns+" FROM call_signals WHERE sender_id=$1 AND client_signal_id=$2", userID, clientID))
	}
	if err != nil {
		return Signal{}, false, err
	}
	return signal, created, tx.Commit(ctx)
}

func (s *Store) Signals(ctx context.Context, id, userID string, after int64) ([]Signal, error) {
	c, err := s.Get(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if c.Status != "accepted" {
		return nil, ErrState
	}
	rows, err := s.db.Query(ctx, "SELECT "+signalColumns+" FROM call_signals WHERE call_id=$1 AND recipient_id=$2 AND id>$3 ORDER BY id LIMIT 100", id, userID, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]Signal, 0)
	for rows.Next() {
		signal, err := scanSignal(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, signal)
	}
	return list, rows.Err()
}
