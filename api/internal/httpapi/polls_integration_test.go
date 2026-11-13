package httpapi

import (
	"encoding/json"
	"fmt"
	"newsChat/api/internal/chat"
	"testing"
)

func TestGroupPollPermissionsVotingAndClosing(t *testing.T) {
	f := newControlFixture(t)
	if _, err := f.store.CreateConversation(f.ctx, f.users[0].ID, f.users[2].ID); err != nil {
		t.Fatal(err)
	}
	group, _, err := f.store.CreateGroup(f.ctx, f.users[0].ID, "Poll group", []string{f.users[1].ID, f.users[2].ID})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.db.Exec(f.ctx, `DELETE FROM group_chats WHERE id=$1`, group.ID) })
	if err = f.store.ManageGroup(f.ctx, group.ID, f.users[0].ID, f.users[2].ID, "remove", ""); err != nil {
		t.Fatal(err)
	}
	base := "/api/groups/" + group.ID + "/polls"
	body := `{"client_message_id":"b1234567-0000-4000-8000-000000000001","question":"Choose","options":["First","Second"],"multiple":false}`
	f.request(t, -1, "POST", base, body, 401)
	f.request(t, 2, "POST", base, body, 404)
	data := f.request(t, 0, "POST", base, body, 201)
	var poll chat.Poll
	if err = json.Unmarshal(data["poll"], &poll); err != nil || len(poll.Options) != 2 {
		t.Fatalf("invalid poll %s %v", data, err)
	}
	replay := f.request(t, 0, "POST", base, body, 201)
	var same chat.Poll
	json.Unmarshal(replay["poll"], &same)
	if same.ID != poll.ID {
		t.Fatal("retry created another poll")
	}
	path := base + "/" + poll.ID
	f.request(t, 2, "GET", path, "", 404)
	f.request(t, 1, "POST", path+"/close", `{}`, 403)
	f.request(t, 1, "PUT", path+"/vote", fmt.Sprintf(`{"option_ids":[%q,%q]}`, poll.Options[0].ID, poll.Options[1].ID), 400)
	vote := fmt.Sprintf(`{"option_ids":[%q]}`, poll.Options[0].ID)
	f.request(t, 1, "PUT", path+"/vote", vote, 200)
	f.request(t, 1, "PUT", path+"/vote", vote, 200)
	result := f.request(t, 0, "GET", path, "", 200)
	var count chat.Poll
	json.Unmarshal(result["poll"], &count)
	if count.Total != 1 || count.Options[0].Votes != 1 {
		t.Fatalf("duplicate votes: %+v", count)
	}
	history, err := f.store.GroupMessages(f.ctx, group.ID, f.users[0].ID, 0, 20)
	if err != nil || len(history) != 1 || history[0].PollID != poll.ID {
		t.Fatalf("poll absent in history: %v", err)
	}
	if _, err = f.store.EditGroupMessage(f.ctx, history[0].ID, f.users[0].ID, "silently changed"); err == nil {
		t.Fatal("poll question edited through text API")
	}
	f.request(t, 0, "POST", path+"/close", `{}`, 204)
	f.request(t, 1, "PUT", path+"/vote", vote, 409)
	second := f.request(t, 1, "POST", base, `{"client_message_id":"b1234567-0000-4000-8000-000000000002","question":"Multiple","options":["One","Two"],"multiple":true}`, 201)
	var multi chat.Poll
	json.Unmarshal(second["poll"], &multi)
	f.request(t, 1, "PUT", base+"/"+multi.ID+"/vote", fmt.Sprintf(`{"option_ids":[%q]}`, poll.Options[0].ID), 400)
	f.request(t, 1, "PUT", base+"/"+multi.ID+"/vote", fmt.Sprintf(`{"option_ids":[%q,%q]}`, multi.Options[0].ID, multi.Options[1].ID), 200)
	if _, err = f.db.Exec(f.ctx, `UPDATE group_polls SET closes_at=now()-interval '1 second' WHERE id=$1`, multi.ID); err != nil {
		t.Fatal(err)
	}
	f.request(t, 1, "PUT", base+"/"+multi.ID+"/vote", `{"option_ids":[]}`, 409)
	f.request(t, 1, "POST", base+"/"+multi.ID+"/close", `{}`, 204)
}
