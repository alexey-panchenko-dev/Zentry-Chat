package httpapi

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http"
	"newsChat/api/internal/auth"
"newsChat/api/internal/chat"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,39}$`)

func (h *chatHandlers) touchPresence(w http.ResponseWriter, r *http.Request, u auth.User) {
	if err := h.store.TouchPresence(r.Context(), u.ID); err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"online": true})
}
func (h *chatHandlers) setUsername(w http.ResponseWriter, r *http.Request, u auth.User) {
	var input struct {
		Username string `json:"username"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	name := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(input.Username), "@"))
	if !usernamePattern.MatchString(name) {
		writeJSON(w, 400, map[string]string{"error": "invalid_username", "message": "От 3 до 40 символов: латиница, цифры и _; первый символ — буква."})
		return
	}
	if err := h.store.SetUsername(r.Context(), u.ID, name); err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			writeJSON(w, 409, map[string]string{"error": "username_taken"})
			return
		}
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"username": name})
}
func (h *chatHandlers) setContactAlias(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := conversationID(w, r)
	if !ok {
		return
	}
	var input struct {
		Alias string `json:"alias"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	alias := strings.TrimSpace(input.Alias)
	if !utf8.ValidString(alias) || strings.ContainsRune(alias, 0) || utf8.RuneCountInString(alias) > 80 {
		writeJSON(w, 400, map[string]string{"error": "invalid_alias"})
		return
	}
	if alias == "" {
		if err := h.store.ClearContactAlias(r.Context(), id, u.ID); err != nil {
			chatFailure(w, err)
			return
		}
	}
	if alias != "" {
		if _, err := h.store.SetContactAlias(r.Context(), id, u.ID, alias); err != nil {
			chatFailure(w, err)
			return
		}
	}
	writeJSON(w, 200, map[string]string{"alias": alias})
}

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
 if errors.Is(err,chat.ErrPhoneExists){writeJSON(w,409,map[string]string{"error":"phone_already_linked"});return}
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
