package httpapi

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"newsChat/api/internal/chat"
)

func TestDiscoveryDescriptionScenarios(t *testing.T) {
	f := newControlFixture(t)
	age := 24
	cases := []struct {
		name, first, second string
		want                bool
	}{
		{"similar/shared_title", "Люблю атаку титанов", "Атака титан — лучшее аниме", true},
		{"similar/reordered_interests", "Аниме, музыка и гитара", "Гитара и музыка по вечерам", true},
		{"similar/opposite_opinions", "Обожаю аниме", "Ненавижу аниме", true},
		{"typos/extra_letter_in_short_word", "люблюю аниме", "анимае нравится", true},
		{"typos/missing_letter", "Люблю программирование", "Обожаю програмирование", true},
		{"typos/replaced_letter", "Люблю аниме", "Обожаю анимэ", true},
		{"other/unrelated_interests", "Люблю аниме", "Обожаю рыбалку", false},
		{"other/generic_words_only", "Люблю общаться", "Люблю общаться", false},
		{"other/same_city_without_shared_topic", "Обожаю автомобили", "Люблю аниме", false},
		{"other/normalization", "  АНИМЕ!!! Ёлки  ", "аниме, елки", true},
		{"other/no_synonym_dictionary", "Люблю путешествия", "Обожаю поездки", false},
		{"other/no_translation", "Атака титанов", "Attack on Titan", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for i, bio := range []string{tc.first, tc.second} {
				if _, err := f.store.UpdateProfile(f.ctx, f.users[i].ID, "Tester", &age, bio, "Москва"); err != nil {
					t.Fatal(err)
				}
			}
			// Assert the public API flag in both directions, not just SQL scores.
			for actor := 0; actor < 2; actor++ {
				body := f.request(t, actor, "GET", "/api/discovery", "", 200)
				var people []chat.Profile
				if err := json.Unmarshal(body["profiles"], &people); err != nil {
					t.Fatal(err)
				}
				if len(people) != 1 || people[0].ID != f.users[1-actor].ID || people[0].Recommended != tc.want {
					t.Fatalf("actor %d: %q / %q: want recommended=%v, got %+v", actor, tc.first, tc.second, tc.want, people)
				}
			}
		})
	}
}

func TestDiscoveryTopicsCitiesAndUpdates(t *testing.T) {
	f := newControlFixture(t)
	age := 24
	update := func(i int, bio, city string) {
		t.Helper()
		if _, err := f.store.UpdateProfile(f.ctx, f.users[i].ID, "Tester", &age, bio, city); err != nil {
			t.Fatal(err)
		}
	}
	page := func(i int) chat.DiscoveryResult {
		t.Helper()
		p, err := f.store.DiscoveryPage(f.ctx, f.users[i].ID, false, "")
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	update(0, "Люблю атаку титанов", "  МОСКВА  ")
	update(1, "Атака титан — лучшее аниме", "Москва")
	update(2, "Ненавижу титанов", "Казань")
	if _, err := f.db.Exec(f.ctx, "UPDATE user_profiles SET age=80 WHERE user_id=$1", f.users[1].ID); err != nil {
		t.Fatal(err)
	}
	first := page(0)
	if len(first.Profiles) != 2 || first.Profiles[0].ID != f.users[1].ID || !first.Profiles[0].Recommended || !first.Profiles[0].SameCity || !first.Profiles[1].Recommended || first.Profiles[1].SameCity {
		t.Fatalf("city groups: %+v", first)
	}
	if !reflect.DeepEqual(first, page(0)) {
		t.Fatal("shuffle changed on identical request")
	}
	reverse := page(1)
	found := false
	for _, p := range reverse.Profiles {
		if p.ID == f.users[0].ID {
			found = p.Recommended
		}
	}
	if !found {
		t.Fatal("recommendation must work in both directions")
	}
	update(1, "Люблю автомобили и рыбалку", "Москва")
	next := page(0)
	if len(next.Profiles) != 2 || next.Profiles[0].ID != f.users[2].ID || !next.Profiles[0].Recommended || next.Profiles[1].Recommended {
		t.Fatalf("profile edit did not refresh terms: %+v", next)
	}
	update(0, "Люблю общаться, просто привет", "Москва")
	for _, p := range page(0).Profiles {
		if p.Recommended {
			t.Fatal("generic words must not recommend")
		}
	}
	update(0, "Люблю программирование, PostgreSQL", "Орёл")
	update(1, "Postgresql — интересная тема", "ОРЕЛ")
	if p := page(0).Profiles[0]; p.ID != f.users[1].ID || !p.Recommended || !p.SameCity {
		t.Fatalf("case, punctuation, city normalization: %+v", p)
	}
	update(0, "Люблю титанов", "")
	if page(0).Profiles[0].SameCity {
		t.Fatal("empty city is not a local match")
	}
	if _, err := f.db.Exec(f.ctx, "INSERT INTO contact_blocks(blocker_id,blocked_id) VALUES($1,$2)", f.users[2].ID, f.users[0].ID); err != nil {
		t.Fatal(err)
	}
	for _, p := range page(0).Profiles {
		if p.ID == f.users[2].ID {
			t.Fatal("reverse block leaked into discovery")
		}
	}
	f.request(t, 0, "GET", "/api/discovery?cursor=broken", "", 400)
	f.request(t, 0, "GET", "/api/discovery?cursor=0.ffffffff-ffff-ffff-ffff-ffffffffffff'", "", 400)
}

func TestDiscoveryCursorAcrossGroups(t *testing.T) {
	f := newControlFixture(t)
	age := 22
	if _, err := f.store.UpdateProfile(f.ctx, f.users[0].ID, "Tester", &age, "Титанов аниме", "Москва"); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i := 0; i < 67; i++ {
		var id string
		if err := f.db.QueryRow(f.ctx, `INSERT INTO users(name,email,password_hash) VALUES('Candidate',$1,'test') RETURNING id::text`, fmt.Sprintf("candidate-%s-%d@example.com", f.users[0].ID, i)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		bio, city := "Титанов", "Москва"
		if i >= 23 {
			city = "Казань"
		}
		if i >= 46 {
			bio = "Автомобили"
		}
		if _, err := f.store.UpdateProfile(f.ctx, id, "Candidate", &age, bio, city); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, id := range ids {
			f.db.Exec(f.ctx, "DELETE FROM users WHERE id=$1", id)
		}
	})
	seen := map[string]bool{}
	cursor, lastGroup, pages := "", 0, 0
	for {
		page, err := f.store.DiscoveryPage(f.ctx, f.users[0].ID, false, cursor)
		if err != nil {
			t.Fatal(err)
		}
		pages++
		if len(page.Profiles) > 30 {
			t.Fatal("page exceeds limit")
		}
		for _, p := range page.Profiles {
			if seen[p.ID] {
				t.Fatalf("duplicate across pages: %s", p.ID)
			}
			seen[p.ID] = true
			group := 2
			if p.Recommended {
				group = 1
				if p.SameCity {
					group = 0
				}
			}
			if group < lastGroup {
				t.Fatal("city/topic group order reversed")
			}
			lastGroup = group
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
		if pages > 4 {
			t.Fatal("pagination did not terminate")
		}
	}
	if len(seen) != 67 || pages != 3 {
		t.Fatalf("lost candidates: %d in %d pages", len(seen), pages)
	}
	// Incoming likes use the same bounded cursor contract and exclude blocks.
	for _, id := range ids[:32] {
		if _, err := f.store.Vote(f.ctx, id, f.users[0].ID, true); err != nil {
			t.Fatal(err)
		}
	}
	body := f.request(t, 0, "GET", "/api/discovery/likes", "", 200)
	var first chat.DiscoveryResult
	data, _ := json.Marshal(body)
	if err := json.Unmarshal(data, &first); err != nil {
		t.Fatal(err)
	}
	if len(first.Profiles) != 30 || first.NextCursor == "" {
		t.Fatalf("incoming pagination: %+v", first)
	}
	second, err := f.store.DiscoveryPage(f.ctx, f.users[0].ID, true, first.NextCursor)
	if err != nil || len(second.Profiles) != 2 {
		t.Fatalf("incoming second page: %+v %v", second, err)
	}
}
