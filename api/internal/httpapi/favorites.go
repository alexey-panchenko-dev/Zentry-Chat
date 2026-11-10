package httpapi

import (
	"net/http"
	"strconv"

	"newsChat/api/internal/auth"
)

func (h *chatHandlers) favorites(w http.ResponseWriter, r *http.Request, user auth.User) {
	items, err := h.store.Favorites(r.Context(), user.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"favorites": items})
}

func (h *chatHandlers) setFavorite(w http.ResponseWriter, r *http.Request, user auth.User) {
	scope := r.PathValue("scope")
	id, err := strconv.ParseInt(r.PathValue("messageID"), 10, 64)
	if err != nil || id < 1 || (scope != "personal" && scope != "group") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err := h.store.SetFavorite(r.Context(), user.ID, scope, id, r.Method == http.MethodPut); err != nil {
		chatFailure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *chatHandlers) firstUnread(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := conversationID(w, r)
	if !ok {
		return
	}
	messageID, err := h.store.FirstUnread(r.Context(), user.ID, id)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message_id": messageID})
}
