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
	"sync"
	"testing"
	"time"
)

func TestDiscoveryMatchAndReadPersistence(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
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
	users := make([]auth.User, 3)
	tokens := make([]string, 3)
	for i := range users {
		email := fmt.Sprintf("discovery-%d-%d@example.com", time.Now().UnixNano(), i)
		users[i], err = svc.Register(ctx, "Tester", email, "password-12345")
		if err != nil {
			t.Fatal(err)
		}
		_, tokens[i], _, err = svc.Login(ctx, email, "password-12345")
		if err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		for _, u := range users {
			_, _ = db.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.ID)
		}
	}()
	pub := &testPublisher{}
	server := httptest.NewServer(newRouter(db, pub))
	defer server.Close()
	request := func(actor int, method, path string, body any, status int) map[string]json.RawMessage {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(string(b)))
		req.Header.Set("Content-Type", "application/json")
		if actor >= 0 {
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tokens[actor]})
		}
		res, e := server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		var result map[string]json.RawMessage
		if e = json.NewDecoder(res.Body).Decode(&result); e != nil {
			t.Fatal(e)
		}
		if res.StatusCode != status {
			t.Fatalf("%s %s: status %d want %d: %s", method, path, res.StatusCode, status, result)
		}
		return result
	}
	request(-1, "GET", "/api/discovery", nil, 401)
	request(0, "GET", "/api/discovery", nil, 403)
	request(0, "POST", "/api/discovery/votes", map[string]any{"user_id": users[1].ID, "liked": true}, 403)
	request(0, "PUT", "/api/profile", map[string]any{"name": "Tester", "age": 1, "bio": strings.Repeat("я", 201)}, 400)
	request(0, "PUT", "/api/profile", map[string]any{"name": "Tester", "age": -1, "bio": "hello"}, 400)
	for i, age := range []int{0, 200} {
		request(i, "PUT", "/api/profile", map[string]any{"name": "Tester", "age": age, "bio": strings.Repeat("я", 200), "city": "Москва"}, 200)
	}
	discovery := request(0, "GET", "/api/discovery", nil, 200)
	var profiles []chat.Profile
	_ = json.Unmarshal(discovery["profiles"], &profiles)
	if len(profiles) != 1 || profiles[0].ID != users[1].ID {
		t.Fatalf("candidate eligibility failed: %s", discovery)
	}
	existing, err := store.CreateConversation(ctx, users[0].ID, users[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	request(0, "POST", "/api/discovery/votes", map[string]any{"user_id": users[1].ID, "liked": false}, 200)
	discovery = request(0, "GET", "/api/discovery", nil, 200)
	if string(discovery["profiles"]) != "[]" {
		t.Fatal("recent rejection must not immediately reappear")
	}
	_, err = db.Exec(ctx, "UPDATE discovery_votes SET updated_at=now()-interval '8 days' WHERE actor_id=$1", users[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	discovery = request(0, "GET", "/api/discovery", nil, 200)
	_ = json.Unmarshal(discovery["profiles"], &profiles)
	if len(profiles) != 1 {
		t.Fatal("old rejection should reappear")
	}
	request(1, "POST", "/api/discovery/votes", map[string]any{"user_id": users[0].ID, "liked": true}, 200)
	// Changing a rejection to a like accepts the incoming like.
	result := request(0, "POST", "/api/discovery/votes", map[string]any{"user_id": users[1].ID, "liked": true}, 200)
	if string(result["matched"]) != "true" || string(result["conversation_id"]) != fmt.Sprintf("%q", existing.ID) {
		t.Fatal("match did not reuse existing conversation")
	}
	var wg sync.WaitGroup
	failures := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			actor := i % 2
			id, e := store.Vote(ctx, users[actor].ID, users[1-actor].ID, true)
			if e == nil && id != existing.ID {
				e = fmt.Errorf("duplicate conversation: %s", id)
			}
			failures <- e
		}(i)
	}
	wg.Wait()
	close(failures)
	for e := range failures {
		if e != nil {
			t.Fatal(e)
		}
	}
	history := request(0, "GET", "/api/conversations/"+existing.ID+"/messages", nil, 200)
	var messages []chat.Message
	_ = json.Unmarshal(history["messages"], &messages)
	if len(messages) != 1 || messages[0].Kind != "system" || messages[0].SenderID != "" {
		t.Fatalf("expected one system message: %s", history)
	}
	for i := 0; i < 2; i++ {
		r := request(i, "GET", "/api/unread", nil, 200)
		if string(r["unread_chats"]) != "1" {
			t.Fatal("match must be unread for both")
		}
	}
	path := "/api/conversations/" + existing.ID + "/read"
	request(2, "GET", path, nil, 404)
	request(2, "POST", path, map[string]string{"last_message_id": messages[0].ID}, 404)
	request(0, "POST", path, map[string]string{"last_message_id": "9223372036854775807"}, 404)
	beforeRead := len(pub.events)
	request(0, "POST", path, map[string]string{"last_message_id": messages[0].ID}, 200)
	readEvents := strings.Join(pub.events[beforeRead:], "\n")
	for _, u := range users[:2] {
		if !strings.Contains(readEvents, "private-user-"+u.ID+":conversation.read") {
			t.Fatal("read was not broadcast to both participants/devices")
		}
	}
	if data, ok := pub.data.(map[string]string); !ok || data["user_id"] != users[0].ID || data["last_read_id"] != messages[0].ID {
		t.Fatal("incorrect persisted read event")
	}
	r := request(0, "GET", "/api/unread", nil, 200)
	if string(r["unread_chats"]) != "0" {
		t.Fatal("reader count not cleared")
	}
	r = request(1, "GET", "/api/unread", nil, 200)
	if string(r["unread_chats"]) != "1" {
		t.Fatal("other user was incorrectly marked read")
	}
	m, _, err := store.Send(ctx, existing.ID, users[0].ID, "77777777-1111-4111-8111-111111111111", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	r = request(0, "GET", "/api/unread", nil, 200)
	if string(r["unread_chats"]) != "0" {
		t.Fatal("own message counted as unread")
	}
	request(1, "POST", path, map[string]string{"last_message_id": m.ID}, 200)
	// Older acknowledgements cannot move the cursor backwards.
	request(1, "POST", path, map[string]string{"last_message_id": messages[0].ID}, 200)
	state := request(0, "GET", path, nil, 200)
	if string(state["peer_last_read_id"]) != fmt.Sprintf("%q", m.ID) {
		t.Fatal("receipt was not persisted monotonically")
	}
	request(0, "GET", "/api/discovery", nil, 200)
	var count int
	err = db.QueryRow(ctx, "SELECT count(*) FROM discovery_matches WHERE conversation_id=$1", existing.ID).Scan(&count)
	if err != nil || count != 1 {
		t.Fatal("match not unique", err)
	}
	// Reading a marker from another conversation is forbidden.
	other, err := store.CreateConversation(ctx, users[0].ID, users[2].ID)
	if err != nil {
		t.Fatal(err)
	}
	request(0, "POST", "/api/conversations/"+other.ID+"/read", map[string]string{"last_message_id": m.ID}, 404)
	// The first opposite likes race with no pre-existing conversation or match.
	age := 25
	if _, err = store.UpdateProfile(ctx, users[2].ID, "Third", &age, "Hello", ""); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan string, 2)
	errors := make(chan error, 2)
	for actor := 1; actor <= 2; actor++ {
		go func(actor int) {
			<-start
			id, e := store.Vote(ctx, users[actor].ID, users[3-actor].ID, true)
			results <- id
			errors <- e
		}(actor)
	}
	close(start)
	for i := 0; i < 2; i++ {
		if e := <-errors; e != nil {
			t.Fatal(e)
		}
	}
	id1, id2 := <-results, <-results
	if id1 == "" {
		id1 = id2
	}
	if id1 == "" || id2 != "" && id2 != id1 {
		t.Fatal("concurrent mutual likes did not create one dialog")
	}
	err = db.QueryRow(ctx, "SELECT count(*) FROM messages WHERE conversation_id=$1 AND kind='system'", id1).Scan(&count)
	if err != nil || count != 1 {
		t.Fatal("simultaneous likes duplicated system message", err)
	}
}
