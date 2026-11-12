package httpapi

import (
	"errors"
	"net/http"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/webnotify"
)

func (h *chatHandlers) pushConfig(w http.ResponseWriter, r *http.Request, u auth.User) {
	writeJSON(w, 200, map[string]any{"enabled": h.push.Enabled(), "public_key": h.push.Public})
}
func (h *chatHandlers) savePush(w http.ResponseWriter, r *http.Request, u auth.User) {
	if !h.push.Enabled() {
		writeJSON(w, 503, map[string]string{"error": "push_unavailable"})
		return
	}
	var v webnotify.Registration
	if !readAuthJSON(w, r, &v) {
		return
	}
	if !webnotify.Valid(v) {
		writeJSON(w, 400, map[string]string{"error": "invalid_push"})
		return
	}
	for _, id := range v.Muted {
		if !uuidPattern.MatchString(id) {
			writeJSON(w, 400, map[string]string{"error": "invalid_push"})
			return
		}
	}
	cookie, e := r.Cookie(sessionCookie)
	if e != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	if e = h.push.Save(r.Context(), u.ID, cookie.Value, v); e != nil {
		if errors.Is(e, webnotify.ErrDeviceLimit) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "push_device_limit"})
			return
		}
		chatFailure(w, e)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (h *chatHandlers) removePush(w http.ResponseWriter, r *http.Request, u auth.User) {
	var v struct {
		Endpoint string `json:"endpoint"`
	}
	if !readAuthJSON(w, r, &v) {
		return
	}
	if e := h.push.Remove(r.Context(), u.ID, v.Endpoint); e != nil {
		chatFailure(w, e)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (h *chatHandlers) pushActivity(w http.ResponseWriter, r *http.Request, u auth.User) {
	var v struct {
		Endpoint string `json:"endpoint"`
		Chat     string `json:"chat_id"`
	}
	if !readAuthJSON(w, r, &v) {
		return
	}
	if len(v.Endpoint) > 2048 || v.Chat != "" && !uuidPattern.MatchString(v.Chat) {
		writeJSON(w, 400, map[string]string{"error": "invalid_push"})
		return
	}
	if e := h.push.Active(r.Context(), u.ID, v.Endpoint, v.Chat); e != nil {
		chatFailure(w, e)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
