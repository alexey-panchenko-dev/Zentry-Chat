package httpapi

import (
	"net/http"
	"newsChat/api/internal/auth"
	"strings"
	"unicode/utf8"
)

func (h *chatHandlers) manageGroup(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}
	var v struct {
		Action string `json:"action"`
		UserID string `json:"user_id"`
		Value  string `json:"value"`
	}
	if !readAuthJSON(w, r, &v) {
		return
	}
	v.Value = strings.TrimSpace(v.Value)
	switch v.Action {
	case "name":
		if !utf8.ValidString(v.Value) || utf8.RuneCountInString(v.Value) < 1 || utf8.RuneCountInString(v.Value) > 80 || strings.ContainsRune(v.Value, 0) {
			writeJSON(w, 400, map[string]string{"error": "invalid_group"})
			return
		}
	case "add", "remove", "role", "owner":
		if !uuidPattern.MatchString(v.UserID) {
			writeJSON(w, 400, map[string]string{"error": "invalid_user"})
			return
		}
		if v.Action == "role" && v.Value != "admin" && v.Value != "member" {
			writeJSON(w, 400, map[string]string{"error": "invalid_role"})
			return
		}
	default:
		writeJSON(w, 400, map[string]string{"error": "invalid_action"})
		return
	}
	before, err := h.store.GroupMembers(r.Context(), id, u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	if err = h.store.ManageGroup(r.Context(), id, u.ID, v.UserID, v.Action, v.Value); err != nil {
		chatFailure(w, err)
		return
	}
	ids := []string{v.UserID}
	for _, m := range before {
		ids = append(ids, m.ID)
	}
	h.publishGroupUpdate(id, ids...)
	writeJSON(w, 200, map[string]bool{"ok": true})
}
