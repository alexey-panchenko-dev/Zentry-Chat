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

func TestBirthDatesOwnerAndPeerPrivacy(t *testing.T) {
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
	svc := auth.New(db)
	store := chat.New(db)
	mailer := &recordingMailer{}
	server := httptest.NewServer(newRouterWithMailer(db, mailer, &testPublisher{}))
	defer server.Close()
	ids := []string{}
	defer func() {
		for _, id := range ids {
			_, _ = db.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
		}
	}()
	request := func(token, method, path, body string, status int) string {
		t.Helper()
		r, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		}
		res, e := server.Client().Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		var raw json.RawMessage
		if e = json.NewDecoder(res.Body).Decode(&raw); e != nil {
			t.Fatal(e)
		}
		if res.StatusCode != status {
			t.Fatalf("%s %s: %d %s", method, path, res.StatusCode, raw)
		}
		return string(raw)
	}
	email := fmt.Sprintf("birthday-%d@example.com", time.Now().UnixNano())
	raw := request("", "POST", "/api/auth/register", fmt.Sprintf(`{"name":"DOB","email":%q,"password":"Strong1&","birth_date":"2000-02-29"}`, email), 202)
	code := mailer.code(email, auth.VerificationPurpose)
	request("", "POST", "/api/auth/verify-email", fmt.Sprintf(`{"email":%q,"code":%q}`, email, code), 200)
	raw = request("", "POST", "/api/auth/login", fmt.Sprintf(`{"email":%q,"password":"Strong1&"}`, email), 200)
	var registered struct {
		User auth.User `json:"user"`
	}
	if err = json.Unmarshal([]byte(raw), &registered); err != nil {
		t.Fatal(err)
	}
	ids = append(ids, registered.User.ID)
	if strings.Contains(raw, "birth_date") {
		t.Fatal("auth leaked DOB")
	}
	_, token, _, err := svc.Login(ctx, email, "Strong1&")
	if err != nil {
		t.Fatal(err)
	}
	raw = request(token, "GET", "/api/profile", "", 200)
	if !strings.Contains(raw, `"birth_date":"2000-02-29"`) {
		t.Fatal(raw)
	}
	request(token, "PUT", "/api/profile", `{"name":"DOB","age":99,"bio":"hello"}`, 200)
	own, err := store.OwnProfile(ctx, registered.User.ID)
	if err != nil || own.BirthDate == nil || *own.BirthDate != "2000-02-29" {
		t.Fatalf("preserved: %+v %v", own, err)
	}
	var expected int
	if err = db.QueryRow(ctx, `SELECT EXTRACT(YEAR FROM age(CURRENT_DATE,DATE '2000-02-29'))::integer`).Scan(&expected); err != nil {
		t.Fatal(err)
	}
	if own.Age == nil || *own.Age != expected {
		t.Fatalf("age %+v expected %d", own.Age, expected)
	}
	for _, date := range []string{"1900-02-29", "2099-01-01", "2024-02-30", ""} {
		request(token, "PUT", "/api/profile", fmt.Sprintf(`{"name":"DOB","birth_date":%q}`, date), 400)
		request("", "POST", "/api/auth/register", fmt.Sprintf(`{"name":"DOB","email":%q,"password":"Strong1&","birth_date":%q}`, fmt.Sprintf("invalid-birth-%d@example.com", time.Now().UnixNano()), date), 400)
	}
	// A stored date takes precedence even if a cached client submits a stale manual age.
	for _, expression := range []string{"CURRENT_DATE - interval '25 years'", "CURRENT_DATE - interval '25 years' + interval '1 day'", "CURRENT_DATE - interval '25 years' - interval '1 day'"} {
		var date string
		var want int
		if err = db.QueryRow(ctx, `SELECT to_char((`+expression+`)::date,'YYYY-MM-DD'),EXTRACT(YEAR FROM age(CURRENT_DATE,(`+expression+`)::date))::integer`).Scan(&date, &want); err != nil {
			t.Fatal(err)
		}
		if _, err = store.UpdateProfileWithBirthDate(ctx, registered.User.ID, "DOB", nil, "hello", "", &date); err != nil {
			t.Fatal(err)
		}
		got, e := store.Profile(ctx, registered.User.ID)
		if e != nil || got.Age == nil || *got.Age != want {
			t.Fatalf("age boundary %s: %+v %v want %d", date, got, e, want)
		}
	}
	leap := "2000-02-29"
	if _, err = store.UpdateProfileWithBirthDate(ctx, registered.User.ID, "DOB", nil, "hello", "", &leap); err != nil {
		t.Fatal(err)
	}
	peer, err := svc.Register(ctx, "Legacy", fmt.Sprintf("birthday-peer-%d@example.com", time.Now().UnixNano()), "Strong1&")
	if err != nil {
		t.Fatal(err)
	}
	ids = append(ids, peer.ID)
	a := 23
	if _, err = store.UpdateProfile(ctx, peer.ID, "Legacy", &a, "hello", ""); err != nil {
		t.Fatal(err)
	}
	profiles, err := store.Discovery(ctx, peer.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(profiles)
	if strings.Contains(string(data), "birth_date") || strings.Contains(string(data), "2000-02-29") {
		t.Fatal(string(data))
	}
	c, err := store.CreateConversation(ctx, peer.ID, registered.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	contact, err := store.ContactProfile(ctx, c.ID, peer.ID)
	if err != nil || contact.Birthday != "2000-02-29" {
		t.Fatalf("contact %+v: %v", contact, err)
	}
	data, _ = json.Marshal(contact)
	if !strings.Contains(string(data), `"birthday":"2000-02-29"`) {
		t.Fatal(string(data))
	}
	legacy, err := store.OwnProfile(ctx, peer.ID)
	if err != nil || legacy.BirthDate != nil || legacy.Age == nil || *legacy.Age != 23 {
		t.Fatalf("legacy %+v %v", legacy, err)
	}
}
