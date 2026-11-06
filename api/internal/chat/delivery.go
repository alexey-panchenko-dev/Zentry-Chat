package chat

import (
	"context"
	"strconv"
)

// PendingDelivery transfers actual message envelopes to a signed-in recipient.
// This fetch alone never changes delivery or read receipts.
func (s *Store) PendingDelivery(ctx context.Context, user string) ([]Message, error) {
	rows, err := s.db.Query(ctx, `SELECT m.id::text,m.conversation_id::text,m.sender_id::text,m.body,m.kind,m.created_at FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id WHERE cm.user_id=$1 AND m.sender_id<>$1 AND m.id>GREATEST(cm.received_through,cm.cleared_through) ORDER BY m.id LIMIT 100`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Message, 0)
	for rows.Next() {
		var m Message
		if err = rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Body, &m.Kind, &m.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
func (s *Store) MarkDelivered(ctx context.Context, user string, ids []int64) ([]string, error) {
	rows, err := s.db.Query(ctx, `WITH received AS (SELECT m.conversation_id,max(m.id) AS last_id FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id WHERE m.id=ANY($2::bigint[]) AND cm.user_id=$1 AND m.sender_id<>$1 AND m.id>cm.cleared_through GROUP BY m.conversation_id),changed AS (UPDATE conversation_members cm SET received_through=GREATEST(cm.received_through,r.last_id) FROM received r WHERE cm.conversation_id=r.conversation_id AND cm.user_id=$1 AND cm.received_through<r.last_id RETURNING cm.conversation_id) SELECT conversation_id::text FROM changed`, user, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}
func DeliveryIDs(values []string) ([]int64, bool) {
	if len(values) == 0 || len(values) > 100 {
		return nil, false
	}
	ids := make([]int64, 0, len(values))
	for _, v := range values {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id < 1 {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}
