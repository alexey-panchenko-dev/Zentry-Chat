package httpapi

import (
	"net/http"
	"sync"
	"time"

	"newsChat/api/internal/auth"
)

var typingEvents = struct {
	sync.Mutex
	last map[string]time.Time
}{last: map[string]time.Time{}}

func allowTypingEvent(userID, scope, chatID string) bool {
	now := time.Now()
	key := userID + ":" + scope + ":" + chatID
	typingEvents.Lock()
	defer typingEvents.Unlock()
	if last, ok := typingEvents.last[key]; ok && now.Sub(last) < time.Second {
		return false
	}
	typingEvents.last[key] = now
	if len(typingEvents.last) > 4096 {
		for k, at := range typingEvents.last {
			if now.Sub(at) > time.Minute {
				delete(typingEvents.last, k)
			}
		}
	}
	return true
}

func (h *chatHandlers) typingGroup(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}
	members, err := h.store.GroupMembers(r.Context(), id, u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	if !allowTypingEvent(u.ID, "group", id) {
		writeJSON(w, 429, map[string]string{"error": "typing_rate_limited"})
		return
	}
	for _, member := range members {
		if member.ID != u.ID {
			_ = h.publisher.Publish("private-user-"+member.ID, "chat.typing", map[string]string{"group_id": id, "user_id": u.ID})
		}
	}
	writeJSON(w, 204, nil)
}
