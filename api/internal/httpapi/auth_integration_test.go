package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"newsChat/api/internal/auth"
	"newsChat/api/internal/database"
)

func TestHTTPAuthLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to a test PostgreSQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	// Also verify migration re-runs are safe.
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:9090")
	mailer := &recordingMailer{}
	server := httptest.NewServer(newRouterWithMailer(db, mailer))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	email := fmt.Sprintf("http-%d@example.com", time.Now().UnixNano())
	defer func() {
		cleanup, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_, _ = db.Exec(cleanup, "DELETE FROM users WHERE email=$1", email)
	}()

	request := func(method, path, body string, status int) *http.Response {
		t.Helper()
		r, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://localhost:9090")
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = response.Body.Close() })
		if response.StatusCode != status {
			t.Fatalf("%s %s: status %d, want %d", method, path, response.StatusCode, status)
		}
		return response
	}
	request("GET", "/api/health", "", 200)
	request("GET", "/api/ready", "", 200)
	request("GET", "/api/auth/me", "", 401)
	handle := fmt.Sprintf("http_test_%d", time.Now().UnixNano())
	registration := fmt.Sprintf(`{"name":%q,"email":%q,"password":"Strong1&","birth_date":"2000-01-01"}`, handle, email)
	registered := request("POST", "/api/auth/register", registration, 202)
	if len(registered.Cookies()) != 0 {
		t.Fatal("registration issued a session before verification")
	}
	request("GET", "/api/auth/me", "", 401)
	var registeredUsers int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM users WHERE email=$1`, email).Scan(&registeredUsers); err != nil || registeredUsers != 0 {
		t.Fatal("unverified signup created a user", err)
	}
	request("POST", "/api/auth/login", fmt.Sprintf(`{"email":%q,"password":"Strong1&"}`, email), 401)
	// Pending registrations cannot replace credentials while an email code lives.
	request("POST", "/api/auth/register", fmt.Sprintf(`{"name":%q,"email":%q,"password":"Stolen1&","birth_date":"2000-01-01"}`, handle+"x", email), 409)
	code := mailer.code(email, auth.VerificationPurpose)
	if len(code) != 6 {
		t.Fatal("verification email was not queued")
	}
	request("POST", "/api/auth/verify-email", fmt.Sprintf(`{"email":%q,"code":%q}`, email, code), 200)
	request("POST", "/api/auth/register", registration, 409)
	request("POST", "/api/auth/register", fmt.Sprintf(`{"name":%q,"email":%q,"password":"Strong1&","birth_date":"2000-01-01"}`, handle, "other-"+email), 409)
	request("POST", "/api/auth/login", fmt.Sprintf(`{"email":%q,"password":"wrong-password-123"}`, email), 401)
	response := request("POST", "/api/auth/login", fmt.Sprintf(`{"email":%q,"password":"Strong1&"}`, email), 200)
	var signup struct {
		User struct{ Name, Username string } `json:"user"`
	}
	if err := json.NewDecoder(response.Body).Decode(&signup); err != nil || signup.User.Name != handle || signup.User.Username != handle {
		t.Fatalf("registration nickname is not the @username: %+v, %v", signup, err)
	}
	cookies := response.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].MaxAge <= 0 {
		t.Fatal("incorrect session cookie")
	}
	response = request("GET", "/api/auth/me", "", 200)
	var result struct {
		User struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil || result.User.Email != email {
		t.Fatalf("incorrect current user: %v", err)
	}
	request("POST", "/api/auth/logout", "{}", 204)
	request("GET", "/api/auth/me", "", 401)
	request("POST", "/api/auth/logout", "{}", 204)
}
