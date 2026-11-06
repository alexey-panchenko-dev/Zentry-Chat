package httpapi

import (
	"errors"
	"net/http"
	"newsChat/api/internal/auth"
	"strings"
	"unicode"
)

func normalizeContactPhone(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) > 100 {
		return "", errors.New("invalid phone")
	}
	var digits strings.Builder
	for i, r := range value {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r == '+' && i == 0:
		case r == '(' || r == ')' || r == '-' || unicode.IsSpace(r):
		default:
			return "", errors.New("invalid phone")
		}
	}
	if digits.Len() < 8 || digits.Len() > 15 || digits.String()[0] == '0' {
		return "", errors.New("invalid phone")
	}
	return "+" + digits.String(), nil
}
func (h *chatHandlers) getContactSettings(w http.ResponseWriter, r *http.Request, u auth.User) {
	v, err := h.store.ContactSettings(r.Context(), u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *chatHandlers) setContactSettings(w http.ResponseWriter, r *http.Request, u auth.User) {
	var input struct {
		Phone     string `json:"phone"`
		ShowPhone bool   `json:"show_phone"`
		ShowEmail bool   `json:"show_email"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	phone, err := normalizeContactPhone(input.Phone)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_phone", "message": "Телефон: от 8 до 15 цифр в международном формате"})
		return
	}
	v, err := h.store.SetContactSettings(r.Context(), u.ID, phone, input.ShowPhone, input.ShowEmail)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *chatHandlers) getContactProfile(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := conversationID(w, r)
	if !ok {
		return
	}
	v, err := h.store.ContactProfile(r.Context(), id, u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"profile": v})
}
