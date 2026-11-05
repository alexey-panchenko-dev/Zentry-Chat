package auth

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"newsChat/api/internal/database"
	"os"
	"sync"
	"testing"
	"time"
)

func TestRecoveryTokenLifecycle(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	email := fmt.Sprintf("recovery-%d@example.com", time.Now().UnixNano())
	user, err := s.Register(ctx, "Recovery Test", email, "old-password-12345")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID)
	_, session, _, err := s.Login(ctx, email, "old-password-12345")
	if err != nil {
		t.Fatal(err)
	}
	if token, err := s.CreateReset(ctx, "missing-"+email); err != nil || token != "" {
		t.Fatal("missing account must be a no-op", err)
	}
	token, err := s.CreateReset(ctx, email)
	if err != nil || len(token) != 64 {
		t.Fatal("reset creation", err)
	}
	var stored []byte
	if err = db.QueryRow(ctx, `SELECT token_hash FROM password_resets WHERE user_id=$1`, user.ID).Scan(&stored); err != nil || len(stored) != 32 || string(stored) == token {
		t.Fatal("hash storage", err)
	}
	if again, err := s.CreateReset(ctx, email); err != nil || again != "" {
		t.Fatal("cooldown", err)
	}
	// Redeeming concurrently must change the password exactly once.
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- s.ResetPassword(ctx, token, "new-password-12345") }()
	}
	wg.Wait()
	close(results)
	successes, invalid := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrReset) {
			invalid++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || invalid != 1 {
		t.Fatalf("success=%d invalid=%d", successes, invalid)
	}
	if _, err = s.CurrentUser(ctx, session); !errors.Is(err, ErrSession) {
		t.Fatal("old session still active", err)
	}
	if _, _, _, err = s.Login(ctx, email, "old-password-12345"); !errors.Is(err, ErrCredentials) {
		t.Fatal("old password still works", err)
	}
	if _, _, _, err = s.Login(ctx, email, "new-password-12345"); err != nil {
		t.Fatal("new password rejected", err)
	}
	token, err = s.CreateReset(ctx, email)
	if err != nil || token == "" {
		t.Fatal("create expiry token", err)
	}
	_, err = db.Exec(ctx, `UPDATE password_resets SET created_at=now()-interval '16 minutes',expires_at=now()-interval '1 minute' WHERE user_id=$1`, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ResetPassword(ctx, token, "unused-password-123"); !errors.Is(err, ErrReset) {
		t.Fatal("expired token accepted", err)
	}
}

func TestLoginCannotIssueSessionUsingPasswordChangedWhileWaiting(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db)
	email := fmt.Sprintf("reset-race-%d@example.com", time.Now().UnixNano())
	u, err := s.Register(ctx, "Race Test", email, "old-password-12345")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, u.ID)
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, u.ID); err != nil {
		t.Fatal(err)
	}
	app := fmt.Sprintf("reset-race-%d", time.Now().UnixNano())
	loginDB, err := database.Open(ctx, url+"&application_name="+app)
	if err != nil {
		t.Fatal(err)
	}
	defer loginDB.Close()
	loginService := New(loginDB)
	done := make(chan error, 1)
	go func() { _, _, _, err := loginService.Login(ctx, email, "old-password-12345"); done <- err }()
	// Wait for the actual database lock, not an arbitrary scheduler delay.
	for {
		var waiting bool
		if err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock')`, app).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("login did not wait on account lock")
		case <-time.After(10 * time.Millisecond):
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("new-password-12345"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$1 WHERE id=$2`, string(hash), u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, u.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-done; !errors.Is(err, ErrCredentials) {
		t.Fatalf("old login issued session after password change: %v", err)
	}
}
