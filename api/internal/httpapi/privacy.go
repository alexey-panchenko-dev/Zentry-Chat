package httpapi

import (
	"net/http"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
)

func (h *chatHandlers) getPrivacy(w http.ResponseWriter, r *http.Request, u auth.User) {
	v, err := h.store.Privacy(r.Context(), u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *chatHandlers) setPrivacy(w http.ResponseWriter, r *http.Request, u auth.User) {
	var v chat.PrivacySettings
	if !readAuthJSON(w, r, &v) {
		return
	}
	if v.Messages != "everyone" && v.Messages != "contacts" && v.Messages != "nobody" && v.Messages != "requests" {
		writeJSON(w, 400, map[string]string{"error": "invalid_privacy"})
		return
	}
	for _, value := range []string{v.Calls, v.Presence, v.Search, v.Birthday} {
		if value != "everyone" && value != "contacts" && value != "nobody" {
			writeJSON(w, 400, map[string]string{"error": "invalid_privacy"})
			return
		}
	}
	if err := h.store.SetPrivacy(r.Context(), u.ID, v); err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, v)
}
