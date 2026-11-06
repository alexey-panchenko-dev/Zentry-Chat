package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
	"newsChat/api/internal/database"
)

type testPublisher struct {
	fail    bool
	calls   int
	channel string
	event   string
	data    any
}

func (p *testPublisher) Enabled() bool { return true }
func (p *testPublisher) PublicConfig() map[string]any {
	return map[string]any{"enabled": true, "key": "public-key", "cluster": "eu"}
}
func (p *testPublisher) Publish(channel, event string, data any) error {
	p.calls++
	p.channel = channel
	p.event = event
	p.data = data
	if p.fail {
		return errors.New("simulated outage")
	}
	return nil
}
func (p *testPublisher) Authorize(socketID, channel string) ([]byte, error) {
	return []byte(`{"auth":"public-key:test-signature"}`), nil
}

func TestChatAccessAndDelivery(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to a test PostgreSQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	db, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	service := auth.New(db)
	var users []auth.User
	var tokens []string
	for i := 0; i < 3; i++ {
		email := fmt.Sprintf("chat-%d-%d@example.com", time.Now().UnixNano(), i)
		user, err := service.Register(ctx, fmt.Sprintf("Chat Tester %d", i), email, "test-password-12345")
		if err != nil {
			t.Fatal(err)
		}
		users = append(users, user)
		_, token, _, err := service.Login(ctx, email, "test-password-12345")
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, token)
	}
	defer func() {
		cleanup, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		for _, u := range users {
			_, _ = db.Exec(cleanup, "DELETE FROM users WHERE id=$1", u.ID)
		}
	}()
	publisher := &testPublisher{}
	server := httptest.NewServer(newRouter(db, publisher))
	defer server.Close()
	request := func(actor int, method, path, body string, status int) map[string]json.RawMessage {
		t.Helper()
		req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if actor >= 0 {
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tokens[actor]})
		}
		response, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var result map[string]json.RawMessage
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != status {
			t.Fatalf("%s %s actor=%d: status=%d want=%d body=%s", method, path, actor, response.StatusCode, status, result)
		}
		return result
	}
	request(-1, "GET", "/api/conversations", "", 401)
	request(-1, "GET", "/api/realtime/config", "", 401)
	capabilities := request(0, "GET", "/api/realtime/config", "", 200)
	if string(capabilities["enabled"]) != "true" || string(capabilities["key"]) != `"public-key"` || string(capabilities["cluster"]) != `"eu"` {
		t.Fatalf("original realtime keys changed: %s", capabilities)
	}
	var media map[string]bool
	if err := json.Unmarshal(capabilities["media"], &media); err != nil || !media["voice"] || !media["profile_photos"] {
		t.Fatalf("missing media capabilities: %s (%v)", capabilities, err)
	}
	found := request(0, "GET", "/api/users?query="+users[1].Email, "", 200)
	var publicUsers []chat.PublicUser
	_ = json.Unmarshal(found["users"], &publicUsers)
	if len(publicUsers) != 1 || publicUsers[0].ID != users[1].ID {
		t.Fatal("user search failed")
	}
	request(0, "POST", "/api/conversations", fmt.Sprintf(`{"user_id":%q}`, users[0].ID), 400)
	request(0, "POST", "/api/conversations", `{"user_id":"11111111-1111-1111-1111-111111111111"}`, 404)
	result := request(0, "POST", "/api/conversations", fmt.Sprintf(`{"user_id":%q}`, users[1].ID), 200)
	var conv chat.Conversation
	_ = json.Unmarshal(result["conversation"], &conv)
	if conv.ID == "" || conv.Participant.ID != users[1].ID {
		t.Fatal("incorrect conversation")
	}
	reverse := request(1, "POST", "/api/conversations", fmt.Sprintf(`{"user_id":%q}`, users[0].ID), 200)
	var reverseConv chat.Conversation
	_ = json.Unmarshal(reverse["conversation"], &reverseConv)
	if conv.ID != reverseConv.ID {
		t.Fatal("duplicate dialog")
	}
	outsider := request(2, "GET", "/api/conversations", "", 200)
	if string(outsider["conversations"]) != "[]" {
		t.Fatal("outsider sees dialog")
	}
	path := "/api/conversations/" + conv.ID + "/messages"
	request(2, "GET", path, "", 404)
	request(2, "POST", path, `{"client_message_id":"10000000-0000-0000-0000-000000000001","body":"forbidden"}`, 404)
	request(0, "POST", path, `{"client_message_id":"10000000-0000-0000-0000-000000000001","body":"  "}`, 400)
	firstBody := `{"client_message_id":"10000000-0000-0000-0000-000000000001","body":"Первое сообщение"}`
	first := request(0, "POST", path, firstBody, 201)
	var firstMessage chat.Message
	_ = json.Unmarshal(first["message"], &firstMessage)
	if string(first["realtime"]) != `"published"` || publisher.channel != "private-conversation-"+conv.ID || publisher.event != "message.created" {
		t.Fatal("event not published")
	}
	replay := request(0, "POST", path, firstBody, 200)
	var replayMessage chat.Message
	_ = json.Unmarshal(replay["message"], &replayMessage)
	if firstMessage.ID != replayMessage.ID {
		t.Fatal("duplicate message on retry")
	}
	request(0, "POST", path, `{"client_message_id":"10000000-0000-0000-0000-000000000001","body":"changed"}`, 409)
	request(1, "POST", path, `{"client_message_id":"10000000-0000-0000-0000-000000000002","body":"Ответ"}`, 201)
	page := request(1, "GET", path+"?limit=1", "", 200)
	var messages []chat.Message
	_ = json.Unmarshal(page["messages"], &messages)
	var cursor string
	_ = json.Unmarshal(page["next_cursor"], &cursor)
	if len(messages) != 1 || cursor == "" {
		t.Fatal("missing pagination cursor")
	}
	older := request(0, "GET", path+"?limit=1&before="+cursor, "", 200)
	_ = json.Unmarshal(older["messages"], &messages)
	if len(messages) != 1 || messages[0].ID != firstMessage.ID || string(older["next_cursor"]) != `""` {
		t.Fatal("incorrect second page")
	}
	request(0, "GET", path+"?before=-1", "", 400)
	request(0, "GET", path+"?limit=101", "", 400)
	channelBody := fmt.Sprintf(`{"socket_id":"123.456","channel_name":"private-conversation-%s"}`, conv.ID)
	request(2, "POST", "/api/pusher/auth", channelBody, 404)
	request(0, "POST", "/api/pusher/auth", channelBody, 200)
	request(0, "POST", "/api/pusher/auth", `{"socket_id":"123.456","channel_name":"public-channel"}`, 400)
	publisher.fail = true
	failed := request(0, "POST", path, `{"client_message_id":"10000000-0000-0000-0000-000000000003","body":"Сохранится при сбое Pusher"}`, 201)
	if string(failed["realtime"]) != `"failed"` {
		t.Fatal("outage is not reported")
	}
	saved := request(1, "GET", path, "", 200)
	_ = json.Unmarshal(saved["messages"], &messages)
	if len(messages) != 3 {
		t.Fatal("message lost during Pusher outage")
	}
	disabled := httptest.NewServer(newRouter(db))
	defer disabled.Close()
	req, _ := http.NewRequest("GET", disabled.URL+"/api/realtime/config", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tokens[0]})
	response, err := disabled.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var config struct {
		Enabled bool            `json:"enabled"`
		Media   map[string]bool `json:"media"`
	}
	_ = json.NewDecoder(response.Body).Decode(&config)
	if response.StatusCode != 200 || config.Enabled || !config.Media["voice"] || !config.Media["profile_photos"] {
		t.Fatal("disabled realtime config incorrect")
	}
}
