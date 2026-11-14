package auth

import (
	"context"
	"time"
)

type SecurityEvent struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) SecurityEvents(ctx context.Context, user string) ([]SecurityEvent, error) {
	rows, err := s.db.Query(ctx, `SELECT id::text,kind,created_at FROM security_events WHERE user_id=$1 ORDER BY created_at DESC,id DESC LIMIT 50`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []SecurityEvent{}
	for rows.Next() {
		var event SecurityEvent
		if err = rows.Scan(&event.ID, &event.Kind, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
