package httpapi

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"newsChat/api/internal/chat"
	"testing"
)

func TestPrivateDeleteAndRemoveContact(t *testing.T) {
	f := newControlFixture(t)
	path := "/api/conversations/" + f.conv.ID + "/controls"
	old, _, err := f.store.Send(f.ctx, f.conv.ID, f.users[1].ID, "99999999-9999-4999-8999-999999999991", "preserved")
	if err != nil {
		t.Fatal(err)
	}
	voice, _, err := f.store.SendVoice(f.ctx, f.conv.ID, f.users[1].ID, "99999999-9999-4999-8999-999999999992", "audio/webm", audioFixture(t, "silence.webm"), 200)
	if err != nil {
		t.Fatal(err)
	}
	f.request(t, -1, "POST", path, `{"action":"delete_chat"}`, 401)
	f.request(t, 2, "POST", path, `{"action":"remove_contact"}`, 404)
	activeCall, _, err := f.calls.Create(f.ctx, f.conv.ID, f.users[0].ID, "99999999-9999-4999-8999-999999999990")
	if err != nil {
		t.Fatal(err)
	}
	offset := len(f.publisher.events)
	if _, err = f.db.Exec(f.ctx, `INSERT INTO contact_aliases(owner_id,peer_id,alias) VALUES($1,$2,'My contact'),($2,$1,'Their contact')`, f.users[0].ID, f.users[1].ID); err != nil {
		t.Fatal(err)
	}
	removed := f.request(t, 0, "POST", path, `{"action":"remove_contact"}`, 200)
	if string(removed["hidden"]) != "true" || string(removed["cleared_through"]) != `"0"` {
		t.Fatalf("remove state%s", removed)
	}
	state, err := f.store.Controls(f.ctx, f.conv.ID, f.users[0].ID)
	if err != nil || !state.Hidden {
		t.Fatalf("hidden state%+v %v", state, err)
	}
	var aliasCount int
	if err = f.db.QueryRow(f.ctx, `SELECT count(*) FROM contact_aliases WHERE owner_id=$1 AND peer_id=$2`, f.users[0].ID, f.users[1].ID).Scan(&aliasCount); err != nil || aliasCount != 0 {
		t.Fatalf("removed contact alias remained: %d %v", aliasCount, err)
	}
	if err = f.db.QueryRow(f.ctx, `SELECT count(*) FROM contact_aliases WHERE owner_id=$1 AND peer_id=$2`, f.users[1].ID, f.users[0].ID).Scan(&aliasCount); err != nil || aliasCount != 1 {
		t.Fatalf("peer contact alias changed: %d %v", aliasCount, err)
	}
	list, err := f.store.Conversations(f.ctx, f.users[0].ID, 0, 20)
	if err != nil || len(list) != 0 {
		t.Fatalf("hidden list%v %v", list, err)
	}
	unread, err := f.store.UnreadChats(f.ctx, f.users[0].ID)
	if err != nil || unread != 0 {
		t.Fatalf("hidden unread%d %v", unread, err)
	}
	history, err := f.store.Messages(f.ctx, f.conv.ID, f.users[0].ID, 0, 20)
	if err != nil || len(history) != 2 {
		t.Fatalf("remove lost history%v %v", history, err)
	}
	if _, _, err = f.store.Audio(f.ctx, voice.ID, f.users[0].ID); err != nil {
		t.Fatal("remove lost audio", err)
	}
	for _, e := range f.publisher.events[offset:] {
		if e.channel != "private-user-"+f.users[0].ID {
			t.Fatalf("private remove notified peer %+v", e)
		}
	}
	ongoing, err := f.calls.Get(f.ctx, activeCall.ID, f.users[0].ID)
	if err != nil || ongoing.Status != "ringing" {
		t.Fatalf("remove altered call%+v %v", ongoing, err)
	}
	peer, err := f.store.Conversations(f.ctx, f.users[1].ID, 0, 20)
	if err != nil || len(peer) != 1 {
		t.Fatalf("peer list lost%v %v", peer, err)
	}
	// A duplicate existing message does not fire the INSERT trigger or reveal a hidden chat.
	replay, created, err := f.store.Send(f.ctx, f.conv.ID, f.users[1].ID, old.ClientMessageID, old.Body)
	if err != nil || created || replay.ID != old.ID {
		t.Fatalf("retrychanged%+v %v", replay, err)
	}
	state, _ = f.store.Controls(f.ctx, f.conv.ID, f.users[0].ID)
	if !state.Hidden {
		t.Fatal("oldretry reopenedhidden chat")
	}
	f.request(t, 1, "POST", path, `{"action":"remove_contact"}`, 200)
	if _, err = f.store.CreateConversation(f.ctx, f.users[0].ID, f.users[1].ID); err != nil {
		t.Fatal(err)
	}
	actor, _ := f.store.Controls(f.ctx, f.conv.ID, f.users[0].ID)
	other, _ := f.store.Controls(f.ctx, f.conv.ID, f.users[1].ID)
	if actor.Hidden || !other.Hidden {
		t.Fatalf("searchreopen mustonlyunhideactor actor%+v peer%+v", actor, other)
	}
	deleted := f.request(t, 0, "POST", path, `{"action":"delete_chat"}`, 200)
	if string(deleted["hidden"]) != "true" || string(deleted["cleared_through"]) != fmt.Sprintf("%q", voice.ID) {
		t.Fatalf("delete state%s", deleted)
	}
	history, err = f.store.Messages(f.ctx, f.conv.ID, f.users[0].ID, 0, 20)
	if err != nil || len(history) != 0 {
		t.Fatal("delete didnotclearactor")
	}
	if _, _, err = f.store.Audio(f.ctx, voice.ID, f.users[0].ID); !errors.Is(err, chat.ErrNotFound) {
		t.Fatalf("delete audio nothidden%v", err)
	}
	history, err = f.store.Messages(f.ctx, f.conv.ID, f.users[1].ID, 0, 20)
	if err != nil || len(history) != 2 {
		t.Fatalf("delete lost peerhistory%v %v", history, err)
	}
	fresh, _, err := f.store.Send(f.ctx, f.conv.ID, f.users[1].ID, "99999999-9999-4999-8999-999999999993", "incoming")
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range f.users[:2] {
		state, _ = f.store.Controls(f.ctx, f.conv.ID, u.ID)
		if state.Hidden {
			t.Fatal("fresh incoming didnotunhideboth")
		}
	}
	history, _ = f.store.Messages(f.ctx, f.conv.ID, f.users[0].ID, 0, 20)
	if len(history) != 1 || history[0].ID != fresh.ID {
		t.Fatal("old clearedhistory resurrected")
	}
	f.request(t, 0, "POST", path, `{"action":"block"}`, 200)
	f.request(t, 0, "POST", path, `{"action":"delete_chat"}`, 200)
	_, _, err = f.store.Send(f.ctx, f.conv.ID, f.users[1].ID, "99999999-9999-4999-8999-999999999994", "blocked")
	if !errors.Is(err, chat.ErrBlocked) {
		t.Fatalf("block notenforced%v", err)
	}
	state, _ = f.store.Controls(f.ctx, f.conv.ID, f.users[0].ID)
	if !state.Hidden {
		t.Fatal("blocked send reopenedchat")
	}
}

func TestNewMessageKindsReopenHiddenConversations(t *testing.T) {
	f := newControlFixture(t)
	for i, kind := range []string{"text", "voice", "image", "system"} {
		for _, u := range f.users[:2] {
			if _, err := f.store.ChangeControls(f.ctx, f.conv.ID, u.ID, "remove_contact"); err != nil {
				t.Fatal(err)
			}
		}
		id := fmt.Sprintf("abcdefab-abcd-4abc-8abc-%012d", i+1)
		var err error
		switch kind {
		case "text":
			_, _, err = f.store.Send(f.ctx, f.conv.ID, f.users[1].ID, id, "new text")
		case "voice":
			_, _, err = f.store.SendVoice(f.ctx, f.conv.ID, f.users[1].ID, id, "audio/webm", audioFixture(t, "silence.webm"), 200)
		case "image":
			var encoded bytes.Buffer
			if err = jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 1, 1)), nil); err == nil {
				_, _, err = f.store.SendImage(f.ctx, f.conv.ID, f.users[1].ID, id, "", encoded.Bytes())
			}
		case "system":
			tx, e := f.db.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			_, err = tx.Exec(f.ctx, `SELECT id FROM conversations WHERE id=$1 FOR UPDATE`, f.conv.ID)
			if err == nil {
				_, err = tx.Exec(f.ctx, `INSERT INTO messages(conversation_id,sender_id,client_message_id,body,kind) VALUES($1,NULL,gen_random_uuid(),'Вы стали друзьями!','system')`, f.conv.ID)
			}
			if err == nil {
				err = tx.Commit(f.ctx)
			} else {
				tx.Rollback(f.ctx)
			}
		}
		if err != nil {
			t.Fatalf("%s insert: %v", kind, err)
		}
		for _, u := range f.users[:2] {
			state, e := f.store.Controls(f.ctx, f.conv.ID, u.ID)
			if e != nil || state.Hidden {
				t.Fatalf("%s didnotunhideboth: %+v %v", kind, state, e)
			}
		}
	}
}

func TestDeleteConversationForEveryoneClearsBothMembers(t *testing.T) {
	f := newControlFixture(t)
	for i, member := range f.users[:2] {
		id := fmt.Sprintf("eaffffff-ffff-4fff-8fff-%012d", i+1)
		if _, _, err := f.store.Send(f.ctx, f.conv.ID, member.ID, id, "visible before delete"); err != nil {
			t.Fatal(err)
		}
	}
	path := "/api/conversations/" + f.conv.ID + "/controls"
	f.request(t, 2, "POST", path, `{"action":"delete_everyone"}`, 404)
	offset := len(f.publisher.events)
	result := f.request(t, 0, "POST", path, `{"action":"delete_everyone"}`, 200)
	if string(result["hidden"]) != "true" {
		t.Fatalf("delete was not confirmed: %s", result)
	}
	for _, member := range f.users[:2] {
		state, err := f.store.Controls(f.ctx, f.conv.ID, member.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !state.Hidden || state.ClearedThrough == "0" {
			t.Fatalf("member retained a deleted chat: %+v", state)
		}
		history, err := f.store.Messages(f.ctx, f.conv.ID, member.ID, 0, 20)
		if err != nil || len(history) != 0 {
			t.Fatalf("deleted history remained for member: %v %v", history, err)
		}
		list, err := f.store.Conversations(f.ctx, member.ID, 0, 20)
		if err != nil || len(list) != 0 {
			t.Fatalf("deleted chat remained in list: %v %v", list, err)
		}
		unread, err := f.store.UnreadChats(f.ctx, member.ID)
		if err != nil || unread != 0 {
			t.Fatalf("deleted chat remained unread: %d %v", unread, err)
		}
	}
	if len(f.publisher.events[offset:]) != 2 {
		t.Fatal("both participants must be notified")
	}
	for i, event := range f.publisher.events[offset:] {
		if event.channel != "private-user-"+f.users[i].ID || event.event != "conversation.updated" {
			t.Fatalf("incorrect deletion event: %+v", event)
		}
	}
	// A lost-response retry cannot resurrect old history or reopen either list.
	_, _, err := f.store.Send(f.ctx, f.conv.ID, f.users[0].ID, "eaffffff-ffff-4fff-8fff-000000000001", "visible before delete")
	if !errors.Is(err, chat.ErrNotFound) {
		t.Fatalf("old retry returned cleared message: %v", err)
	}
	fresh, _, err := f.store.Send(f.ctx, f.conv.ID, f.users[1].ID, "eaffffff-ffff-4fff-8fff-000000000003", "new conversation")
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range f.users[:2] {
		list, err := f.store.Conversations(f.ctx, member.ID, 0, 20)
		if err != nil || len(list) != 1 {
			t.Fatalf("fresh message did not reopen chat: %v %v", list, err)
		}
		history, err := f.store.Messages(f.ctx, f.conv.ID, member.ID, 0, 20)
		if err != nil || len(history) != 1 || history[0].ID != fresh.ID {
			t.Fatalf("cleared history reappeared: %v %v", history, err)
		}
	}
}
