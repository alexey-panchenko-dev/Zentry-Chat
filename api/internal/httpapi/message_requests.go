package httpapi

import (
	"net/http"
	"newsChat/api/internal/auth"
)

func (h *chatHandlers) getMessageRequests(w http.ResponseWriter, r *http.Request, u auth.User) {
	v, err := h.store.ConversationsScope(r.Context(), u.ID, 0, 100, true)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"conversations": v})
}
func (h *chatHandlers) answerMessageRequest(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := conversationID(w, r)
	if !ok {
		return
	}
	var v struct {
		Action string `json:"action"`
	}
	if !readAuthJSON(w, r, &v) {
		return
	}
	if v.Action != "accepted" && v.Action != "declined" {
		writeJSON(w, 400, map[string]string{"error": "invalid_action"})
		return
	}
	if err := h.store.AnswerMessageRequest(r.Context(), id, u.ID, v.Action); err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
