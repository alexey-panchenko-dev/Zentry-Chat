package httpapi

import (
	"errors"
	"newsChat/api/internal/chat"
	"testing"
)

func TestBlockedPeerCannotCreateMatchSystemMessage(t *testing.T) {
	f := newControlFixture(t)
	age := 25
	for _, u := range f.users[:2] {
		if _, err := f.store.UpdateProfile(f.ctx, u.ID, u.Name, &age, "A test profile", "Test"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.store.Vote(f.ctx, f.users[0].ID, f.users[1].ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.ChangeControls(f.ctx, f.conv.ID, f.users[0].ID, "block"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Vote(f.ctx, f.users[1].ID, f.users[0].ID, true); !errors.Is(err, chat.ErrBlocked) {
		t.Fatalf("blocked discovery vote: %v", err)
	}
	var count int
	if err := f.db.QueryRow(f.ctx, "SELECT count(*) FROM messages WHERE conversation_id=$1", f.conv.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("blocked match created %d messages", count)
	}
	if _, err := f.store.ChangeControls(f.ctx, f.conv.ID, f.users[0].ID, "unblock"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Vote(f.ctx, f.users[1].ID, f.users[0].ID, true); err != nil {
		t.Fatal(err)
	}
}
