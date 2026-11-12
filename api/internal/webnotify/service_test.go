package webnotify

import (
	"context"
	"encoding/base64"
	"fmt"
	webpush "github.com/SherClockHolmes/webpush-go"
	"io"
	"net/http"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
	"newsChat/api/internal/database"
	"os"
	"strings"
	"testing"
	"time"
)

type fakeTransport struct {
	calls  int
	status int
}

func (f *fakeTransport) Do(r *http.Request) (*http.Response, error) {
	f.calls++
	return &http.Response{StatusCode: f.status, Body: io.NopCloser(strings.NewReader(""))}, nil
}
func registration(t *testing.T) Registration {
	t.Helper()
	_, pub, e := webpush.GenerateVAPIDKeys()
	if e != nil {
		t.Fatal(e)
	}
	return Registration{Endpoint: "https://fcm.googleapis.com/fcm/send/test", Keys: webpush.Keys{P256dh: pub, Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16))}, Groups: true}
}
func TestPushEndpointValidation(t *testing.T) {
	v := registration(t)
	if !Valid(v) {
		t.Fatal("valid subscription rejected")
	}
	for _, endpoint := range []string{"http://fcm.googleapis.com/a", "https://127.0.0.1/a", "https://fcm.googleapis.com.attacker.test/a", "https://user@fcm.googleapis.com/a", "https://fcm.googleapis.com:1234/a", "https://localhost/a"} {
		v.Endpoint = endpoint
		if Valid(v) {
			t.Fatal("unsafe endpoint accepted")
		}
	}
}

func TestPushConfigRequiresMatchingVAPIDKeys(t *testing.T) {
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	if !(&Service{Public: public, Private: private}).Enabled() {
		t.Fatal("valid VAPID pair was disabled")
	}
	otherPrivate, _, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	if (&Service{Public: public, Private: otherPrivate}).Enabled() {
		t.Fatal("mismatched VAPID pair was enabled")
	}
	if (&Service{Public: "invalid", Private: private}).Enabled() {
		t.Fatal("malformed VAPID pair was enabled")
	}
}

func TestPushDeviceLimitIsEnforced(t *testing.T) {
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
	service := &Service{DB: db}
	account, err := auth.New(db).Register(ctx, "Push limit", fmt.Sprintf("push-limit-%d@example.com", time.Now().UnixNano()), "password-12345")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Exec(ctx, "DELETE FROM users WHERE id=$1", account.ID)
	_, token, _, err := auth.New(db).Login(ctx, account.Email, "password-12345")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		v := registration(t)
		v.Endpoint = fmt.Sprintf("%s-%d", v.Endpoint, i)
		if err = service.Save(ctx, account.ID, token, v); err != nil {
			t.Fatalf("device %d was rejected: %v", i+1, err)
		}
	}
	v := registration(t)
	if err = service.Save(ctx, account.ID, token, v); err != ErrDeviceLimit {
		t.Fatalf("21st device: got %v, want ErrDeviceLimit", err)
	}
}

func TestPushQueueDedupSessionAndExpiredSubscription(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	db, e := database.Open(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = database.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	authService := auth.New(db)
	users := make([]auth.User, 2)
	tokens := make([]string, 2)
	defer func() {
		for _, u := range users {
			if u.ID != "" {
				db.Exec(ctx, "DELETE FROM users WHERE id=$1", u.ID)
			}
		}
	}()
	for i := range users {
		email := fmt.Sprintf("push-%d-%d@example.com", time.Now().UnixNano(), i)
		users[i], e = authService.Register(ctx, "Push test", email, "password-12345")
		if e != nil {
			t.Fatal(e)
		}
		_, tokens[i], _, e = authService.Login(ctx, email, "password-12345")
		if e != nil {
			t.Fatal(e)
		}
	}
	priv, pub, e := webpush.GenerateVAPIDKeys()
	if e != nil {
		t.Fatal(e)
	}
	transport := &fakeTransport{status: 201}
	service := &Service{DB: db, Private: priv, Public: pub, Subject: "https://zently.test", HTTPClient: transport}
	sub := registration(t)
	if e = service.Save(ctx, users[1].ID, tokens[1], sub); e != nil {
		t.Fatal(e)
	}
	store := chat.New(db)
	conversation, e := store.CreateConversation(ctx, users[0].ID, users[1].ID)
	if e != nil {
		t.Fatal(e)
	}
	send := func(id string) {
		t.Helper()
		if _, _, e := store.Send(ctx, conversation.ID, users[0].ID, id, "private message"); e != nil {
			t.Fatal(e)
		}
	}
	send("15111111-0000-4000-8000-000000000001")
	send("15111111-0000-4000-8000-000000000001")
	var count int
	if e = db.QueryRow(ctx, "SELECT count(*) FROM push_jobs j JOIN push_subscriptions s ON s.id=j.subscription_id WHERE s.user_id=$1", users[1].ID).Scan(&count); e != nil || count != 1 {
		t.Fatalf("push job duplicated: %d %v", count, e)
	}
	if !service.deliver(ctx) || transport.calls != 1 {
		t.Fatalf("notification not sent: %d", transport.calls)
	}
	if e = service.Active(ctx, users[1].ID, sub.Endpoint, conversation.ID); e != nil {
		t.Fatal(e)
	}
	send("15111111-0000-4000-8000-000000000002")
	service.deliver(ctx)
	if transport.calls != 1 {
		t.Fatal("active conversation notified")
	}
	if e = service.Active(ctx, users[1].ID, sub.Endpoint, ""); e != nil {
		t.Fatal(e)
	}
	transport.status = 410
	send("15111111-0000-4000-8000-000000000003")
	service.deliver(ctx)
	db.QueryRow(ctx, "SELECT count(*) FROM push_subscriptions WHERE user_id=$1", users[1].ID).Scan(&count)
	if count != 0 {
		t.Fatal("expired subscription retained")
	}
	if e = service.Save(ctx, users[1].ID, tokens[1], sub); e != nil {
		t.Fatal(e)
	}
	if e = authService.Logout(ctx, tokens[1]); e != nil {
		t.Fatal(e)
	}
	db.QueryRow(ctx, "SELECT count(*) FROM push_subscriptions WHERE user_id=$1", users[1].ID).Scan(&count)
	if count != 0 {
		t.Fatal("logout retained private push subscription")
	}
}
