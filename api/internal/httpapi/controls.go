package httpapi

import (
	"net/http"
	"newsChat/api/internal/auth"
)

func (h *chatHandlers) getControls(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := conversationID(w, r)
	if !ok {
		return
	}
	v, err := h.store.Controls(r.Context(), id, u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *chatHandlers) changeControls(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := conversationID(w, r)
	if !ok {
		return
	}
	var input struct {
		Action string `json:"action"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	if input.Action != "clear" && input.Action != "block" && input.Action != "unblock" && input.Action != "delete_chat" && input.Action != "remove_contact" && input.Action != "delete_everyone" {
		writeJSON(w, 400, map[string]string{"error": "invalid_control_action"})
		return
	}
	change, err := h.store.ChangeControls(r.Context(), id, u.ID, input.Action)
	if err != nil {
		chatFailure(w, err)
		return
	}
	h.publishConversationUpdate("conversation.updated", id, change.Users...)
	callHandler := callHandlers{publisher: h.publisher}
	for _, c := range change.Calls {
		callHandler.notify(c)
	}
	writeJSON(w, 200, change.State)
}
