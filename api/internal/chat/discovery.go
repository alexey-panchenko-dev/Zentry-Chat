package chat

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/contactguard"
	"time"
)

var ErrProfileRequired = errors.New("profile required")

type Profile struct {
	PublicUser
	Age  *int   `json:"age"`
	Bio  string `json:"bio"`
	City string `json:"city"`
}

const profileColumns = `u.id::text,u.name,COALESCE((SELECT '/api/profiles/'||u.id::text||'/photo?v='||p.revision::text FROM profile_photos p WHERE p.user_id=u.id),''),CASE WHEN p.birth_date IS NOT NULL THEN EXTRACT(YEAR FROM age(CURRENT_DATE,p.birth_date))::integer ELSE p.age END,COALESCE(p.bio,''),COALESCE(p.city,'')`

func scanProfile(row pgx.Row) (Profile, error) {
	var p Profile
	err := row.Scan(&p.ID, &p.Name, &p.AvatarURL, &p.Age, &p.Bio, &p.City)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return p, err
}
func (s *Store) Profile(ctx context.Context, id string) (Profile, error) {
	return scanProfile(s.db.QueryRow(ctx, `SELECT `+profileColumns+` FROM users u LEFT JOIN user_profiles p ON p.user_id=u.id WHERE u.id=$1`, id))
}

type OwnerProfile struct {
	Profile
	BirthDate *string `json:"birth_date"`
}

func (s *Store) OwnProfile(ctx context.Context, id string) (OwnerProfile, error) {
	p, err := s.Profile(ctx, id)
	if err != nil {
		return OwnerProfile{}, err
	}
	result := OwnerProfile{Profile: p}
	err = s.db.QueryRow(ctx, `SELECT to_char(p.birth_date,'YYYY-MM-DD') FROM users u LEFT JOIN user_profiles p ON p.user_id=u.id WHERE u.id=$1`, id).Scan(&result.BirthDate)
	return result, err
}
func (s *Store) UpdateProfile(ctx context.Context, id, name string, age *int, bio, city string) (Profile, error) {
	return s.UpdateProfileWithBirthDate(ctx, id, name, age, bio, city, nil)
}
func (s *Store) UpdateProfileWithBirthDate(ctx context.Context, id, name string, age *int, bio, city string, birthDate *string) (Profile, error) {
	var date any
	if birthDate != nil {
		d, err := auth.ParseBirthDate(*birthDate, time.Now())
		if err != nil {
			return Profile{}, err
		}
		date = d
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Profile{}, err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `UPDATE users SET name=$2 WHERE id=$1`, id, name); err != nil {
		return Profile{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO user_profiles(user_id,age,bio,city,birth_date) VALUES($1,$2,$3,$4,$5) ON CONFLICT(user_id) DO UPDATE SET age=EXCLUDED.age,bio=EXCLUDED.bio,city=EXCLUDED.city,birth_date=COALESCE(EXCLUDED.birth_date,user_profiles.birth_date)`, id, age, bio, city, date); err != nil {
		return Profile{}, err
	}
	p, err := scanProfile(tx.QueryRow(ctx, `SELECT `+profileColumns+` FROM users u LEFT JOIN user_profiles p ON p.user_id=u.id WHERE u.id=$1`, id))
	if err != nil {
		return p, err
	}
	return p, tx.Commit(ctx)
}
func (s *Store) Discovery(ctx context.Context, id string, incoming bool) ([]Profile, error) {
	p, err := s.Profile(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Age == nil || p.Bio == "" {
		return nil, ErrProfileRequired
	}
	filter := `NOT EXISTS(SELECT 1 FROM discovery_votes v WHERE v.actor_id=$1 AND v.target_id=u.id AND (v.liked OR v.updated_at>now()-interval '7 days'))`
	if incoming {
		filter = `EXISTS(SELECT 1 FROM discovery_votes v WHERE v.actor_id=u.id AND v.target_id=$1 AND v.liked) AND NOT EXISTS(SELECT 1 FROM discovery_votes v WHERE v.actor_id=$1 AND v.target_id=u.id)`
	}
	rows, err := s.db.Query(ctx, `SELECT `+profileColumns+` FROM users u JOIN user_profiles p ON p.user_id=u.id WHERE u.id<>$1 AND (p.age IS NOT NULL OR p.birth_date IS NOT NULL) AND p.bio<>'' AND NOT EXISTS(SELECT 1 FROM discovery_matches m WHERE m.user_low=LEAST($1::uuid,u.id) AND m.user_high=GREATEST($1::uuid,u.id)) AND `+filter+` ORDER BY u.id LIMIT 30`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Profile, 0)
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// A transaction-level pair lock serializes opposite likes even before a match row exists.
func (s *Store) Vote(ctx context.Context, actor, target string, liked bool) (string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.Background())
	if err = contactguard.LockUsers(ctx, tx, actor, target); err != nil {
		return "", err
	}
	if err = contactguard.CheckBlocked(ctx, tx, actor, target); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(LEAST($1::uuid,$2::uuid)::text||GREATEST($1::uuid,$2::uuid)::text, 101))`, actor, target); err != nil {
		return "", err
	}
	for i, id := range []string{actor, target} {
		var ready bool
		err = tx.QueryRow(ctx, `SELECT (age IS NOT NULL OR birth_date IS NOT NULL) AND bio<>'' FROM user_profiles WHERE user_id=$1`, id).Scan(&ready)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && !ready {
			if i == 0 {
				return "", ErrProfileRequired
			}
			return "", ErrNotFound
		}
		if err != nil {
			return "", err
		}
	}
	var existing string
	err = tx.QueryRow(ctx, `SELECT conversation_id::text FROM discovery_matches WHERE user_low=LEAST($1::uuid,$2::uuid) AND user_high=GREATEST($1::uuid,$2::uuid)`, actor, target).Scan(&existing)
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	// Identical retries do not move the rejection date or create new records.
	_, err = tx.Exec(ctx, `INSERT INTO discovery_votes(actor_id,target_id,liked) VALUES($1,$2,$3) ON CONFLICT(actor_id,target_id) DO UPDATE SET liked=EXCLUDED.liked,updated_at=now() WHERE discovery_votes.liked IS DISTINCT FROM EXCLUDED.liked`, actor, target, liked)
	if err != nil {
		return "", err
	}
	var mutual bool
	err = tx.QueryRow(ctx, `SELECT $3::boolean AND EXISTS(SELECT 1 FROM discovery_votes WHERE actor_id=$2 AND target_id=$1 AND liked)`, actor, target, liked).Scan(&mutual)
	if err != nil {
		return "", err
	}
	if !mutual {
		return "", tx.Commit(ctx)
	}
	var id, mid string
	err = tx.QueryRow(ctx, `INSERT INTO conversations(user_low,user_high) VALUES(LEAST($1::uuid,$2::uuid),GREATEST($1::uuid,$2::uuid)) ON CONFLICT(user_low,user_high) DO UPDATE SET user_low=EXCLUDED.user_low RETURNING id::text`, actor, target).Scan(&id)
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id) VALUES($1,$2),($1,$3) ON CONFLICT DO NOTHING`, id, actor, target); err != nil {
		return "", err
	}
	err = tx.QueryRow(ctx, `INSERT INTO messages(conversation_id,sender_id,client_message_id,body,kind) VALUES($1,NULL,gen_random_uuid(),'Вы стали друзьями!','system') RETURNING id::text`, id).Scan(&mid)
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO discovery_matches(user_low,user_high,conversation_id,system_message_id) VALUES(LEAST($1::uuid,$2::uuid),GREATEST($1::uuid,$2::uuid),$3,$4)`, actor, target, id, mid); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}
