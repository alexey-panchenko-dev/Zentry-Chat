package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoveryRejectsInvalidInput(t *testing.T) {
	for _, body := range []string{`{}`, `{"email":"bad"}`, `{"email":"Name <test@example.com>"}`} {
		r := httptest.NewRequest("POST", "/api/auth/forgot-password", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		(&authHandlers{}).forgotPassword(w, r)
		if w.Code != 400 {
			t.Fatalf("got %d for %s", w.Code, body)
		}
	}
}

func TestResetRejectsMalformedTokens(t *testing.T) {
	for _, token := range []string{"", "short", strings.Repeat("z", 64)} {
		r := httptest.NewRequest("POST", "/api/auth/reset-password", strings.NewReader(`{"token":"`+token+`","password":"test-password-12345"}`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		(&authHandlers{}).resetPassword(w, r)
		if w.Code != 400 {
			t.Fatalf("got %d", w.Code)
		}
	}
}

func TestRecoveryUnconfiguredIsNotFakeSuccess(t *testing.T) {
	t.Setenv("PUBLIC_APP_URL", "")
	r := httptest.NewRequest("POST", "/api/auth/forgot-password", strings.NewReader(`{"email":"test@example.com"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	(&authHandlers{}).forgotPassword(w, r)
	if w.Code != 503 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestRecoveryLinksUseConfiguredHTTPSOrigin(t *testing.T) {
	for _, key := range []string{"GMAIL_CLIENT_ID", "GMAIL_CLIENT_SECRET", "GMAIL_REFRESH_TOKEN"} {
		t.Setenv(key, "test-only")
	}
	t.Setenv("GMAIL_SENDER", "sender@example.com")
	for _, base := range []string{"http://example.com", "https://example.com/?bad=1", "https://example.com/#bad", "https://user@example.com", "https://example.com/reset"} {
		t.Setenv("PUBLIC_APP_URL", base)
		if _, _, ok := recoveryConfig(); ok {
			t.Fatalf("accepted %s", base)
		}
	}
	t.Setenv("PUBLIC_APP_URL", "https://zentrychat.vercel.app")
	if _, _, ok := recoveryConfig(); !ok {
		t.Fatal("valid config rejected")
	}
}

func TestRecoverySaturationDoesNotExposeAccountLookup(t *testing.T) {
	t.Setenv("PUBLIC_APP_URL", "https://zentrychat.vercel.app")
	t.Setenv("GMAIL_SENDER", "sender@example.com")
	for _, key := range []string{"GMAIL_CLIENT_ID", "GMAIL_CLIENT_SECRET", "GMAIL_REFRESH_TOKEN"} {
		t.Setenv(key, "test-only")
	}
	for range cap(recoverySlots) {
		recoverySlots <- struct{}{}
	}
	defer func() {
		for range cap(recoverySlots) {
			<-recoverySlots
		}
	}()
	r := httptest.NewRequest("POST", "/api/auth/forgot-password", strings.NewReader(`{"email":"unknown@example.com"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	(&authHandlers{}).forgotPassword(w, r)
	if w.Code != 202 || strings.Contains(w.Body.String(), "busy") {
		t.Fatalf("account-dependent saturation response: %d %s", w.Code, w.Body.String())
	}
}
