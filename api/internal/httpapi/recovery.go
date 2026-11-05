package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"

	"newsChat/api/internal/auth"
)

var recoverySlots = make(chan struct{}, 4)

func recoveryConfig() (string, string, bool) {
	base := strings.TrimRight(os.Getenv("PUBLIC_APP_URL"), "/")
	u, err := url.Parse(base)
	from := os.Getenv("GMAIL_SENDER")
	address, mailErr := mail.ParseAddress(from)
	ok := err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == "" && mailErr == nil && address.Address == from
	for _, key := range []string{"GMAIL_CLIENT_ID", "GMAIL_CLIENT_SECRET", "GMAIL_REFRESH_TOKEN"} {
		ok = ok && os.Getenv(key) != ""
	}
	return base, from, ok
}

func (h *authHandlers) forgotPassword(w http.ResponseWriter, r *http.Request) {
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
	base, from, configured := recoveryConfig()
	if !configured {
		writeJSON(w, 503, map[string]string{"error": "recovery_unavailable"})
		return
	}
	select {
	case recoverySlots <- struct{}{}:
	default:
		// Saturation must not reveal whether previous requests found accounts.
		fmt.Fprintln(os.Stderr, "Password recovery: delivery capacity reached")
		writeJSON(w, 202, map[string]string{"message": "Если аккаунт с этой почтой существует, письмо придёт в ближайшие минуты."})
		return
	}
	go func() {
		defer func() { <-recoverySlots }()
		mailCtx, mailCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer mailCancel()
		token, err := h.service.CreateReset(mailCtx, email)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Password recovery: request failed")
			return
		}
		if token == "" {
			return
		}
		if err := sendRecoveryMail(mailCtx, from, email, base+"/#reset-password="+token); err != nil {
			cleanup, done := context.WithTimeout(context.Background(), 3*time.Second)
			defer done()
			h.service.CancelReset(cleanup, token)
			// No account-specific delivery response or secret-bearing upstream body.
			// An operator can diagnose transport configuration without exposing tokens.
			fmt.Fprintln(os.Stderr, "Password recovery: Gmail delivery failed")
		}
	}()
	writeJSON(w, 202, map[string]string{"message": "Если аккаунт с этой почтой существует, письмо придёт в ближайшие минуты."})
}

func (h *authHandlers) resetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	if len(input.Token) != 64 || strings.ContainsAny(input.Token, "\r\n") || !validPassword(input.Password) {
		writeJSON(w, 400, map[string]string{"error": "invalid_reset_input"})
		return
	}
	for _, c := range input.Token {
		if !strings.ContainsRune("0123456789abcdef", c) {
			writeJSON(w, 400, map[string]string{"error": "invalid_reset_input"})
			return
		}
	}
	err := h.service.ResetPassword(r.Context(), input.Token, input.Password)
	if errors.Is(err, auth.ErrReset) {
		writeJSON(w, 400, map[string]string{"error": "reset_expired"})
		return
	}
	if err != nil {
		authFailure(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: os.Getenv("COOKIE_SECURE") == "true", SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(204)
}

func sendRecoveryMail(ctx context.Context, from, to, link string) error {
	client := &http.Client{Timeout: 15 * time.Second}
	form := url.Values{"client_id": {os.Getenv("GMAIL_CLIENT_ID")}, "client_secret": {os.Getenv("GMAIL_CLIENT_SECRET")}, "refresh_token": {os.Getenv("GMAIL_REFRESH_TOKEN")}, "grant_type": {"refresh_token"}}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(req)
	if err != nil {
		return errors.New("oauth transport failed")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("oauth rejected")
	}
	var access struct {
		Token string `json:"access_token"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&access) != nil || access.Token == "" {
		return errors.New("oauth response invalid")
	}
	message := "From: " + from + "\r\nTo: " + to + "\r\nSubject: Zently - password reset\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n" + base64.StdEncoding.EncodeToString([]byte("Сброс пароля Zently\n\nОткрой ссылку и задай новый пароль:\n"+link+"\n\nСсылка действует 15 минут и используется один раз. Если ты не запрашивал смену пароля, проигнорируй письмо.\n"))
	body, _ := json.Marshal(map[string]string{"raw": base64.RawURLEncoding.EncodeToString([]byte(message))})
	req, err = http.NewRequestWithContext(ctx, "POST", "https://gmail.googleapis.com/gmail/v1/users/me/messages/send", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+access.Token)
	req.Header.Set("Content-Type", "application/json")
	sent, err := client.Do(req)
	if err != nil {
		return errors.New("gmail transport failed")
	}
	defer sent.Body.Close()
	if sent.StatusCode < 200 || sent.StatusCode >= 300 {
		return errors.New("gmail rejected")
	}
	return nil
}
