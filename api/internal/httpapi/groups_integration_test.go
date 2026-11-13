package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"newsChat/api/internal/auth"
	"strings"
	"testing"
	"time"
)

func TestGroupsRequireExistingContactsAndShareMessages(t *testing.T) {
	f := newControlFixture(t)
	svc := auth.New(f.db)
	email := fmt.Sprintf("group-%d@example.com", time.Now().UnixNano())
	fourth, err := svc.Register(f.ctx, "Fourth", email, "password-12345")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.db.Exec(f.ctx, "DELETE FROM users WHERE id=$1", fourth.ID) })
	input := fmt.Sprintf(`{"name":"Друзья","member_ids":[%q,%q,%q]}`, f.users[1].ID, f.users[2].ID, fourth.ID)
	f.request(t, 0, "POST", "/api/groups", input, 400)
	for _, person := range []string{f.users[2].ID, fourth.ID} {
		if _, err := f.store.CreateConversation(f.ctx, f.users[0].ID, person); err != nil {
			t.Fatal(err)
		}
	}
	contacts := f.request(t, 0, "GET", "/api/groups/contacts", "", 200)
	if !strings.Contains(string(contacts["contacts"]), fourth.ID) {
		t.Fatal("eligible contact absent")
	}
	f.request(t, 0, "POST", "/api/groups", fmt.Sprintf(`{"name":"Друзья","member_ids":[%q,%q,%q]}`, f.users[1].ID, f.users[1].ID, f.users[2].ID), 400)
	created := f.request(t, 0, "POST", "/api/groups", input, 201)
	var group struct {
		ID          string `json:"id"`
		MemberCount int    `json:"member_count"`
	}
	if err = json.Unmarshal(created["group"], &group); err != nil || group.MemberCount != 4 {
		t.Fatalf("group %s %v", created, err)
	}
	outsiderEmail := fmt.Sprintf("outsider-%d@example.com", time.Now().UnixNano())
	outsider, err := svc.Register(f.ctx, "Outsider", outsiderEmail, "password-12345")
	if err != nil {
		t.Fatal(err)
	}
	f.users = append(f.users, outsider)
	_, outsiderToken, _, err := svc.Login(f.ctx, outsiderEmail, "password-12345")
	if err != nil {
		t.Fatal(err)
	}
	f.tokens = append(f.tokens, outsiderToken)
	t.Cleanup(func() {
		if _, err := f.db.Exec(f.ctx, "DELETE FROM group_chats WHERE id=$1", group.ID); err != nil {
			t.Error(err)
		}
	})
	path := "/api/groups/" + group.ID
	f.request(t, 1, "POST", path+"/typing", `{}`, 204)
	f.request(t, 3, "POST", path+"/typing", `{}`, 404)
	f.request(t, -1, "GET", path+"/messages", "", 401)
	f.request(t, 3, "GET", path+"/messages", "", 404)
	f.request(t, 3, "GET", path+"/members", "", 404)
	f.request(t, 3, "POST", path+"/messages", `{"client_message_id":"e4444444-4444-4444-8444-444444444446","body":"intrusion"}`, 404)
	memberList := f.request(t, 0, "GET", path+"/members", "", 200)
	if !strings.Contains(string(memberList["members"]), fourth.ID) {
		t.Fatal("member list incomplete")
	}

	f.request(t, 2, "GET", path+"/messages", "", 200)
	sent := f.request(t, 1, "POST", path+"/messages", `{"client_message_id":"e4444444-4444-4444-8444-444444444444","body":"Привет группе"}`, 201)
	retry := f.request(t, 1, "POST", path+"/messages", `{"client_message_id":"e4444444-4444-4444-8444-444444444444","body":"Привет группе"}`, 200)
	if string(sent["message"]) != string(retry["message"]) {
		t.Fatal("retry duplicated group message")
	}
	for _, actor := range []int{0, 1, 2} {
		history := f.request(t, actor, "GET", path+"/messages", "", 200)
		if !strings.Contains(string(history["messages"]), "Привет группе") {
			t.Fatalf("member %d missing message", actor)
		}
	}
	f.request(t, 0, "POST", path+"/messages", `{"client_message_id":"e4444444-4444-4444-8444-444444444445","body":"Ответ"}`, 201)
	var username string
	if err := f.db.QueryRow(f.ctx, `SELECT username FROM users WHERE id=$1`, f.users[1].ID).Scan(&username); err != nil {
		t.Fatal(err)
	}
	mentionBody := "e4444444-4444-4444-8444-444444444449"
	mentioned := f.request(t, 0, "POST", path+"/messages", fmt.Sprintf(`{"client_message_id":%q,"body":%q,"mentions":[%q]}`, mentionBody, "Привет @"+username, f.users[1].ID), 201)
	if !strings.Contains(string(mentioned["message"]), `"user_id":"`+f.users[1].ID+`"`) {
		t.Fatalf("structured mention missing: %s", mentioned["message"])
	}
	mentionRetry := f.request(t, 0, "POST", path+"/messages", fmt.Sprintf(`{"client_message_id":%q,"body":%q,"mentions":[%q]}`, mentionBody, "Привет @"+username, f.users[1].ID), 200)
	if !strings.Contains(string(mentionRetry["message"]), `"username":"`+username+`"`) {
		t.Fatalf("mention retry missing: %s", mentionRetry["message"])
	}
	f.request(t, 0, "POST", path+"/messages", fmt.Sprintf(`{"client_message_id":"e4444444-4444-4444-8444-444444444450","body":"@someone","mentions":[%q]}`, f.users[1].ID), 400)
	f.request(t, 0, "POST", path+"/messages", fmt.Sprintf(`{"client_message_id":"e4444444-4444-4444-8444-444444444451","body":"@someone","mentions":[%q]}`, f.users[3].ID), 400)
	found := f.request(t, 0, "GET", "/api/groups", "", 200)
	if !strings.Contains(string(found["groups"]), group.ID) {
		t.Fatal("group absent from list")
	}
	if _, err := f.store.ChangeControls(f.ctx, f.conv.ID, f.users[0].ID, "block"); err != nil {
		t.Fatal(err)
	}
	f.request(t, 1, "POST", path+"/messages", `{"client_message_id":"e4444444-4444-4444-8444-444444444447","body":"bypass"}`, 403)
	f.request(t, 0, "POST", path+"/messages", `{"client_message_id":"e4444444-4444-4444-8444-444444444448","body":"bypass"}`, 403)
	req, _ := http.NewRequest("GET", f.server.URL+path+"/messages", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.tokens[0]})
	res, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
}
