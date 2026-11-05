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

func TestSavedMessagesPrivacyAndPersistence(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL")
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
	s := auth.New(db)
	var ids, tokens []string
	for i := range 2 {
		email := fmt.Sprintf("saved-%d-%d@example.com", time.Now().UnixNano(), i)
		u, e := s.Register(ctx, "Saved Test", email, "test-password-12345")
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, u.ID)
		_, token, _, e := s.Login(ctx, email, "test-password-12345")
		if e != nil {
			t.Fatal(e)
		}
		tokens = append(tokens, token)
	}
	defer func() {
		for _, id := range ids {
			db.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, id)
		}
	}()
	handler := newRouter(db)
	request := func(actor int, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		method := "POST"
		if body == "" {
			method = "GET"
		}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if actor >= 0 {
			r.AddCookie(&http.Cookie{Name: sessionCookie, Value: tokens[actor]})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w
	}
	request(-1, "/api/conversations/saved", "{}", 401)
	var first, again struct {
		Conversation chat.Conversation `json:"conversation"`
	}
	json.Unmarshal(request(0, "/api/conversations/saved", "{}", 200).Body.Bytes(), &first)
	json.Unmarshal(request(0, "/api/conversations/saved", "{}", 200).Body.Bytes(), &again)
	if first.Conversation.ID == "" || first.Conversation.ID != again.Conversation.ID || first.Conversation.Participant.ID != ids[0] {
		t.Fatal("saved dialog is not idempotent/private")
	}
	path := "/api/conversations/" + first.Conversation.ID + "/messages"
	body := `{"client_message_id":"00000000-0000-4000-8000-000000000321","body":"Моя заметка 🔖"}`
	request(0, path, body, 201)
	request(0, path, body, 200)
	var list struct {
		Messages []chat.Message `json:"messages"`
	}
	json.Unmarshal(request(0, path, "", 200).Body.Bytes(), &list)
	if len(list.Messages) != 1 || list.Messages[0].Body != "Моя заметка 🔖" {
		t.Fatal("note not stored exactly once")
	}
	request(1, path, "", 404)
	request(1, path, body, 404)
	request(0, "/api/conversations/"+first.Conversation.ID+"/calls", `{"client_call_id":"00000000-0000-4000-8000-000000000322"}`, 409)
}
