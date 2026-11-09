package chat

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"newsChat/api/internal/database"
)

// Run only against a disposable database:
// go test ./internal/chat -run '^$' -bench BenchmarkDiscoveryPopulation -benchtime=5x
func BenchmarkDiscoveryPopulation(b *testing.B) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		b.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, url)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(db.Close)
	if err := database.Migrate(ctx, db); err != nil {
		b.Fatal(err)
	}
	prefix := fmt.Sprintf("discovery-bench-%d-", time.Now().UnixNano())
	b.Cleanup(func() {
		if _, err := db.Exec(ctx, "DELETE FROM users WHERE starts_with(email,$1)", prefix); err != nil {
			b.Error(err)
		}
	})
	if _, err := db.Exec(ctx, `WITH added AS (
 INSERT INTO users(name,email,password_hash)
 SELECT 'Candidate',$1||g||'@example.com','test' FROM generate_series(1,20000) g RETURNING id,email
 ) INSERT INTO user_profiles(user_id,age,bio,city)
 SELECT id,24,CASE WHEN email LIKE $1||'1%' THEN 'Титанов аниме' ELSE 'Рыбалка автомобили' END,
 CASE WHEN email LIKE $1||'2%' THEN 'Казань' ELSE 'Москва' END FROM added`, prefix); err != nil {
		b.Fatal(err)
	}
	var owner string
	if err := db.QueryRow(ctx, `INSERT INTO users(name,email,password_hash) VALUES('Viewer',$1||'owner@example.com','test') RETURNING id::text`, prefix).Scan(&owner); err != nil {
		b.Fatal(err)
	}
	for _, table := range []string{"users", "user_profiles", "discovery_profile_terms"} {
		if _, err := db.Exec(ctx, "ANALYZE "+table); err != nil {
			b.Fatal(err)
		}
	}
	store := New(db)
	age := 24
	for _, scenario := range []struct{ name, bio string }{{"common_topic", "Титанов"}, {"no_shared_topics", "Астрономия"}, {"no_search_terms", "Люблю общаться"}} {
		b.Run(scenario.name, func(b *testing.B) {
			if _, err := store.UpdateProfile(ctx, owner, "Viewer", &age, scenario.bio, "Москва"); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				page, err := store.DiscoveryPage(ctx, owner, false, "")
				if err != nil {
					b.Fatal(err)
				}
				if len(page.Profiles) != 30 || page.NextCursor == "" {
					b.Fatal("incomplete page")
				}
			}
		})
	}
}
