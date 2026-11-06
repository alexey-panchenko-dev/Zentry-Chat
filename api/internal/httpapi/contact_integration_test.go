package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
	"newsChat/api/internal/database"
	"os"
	"strings"
	"testing"
	"time"
)

func TestContactProfilePrivacyAndEditing(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	service := auth.New(db)
	var users []auth.User
	var tokens []string
	for i := 0; i < 3; i++ {
		email := fmt.Sprintf("contact-%d-%d@example.com", time.Now().UnixNano(), i)
		u, e := service.Register(ctx, fmt.Sprintf("Contact %d", i), email, "password-12345")
		if e != nil {
			t.Fatal(e)
		}
		users = append(users, u)
		_, token, _, e := service.Login(ctx, email, "password-12345")
		if e != nil {
			t.Fatal(e)
		}
		tokens = append(tokens, token)
	}
	defer func() {
		for _, u := range users {
			db.Exec(ctx, "DELETE FROM users WHERE id=$1", u.ID)
		}
	}()
	store := chat.New(db)
	conv, err := store.CreateConversation(ctx, users[0].ID, users[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	pub := &testPublisher{}
	server := httptest.NewServer(newRouter(db, pub))
	defer server.Close()
	request := func(actor int, method, path, body string, status int) map[string]json.RawMessage {
		t.Helper()
		req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if actor >= 0 {
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tokens[actor]})
		}
		res, e := server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		var data map[string]json.RawMessage
		if e = json.NewDecoder(res.Body).Decode(&data); e != nil {
			t.Fatal(e)
		}
		if res.StatusCode != status {
			t.Fatalf("%s %s actor%d status%d expected%d: %s", method, path, actor, res.StatusCode, status, data)
		}
		return data
	}
	own := "/api/profile/contact"
	peer := "/api/conversations/" + conv.ID + "/profile"
	request(-1, "GET", own, "", 401)
	request(-1, "POST", own, `{"phone":"+77771234567"}`, 401)
	request(-1, "GET", peer, "", 401)
	request(2, "GET", peer, "", 404)
	request(0, "GET", "/api/conversations/11111111-1111-1111-1111-111111111111/profile", "", 404)
	defaults := request(1, "GET", own, "", 200)
	if string(defaults["phone"]) != `""` || string(defaults["show_phone"]) != "false" || string(defaults["show_email"]) != "false" || string(defaults["email"]) != fmt.Sprintf("%q", users[1].Email) {
		t.Fatalf("wrong defaults: %s", defaults)
	}
	profile := func() map[string]json.RawMessage {
		t.Helper()
		data := request(0, "GET", peer, "", 200)
		var v map[string]json.RawMessage
		if e := json.Unmarshal(data["profile"], &v); e != nil {
			t.Fatal(e)
		}
		if string(v["id"]) != fmt.Sprintf("%q", users[1].ID) || string(v["name"]) != fmt.Sprintf("%q", users[1].Name) {
			t.Fatalf("wrong peer: %s", v)
		}
		return v
	}
	hidden := func() {
		t.Helper()
		v := profile()
		if _, ok := v["phone"]; ok {
			t.Fatalf("hidden phone present %s", v)
		}
		if _, ok := v["email"]; ok {
			t.Fatalf("hidden email present %s", v)
		}
	}
	hidden()
	saved := request(1, "POST", own, `{"phone":"+7 (777) 123-45-67","show_phone":false,"show_email":false}`, 200)
	if string(saved["phone"]) != `"+77771234567"` {
		t.Fatalf("phone not normalized %s", saved)
	}
	hidden()
	request(1, "POST", own, `{"phone":"invalid","show_phone":true,"show_email":true}`, 400)
	persisted := request(1, "GET", own, "", 200)
	if string(persisted["phone"]) != `"+77771234567"` || string(persisted["show_phone"]) != "false" {
		t.Fatalf("invalid edit replaced saved contact %s", persisted)
	}
	request(1, "POST", own, `{"phone":"+77771234567","show_phone":true,"show_email":true}`, 200)
	visible := profile()
	if string(visible["phone"]) != `"+77771234567"` || string(visible["email"]) != fmt.Sprintf("%q", users[1].Email) {
		t.Fatalf("opt-in missing %s", visible)
	}
	request(1, "POST", own, `{"phone":"+77771234567","show_phone":true,"show_email":true,"email":"attacker@example.com"}`, 400)
	current, e := service.CurrentUser(ctx, tokens[1])
	if e != nil || current.Email != users[1].Email {
		t.Fatalf("auth email changed: %v %v", current, e)
	}
	for _, path := range []string{"/api/users?query=" + users[1].Email, "/api/conversations"} {
		data := request(0, "GET", path, "", 200)
		raw, _ := json.Marshal(data)
		if strings.Contains(string(raw), "77771234567") || strings.Contains(string(raw), users[1].Email) || strings.Contains(string(raw), `\"email\"`) || strings.Contains(string(raw), `\"phone\"`) {
			t.Fatalf("contact leaked through %s: %s", path, raw)
		}
	}
	outsiderOwn := request(0, "GET", own, "", 200)
	if string(outsiderOwn["phone"]) != `""` || string(outsiderOwn["email"]) != fmt.Sprintf("%q", users[0].Email) {
		t.Fatalf("own route read peer: %s", outsiderOwn)
	}
	request(2, "GET", peer, "", 404)
	removed := request(1, "POST", own, `{"phone":"","show_phone":true,"show_email":false}`, 200)
	if string(removed["phone"]) != `""` || string(removed["show_phone"]) != "false" {
		t.Fatalf("remove did not disable phone %s", removed)
	}
	hidden()
	persisted = request(1, "GET", own, "", 200)
	if string(persisted["phone"]) != `""` || string(persisted["show_phone"]) != "false" {
		t.Fatalf("remove not persisted %s", persisted)
	}
	capabilities := request(0, "GET", "/api/realtime/config", "", 200)
	var media map[string]bool
	json.Unmarshal(capabilities["media"], &media)
	if !media["contact_profiles"] || !media["voice"] || !media["profile_photos"] || string(capabilities["enabled"]) != "true" || string(capabilities["key"]) != `"public-key"` {
		t.Fatalf("capability changed existing config %s", capabilities)
	}
	if pub.calls != 0 {
		t.Fatalf("contact settings unexpectedly published %d events", pub.calls)
	}
}
