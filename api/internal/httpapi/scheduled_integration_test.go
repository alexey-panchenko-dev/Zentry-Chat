package httpapi

import (
	"encoding/json"
	"fmt"
	"newsChat/api/internal/chat"
	"sync"
	"testing"
	"time"
)

func TestScheduledGroupDeliveryRechecksMembershipAndSurvivesStoreRestart(t *testing.T) {
	f := newControlFixture(t)
	if _, err := f.store.CreateConversation(f.ctx, f.users[0].ID, f.users[2].ID); err != nil {
		t.Fatal(err)
	}
	group, _, err := f.store.CreateGroup(f.ctx, f.users[0].ID, "Scheduled group", []string{f.users[1].ID, f.users[2].ID})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.db.Exec(f.ctx, `DELETE FROM group_chats WHERE id=$1`, group.ID) })
	at := time.Now().Add(time.Hour)
	first, err := f.store.Schedule(f.ctx, f.users[0].ID, "group", group.ID, "a2234567-0000-4000-8000-000000000001", "persistent delivery", at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.Schedule(f.ctx, f.users[0].ID, "group", group.ID, first.ClientID, first.Body, at); err != nil {
		t.Fatal("nanosecond retry", err)
	}
	second, err := f.store.Schedule(f.ctx, f.users[1].ID, "group", group.ID, "a2234567-0000-4000-8000-000000000002", "removed sender", at)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.ManageGroup(f.ctx, group.ID, f.users[0].ID, f.users[1].ID, "remove", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(f.ctx, `UPDATE scheduled_messages SET send_at=now()-interval '1 second' WHERE id IN ($1,$2)`, first.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	// A new store has no browser state or original worker memory.
	restarted := chat.New(f.db)
	for index := 0; index < 2; index++ {
		if _, found, dispatchErr := restarted.DispatchScheduled(f.ctx); dispatchErr != nil || !found {
			t.Fatalf("dispatch: %v %v", found, dispatchErr)
		}
	}
	var sent, failed int
	if err = f.db.QueryRow(f.ctx, `SELECT count(*) FILTER (WHERE status='sent'),count(*) FILTER (WHERE status='failed') FROM scheduled_messages WHERE id IN ($1,$2)`, first.ID, second.ID).Scan(&sent, &failed); err != nil || sent != 1 || failed != 1 {
		t.Fatalf("delivery statuses: %d %d %v", sent, failed, err)
	}
}

func TestScheduledAuthorizationEditingCancellationAndAtomicDelivery(t *testing.T) {
	f := newControlFixture(t)
	at := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	create := func(client, body string) chat.ScheduledMessage {
		t.Helper()
		payload := fmt.Sprintf(`{"scope":"personal","chat_id":%q,"client_message_id":%q,"body":%q,"send_at":%q}`, f.conv.ID, client, body, at.Format(time.RFC3339))
		data := f.request(t, 0, "POST", "/api/scheduled", payload, 201)
		var item chat.ScheduledMessage
		if err := json.Unmarshal(data["scheduled"], &item); err != nil {
			t.Fatal(err)
		}
		return item
	}
	f.request(t, -1, "GET", "/api/scheduled?scope=personal&chat_id="+f.conv.ID, "", 401)
	one := create("a1234567-0000-4000-8000-000000000001", "scheduled text")
	replay := create("a1234567-0000-4000-8000-000000000001", "scheduled text")
	if replay.ID != one.ID {
		t.Fatal("schedule retry duplicated job")
	}
	f.request(t, 1, "DELETE", "/api/scheduled/"+one.ID, "", 404)
	f.request(t, 0, "PUT", "/api/scheduled/"+one.ID, fmt.Sprintf(`{"body":"edited text","send_at":%q}`, at.Add(time.Hour).Format(time.RFC3339)), 204)
	cancelled := create("a1234567-0000-4000-8000-000000000002", "cancelled text")
	f.request(t, 0, "DELETE", "/api/scheduled/"+cancelled.ID, "", 204)
	if _, err := f.db.Exec(f.ctx, `UPDATE scheduled_messages SET send_at=now()-interval '1 second' WHERE sender_id=$1`, f.users[0].ID); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, _, err := f.store.DispatchScheduled(f.ctx)
			if err != nil {
				errs <- err
			}
		}()
	}
	workers.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	var count int
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM messages WHERE conversation_id=$1 AND body='edited text'`, f.conv.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("delivery count %d %v", count, err)
	}
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM messages WHERE conversation_id=$1 AND body='cancelled text'`, f.conv.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("cancelled message delivered")
	}
	f.request(t, 0, "DELETE", "/api/scheduled/"+one.ID, "", 409)
	blocked := create("a1234567-0000-4000-8000-000000000003", "blocked text")
	f.request(t, 1, "POST", "/api/conversations/"+f.conv.ID+"/controls", `{"action":"block"}`, 200)
	if _, err := f.db.Exec(f.ctx, `UPDATE scheduled_messages SET send_at=now()-interval '1 second' WHERE id=$1`, blocked.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.store.DispatchScheduled(f.ctx); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := f.db.QueryRow(f.ctx, `SELECT status FROM scheduled_messages WHERE id=$1`, blocked.ID).Scan(&state); err != nil || state != "failed" {
		t.Fatalf("blocked delivery: %s %v", state, err)
	}
}
