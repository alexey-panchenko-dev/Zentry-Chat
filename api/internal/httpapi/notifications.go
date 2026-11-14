package httpapi

import (
	"errors"
	"net/http"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/webnotify"
)

func (h *chatHandlers) getNotificationSettings(w http.ResponseWriter, r *http.Request, u auth.User) {
	v, err := h.push.GetPreferences(r.Context(), u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"settings": v})
}
func (h *chatHandlers) saveNotificationSettings(w http.ResponseWriter, r *http.Request, u auth.User) {
	var v webnotify.Preferences
	if !readAuthJSON(w, r, &v) {
		return
	}
	if err := h.push.SavePreferences(r.Context(), u.ID, v); err != nil {
		if errors.Is(err, webnotify.ErrInvalidPreference) {
			writeJSON(w, 400, map[string]string{"error": "invalid_notification_preferences"})
			return
		}
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (h *chatHandlers) saveChatNotification(w http.ResponseWriter, r *http.Request, u auth.User) {
	var v struct {
		Scope    string `json:"scope"`
		ChatID   string `json:"chat_id"`
		Mode     string `json:"mode"`
		Duration string `json:"duration"`
	}
	if !readAuthJSON(w, r, &v) {
		return
	}
	if !uuidPattern.MatchString(v.ChatID) {
		writeJSON(w, 400, map[string]string{"error": "invalid_chat_id"})
		return
	}
	err := h.push.SetChatPreference(r.Context(), u.ID, v.Scope, v.ChatID, v.Mode, v.Duration)
	if errors.Is(err, webnotify.ErrInvalidPreference) {
		writeJSON(w, 400, map[string]string{"error": "invalid_notification_preferences"})
		return
	}
	if errors.Is(err, webnotify.ErrChatUnavailable) {
		writeJSON(w, 404, map[string]string{"error": "chat_not_found"})
		return
	}
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (h *chatHandlers) saveNotificationException(w http.ResponseWriter, r *http.Request, u auth.User) {
	var v webnotify.Exception
	if !readAuthJSON(w, r, &v) {
		return
	}
	v.ContactID = r.PathValue("contactID")
	if !uuidPattern.MatchString(v.ContactID) {
		writeJSON(w, 400, map[string]string{"error": "invalid_contact_id"})
		return
	}
	err := h.push.SetException(r.Context(), u.ID, v.ContactID, v)
	if errors.Is(err, webnotify.ErrInvalidPreference) {
		writeJSON(w, 400, map[string]string{"error": "invalid_notification_exception"})
		return
	}
	if errors.Is(err, webnotify.ErrChatUnavailable) {
		writeJSON(w, 404, map[string]string{"error": "contact_not_found"})
		return
	}
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (h *chatHandlers) deleteNotificationException(w http.ResponseWriter, r *http.Request, u auth.User) {
	if !uuidPattern.MatchString(r.PathValue("contactID")) {
		writeJSON(w, 400, map[string]string{"error": "invalid_contact_id"})
		return
	}
	if _, err := h.push.DB.Exec(r.Context(), `DELETE FROM notification_exceptions WHERE user_id=$1 AND contact_id=$2`, u.ID, r.PathValue("contactID")); err != nil {
		chatFailure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
