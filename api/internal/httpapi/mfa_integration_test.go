package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/pquerna/otp/totp"
	"net/http"
	"newsChat/api/internal/auth"
	"strings"
	"testing"
	"time"
)

func TestMFAEnrollmentChallengeReplayAndRecovery(t *testing.T) {
	t.Setenv("TOTP_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{42}, 32)))
	f := newControlFixture(t)
	s := auth.New(f.db)
	user := f.users[0]
	f.request(t, -1, "GET", "/api/security/mfa", "", 401)
	if _, err := s.BeginMFA(f.ctx, user.ID, user.Email, "wrong"); !errors.Is(err, auth.ErrCredentials) {
		t.Fatalf("bad password accepted: %v", err)
	}
	setup, err := s.BeginMFA(f.ctx, user.ID, user.Email, "password-12345")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(setup.QR, "data:image/png;base64,") || setup.Secret == "" {
		t.Fatal("no provisioning data")
	}
	var cipher []byte
	if err = f.db.QueryRow(f.ctx, `SELECT secret_cipher FROM user_totp WHERE user_id=$1`, user.ID).Scan(&cipher); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(cipher, []byte(setup.Secret)) {
		t.Fatal("secret stored as plaintext")
	}
	if enabled, _ := s.MFAEnabled(f.ctx, user.ID); enabled {
		t.Fatal("enabled before verification")
	}
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	codes, err := s.VerifyMFASetup(f.ctx, user.ID, code)
	if err != nil || len(codes) != 10 {
		t.Fatalf("setup verification: %v", err)
	}
	if enabled, _ := s.MFAEnabled(f.ctx, user.ID); enabled {
		t.Fatal("enabled before recovery codes saved")
	}
	if err = s.ActivateMFA(f.ctx, user.ID, f.tokens[0]); err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest("POST", f.server.URL+"/api/auth/login", strings.NewReader(`{"email":"`+user.Email+`","password":"password-12345"}`))
	request.Header.Set("Content-Type", "application/json")
	response, err := f.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 202 || len(response.Cookies()) != 0 {
		t.Fatal("full session issued before MFA")
	}
	var challenge struct {
		Challenge string `json:"challenge"`
	}
	if err = json.NewDecoder(response.Body).Decode(&challenge); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CurrentUser(f.ctx, challenge.Challenge); !errors.Is(err, auth.ErrSession) {
		t.Fatal("challenge authorizes API")
	}
	if _, _, _, err = s.CompleteMFA(f.ctx, challenge.Challenge, code); !errors.Is(err, auth.ErrMFAInvalid) {
		t.Fatalf("setup code replayed: %v", err)
	}
	next, err := totp.GenerateCode(setup.Secret, time.Now().Add(30*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, token, _, err := s.CompleteMFA(f.ctx, challenge.Challenge, next)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CurrentUser(f.ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = s.CompleteMFA(f.ctx, challenge.Challenge, codes[0]); !errors.Is(err, auth.ErrMFAInvalid) {
		t.Fatal("challenge reusable")
	}
	_, second, _, err := s.Login(f.ctx, user.Email, "password-12345")
	if !errors.Is(err, auth.ErrMFARequired) {
		t.Fatal("MFA bypassed")
	}
	if _, _, _, err = s.CompleteMFA(f.ctx, second, codes[0]); err != nil {
		t.Fatal(err)
	}
	_, third, _, _ := s.Login(f.ctx, user.Email, "password-12345")
	for attempt := 0; attempt < 5; attempt++ {
		if _, _, _, err = s.CompleteMFA(f.ctx, third, codes[0]); !errors.Is(err, auth.ErrMFAInvalid) {
			t.Fatal("recovery code replayed")
		}
	}
	if _, _, _, err = s.CompleteMFA(f.ctx, third, codes[1]); !errors.Is(err, auth.ErrMFAInvalid) {
		t.Fatal("attempt limit bypassed")
	}
	_, pending, _, _ := s.Login(f.ctx, user.Email, "password-12345")
	recovery, err := s.IssueEmailCode(f.ctx, user.Email, auth.RecoveryPurpose)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ResetPasswordWithCode(f.ctx, user.Email, recovery, "Changed1&xx"); err != nil {
		t.Fatal(err)
	}
	if enabled, _ := s.MFAEnabled(f.ctx, user.ID); !enabled {
		t.Fatal("password reset disabled MFA")
	}
	if _, _, _, err = s.CompleteMFA(f.ctx, pending, codes[1]); !errors.Is(err, auth.ErrMFAInvalid) {
		t.Fatal("old challenge survived reset")
	}
	_, pending, _, err = s.Login(f.ctx, user.Email, "Changed1&xx")
	if !errors.Is(err, auth.ErrMFARequired) {
		t.Fatal("reset bypasses MFA")
	}
	_, current, _, err := s.CompleteMFA(f.ctx, pending, codes[1])
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ChangeMFA(f.ctx, user.ID, current, "wrong", codes[2], true); !errors.Is(err, auth.ErrCredentials) {
		t.Fatal("disabled without password")
	}
	if _, err = s.ChangeMFA(f.ctx, user.ID, current, "Changed1&xx", codes[2], true); err != nil {
		t.Fatal(err)
	}
	if enabled, _ := s.MFAEnabled(f.ctx, user.ID); enabled {
		t.Fatal("disable failed")
	}
	if _, _, _, err = s.Login(f.ctx, user.Email, "Changed1&xx"); err != nil {
		t.Fatal(err)
	}
}

func TestMFAEncryptionFailsClosed(t *testing.T) {
	t.Setenv("TOTP_ENCRYPTION_KEY", "")
	f := newControlFixture(t)
	s := auth.New(f.db)
	if _, err := s.BeginMFA(f.ctx, f.users[0].ID, f.users[0].Email, "password-12345"); !errors.Is(err, auth.ErrMFAUnavailable) {
		t.Fatalf("missing encryption key: %v", err)
	}
}
