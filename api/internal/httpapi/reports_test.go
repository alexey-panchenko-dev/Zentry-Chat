package httpapi

import (
	"fmt"
	"testing"
)

func TestReportsLimitAndMessageAuthorization(t *testing.T) {
	f := newControlFixture(t)
	m, _, err := f.store.Send(f.ctx, f.conv.ID, f.users[1].ID, "13111111-0000-4000-8000-000000000001", "private test")
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"user_id":%q,"scope":"message","message_id":%q,"category":"danger","details":"Test"}`, f.users[1].ID, m.ID)
	f.request(t, -1, "POST", "/api/reports", body, 401)
	f.request(t, 2, "POST", "/api/reports", body, 404)
	f.request(t, 1, "POST", "/api/reports", body, 400)
	f.request(t, 0, "POST", "/api/reports", body, 200)
	f.request(t, 0, "POST", "/api/reports", body, 200)
	var count int
	if err = f.db.QueryRow(f.ctx, `SELECT count(*) FROM abuse_reports WHERE reporter_id=$1`, f.users[0].ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate report: %d %v", count, err)
	}
	for _, subject := range []string{f.users[1].ID, f.users[2].ID} {
		for _, scope := range []string{"user", "discovery"} {
			f.request(t, 0, "POST", "/api/reports", fmt.Sprintf(`{"user_id":%q,"scope":%q,"category":"spam"}`, subject, scope), 200)
		}
	}
	m, _, err = f.store.Send(f.ctx, f.conv.ID, f.users[1].ID, "13111111-0000-4000-8000-000000000002", "second")
	if err != nil {
		t.Fatal(err)
	}
	f.request(t, 0, "POST", "/api/reports", fmt.Sprintf(`{"user_id":%q,"scope":"message","message_id":%q,"category":"spam"}`, f.users[1].ID, m.ID), 429)
	f.request(t, 0, "GET", "/api/reports", "", 405)
}
