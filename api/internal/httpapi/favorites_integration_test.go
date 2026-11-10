package httpapi

import (
	"encoding/json"
	"strconv"
	"testing"

	"newsChat/api/internal/chat"
)

func TestFavoritesArePrivateAndRespectClearedHistory(t *testing.T) {
	f := newControlFixture(t)
	message, _, err := f.store.Send(f.ctx, f.conv.ID, f.users[1].ID, "d5494c1f-6017-4d56-9a51-76f55f32a601", "favorite test")
	if err != nil {
		t.Fatal(err)
	}
	item := "/api/favorites/personal/" + message.ID
	f.request(t, -1, "GET", "/api/favorites", "", 401)
	f.request(t, 2, "PUT", item, "", 404)
	f.request(t, 0, "PUT", item, "", 204)
	f.request(t, 0, "PUT", item, "", 204)
	list := func(actor int) []chat.Favorite {
		t.Helper()
		var items []chat.Favorite
		data := f.request(t, actor, "GET", "/api/favorites", "", 200)
		if err := json.Unmarshal(data["favorites"], &items); err != nil {
			t.Fatal(err)
		}
		return items
	}
	if items := list(0); len(items) != 1 || items[0].ID != message.ID {
		t.Fatalf("owner favorites: %+v", items)
	}
	if items := list(1); len(items) != 0 {
		t.Fatalf("peer favorites: %+v", items)
	}
	f.request(t, 1, "DELETE", item, "", 204)
	if len(list(0)) != 1 {
		t.Fatal("peer removed another user's favorite")
	}
	f.request(t, 0, "DELETE", item, "", 204)
	if len(list(0)) != 0 {
		t.Fatal("favorite still present after removing it")
	}
	f.request(t, 0, "PUT", item, "", 204)
	path := "/api/conversations/" + f.conv.ID
	first := f.request(t, 0, "GET", path+"/first-unread", "", 200)
	if string(first["message_id"]) != strconv.Quote(message.ID) {
		t.Fatalf("first unread = %s", first["message_id"])
	}
	f.request(t, 0, "POST", path+"/controls", `{"action":"clear"}`, 200)
	if len(list(0)) != 0 {
		t.Fatal("favorite exposed cleared history")
	}
	f.request(t, 0, "PUT", item, "", 404)
}
