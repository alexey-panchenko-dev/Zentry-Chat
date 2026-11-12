package httpapi

import (
	"errors"
	"newsChat/api/internal/chat"
	"testing"
)

func TestGroupRolesAndOwnershipTransfer(t *testing.T) {
	f := newControlFixture(t)
	if _, err := f.store.CreateConversation(f.ctx, f.users[0].ID, f.users[2].ID); err != nil {
		t.Fatal(err)
	}
	g, _, err := f.store.CreateGroup(f.ctx, f.users[0].ID, "Roles", []string{f.users[1].ID, f.users[2].ID})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.db.Exec(f.ctx, "DELETE FROM group_chats WHERE id=$1", g.ID) })
	manage := func(actor, target int, action, value string) error {
		return f.store.ManageGroup(f.ctx, g.ID, f.users[actor].ID, f.users[target].ID, action, value)
	}
	if err = manage(1, 1, "name", "unauthorized"); !errors.Is(err, chat.ErrForbidden) {
		t.Fatalf("member rename: %v", err)
	}
	if err = manage(0, 1, "role", "admin"); err != nil {
		t.Fatal(err)
	}
	if err = manage(1, 1, "name", "Admin rename"); err != nil {
		t.Fatal(err)
	}
	if err = manage(1, 2, "role", "admin"); !errors.Is(err, chat.ErrForbidden) {
		t.Fatalf("admin promotion: %v", err)
	}
	if err = manage(0, 0, "remove", ""); !errors.Is(err, chat.ErrForbidden) {
		t.Fatalf("owner removed: %v", err)
	}
	if err = manage(0, 2, "role", "admin"); err != nil {
		t.Fatal(err)
	}
	if err = manage(1, 2, "remove", ""); !errors.Is(err, chat.ErrForbidden) {
		t.Fatalf("admin removed another admin: %v", err)
	}
	message, _, _, err := f.store.SendGroupMessage(f.ctx, g.ID, f.users[0].ID, "14111111-0000-4000-8000-000000000001", "pin")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.SetGroupPin(f.ctx, g.ID, message.ID, f.users[1].ID, true); err != nil {
		t.Fatalf("admin pin: %v", err)
	}
	if err = manage(0, 1, "owner", ""); err != nil {
		t.Fatal(err)
	}
	old, err := f.store.Group(f.ctx, g.ID, f.users[0].ID)
	if err != nil || old.OwnerID != f.users[1].ID || old.Role != "admin" {
		t.Fatalf("transfer failed: %+v %v", old, err)
	}
	if err = manage(0, 1, "remove", ""); !errors.Is(err, chat.ErrForbidden) {
		t.Fatalf("new owner removed: %v", err)
	}
	if err = manage(0, 0, "remove", ""); err != nil {
		t.Fatalf("former owner cannot leave: %v", err)
	}
	if _, err = f.store.Group(f.ctx, g.ID, f.users[0].ID); !errors.Is(err, chat.ErrNotFound) {
		t.Fatalf("departed member retains access: %v", err)
	}
	notes, err := f.store.Notes(f.ctx, f.users[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.ManageGroup(f.ctx, notes.ID, f.users[1].ID, f.users[1].ID, "remove", ""); !errors.Is(err, chat.ErrForbidden) {
		t.Fatalf("notes leave permitted: %v", err)
	}
}
