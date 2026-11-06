package chat

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

type ContactSettings struct {
	Phone     string `json:"phone"`
	ShowPhone bool   `json:"show_phone"`
	ShowEmail bool   `json:"show_email"`
	Email     string `json:"email"`
}
type ContactProfile struct {
	PublicUser
	Phone string `json:"phone,omitempty"`
	Email string `json:"email,omitempty"`
}

func (s *Store) ContactSettings(ctx context.Context, userID string) (ContactSettings, error) {
	var v ContactSettings
	err := s.db.QueryRow(ctx, `SELECT COALESCE(p.phone,''),COALESCE(p.show_phone,false),COALESCE(p.show_email,false),u.email FROM users u LEFT JOIN profile_contacts p ON p.user_id=u.id WHERE u.id=$1`, userID).Scan(&v.Phone, &v.ShowPhone, &v.ShowEmail, &v.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return v, err
}
func (s *Store) SetContactSettings(ctx context.Context, userID, phone string, showPhone, showEmail bool) (ContactSettings, error) {
	if phone == "" {
		showPhone = false
	}
	var v ContactSettings
	err := s.db.QueryRow(ctx, `WITH saved AS (INSERT INTO profile_contacts(user_id,phone,show_phone,show_email) VALUES($1,$2,$3,$4) ON CONFLICT(user_id) DO UPDATE SET phone=EXCLUDED.phone,show_phone=EXCLUDED.show_phone,show_email=EXCLUDED.show_email RETURNING user_id,phone,show_phone,show_email) SELECT s.phone,s.show_phone,s.show_email,u.email FROM saved s JOIN users u ON u.id=s.user_id`, userID, phone, showPhone, showEmail).Scan(&v.Phone, &v.ShowPhone, &v.ShowEmail, &v.Email)
	return v, err
}
func (s *Store) ContactProfile(ctx context.Context, conversationID, userID string) (ContactProfile, error) {
	var v ContactProfile
	err := s.db.QueryRow(ctx, `SELECT u.id::text,u.name,COALESCE((SELECT '/api/profiles/'||u.id::text||'/photo?v='||ph.revision::text FROM profile_photos ph WHERE ph.user_id=u.id),''),CASE WHEN p.show_phone THEN p.phone ELSE '' END,CASE WHEN p.show_email THEN u.email ELSE '' END FROM conversation_members m JOIN conversations c ON c.id=m.conversation_id JOIN users u ON u.id=CASE WHEN c.user_low=$2 THEN c.user_high ELSE c.user_low END LEFT JOIN profile_contacts p ON p.user_id=u.id WHERE m.conversation_id=$1 AND m.user_id=$2`, conversationID, userID).Scan(&v.ID, &v.Name, &v.AvatarURL, &v.Phone, &v.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return v, err
}
