package httpapi

import (
	"bytes"
	"encoding/base64"
	"errors"
	"github.com/pquerna/otp/totp"
	"newsChat/api/internal/auth"
	"sync"
	"testing"
	"time"
)

func mfaFixture(t *testing.T) (*controlFixture, *auth.Service, []string) {
	t.Helper()
	t.Setenv("TOTP_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{91}, 32)))
	f := newControlFixture(t)
	s := auth.New(f.db)
	setup, err := s.BeginMFA(f.ctx, f.users[0].ID, f.users[0].Email, "password-12345")
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	codes, err := s.VerifyMFASetup(f.ctx, f.users[0].ID, code)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ActivateMFA(f.ctx, f.users[0].ID, f.tokens[0]); err != nil {
		t.Fatal(err)
	}
	return f, s, codes
}
func TestMFALoginLimitSurvivesNewChallenges(t *testing.T) {
	f, s, codes := mfaFixture(t)
	for index := 0; index < 10; index++ {
		_, challenge, _, err := s.Login(f.ctx, f.users[0].Email, "password-12345")
		if !errors.Is(err, auth.ErrMFARequired) {
			t.Fatal(err)
		}
		if _, _, _, err = s.CompleteMFA(f.ctx, challenge, "invalid"); !errors.Is(err, auth.ErrMFAInvalid) {
			t.Fatalf("attempt %d: %v", index, err)
		}
	}
	_, challenge, _, _ := s.Login(f.ctx, f.users[0].Email, "password-12345")
	if _, _, _, err := s.CompleteMFA(f.ctx, challenge, codes[0]); !errors.Is(err, auth.ErrSecurityLimit) {
		t.Fatalf("new challenge bypassed account limit: %v", err)
	}
}
func TestMFABackupAtomicUseRegenerationAndMissingKey(t *testing.T) {
	f, s, codes := mfaFixture(t)
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{91}, 32))
	t.Setenv("TOTP_ENCRYPTION_KEY", "")
	if _, token, _, err := s.Login(f.ctx, f.users[0].Email, "password-12345"); !errors.Is(err, auth.ErrMFAUnavailable) || token != "" {
		t.Fatal("missing key bypassed MFA")
	}
	t.Setenv("TOTP_ENCRYPTION_KEY", key)
	var challenges []string
	for i := 0; i < 2; i++ {
		_, value, _, err := s.Login(f.ctx, f.users[0].Email, "password-12345")
		if !errors.Is(err, auth.ErrMFARequired) {
			t.Fatal(err)
		}
		challenges = append(challenges, value)
	}
	var wait sync.WaitGroup
	results := make(chan string, 2)
	errs := make(chan error, 2)
	for _, challenge := range challenges {
		wait.Add(1)
		go func(value string) {
			defer wait.Done()
			_, token, _, err := s.CompleteMFA(f.ctx, value, codes[0])
			if err != nil {
				errs <- err
			} else {
				results <- token
			}
		}(challenge)
	}
	wait.Wait()
	close(results)
	close(errs)
	if len(results) != 1 || len(errs) != 1 {
		t.Fatal("backup code was not atomic")
	}
	current := <-results
	for err := range errs {
		if !errors.Is(err, auth.ErrMFAInvalid) {
			t.Fatal(err)
		}
	}
	fresh, err := s.ChangeMFA(f.ctx, f.users[0].ID, current, "password-12345", codes[1], false)
	if err != nil || len(fresh) != 10 {
		t.Fatal(err)
	}
	if _, err = s.CurrentUser(f.ctx, f.tokens[0]); !errors.Is(err, auth.ErrSession) {
		t.Fatal("old device survived regeneration")
	}
	_, challenge, _, _ := s.Login(f.ctx, f.users[0].Email, "password-12345")
	if _, _, _, err = s.CompleteMFA(f.ctx, challenge, codes[2]); !errors.Is(err, auth.ErrMFAInvalid) {
		t.Fatal("old backup code survived regeneration")
	}
	if _, _, _, err = s.CompleteMFA(f.ctx, challenge, fresh[0]); err != nil {
		t.Fatal(err)
	}
	_, expired, _, _ := s.Login(f.ctx, f.users[0].Email, "password-12345")
	if _, err = f.db.Exec(f.ctx, `UPDATE mfa_challenges SET expires_at=now()-interval '1 second' WHERE user_id=$1`, f.users[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = s.CompleteMFA(f.ctx, expired, fresh[1]); !errors.Is(err, auth.ErrMFAInvalid) {
		t.Fatal("expired challenge accepted")
	}
}
