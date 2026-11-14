package httpapi

import (
	"encoding/json"
	"errors"
	"newsChat/api/internal/chat"
	"newsChat/api/internal/contactguard"
	"testing"
)

func TestPrivacyRulesEnforcedByServer(t *testing.T) {
	f := newControlFixture(t)
	f.request(t, -1, "GET", "/api/profile/privacy", "", 401)
	f.request(t, 1, "PUT", "/api/profile/privacy", `{"messages":"nobody","calls":"nobody","presence":"nobody","search":"nobody","birthday":"nobody"}`, 200)
	if _, _, err := f.store.Send(f.ctx, f.conv.ID, f.users[0].ID, "11111111-0000-4000-8000-000000000001", "hello"); !errors.Is(err, chat.ErrPrivacy) {
		t.Fatalf("message bypass: %v", err)
	}
	if _, _, err := f.calls.Create(f.ctx, f.conv.ID, f.users[0].ID, "11111111-0000-4000-8000-000000000002"); !errors.Is(err, contactguard.ErrPrivacy) {
		t.Fatalf("call bypass: %v", err)
	}
	if err := f.store.TouchPresence(f.ctx, f.users[1].ID); err != nil {
		t.Fatal(err)
	}
	profile, err := f.store.ContactProfile(f.ctx, f.conv.ID, f.users[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !profile.PresenceHidden || profile.Online || profile.LastSeen != nil || profile.Birthday != "" {
		t.Fatalf("profile privacy leaked: %+v", profile)
	}
	found, err := f.store.SearchUsers(f.ctx, f.users[0].ID, f.users[1].Email)
	if err != nil || len(found) != 0 {
		t.Fatalf("search privacy leaked: %v %v", found, err)
	}
	f.request(t, 1, "PUT", "/api/profile/privacy", `{"messages":"contacts","calls":"contacts","presence":"contacts","search":"contacts","birthday":"contacts"}`, 200)
	if _, err = f.store.SetContactAlias(f.ctx, f.conv.ID, f.users[1].ID, "Friend"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = f.store.Send(f.ctx, f.conv.ID, f.users[0].ID, "11111111-0000-4000-8000-000000000003", "hello"); err != nil {
		t.Fatalf("saved contact denied: %v", err)
	}
	profile, err = f.store.ContactProfile(f.ctx, f.conv.ID, f.users[0].ID)
	if err != nil || profile.PresenceHidden || !profile.Online {
		t.Fatalf("saved contact presence denied: %+v %v", profile, err)
	}
	f.request(t, 1, "PUT", "/api/profile/privacy", `{"messages":"invalid","calls":"everyone","presence":"everyone","search":"everyone","birthday":"everyone"}`, 400)
	own := f.request(t, 0, "GET", "/api/profile/privacy", "", 200)
	var first string
	_ = json.Unmarshal(own["messages"], &first)
	if first != "everyone" {
		t.Fatal("another user's settings changed")
	}
	// Once the recipient has replied, first-contact restrictions do not break the dialog.
	if _, _, err = f.store.Send(f.ctx, f.conv.ID, f.users[1].ID, "11111111-0000-4000-8000-000000000004", "reply"); err != nil {
		t.Fatal(err)
	}
	f.request(t, 1, "PUT", "/api/profile/privacy", `{"messages":"nobody","calls":"nobody","presence":"nobody","search":"nobody","birthday":"nobody"}`, 200)
	if _, _, err = f.store.Send(f.ctx, f.conv.ID, f.users[0].ID, "11111111-0000-4000-8000-000000000005", "follow up"); err != nil {
		t.Fatalf("existing reply conversation broken: %v", err)
	}
}
