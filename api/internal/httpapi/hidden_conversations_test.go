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
	removed := f.request(t, 0, "POST", path, `{"action":"remove_contact"}`, 200)
	if string(removed["hidden"]) != "true" || string(removed["cleared_through"]) != `"0"` {
		t.Fatalf("remove state%s", removed)
	}
	state, err := f.store.Controls(f.ctx, f.conv.ID, f.users[0].ID)
	if err != nil || !state.Hidden {
		t.Fatalf("hidden state%+v %v", state, err)
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
