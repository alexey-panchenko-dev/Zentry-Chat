package httpapi

import (
	"errors"
	"testing"

	"newsChat/api/internal/auth"
)

func TestRecoveryCodesExpireLimitAttemptsAndCannotBeReused(t *testing.T) {
	f := newControlFixture(t)
	s := auth.New(f.db)
	email := f.users[0].Email
	t.Cleanup(func() { _, _ = f.db.Exec(f.ctx, `DELETE FROM auth_email_codes WHERE email=$1`, email) })
	code, exists, err := s.IssueRecoveryCode(f.ctx, email)
	if err != nil || !exists {
		t.Fatal("issue code", err)
	}
	if _, _, err := s.IssueRecoveryCode(f.ctx, email); !errors.Is(err, auth.ErrCodeCooldown) {
		t.Fatal("resend bypassed cooldown", err)
	}
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	for i := 1; i <= auth.EmailCodeMaxAttempts; i++ {
		err := s.ResetPasswordWithCode(f.ctx, email, wrong, "Changed1&")
		want := auth.ErrCodeInvalid
		if i == auth.EmailCodeMaxAttempts {
			want = auth.ErrCodeAttempts
		}
		if !errors.Is(err, want) {
			t.Fatal("incorrect attempt limit", i, err)
		}
	}
	if err := s.ResetPasswordWithCode(f.ctx, email, code, "Changed1&"); !errors.Is(err, auth.ErrCodeAttempts) {
		t.Fatal("exhausted code still accepted", err)
	}
	if _, err := f.db.Exec(f.ctx, `UPDATE auth_email_codes SET resend_after=now()-interval '1 second' WHERE email=$1`, email); err != nil {
		t.Fatal(err)
	}
	code, _, err = s.IssueRecoveryCode(f.ctx, email)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(f.ctx, `UPDATE auth_email_codes SET expires_at=now()-interval '1 second' WHERE email=$1`, email); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetPasswordWithCode(f.ctx, email, code, "Changed1&"); !errors.Is(err, auth.ErrCodeExpired) {
		t.Fatal("expired code accepted", err)
	}
	code, _, err = s.IssueRecoveryCode(f.ctx, email)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ResetPasswordWithCode(f.ctx, email, code, "Changed1&"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CurrentUser(f.ctx, f.tokens[0]); !errors.Is(err, auth.ErrSession) {
		t.Fatal("password reset left old session active", err)
	}
	if err := s.ResetPasswordWithCode(f.ctx, email, code, "Other1&"); !errors.Is(err, auth.ErrCodeInvalid) {
		t.Fatal("code was reused", err)
	}
	if _, _, _, err := s.Login(f.ctx, email, "Changed1&"); err != nil {
		t.Fatal("new password failed", err)
	}
}
