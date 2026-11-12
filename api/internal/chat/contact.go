package chat

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"newsChat/api/internal/contactguard"
)

var ErrPhoneExists = errors.New("phone already linked")

type ContactSettings struct {
	Phone     string `json:"phone"`
	ShowPhone bool   `json:"show_phone"`
	ShowEmail bool   `json:"show_email"`
	Email     string `json:"email"`
}
type ContactProfile struct {
	Birthday string `json:"birthday,omitempty"`
	PublicUser
	Phone          string     `json:"phone,omitempty"`
	Email          string     `json:"email,omitempty"`
	SentCount      int64      `json:"sent_count"`
	ReceivedCount  int64      `json:"received_count"`
	Online         bool       `json:"online"`
	PresenceHidden bool       `json:"presence_hidden,omitempty"`
	LastSeen       *time.Time `json:"last_seen,omitempty"`
	DisplayName    string     `json:"display_name,omitempty"`
}

func (s *Store) TouchPresence(ctx context.Context, userID string) error {
	_, err := s.db.Exec(ctx, `INSERT INTO user_presence(user_id,last_seen) VALUES($1,now()) ON CONFLICT(user_id) DO UPDATE SET last_seen=now()`, userID)
	return err
}

func (s *Store) SetContactAlias(ctx context.Context, conversationID, userID, alias string) (string, error) {
	if err := s.CheckMember(ctx, conversationID, userID); err != nil {
		return "", err
	}
	var saved string
	err := s.db.QueryRow(ctx, `WITH peer AS (SELECT CASE WHEN c.user_low=$2 THEN c.user_high ELSE c.user_low END AS id FROM conversations c WHERE c.id=$1) INSERT INTO contact_aliases(owner_id,peer_id,alias) SELECT $2,id,$3 FROM peer ON CONFLICT(owner_id,peer_id) DO UPDATE SET alias=EXCLUDED.alias RETURNING alias`, conversationID, userID, alias).Scan(&saved)
	return saved, err
}

func (s *Store) ClearContactAlias(ctx context.Context, conversationID, userID string) error {
	if err := s.CheckMember(ctx, conversationID, userID); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `DELETE FROM contact_aliases WHERE owner_id=$2 AND peer_id=(SELECT CASE WHEN c.user_low=$2 THEN c.user_high ELSE c.user_low END FROM conversations c WHERE c.id=$1)`, conversationID, userID)
	return err
}

func (s *Store) SetUsername(ctx context.Context, userID, username string) error {
	_, err := s.db.Exec(ctx, `UPDATE users SET username=$2,name=$2 WHERE id=$1`, userID, username)
	return err
}

func (s *Store) ContactSettings(ctx context.Context, userID string) (ContactSettings, error) {
	var v ContactSettings
	err := s.db.QueryRow(ctx, `SELECT COALESCE(p.phone,''),COALESCE(p.show_phone,false),false,u.email FROM users u LEFT JOIN profile_contacts p ON p.user_id=u.id WHERE u.id=$1`, userID).Scan(&v.Phone, &v.ShowPhone, &v.ShowEmail, &v.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return v, err
}
func (s *Store) SetContactSettings(ctx context.Context, userID, phone string, showPhone, showEmail bool) (ContactSettings, error) {
	// Keep the legacy argument for cached clients; login email is always private.
	showEmail = false
	if phone == "" {
		showPhone = false
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return ContactSettings{}, err
	}
	defer tx.Rollback(context.Background())
	if phone != "" {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,313))`, phone); err != nil {
			return ContactSettings{}, err
		}
		var taken bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM profile_contacts WHERE user_id<>$1 AND regexp_replace(phone,'[^0-9]','','g')=regexp_replace($2,'[^0-9]','','g'))`, userID, phone).Scan(&taken); err != nil {
			return ContactSettings{}, err
		}
		if taken {
			return ContactSettings{}, ErrPhoneExists
		}
	}
	var v ContactSettings
	err = tx.QueryRow(ctx, `WITH saved AS (INSERT INTO profile_contacts(user_id,phone,show_phone,show_email) VALUES($1,$2,$3,$4) ON CONFLICT(user_id) DO UPDATE SET phone=EXCLUDED.phone,show_phone=EXCLUDED.show_phone,show_email=EXCLUDED.show_email RETURNING user_id,phone,show_phone,show_email) SELECT s.phone,s.show_phone,s.show_email,u.email FROM saved s JOIN users u ON u.id=s.user_id`, userID, phone, showPhone, showEmail).Scan(&v.Phone, &v.ShowPhone, &v.ShowEmail, &v.Email)
	if err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
func (s *Store) ContactProfile(ctx context.Context, conversationID, userID string) (ContactProfile, error) {
	var v ContactProfile
	var lastSeen pgtype.Timestamptz
	err := s.db.QueryRow(ctx, `SELECT u.id::text,u.name,u.username,COALESCE((SELECT '/api/profiles/'||u.id::text||'/photo?v='||ph.revision::text FROM profile_photos ph WHERE ph.user_id=u.id),''),CASE WHEN p.show_phone THEN p.phone ELSE '' END,''::text,COALESCE(to_char(d.birth_date,'YYYY-MM-DD'),''),COALESCE(stats.sent_count,0),COALESCE(stats.received_count,0),COALESCE((SELECT last_seen > now()-interval '75 seconds' FROM user_presence WHERE user_id=u.id),false),COALESCE(a.alias,''),(SELECT last_seen FROM user_presence WHERE user_id=u.id) FROM conversation_members m JOIN conversations c ON c.id=m.conversation_id JOIN users u ON u.id=CASE WHEN c.user_low=$2 THEN c.user_high ELSE c.user_low END LEFT JOIN profile_contacts p ON p.user_id=u.id LEFT JOIN user_profiles d ON d.user_id=u.id LEFT JOIN contact_aliases a ON a.owner_id=$2 AND a.peer_id=u.id LEFT JOIN LATERAL (SELECT count(*) FILTER (WHERE sender_id=$2) AS sent_count,count(*) FILTER (WHERE sender_id=u.id) AS received_count FROM messages WHERE conversation_id=m.conversation_id AND id>m.cleared_through) stats ON true WHERE m.conversation_id=$1 AND m.user_id=$2`, conversationID, userID).Scan(&v.ID, &v.Name, &v.Username, &v.AvatarURL, &v.Phone, &v.Email, &v.Birthday, &v.SentCount, &v.ReceivedCount, &v.Online, &v.DisplayName, &lastSeen)
	if lastSeen.Valid {
		v.LastSeen = &lastSeen.Time
	}
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	if err != nil {
		return v, err
	}
	visible, e := contactguard.Allowed(ctx, s.db, v.ID, userID, "presence")
	if e != nil {
		return v, e
	}
	if !visible {
		v.Online = false
		v.LastSeen = nil
		v.PresenceHidden = true
	}
	visible, e = contactguard.Allowed(ctx, s.db, v.ID, userID, "birthday")
	if e != nil {
		return v, e
	}
	if !visible {
		v.Birthday = ""
	}
	return v, nil
}
