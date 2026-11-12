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
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"newsChat/api/internal/auth"
)

const sessionCookie = "chat_session"

type authHandlers struct {
	service *auth.Service
	mailer  codeMailer
}

var emailCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

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
			"message": "Нужны @юзер, почта и пароль длиной 6–32 символов: латинские строчные и заглавные буквы, цифра и один из символов & * $ %",
		})
		return
	}

	if input.BirthDate == nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_birth_date"})
		return
	}
	if _, err := auth.ParseBirthDate(*input.BirthDate, time.Now()); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_birth_date"})
		return
	}
	if h.mailer == nil || !h.mailer.Configured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "email_delivery_not_configured"})
		return
	}
	_, err := h.service.RegisterPendingWithBirthDate(
		r.Context(),
		username,
		email,
		input.Password, input.BirthDate, username)
	if errors.Is(err, auth.ErrRegistrationPending) {
		writeJSON(w, 409, map[string]string{"error": "registration_pending"})
		return
	}
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
	code, err := h.service.IssueEmailCode(r.Context(), email, auth.VerificationPurpose)
	if errors.Is(err, auth.ErrCodeCooldown) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "code_resend_too_soon"})
		return
	}
	if err != nil {
		authFailure(w, err)
		return
	}
	delivery := true
	if err := h.mailer.SendCode(email, code, auth.VerificationPurpose); err != nil {
		log.Printf("Не удалось отправить код подтверждения")
		delivery = false
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"verification_required": true, "delivery_sent": delivery, "email": email})
}

func (h *authHandlers) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	email, ok := normalizedEmail(input.Email)
	if !ok || !emailCodePattern.MatchString(input.Code) {
		writeJSON(w, 400, map[string]string{"error": "invalid_verification_code"})
		return
	}
	err := h.service.VerifyEmailCode(r.Context(), email, input.Code)
	if errors.Is(err, auth.ErrUsernameExists) {
		writeJSON(w, 409, map[string]string{"error": "username_taken"})
		return
	}
	if errors.Is(err, auth.ErrEmailExists) {
		writeJSON(w, 409, map[string]string{"error": "email_already_registered"})
		return
	}
	if err != nil {
		authCodeFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"verified": true})
}

func (h *authHandlers) resendVerification(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	email, ok := normalizedEmail(input.Email)
	if !ok {
		writeJSON(w, 400, map[string]string{"error": "invalid_email"})
		return
	}
	if h.mailer == nil || !h.mailer.Configured() {
		writeJSON(w, 503, map[string]string{"error": "email_delivery_not_configured"})
		return
	}
	code, exists, err := h.service.IssueVerificationCode(r.Context(), email)
	if err == nil && !exists {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if errors.Is(err, auth.ErrCodeCooldown) {
		writeJSON(w, 429, map[string]string{"error": "code_resend_too_soon"})
		return
	}
	if err != nil {
		authFailure(w, err)
		return
	}
	if err := h.mailer.SendCode(email, code, auth.VerificationPurpose); err != nil {
		log.Printf("Не удалось отправить код подтверждения")
		writeJSON(w, 503, map[string]string{"error": "email_delivery_failed"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandlers) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	email, ok := normalizedEmail(input.Email)
	if !ok {
		writeJSON(w, 400, map[string]string{"error": "invalid_email"})
		return
	}
	if h.mailer == nil || !h.mailer.Configured() {
		writeJSON(w, 503, map[string]string{"error": "email_delivery_not_configured"})
		return
	}
	code, exists, err := h.service.IssueRecoveryCode(r.Context(), email)
	if err != nil && !errors.Is(err, auth.ErrCodeCooldown) {
		authFailure(w, err)
		return
	}
	if exists && code != "" {
		if err := h.mailer.SendCode(email, code, auth.RecoveryPurpose); err != nil {
			log.Printf("Не удалось отправить письмо восстановления")
		}
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"message": "Если аккаунт с такой почтой существует, код отправлен."})
}

func (h *authHandlers) resetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	email, ok := normalizedEmail(input.Email)
	if !ok || !emailCodePattern.MatchString(input.Code) || !validPassword(input.Password) {
		writeJSON(w, 400, map[string]string{"error": "invalid_reset_input"})
		return
	}
	if err := h.service.ResetPasswordWithCode(r.Context(), email, input.Code, input.Password); err != nil {
		authCodeFailure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func authCodeFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrCodeInvalid):
		writeJSON(w, 400, map[string]string{"error": "invalid_verification_code"})
	case errors.Is(err, auth.ErrCodeExpired):
		writeJSON(w, 400, map[string]string{"error": "verification_code_expired"})
	case errors.Is(err, auth.ErrCodeAttempts):
		writeJSON(w, 429, map[string]string{"error": "verification_attempts_exceeded"})
	default:
		authFailure(w, err)
	}
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
	if !ok || !validLoginPassword(input.Password) {
		writeJSON(w, 401, map[string]string{"error": "invalid_credentials"})
		return
	}
	u, token, expires, err := h.service.Login(r.Context(), email, input.Password)
	if errors.Is(err, auth.ErrLoginLocked) {
		w.Header().Set("Retry-After", "600")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "login_temporarily_locked"})
		return
	}
	if errors.Is(err, auth.ErrEmailNotVerified) {
		writeJSON(w, 403, map[string]string{"error": "email_not_verified"})
		return
	}
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
	if !utf8.ValidString(value) || len(value) < 6 || len(value) > 32 {
		return false
	}
	var lower, upper, digit, special bool
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= '0' && r <= '9':
			digit = true
		case strings.ContainsRune("&*$%", r):
			special = true
		default:
			return false
		}
	}
	return lower && upper && digit && special
}

func validLoginPassword(value string) bool {
	return utf8.ValidString(value) && len(value) > 0 && len(value) <= 72
}

func authFailure(w http.ResponseWriter, err error) {
	log.Printf("Ошибка авторизации: %v", err)
	writeJSON(w, 500, map[string]string{"error": "internal_error"})
}
