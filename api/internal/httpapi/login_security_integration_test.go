package httpapi

import (
	"fmt"
	"testing"
)

func TestLoginFailuresAccumulateAndLockExpires(t *testing.T) {
	f := newControlFixture(t)
	email := f.users[0].Email
	t.Cleanup(func() { _, _ = f.db.Exec(f.ctx, `DELETE FROM auth_login_limits WHERE email=$1`, email) })
	login := func(password string, status int) {
		t.Helper()
		f.request(t, -1, "POST", "/api/auth/login", fmt.Sprintf(`{"email":%q,"password":%q}`, email, password), status)
	}
	login("incorrect", 401)
	login("incorrect", 401)
	login("incorrect", 429)
	// Correct credentials cannot bypass an active lock; peers remain independent.
	login("password-12345", 429)
	f.request(t, -1, "POST", "/api/auth/login", fmt.Sprintf(`{"email":%q,"password":"password-12345"}`, f.users[1].Email), 200)
	if _, err := f.db.Exec(f.ctx, `UPDATE auth_login_limits SET locked_until=now()-interval '1 second' WHERE email=$1`, email); err != nil {
		t.Fatal(err)
	}
	login("password-12345", 200)
	login("incorrect", 401)
	login("password-12345", 200)
}
