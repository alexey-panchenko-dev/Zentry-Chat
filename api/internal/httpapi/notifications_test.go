package httpapi

import (
	"testing"
)

func TestNotificationSettingsArePerUserAndChatAccessIsChecked(t *testing.T) {
	f := newControlFixture(t)
	f.request(t, -1, "GET", "/api/notifications", "", 401)
	f.request(t, 0, "GET", "/api/notifications", "", 200)
	f.request(t, 0, "PUT", "/api/notifications", `{"sound_messages":false,"push_enabled":true,"sound_calls":true,"mentions_enabled":true,"dnd_mute_message_sound":true,"dnd_suppress_push":true,"dnd_mute_call_sound":true,"dnd_suppress_mentions":false,"quiet_enabled":true,"quiet_days":127,"quiet_start":"23:00","quiet_end":"09:00","timezone":"Asia/Qyzylorda"}`, 200)
	f.request(t, 0, "PUT", "/api/notifications", `{"timezone":"Not/A_Timezone"}`, 400)
	f.request(t, 0, "PUT", "/api/notifications/chats", `{"scope":"personal","chat_id":"00000000-0000-0000-0000-000000000000","mode":"off","duration":"1h"}`, 404)
	path := `{"scope":"personal","chat_id":"` + f.conv.ID + `","mode":"silent"}`
	f.request(t, 0, "PUT", "/api/notifications/chats", path, 200)
	f.request(t, 1, "PUT", "/api/notifications/chats", `{"scope":"personal","chat_id":"`+f.conv.ID+`","mode":"off"}`, 200)
	user0 := f.request(t, 0, "GET", "/api/notifications", "", 200)
	user1 := f.request(t, 1, "GET", "/api/notifications", "", 200)
	if len(user0["settings"]) == 0 || len(user1["settings"]) == 0 {
		t.Fatal("settings were not returned")
	}
}
