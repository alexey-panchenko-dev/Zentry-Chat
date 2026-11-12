package chat

import (
	"context"
	"newsChat/api/internal/contactguard"
)

var ErrPrivacy = contactguard.ErrPrivacy

type PrivacySettings struct {
	Messages string `json:"messages"`
	Calls    string `json:"calls"`
	Presence string `json:"presence"`
	Search   string `json:"search"`
	Birthday string `json:"birthday"`
}

func (s *Store) Privacy(ctx context.Context, user string) (PrivacySettings, error) {
	v := PrivacySettings{}
	err := s.db.QueryRow(ctx, `SELECT COALESCE(p.messages,'everyone'),COALESCE(p.calls,'everyone'),COALESCE(p.presence,'everyone'),COALESCE(p.search,'everyone'),COALESCE(p.birthday,'everyone') FROM users u LEFT JOIN user_privacy p ON p.user_id=u.id WHERE u.id=$1`, user).Scan(&v.Messages, &v.Calls, &v.Presence, &v.Search, &v.Birthday)
	return v, err
}
func (s *Store) SetPrivacy(ctx context.Context, user string, v PrivacySettings) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, user); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO user_privacy(user_id,messages,calls,presence,search,birthday) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(user_id) DO UPDATE SET messages=EXCLUDED.messages,calls=EXCLUDED.calls,presence=EXCLUDED.presence,search=EXCLUDED.search,birthday=EXCLUDED.birthday,updated_at=now()`, user, v.Messages, v.Calls, v.Presence, v.Search, v.Birthday)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
