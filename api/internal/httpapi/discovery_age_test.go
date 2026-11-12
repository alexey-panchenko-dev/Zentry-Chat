package httpapi

import (
	"context"
	"errors"
	"fmt"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
	"newsChat/api/internal/database"
	"os"
	"testing"
	"time"
)

func TestDiscoveryAgeGroupsCannotBeBypassed(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	svc := auth.New(db)
	store := chat.New(db)
	users := make([]auth.User, 4)
	defer func() {
		for _, u := range users {
			if u.ID != "" {
				_, _ = db.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.ID)
			}
		}
	}()
	for i, age := range []int{16, 17, 25, 26} {
		users[i], err = svc.Register(ctx, "Age test", fmt.Sprintf("age-guard-%d-%d@example.com", time.Now().UnixNano(), i), "password-12345")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = store.UpdateProfile(ctx, users[i].ID, "Age test", &age, "Hello", ""); err != nil {
			t.Fatal(err)
		}
	}
	for _, pair := range [][2]int{{0, 2}, {2, 0}, {1, 3}, {3, 1}} {
		if _, err = store.Vote(ctx, users[pair[0]].ID, users[pair[1]].ID, true); !errors.Is(err, chat.ErrNotFound) {
			t.Fatalf("cross-age vote %v: %v", pair, err)
		}
	}
	t.Setenv("DISCOVERY_MINORS_ENABLED", "true")
	for _, i := range []int{0, 2} {
		for _, incoming := range []bool{false, true} {
			profiles, e := store.Discovery(ctx, users[i].ID, incoming)
			if e != nil {
				t.Fatal(e)
			}
			for _, p := range profiles {
				if p.Age == nil || (*p.Age >= 18) != (i >= 2) {
					t.Fatalf("cross-age profile exposed: %+v", p)
				}
			}
		}
	}
	adultAge := 25
	if _, e := store.UpdateProfile(ctx, users[0].ID, "Age test", &adultAge, "Hello", ""); !errors.Is(e, chat.ErrAgeGroupLocked) {
		t.Fatalf("age group editing bypass: %v", e)
	}
	// A birthday takes priority over a forged legacy age value.
	if _, err = db.Exec(ctx, `UPDATE user_profiles SET birth_date=CURRENT_DATE-interval '16 years',age=25 WHERE user_id=$1`, users[3].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Vote(ctx, users[2].ID, users[3].ID, true); !errors.Is(err, chat.ErrNotFound) {
		t.Fatalf("legacy age bypass: %v", err)
	}
	t.Setenv("DISCOVERY_MINORS_ENABLED", "false")
	if _, e := store.Vote(ctx, users[0].ID, users[1].ID, true); !errors.Is(e, chat.ErrDiscoveryMinorDisabled) {
		t.Fatalf("minor discovery enabled by default: %v", e)
	}
	t.Setenv("DISCOVERY_MINORS_ENABLED", "true")
	if _, err = store.Vote(ctx, users[0].ID, users[1].ID, true); err != nil {
		t.Fatalf("same age group rejected: %v", err)
	}
}
