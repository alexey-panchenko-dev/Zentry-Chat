package httpapi

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestDeliveryAndReadAreSeparate(t *testing.T) {
	f := newControlFixture(t)
	m, _, err := f.store.Send(f.ctx, f.conv.ID, f.users[0].ID, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaa81", "delivery test")
	if err != nil {
		t.Fatal(err)
	}
	state, err := f.store.ReadState(f.ctx, f.conv.ID, f.users[0].ID)
	if err != nil || state.PeerLastDeliveredID != "0" || state.PeerLastReadID != "0" {
		t.Fatalf("initial %+v %v", state, err)
	}
	f.request(t, -1, "GET", "/api/delivery", "", 401)
	inbox := f.request(t, 1, "GET", "/api/delivery", "", 200)
	var messages []struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(inbox["messages"], &messages); err != nil || len(messages) != 1 || messages[0].ID != m.ID {
		t.Fatalf("inbox %s %v", inbox, err)
	}
	state, _ = f.store.ReadState(f.ctx, f.conv.ID, f.users[0].ID)
	if state.PeerLastDeliveredID != "0" {
		t.Fatal("fetch incorrectly acknowledged")
	}
	payload := fmt.Sprintf(`{"message_ids":[%q]}`, m.ID)
	f.request(t, 2, "POST", "/api/delivery", payload, 200)
	state, _ = f.store.ReadState(f.ctx, f.conv.ID, f.users[0].ID)
	if state.PeerLastDeliveredID != "0" {
		t.Fatal("non-member acknowledged")
	}
	f.request(t, 1, "POST", "/api/delivery", payload, 200)
	state, _ = f.store.ReadState(f.ctx, f.conv.ID, f.users[0].ID)
	if state.PeerLastDeliveredID != m.ID || state.PeerLastReadID != "0" {
		t.Fatalf("delivered %+v", state)
	}
	f.request(t, 1, "POST", "/api/conversations/"+f.conv.ID+"/read", fmt.Sprintf(`{"last_message_id":%q}`, m.ID), 200)
	state, _ = f.store.ReadState(f.ctx, f.conv.ID, f.users[0].ID)
	if state.PeerLastReadID != m.ID {
		t.Fatalf("read %+v", state)
	}
	m2, _, err := f.store.Send(f.ctx, f.conv.ID, f.users[0].ID, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaa82", "not viewed")
	if err != nil {
		t.Fatal(err)
	}
	f.request(t, 1, "POST", "/api/conversations/"+f.conv.ID+"/controls", `{"action":"clear"}`, 200)
	state, _ = f.store.ReadState(f.ctx, f.conv.ID, f.users[0].ID)
	if state.PeerLastReadID == m2.ID {
		t.Fatal("clearing falsely marked read")
	}
	f.request(t, 1, "POST", "/api/delivery", `{"message_ids":["bad"]}`, 400)
}
