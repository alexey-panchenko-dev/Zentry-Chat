package httpapi

import (
	"encoding/json"
	"errors"
	"newsChat/api/internal/auth"
	"testing"
)

func TestDeviceSessionOwnershipAndRevocation(t *testing.T) {
	f := newControlFixture(t)
	const path = "/api/security/sessions"
	f.request(t, -1, "GET", path, "", 401)
	data := f.request(t, 0, "GET", path, "", 200)
	var devices []auth.DeviceSession
	if err := json.Unmarshal(data["sessions"], &devices); err != nil || len(devices) != 1 || !devices[0].Current {
		t.Fatalf("invalid devices: %s", data)
	}
	f.request(t, 1, "DELETE", path+"/"+devices[0].ID, "", 404)
	f.request(t, 0, "POST", path+"/revoke-others", `{"confirm":false}`, 400)
	s := auth.New(f.db)
	_, other, _, err := s.Login(f.ctx, f.users[0].Email, "password-12345", "Test Browser")
	if err != nil {
		t.Fatal(err)
	}
	f.request(t, 0, "POST", path+"/revoke-others", `{"confirm":true}`, 204)
	if _, err = s.CurrentUser(f.ctx, other); !errors.Is(err, auth.ErrSession) {
		t.Fatalf("other token remains valid: %v", err)
	}
	f.request(t, 0, "GET", path, "", 200)
	f.request(t, 1, "GET", path, "", 200)
	f.request(t, 0, "DELETE", path+"/"+devices[0].ID, "", 204)
	f.request(t, 0, "GET", path, "", 401)
}

func TestRealtimeMessageNoticeContainsNoMessageContent(t *testing.T) {
	f := newControlFixture(t)
	f.request(t, 0, "POST", "/api/conversations/"+f.conv.ID+"/messages", `{"body":"private test message","client_message_id":"f9a12345-1234-4234-8234-123456789abc"}`, 201)
	f.publisher.mu.Lock()
	defer f.publisher.mu.Unlock()
	found := false
	for _, event := range f.publisher.events {
		if event.event != "message.created" {
			continue
		}
		found = true
		payload, ok := event.data.(map[string]string)
		if !ok || len(payload) != 1 || payload["conversation_id"] != f.conv.ID {
			t.Fatalf("realtime carries private content: %#v", event.data)
		}
	}
	if !found {
		t.Fatal("no notification emitted")
	}
}
