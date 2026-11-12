package httpapi

import (
	"encoding/json"
	"errors"
	"newsChat/api/internal/chat"
	"testing"
)

func TestMessageRequestAcceptanceIsolation(t *testing.T) {
	f := newControlFixture(t)
	f.request(t, 1, "PUT", "/api/profile/privacy", `{"messages":"requests","calls":"contacts","presence":"everyone","search":"everyone","birthday":"everyone"}`, 200)
	message, created, err := f.store.Send(f.ctx, f.conv.ID, f.users[0].ID, "12111111-0000-4000-8000-000000000001", "hello request")
	if err != nil || !created {
		t.Fatalf("first request failed: %v", err)
	}
	if repeat, fresh, e := f.store.Send(f.ctx, f.conv.ID, f.users[0].ID, "12111111-0000-4000-8000-000000000001", "hello request"); e != nil || fresh || repeat.ID != message.ID {
		t.Fatalf("pending request retry not idempotent: %v", e)
	}
	if _, _, err = f.store.Send(f.ctx, f.conv.ID, f.users[0].ID, "12111111-0000-4000-8000-000000000002", "spam"); !errors.Is(err, chat.ErrPrivacy) {
		t.Fatalf("pending request spam allowed: %v", err)
	}
	normal, err := f.store.Conversations(f.ctx, f.users[1].ID, 0, 20)
	if err != nil || len(normal) != 0 {
		t.Fatalf("pending request mixed with chats: %v %v", normal, err)
	}
	requests, err := f.store.ConversationsScope(f.ctx, f.users[1].ID, 0, 20, true)
	if err != nil || len(requests) != 1 || requests[0].LastMessageID != message.ID {
		t.Fatalf("request missing: %+v %v", requests, err)
	}
	path := "/api/message-requests/" + f.conv.ID
	f.request(t, 0, "POST", path, `{"action":"accepted"}`, 404)
	f.request(t, 2, "POST", path, `{"action":"accepted"}`, 404)
	f.request(t, 1, "POST", path, `{"action":"accepted"}`, 200)
	if _, _, err = f.store.Send(f.ctx, f.conv.ID, f.users[0].ID, "12111111-0000-4000-8000-000000000002", "accepted follow up"); err != nil {
		t.Fatalf("accepted request blocked: %v", err)
	}
	list := f.request(t, 1, "GET", "/api/message-requests", "", 200)
	var remaining []chat.Conversation
	_ = json.Unmarshal(list["conversations"], &remaining)
	if len(remaining) != 0 {
		t.Fatal("accepted request remains pending")
	}
}
