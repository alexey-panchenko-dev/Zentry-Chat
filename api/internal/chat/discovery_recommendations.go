package chat

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrDiscoveryCursor = errors.New("invalid discovery cursor")
var discoveryCursorPattern = regexp.MustCompile(`^[0-2]\.[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type DiscoveryResult struct {
	Profiles   []Profile `json:"profiles"`
	NextCursor string    `json:"next_cursor"`
}

// Pages contain groups, never a compatibility ranking. A persisted random UUID
// keeps the shuffle stable across votes, polling and cursor-based page requests.
func (s *Store) DiscoveryPage(ctx context.Context, id string, incoming bool, cursor string) (DiscoveryResult, error) {
	result := DiscoveryResult{Profiles: make([]Profile, 0)}
	stage, key := 0, "00000000-0000-0000-0000-000000000000"
	if cursor != "" {
		if !discoveryCursorPattern.MatchString(cursor) {
			return result, ErrDiscoveryCursor
		}
		stage = int(cursor[0] - '0')
		key = strings.SplitN(cursor, ".", 2)[1]
	}
	owner, err := s.Profile(ctx, id)
	if err != nil {
		return result, err
	}
	if owner.Age == nil || strings.TrimSpace(owner.Bio) == "" {
		return result, ErrProfileRequired
	}

	// Query groups in order, stopping once a page is full. Static predicates let
	// PostgreSQL use the city/key indexes instead of sorting the entire population.
	lastCursor := ""
	for group := stage; group <= 2; group++ {
		prefix := `WITH recommended AS MATERIALIZED (
 SELECT DISTINCT other.user_id FROM discovery_profile_terms mine
 JOIN discovery_profile_terms other ON (other.term=mine.term OR other.term % mine.term)
 WHERE mine.user_id=$1 AND (other.term=mine.term OR similarity(other.term,mine.term)>=0.4)
 ) `
		filter := `NOT EXISTS(SELECT 1 FROM discovery_votes v WHERE v.actor_id=$1 AND v.target_id=p.user_id AND (v.liked OR v.updated_at>now()-interval '7 days')) AND `
		switch group {
		case 0:
			filter += `discovery_normalize($3)<>'' AND p.discovery_city=discovery_normalize($3) AND p.user_id IN (SELECT user_id FROM recommended)`
		case 1:
			filter += `(discovery_normalize($3)='' OR p.discovery_city<>discovery_normalize($3)) AND p.user_id IN (SELECT user_id FROM recommended)`
		case 2:
			filter += `p.user_id NOT IN (SELECT user_id FROM recommended)`
		}
		if incoming {
			prefix = ""
			filter = `EXISTS(SELECT 1 FROM discovery_votes v WHERE v.actor_id=p.user_id AND v.target_id=$1 AND v.liked) AND NOT EXISTS(SELECT 1 FROM discovery_votes v WHERE v.actor_id=$1 AND v.target_id=p.user_id)`
		}
		query := prefix + `SELECT ` + profileColumns + `,
 (p.discovery_city<>'' AND p.discovery_city=discovery_normalize($3)), page.discovery_key::text
 FROM (
 SELECT p.user_id,p.discovery_key FROM user_profiles p
 WHERE p.user_id<>$1 AND (p.age IS NOT NULL OR p.birth_date IS NOT NULL) AND btrim(p.bio)<>''
 AND p.discovery_key>$2::uuid
 AND NOT EXISTS(SELECT 1 FROM contact_blocks b WHERE
   (b.blocker_id=$1 AND b.blocked_id=p.user_id) OR (b.blocked_id=$1 AND b.blocker_id=p.user_id))
 AND NOT EXISTS(SELECT 1 FROM discovery_matches m WHERE m.user_low=LEAST($1::uuid,p.user_id) AND m.user_high=GREATEST($1::uuid,p.user_id))
 AND ` + filter + ` ORDER BY p.discovery_key LIMIT $4
 ) page JOIN users u ON u.id=page.user_id JOIN user_profiles p ON p.user_id=u.id
 ORDER BY page.discovery_key`
		rows, err := s.db.Query(ctx, query, id, key, owner.City, 31-len(result.Profiles))
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var profile Profile
			var shuffleKey string
			if err := rows.Scan(&profile.ID, &profile.Name, &profile.Username, &profile.AvatarURL, &profile.Age, &profile.Bio, &profile.City, &profile.SameCity, &shuffleKey); err != nil {
				rows.Close()
				return result, err
			}
			if len(result.Profiles) == 30 {
				result.NextCursor = lastCursor
				rows.Close()
				return result, nil
			}
			profile.Recommended = !incoming && group < 2
			result.Profiles = append(result.Profiles, profile)
			lastCursor = fmt.Sprintf("%d.%s", group, shuffleKey)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return result, err
		}
		if incoming {
			break
		}
		key = "00000000-0000-0000-0000-000000000000"
	}
	return result, nil
}
