package httpapi

import (
	"errors"
	"net/http"
	"os"

	"newsChat/api/internal/auth"
)

func mfaFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrMFAUnavailable):
		writeJSON(w, 503, map[string]string{"error": "mfa_unavailable"})
	case errors.Is(err, auth.ErrSecurityLimit):
		w.Header().Set("Retry-After", "600")
		writeJSON(w, 429, map[string]string{"error": "security_action_limit"})
	case errors.Is(err, auth.ErrMFAInvalid):
		writeJSON(w, 400, map[string]string{"error": "mfa_invalid"})
	case errors.Is(err, auth.ErrMFAState):
		writeJSON(w, 409, map[string]string{"error": "mfa_state"})
	case errors.Is(err, auth.ErrCredentials):
		writeJSON(w, 403, map[string]string{"error": "reauth_failed"})
	case errors.Is(err, auth.ErrSession):
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
	default:
		authFailure(w, err)
	}
}
func (h *authHandlers) completeMFA(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Challenge string `json:"challenge"`
		Code      string `json:"code"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	if len(input.Code) > 64 {
		writeJSON(w, 400, map[string]string{"error": "mfa_invalid"})
		return
	}
	user, token, expires, err := h.service.CompleteMFA(r.Context(), input.Challenge, input.Code)
	if err != nil {
		mfaFailure(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: os.Getenv("COOKIE_SECURE") == "true", SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(auth.SessionTTL.Seconds())})
	if h.securityNotify != nil {
		h.securityNotify(user.ID)
	}
	writeJSON(w, 200, map[string]any{"user": user})
}
func (h *chatHandlers) mfaStatus(w http.ResponseWriter, r *http.Request, u auth.User) {
	enabled, err := h.auth.MFAEnabled(r.Context(), u.ID)
	if err != nil {
		mfaFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"enabled": enabled, "configured": auth.MFAConfigured()})
}
func (h *chatHandlers) beginMFA(w http.ResponseWriter, r *http.Request, u auth.User) {
	var input struct {
		Password string `json:"password"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	setup, err := h.auth.BeginMFA(r.Context(), u.ID, u.Email, input.Password)
	if err != nil {
		mfaFailure(w, err)
		return
	}
	writeJSON(w, 200, setup)
}
func (h *chatHandlers) verifyMFASetup(w http.ResponseWriter, r *http.Request, u auth.User) {
	var input struct {
		Code string `json:"code"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	codes, err := h.auth.VerifyMFASetup(r.Context(), u.ID, input.Code)
	if err != nil {
		mfaFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"backup_codes": codes})
}
func (h *chatHandlers) activateMFA(w http.ResponseWriter, r *http.Request, u auth.User) {
	var input struct {
		Saved bool `json:"saved"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	if !input.Saved {
		writeJSON(w, 400, map[string]string{"error": "confirmation_required"})
		return
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		mfaFailure(w, auth.ErrSession)
		return
	}
	if err = h.auth.ActivateMFA(r.Context(), u.ID, cookie.Value); err != nil {
		mfaFailure(w, err)
		return
	}
	h.securityChanged(u.ID)
	w.WriteHeader(204)
}
func (h *chatHandlers) changeMFA(w http.ResponseWriter, r *http.Request, u auth.User) {
	var input struct {
		Password string `json:"password"`
		Code     string `json:"code"`
		Disable  bool   `json:"disable"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		mfaFailure(w, auth.ErrSession)
		return
	}
	codes, err := h.auth.ChangeMFA(r.Context(), u.ID, cookie.Value, input.Password, input.Code, input.Disable)
	if err != nil {
		mfaFailure(w, err)
		return
	}
	h.securityChanged(u.ID)
	writeJSON(w, 200, map[string]any{"backup_codes": codes})
}
func (h *chatHandlers) securityChanged(user string) {
	if h.publisher.Enabled() {
		_ = h.publisher.Publish("private-user-"+user, "security.updated", map[string]string{"kind": "security_changed"})
	}
}
func (h *chatHandlers) securityEvents(w http.ResponseWriter, r *http.Request, u auth.User) {
	events, err := h.auth.SecurityEvents(r.Context(), u.ID)
	if err != nil {
		authFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"events": events})
}
