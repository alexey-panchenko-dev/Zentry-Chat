package httpapi

import (
	"errors"
	"net/http"
	"newsChat/api/internal/auth"
)

func (h *chatHandlers) listDevices(w http.ResponseWriter, r *http.Request, u auth.User) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	devices, err := h.auth.DeviceSessions(r.Context(), u.ID, cookie.Value)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"sessions": devices})
}
func (h *chatHandlers) revokeDevice(w http.ResponseWriter, r *http.Request, u auth.User) {
	id := r.PathValue("sessionID")
	if !uuidPattern.MatchString(id) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	err := h.auth.RevokeDeviceSession(r.Context(), u.ID, id)
	if errors.Is(err, auth.ErrDeviceNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		chatFailure(w, err)
		return
	}
	h.securityChanged(u.ID)
	w.WriteHeader(http.StatusNoContent)
}
func (h *chatHandlers) revokeOtherDevices(w http.ResponseWriter, r *http.Request, u auth.User) {
	var input struct {
		Confirm bool `json:"confirm"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	if !input.Confirm {
		writeJSON(w, 400, map[string]string{"error": "confirmation_required"})
		return
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	if err = h.auth.RevokeOtherSessions(r.Context(), u.ID, cookie.Value); err != nil {
		chatFailure(w, err)
		return
	}
	h.securityChanged(u.ID)
	w.WriteHeader(http.StatusNoContent)
}
