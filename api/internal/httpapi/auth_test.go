package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistrationRejectsInvalidRequests(t *testing.T) {
	cases := []struct {
		name, body, contentType string
		status                  int
	}{
		{"form", "email=test", "application/x-www-form-urlencoded", 415},
		{"unknown field", `{"extra":1}`, "application/json", 400},
		{"multiple objects", `{} {}`, "application/json", 400},
		{"short password", `{"name":"Test","email":"test@example.com","password":"short"}`, "application/json", 400},
		{"missing birthday", `{"name":"Test","email":"test@example.com","password":"Strong1&"}`, "application/json", 400},
		{"display name email", `{"name":"Test","email":"Name <test@example.com>","password":"long-password-123"}`, "application/json", 400},
		{"too large", `{"name":"` + strings.Repeat("a", 17000) + `"}`, "application/json", 413},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.contentType)
			w := httptest.NewRecorder()
			h := &authHandlers{}
			h.register(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d, want=%d, body=%s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}

func TestAuthOriginAndRateLimit(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173")
	h := protectAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	r := httptest.NewRequest("POST", "/api/auth/login", nil)
	r.Header.Set("Origin", "https://untrusted.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("untrusted origin: %d", w.Code)
	}
	r = httptest.NewRequest("OPTIONS", "/api/auth/login", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("preflight failed")
	}
	for i := 0; i < 21; i++ {
		r = httptest.NewRequest("POST", "/api/auth/login", nil)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, r)
		want := 204
		if i == 20 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("attempt %d: status=%d want=%d", i+1, w.Code, want)
		}
	}
}
