package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"newsChat/api/internal/auth"
)

const sessionCookie = "chat_session"

type authHandlers struct{ service *auth.Service }

func (h *authHandlers) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		BirthDate *string `json:"birth_date"`
		Name      string  `json:"name"`
		Email     string  `json:"email"`
		Password  string  `json:"password"`
	}

	if !readAuthJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	username := strings.ToLower(strings.TrimPrefix(input.Name, "@"))

	email, ok := normalizedEmail(input.Email)

	if !ok ||
		!utf8.ValidString(input.Name) ||
		!usernamePattern.MatchString(username) ||
		!validPassword(input.Password) {
		writeJSON(w, 400, map[string]string{
			"error":   "invalid_input",
			"message": "Нужны @юзер (3–40 символов: латиница, цифры и _), почта и пароль (12–72 байта UTF-8)",
		})
		return
	}

	if input.BirthDate != nil {
		if _, err := auth.ParseBirthDate(*input.BirthDate, time.Now()); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid_birth_date"})
			return
		}
	}
	u, err := h.service.RegisterWithBirthDate(
		r.Context(),
		username,
		email,
		input.Password, input.BirthDate, username)
	if errors.Is(err, auth.ErrEmailExists) {
		writeJSON(w, 409, map[string]string{"error": "email_already_registered"})
		return
	}
	if errors.Is(err, auth.ErrUsernameExists) {
		writeJSON(w, 409, map[string]string{"error": "username_taken"})
		return
	}
	if err != nil {
		authFailure(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"user": u})
}

func (h *authHandlers) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	email, ok := normalizedEmail(input.Email)
	if !ok || !validPassword(input.Password) {
		writeJSON(w, 401, map[string]string{"error": "invalid_credentials"})
		return
	}
	u, token, expires, err := h.service.Login(r.Context(), email, input.Password)
	if errors.Is(err, auth.ErrCredentials) {
		writeJSON(w, 401, map[string]string{"error": "invalid_credentials"})
		return
	}
	if err != nil {
		authFailure(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: os.Getenv("COOKIE_SECURE") == "true", SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(auth.SessionTTL.Seconds())})
	writeJSON(w, 200, map[string]any{"user": u})
}

func (h *authHandlers) me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	u, err := h.service.CurrentUser(r.Context(), cookie.Value)
	if errors.Is(err, auth.ErrSession) {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	if err != nil {
		authFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"user": u})
}

func (h *authHandlers) logout(w http.ResponseWriter, r *http.Request) {
	if !isJSON(r) {
		writeJSON(w, 415, map[string]string{"error": "content_type_must_be_json"})
		return
	}
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := h.service.Logout(r.Context(), cookie.Value); err != nil {
			authFailure(w, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: os.Getenv("COOKIE_SECURE") == "true", SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	w.WriteHeader(http.StatusNoContent)
}

func isJSON(r *http.Request) bool {
	t, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && t == "application/json"
}

func readAuthJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	return readJSONLimit(w, r, v, 16*1024)
}

func readJSONLimit(w http.ResponseWriter, r *http.Request, v any, maxBytes int64) bool {
	if !isJSON(r) {
		writeJSON(w, 415, map[string]string{"error": "content_type_must_be_json"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	err := d.Decode(v)
	if err == nil {
		var extra any
		err = d.Decode(&extra)
		if errors.Is(err, io.EOF) {
			return true
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeJSON(w, 413, map[string]string{"error": "body_too_large"})
	} else {
		writeJSON(w, 400, map[string]string{"error": "invalid_json"})
	}
	return false
}

func normalizedEmail(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(value)
	return value, err == nil && parsed.Address == value && len(value) <= 254
}

func validPassword(value string) bool {
	return utf8.ValidString(value) && len(value) >= 12 && len(value) <= 72
}

func authFailure(w http.ResponseWriter, err error) {
	log.Printf("Ошибка авторизации: %v", err)
	writeJSON(w, 500, map[string]string{"error": "internal_error"})
}
