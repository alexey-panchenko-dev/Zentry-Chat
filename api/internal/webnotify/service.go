package webnotify

import (
	"context"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	HTTPClient               webpush.HTTPClient
	DB                       *pgxpool.Pool
	Public, Private, Subject string
}

func New(db *pgxpool.Pool) *Service {
	return &Service{DB: db, Public: os.Getenv("VAPID_PUBLIC_KEY"), Private: os.Getenv("VAPID_PRIVATE_KEY"), Subject: "https://zentrychat.vercel.app"}
}
func (s *Service) Enabled() bool {
	public, err := base64.RawURLEncoding.DecodeString(s.Public)
	if err != nil || len(public) != 65 || public[0] != 4 {
		return false
	}
	px, py := elliptic.Unmarshal(elliptic.P256(), public)
	if px == nil || py == nil {
		return false
	}
	private, err := base64.RawURLEncoding.DecodeString(s.Private)
	if err != nil || len(private) != 32 {
		return false
	}
	d := new(big.Int).SetBytes(private)
	curve := elliptic.P256()
	if d.Sign() <= 0 || d.Cmp(curve.Params().N) >= 0 {
		return false
	}
	x, y := curve.ScalarBaseMult(private)
	return x.Cmp(px) == 0 && y.Cmp(py) == 0
}

type Registration struct {
	Endpoint string       `json:"endpoint"`
	Keys     webpush.Keys `json:"keys"`
	Preview  bool         `json:"preview"`
	Groups   bool         `json:"groups"`
	Muted    []string     `json:"muted_chats"`
}

func Valid(v Registration) bool {
	u, e := url.Parse(v.Endpoint)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || len(v.Endpoint) > 2048 {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host != "fcm.googleapis.com" && host != "updates.push.services.mozilla.com" && host != "web.push.apple.com" && !strings.HasSuffix(host, ".notify.windows.com") {
		return false
	}
	dh, e := base64.RawURLEncoding.DecodeString(v.Keys.P256dh)
	if e != nil || len(dh) != 65 || dh[0] != 4 {
		return false
	}
	if x, _ := elliptic.Unmarshal(elliptic.P256(), dh); x == nil {
		return false
	}
	auth, e := base64.RawURLEncoding.DecodeString(v.Keys.Auth)
	return e == nil && len(auth) == 16 && len(v.Muted) <= 500
}

var ErrDeviceLimit = errors.New("push device limit reached")

func (s *Service) Save(ctx context.Context, user, token string, v Registration) error {
	if v.Muted == nil {
		v.Muted = []string{}
	}
	hash := sha256.Sum256([]byte(token))
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,712))`, user); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM push_subscriptions p USING sessions s WHERE p.session_hash=s.token_hash AND s.expires_at<=now() AND p.user_id=$1`, user); err != nil {
		return err
	}
	var count int
	var existing bool
	if err = tx.QueryRow(ctx, `SELECT count(*),COALESCE(bool_or(endpoint=$2),false) FROM push_subscriptions WHERE user_id=$1`, user, v.Endpoint).Scan(&count, &existing); err != nil {
		return err
	}
	if count >= 20 && !existing {
		return ErrDeviceLimit
	}
	_, err = tx.Exec(ctx, `INSERT INTO push_subscriptions(user_id,session_hash,endpoint,p256dh,auth,preview,groups,muted_chats) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(endpoint) DO UPDATE SET user_id=EXCLUDED.user_id,session_hash=EXCLUDED.session_hash,p256dh=EXCLUDED.p256dh,auth=EXCLUDED.auth,preview=EXCLUDED.preview,groups=EXCLUDED.groups,muted_chats=EXCLUDED.muted_chats,updated_at=now()`, user, hash[:], v.Endpoint, v.Keys.P256dh, v.Keys.Auth, v.Preview, v.Groups, v.Muted)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Service) Remove(ctx context.Context, user, endpoint string) error {
	_, e := s.DB.Exec(ctx, `DELETE FROM push_subscriptions WHERE user_id=$1 AND endpoint=$2`, user, endpoint)
	return e
}
func (s *Service) Active(ctx context.Context, user, endpoint, chat string) error {
	var value any
	if chat != "" {
		value = chat
	}
	_, e := s.DB.Exec(ctx, `UPDATE push_subscriptions SET active_chat=$3,active_until=now()+interval '35 seconds' WHERE user_id=$1 AND endpoint=$2`, user, endpoint, value)
	return e
}
func (s *Service) Run(ctx context.Context) {
	if !s.Enabled() {
		return
	}
	timer := time.NewTicker(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			for i := 0; i < 5; i++ {
				if !s.deliver(ctx) {
					break
				}
			}
		}
	}
}
func (s *Service) deliver(ctx context.Context) bool {
	var id, subID, messageID int64
	var scope, chat string
	var attempts int
	err := s.DB.QueryRow(ctx, `UPDATE push_jobs SET locked_until=now()+interval '30 seconds',attempts=attempts+1 WHERE id=(SELECT id FROM push_jobs WHERE available_at<=now() AND (locked_until IS NULL OR locked_until<now()) ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,subscription_id,scope,chat_id::text,message_id,attempts`).Scan(&id, &subID, &scope, &chat, &messageID, &attempts)
	if err != nil {
		return false
	}
	done := func() { s.DB.Exec(ctx, `DELETE FROM push_jobs WHERE id=$1`, id) }
	var sub webpush.Subscription
	var user string
	var preview, groups, active, muted bool
	err = s.DB.QueryRow(ctx, `SELECT p.endpoint,p.p256dh,p.auth,p.user_id::text,p.preview,p.groups,COALESCE(p.active_chat=$2 AND p.active_until>now(),false),$2=ANY(p.muted_chats) FROM push_subscriptions p JOIN sessions s ON s.token_hash=p.session_hash AND s.expires_at>now() WHERE p.id=$1`, subID, chat).Scan(&sub.Endpoint, &sub.Keys.P256dh, &sub.Keys.Auth, &user, &preview, &groups, &active, &muted)
	if err != nil || active || muted || (scope == "group" && !groups) {
		done()
		return true
	}
	var body string
	var allowed bool
	if scope == "personal" {
		err = s.DB.QueryRow(ctx, `SELECT m.body,EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=m.conversation_id AND cm.user_id=$2 AND m.id>cm.cleared_through AND m.id>cm.last_read_id) AND NOT EXISTS(SELECT 1 FROM contact_blocks WHERE (blocker_id=$2 AND blocked_id=m.sender_id) OR (blocker_id=m.sender_id AND blocked_id=$2)) FROM messages m WHERE m.id=$1 AND m.conversation_id=$3`, messageID, user, chat).Scan(&body, &allowed)
	} else {
		err = s.DB.QueryRow(ctx, `SELECT body,EXISTS(SELECT 1 FROM group_members WHERE group_id=$3 AND user_id=$2) FROM group_messages WHERE id=$1 AND group_id=$3`, messageID, user, chat).Scan(&body, &allowed)
	}
	if err != nil || !allowed {
		done()
		return true
	}
	if !preview || body == "" {
		body = "Новое сообщение"
	}
	runes := []rune(body)
	if len(runes) > 120 {
		body = string(runes[:120]) + "…"
	}
	payload, _ := json.Marshal(map[string]string{"title": "Zently", "body": body, "scope": scope, "chat_id": chat, "tag": scope + ":" + chat + ":" + strconv.FormatInt(messageID, 10), "url": "/?chat=" + chat + "&scope=" + scope})
	deadline, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	var transport webpush.HTTPClient = client
	if s.HTTPClient != nil {
		transport = s.HTTPClient
	}
	response, err := webpush.SendNotificationWithContext(deadline, payload, &sub, &webpush.Options{HTTPClient: transport, Subscriber: s.Subject, VAPIDPublicKey: s.Public, VAPIDPrivateKey: s.Private, TTL: 300})
	if response != nil {
		io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		response.Body.Close()
		if response.StatusCode == 404 || response.StatusCode == 410 {
			s.DB.Exec(ctx, `DELETE FROM push_subscriptions WHERE id=$1`, subID)
			return true
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			done()
			return true
		}
	}
	if attempts >= 3 {
		done()
	} else {
		s.DB.Exec(ctx, `UPDATE push_jobs SET available_at=now()+interval '1 minute',locked_until=NULL WHERE id=$1`, id)
	}
	_ = err
	return true
}
