package auth

import (
	"context"
	"fmt"
	"newsChat/api/internal/database"
	"os"
	"testing"
	"time"
)

func TestBirthDateValidation(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	for _, s := range []string{"2000-02-29", "1900-01-01", "2012-11-06"} {
		if _, err := ParseBirthDate(s, now); err != nil {
			t.Fatalf("valid %s: %v", s, err)
		}
	}
	for _, s := range []string{"1900-02-29", "2025-02-29", "2012-11-07", "2026-11-06", "2026-11-07", "1899-12-31", "2000-2-29", ""} {
		if _, err := ParseBirthDate(s, now); err == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}

func TestBirthDateRegistrationRejectsWithoutOrphan(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	svc := New(db)
	email := fmt.Sprintf("invalid-birth-%d@example.com", time.Now().UnixNano())
	bad := "2001-02-29"
	if _, err = svc.RegisterWithBirthDate(ctx, "Invalid", email, "password-12345", &bad); err != ErrBirthDate {
		t.Fatalf("invalid date: %v", err)
	}
	var count int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM users WHERE email=$1`, email).Scan(&count); err != nil || count != 0 {
		t.Fatalf("orphan: %d %v", count, err)
	}
}
