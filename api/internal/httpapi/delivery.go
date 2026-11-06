package httpapi

import (
	"net/http"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
)

func (h *chatHandlers) pendingDelivery(w http.ResponseWriter, r *http.Request, u auth.User) {
	messages, err := h.store.PendingDelivery(r.Context(), u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, map[string]any{"messages": messages})
}
func (h *chatHandlers) delivered(w http.ResponseWriter, r *http.Request, u auth.User) {
	var input struct {
		IDs []string `json:"message_ids"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	ids, ok := chat.DeliveryIDs(input.IDs)
	if !ok {
		writeJSON(w, 400, map[string]string{"error": "invalid_message_id"})
		return
	}
	conversations, err := h.store.MarkDelivered(r.Context(), u.ID, ids)
	if err != nil {
		chatFailure(w, err)
		return
	}
	if h.publisher.Enabled() {
		for _, id := range conversations {
			users, e := h.store.Members(r.Context(), id)
			if e != nil {
				continue
			}
			for _, user := range users {
				_ = h.publisher.Publish("private-user-"+user, "conversation.read", map[string]string{"conversation_id": id, "user_id": u.ID})
			}
		}
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
