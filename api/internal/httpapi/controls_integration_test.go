package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/calls"
	"newsChat/api/internal/chat"
	"newsChat/api/internal/database"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type controlFixture struct {
	ctx       context.Context
	db        *pgxpool.Pool
	store     *chat.Store
	calls     *calls.Store
	users     []auth.User
	tokens    []string
	conv      chat.Conversation
	server    *httptest.Server
	publisher *controlPublisher
}
type controlEvent struct {
	channel, event string
	data           any
}
type controlPublisher struct {
	mu     sync.Mutex
	events []controlEvent
}

func (p *controlPublisher) Enabled() bool                         { return true }
func (p *controlPublisher) PublicConfig() map[string]any          { return map[string]any{"enabled": true} }
func (p *controlPublisher) Authorize(s, c string) ([]byte, error) { return []byte(`{}`), nil }
func (p *controlPublisher) Publish(c, e string, d any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, controlEvent{c, e, d})
	return nil
}
func newControlFixture(t *testing.T) *controlFixture {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	f := &controlFixture{ctx: context.Background(), publisher: &controlPublisher{}}
	var err error
	f.db, err = database.Open(f.ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(f.ctx, f.db); err != nil {
		t.Fatal(err)
	}
	s := auth.New(f.db)
	for i := 0; i < 3; i++ {
		email := fmt.Sprintf("controls-%d-%d@example.com", time.Now().UnixNano(), i)
		u, e := s.Register(f.ctx, "Controls", email, "password-12345")
		if e != nil {
			t.Fatal(e)
		}
		f.users = append(f.users, u)
		_, token, _, e := s.Login(f.ctx, email, "password-12345")
		if e != nil {
			t.Fatal(e)
		}
		f.tokens = append(f.tokens, token)
	}
	f.store = chat.New(f.db)
	f.calls = calls.New(f.db)
	f.conv, err = f.store.CreateConversation(f.ctx, f.users[0].ID, f.users[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	f.server = httptest.NewServer(newRouter(f.db, f.publisher))
	t.Cleanup(func() {
		f.server.Close()
		for _, u := range f.users {
			f.db.Exec(f.ctx, "DELETE FROM users WHERE id=$1", u.ID)
		}
		f.db.Close()
	})
	return f
}
func (f *controlFixture) request(t *testing.T, actor int, method, path, body string, status int) map[string]json.RawMessage {
	t.Helper()
	req, _ := http.NewRequest(method, f.server.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if actor >= 0 {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.tokens[actor]})
	}
	res, e := f.server.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	var data map[string]json.RawMessage
	json.NewDecoder(res.Body).Decode(&data)
	if res.StatusCode != status {
		t.Fatalf("%s %s actor%d got%d want%d: %s", method, path, actor, res.StatusCode, status, data)
	}
	return data
}
func TestConversationClearIsolation(t *testing.T) {
	f := newControlFixture(t)
	path := "/api/conversations/" + f.conv.ID
	controls := path + "/controls"
	f.request(t, -1, "GET", controls, "", 401)
	f.request(t, 2, "GET", controls, "", 404)
	f.request(t, 2, "POST", controls, `{"action":"clear"}`, 404)
	f.request(t, 0, "POST", controls, `{"action":"invalid"}`, 400)
	m, _, err := f.store.Send(f.ctx, f.conv.ID, f.users[1].ID, "11111111-1111-4111-8111-111111111111", "old")
	if err != nil {
		t.Fatal(err)
	}
	voice, _, err := f.store.SendVoice(f.ctx, f.conv.ID, f.users[1].ID, "11111111-1111-4111-8111-111111111112", "audio/webm", audioFixture(t, "silence.webm"), 200)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := f.store.ReadState(f.ctx, f.conv.ID, f.users[0].ID)
	if before.UnreadCount != 2 {
		t.Fatalf("unread beforeclear%+v", before)
	}
	start := len(f.publisher.events)
	cleared := f.request(t, 0, "POST", controls, `{"action":"clear"}`, 200)
	if string(cleared["cleared_through"]) != fmt.Sprintf("%q", voice.ID) {
		t.Fatalf("cursor%s", cleared)
	}
	for _, e := range f.publisher.events[start:] {
		if e.event == "conversation.updated" && e.channel != "private-user-"+f.users[0].ID {
			t.Fatalf("clear notified peer %+v", e)
		}
	}
	history, err := f.store.Messages(f.ctx, f.conv.ID, f.users[0].ID, 0, 20)
	if err != nil || len(history) != 0 {
		t.Fatalf("clear history%v %v", history, err)
	}
	peer, err := f.store.Messages(f.ctx, f.conv.ID, f.users[1].ID, 0, 20)
	if err != nil || len(peer) != 2 {
		t.Fatalf("peerhistory%v %v", peer, err)
	}
	f.request(t, 0, "GET", "/api/messages/"+voice.ID+"/audio", "", 404)
	f.request(t, 1, "GET", "/api/messages/"+voice.ID+"/audio", "", 200)
	state, _ := f.store.ReadState(f.ctx, f.conv.ID, f.users[0].ID)
	if state.UnreadCount != 0 || state.LastMessageID != "0" {
		t.Fatalf("cleared readstate%+v", state)
	}
	unread, _ := f.store.UnreadChats(f.ctx, f.users[0].ID)
	if unread != 0 {
		t.Fatalf("cleared unread%d", unread)
	}
	list, _ := f.store.Conversations(f.ctx, f.users[0].ID, 0, 20)
	if list[0].UnreadCount != 0 {
		t.Fatal("list unreadresurrected")
	}
	f.request(t, 0, "POST", path+"/read", fmt.Sprintf(`{"last_message_id":%q}`, m.ID), 404)
	fresh, _, err := f.store.Send(f.ctx, f.conv.ID, f.users[1].ID, "11111111-1111-4111-8111-111111111113", "fresh")
	if err != nil {
		t.Fatal(err)
	}
	history, err = f.store.Messages(f.ctx, f.conv.ID, f.users[0].ID, 0, 1)
	if err != nil || len(history) != 1 || history[0].ID != fresh.ID {
		t.Fatalf("newmessagehidden%v %v", history, err)
	}
	history, err = f.store.Messages(f.ctx, f.conv.ID, f.users[0].ID, mustMessageInt(t, fresh.ID), 20)
	if err != nil || len(history) != 0 {
		t.Fatal("pagination leaked clearedhistory")
	}
	state, _ = f.store.ReadState(f.ctx, f.conv.ID, f.users[0].ID)
	if state.UnreadCount != 1 {
		t.Fatalf("newunread%+v", state)
	}
}
func TestConversationBlockAndCallTermination(t *testing.T) {
	f := newControlFixture(t)
	path := "/api/conversations/" + f.conv.ID
	call, _, err := f.calls.Create(f.ctx, f.conv.ID, f.users[0].ID, "22222222-2222-4222-8222-222222222221")
	if err != nil {
		t.Fatal(err)
	}
	blocked := f.request(t, 0, "POST", path+"/controls", `{"action":"block"}`, 200)
	if string(blocked["blocked_by_me"]) != "true" || string(blocked["blocked_me"]) != "false" {
		t.Fatalf("wrong blockdirection%s", blocked)
	}
	other := f.request(t, 1, "GET", path+"/controls", "", 200)
	if string(other["blocked_me"]) != "true" {
		t.Fatal("wrongpeerdirection")
	}
	for _, actor := range []int{0, 1} {
		body := `{"client_message_id":"22222222-2222-4222-8222-222222222222","body":"blocked"}`
		response := f.request(t, actor, "POST", path+"/messages", body, 403)
		if string(response["error"]) != `"contact_blocked"` {
			t.Fatalf("wrongblockerror%s", response)
		}
		_, _, err = f.store.SendVoice(f.ctx, f.conv.ID, f.users[actor].ID, "22222222-2222-4222-8222-222222222223", "audio/webm", audioFixture(t, "silence.webm"), 200)
		if !errors.Is(err, chat.ErrBlocked) {
			t.Fatalf("voice notblocked%v", err)
		}
		response = f.request(t, actor, "POST", path+"/calls", `{"client_call_id":"22222222-2222-4222-8222-222222222224"}`, 403)
		if string(response["error"]) != `"contact_blocked"` {
			t.Fatalf("wrongcallerror%s", response)
		}
	}
	terminated, err := f.calls.Get(f.ctx, call.ID, f.users[0].ID)
	if err != nil || terminated.Status != "cancelled" {
		t.Fatalf("ringing notcancelled%+v %v", terminated, err)
	}
	f.request(t, 1, "POST", "/api/calls/"+call.ID+"/actions", `{"action":"accept"}`, 403)
	f.request(t, 1, "POST", path+"/controls", `{"action":"unblock"}`, 200)
	still := f.request(t, 1, "GET", path+"/controls", "", 200)
	if string(still["blocked_me"]) != "true" {
		t.Fatal("peerunblockedowner")
	}
	f.request(t, 0, "POST", path+"/controls", `{"action":"unblock"}`, 200)
	accepted, _, err := f.calls.Create(f.ctx, f.conv.ID, f.users[1].ID, "22222222-2222-4222-8222-222222222225")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.calls.Transition(f.ctx, accepted.ID, f.users[0].ID, "accept"); err != nil {
		t.Fatal(err)
	}
	f.request(t, 1, "POST", path+"/controls", `{"action":"block"}`, 200)
	ended, err := f.calls.Get(f.ctx, accepted.ID, f.users[1].ID)
	if err != nil || ended.Status != "ended" {
		t.Fatalf("active notended%+v %v", ended, err)
	}
	active, _ := f.calls.Active(f.ctx, f.users[0].ID)
	if len(active) != 0 {
		t.Fatal("activecall slot notfreed")
	}
	events := f.publisher.events
	updates := map[string]bool{}
	callUpdates := 0
	for _, e := range events {
		if e.event == "conversation.updated" {
			updates[e.channel] = true
		}
		if e.event == "call.updated" {
			callUpdates++
		}
	}
	if !updates["private-user-"+f.users[0].ID] || !updates["private-user-"+f.users[1].ID] || callUpdates < 4 {
		t.Fatalf("missingcontrol/call notifications%v %d", updates, callUpdates)
	}
}

func mustMessageInt(t *testing.T, s string) int64 {
	t.Helper()
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil {
		t.Fatal(e)
	}
	return n
}

func TestClearSendSerializationAndClearedRetry(t *testing.T) {
	f := newControlFixture(t)
	old, _, err := f.store.Send(f.ctx, f.conv.ID, f.users[0].ID, "33333333-3333-4333-8333-333333333330", "old own message")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
	defer cancel()
	start := make(chan struct{})
	fail := make(chan error, 17)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, _, e := f.store.Send(ctx, f.conv.ID, f.users[1].ID, fmt.Sprintf("33333333-3333-4333-8333-%012d", i+1), "race")
			fail <- e
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		_, e := f.store.ChangeControls(ctx, f.conv.ID, f.users[0].ID, "clear")
		fail <- e
	}()
	close(start)
	wg.Wait()
	close(fail)
	for e := range fail {
		if e != nil {
			t.Fatal(e)
		}
	}
	state, e := f.store.Controls(ctx, f.conv.ID, f.users[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	cursor := mustMessageInt(t, state.ClearedThrough)
	history, e := f.store.Messages(ctx, f.conv.ID, f.users[0].ID, 0, 100)
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range history {
		if mustMessageInt(t, m.ID) <= cursor {
			t.Fatalf("racing message leaked belowcursor %+v", m)
		}
	}
	peer, e := f.store.Messages(ctx, f.conv.ID, f.users[1].ID, 0, 100)
	if e != nil || len(peer) != 17 {
		t.Fatalf("peer lost racinghistory count%d err%v", len(peer), e)
	}
	_, _, err = f.store.Send(ctx, f.conv.ID, f.users[0].ID, old.ClientMessageID, old.Body)
	if !errors.Is(err, chat.ErrNotFound) {
		t.Fatalf("cleared UUID replay mustnotresurrect %v", err)
	}
	fresh, _, err := f.store.Send(ctx, f.conv.ID, f.users[1].ID, "33333333-3333-4333-8333-333333333331", "new afterclear")
	if err != nil || mustMessageInt(t, fresh.ID) <= cursor {
		t.Fatalf("new send missing%v %v", fresh, err)
	}
}
func TestBlockSendAndCallAcceptSerialization(t *testing.T) {
	f := newControlFixture(t)
	ctx, cancel := context.WithTimeout(f.ctx, 20*time.Second)
	defer cancel()
	start := make(chan struct{})
	fail := make(chan error, 17)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, _, e := f.store.Send(ctx, f.conv.ID, f.users[i%2].ID, fmt.Sprintf("44444444-4444-4444-8444-%012d", i+1), "race")
			if errors.Is(e, chat.ErrBlocked) {
				e = nil
			}
			fail <- e
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		_, e := f.store.ChangeControls(ctx, f.conv.ID, f.users[0].ID, "block")
		fail <- e
	}()
	close(start)
	wg.Wait()
	close(fail)
	for e := range fail {
		if e != nil {
			t.Fatal(e)
		}
	}
	if _, _, e := f.store.Send(ctx, f.conv.ID, f.users[1].ID, "44444444-4444-4444-8444-444444444444", "after block"); !errors.Is(e, chat.ErrBlocked) {
		t.Fatalf("send afterblock%v", e)
	}
	for i := 0; i < 8; i++ {
		if _, e := f.store.ChangeControls(ctx, f.conv.ID, f.users[0].ID, "unblock"); e != nil {
			t.Fatal(e)
		}
		call, _, e := f.calls.Create(ctx, f.conv.ID, f.users[0].ID, fmt.Sprintf("55555555-5555-4555-8555-%012d", i+1))
		if e != nil {
			t.Fatal(e)
		}
		begin := make(chan struct{})
		results := make(chan error, 2)
		go func() {
			<-begin
			_, e := f.calls.Transition(ctx, call.ID, f.users[1].ID, "accept")
			if errors.Is(e, calls.ErrBlocked) {
				e = nil
			}
			results <- e
		}()
		go func() { <-begin; _, e := f.store.ChangeControls(ctx, f.conv.ID, f.users[0].ID, "block"); results <- e }()
		close(begin)
		for n := 0; n < 2; n++ {
			if e := <-results; e != nil {
				t.Fatal(e)
			}
		}
		ended, e := f.calls.Get(ctx, call.ID, f.users[0].ID)
		if e != nil || (ended.Status != "cancelled" && ended.Status != "ended") {
			t.Fatalf("accept/block race left activecall%+v %v", ended, e)
		}
	}
}

func TestBlockSerializesCallCreationSignalsAndConversationWrites(t *testing.T) {
	f := newControlFixture(t)
	ctx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
	defer cancel()
	for i := 0; i < 4; i++ {
		if _, err := f.store.ChangeControls(ctx, f.conv.ID, f.users[0].ID, "unblock"); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		results := make(chan error, 2)
		go func() {
			<-start
			_, _, e := f.calls.Create(ctx, f.conv.ID, f.users[1].ID, fmt.Sprintf("66666666-6666-4666-8666-%012d", i+1))
			if errors.Is(e, calls.ErrBlocked) {
				e = nil
			}
			results <- e
		}()
		go func() { <-start; _, e := f.store.ChangeControls(ctx, f.conv.ID, f.users[0].ID, "block"); results <- e }()
		close(start)
		for n := 0; n < 2; n++ {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		active, e := f.calls.Active(ctx, f.users[1].ID)
		if e != nil || len(active) != 0 {
			t.Fatalf("create/block left activecall%+v %v", active, e)
		}
		if _, e = f.store.ChangeControls(ctx, f.conv.ID, f.users[0].ID, "unblock"); e != nil {
			t.Fatal(e)
		}
		c, _, e := f.calls.Create(ctx, f.conv.ID, f.users[0].ID, fmt.Sprintf("77777777-7777-4777-8777-%012d", i+1))
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.calls.Transition(ctx, c.ID, f.users[1].ID, "accept"); e != nil {
			t.Fatal(e)
		}
		begin := make(chan struct{})
		done := make(chan error, 3)
		go func() {
			<-begin
			_, _, e := f.calls.SendSignal(ctx, c.ID, f.users[0].ID, fmt.Sprintf("88888888-8888-4888-8888-%012d", i+1), "offer", json.RawMessage(`{"type":"offer","sdp":"v=0\r\n"}`))
			if errors.Is(e, calls.ErrBlocked) {
				e = nil
			}
			done <- e
		}()
		go func() { <-begin; _, e := f.store.CreateConversation(ctx, f.users[1].ID, f.users[0].ID); done <- e }()
		go func() { <-begin; _, e := f.store.ChangeControls(ctx, f.conv.ID, f.users[0].ID, "block"); done <- e }()
		close(begin)
		for n := 0; n < 3; n++ {
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		}
		var signals int
		if e = f.db.QueryRow(ctx, "SELECT count(*) FROM call_signals WHERE call_id=$1", c.ID).Scan(&signals); e != nil || signals != 0 {
			t.Fatalf("block retainedcall signaling%d %v", signals, e)
		}
	}
}
