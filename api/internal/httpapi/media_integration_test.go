package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
	"newsChat/api/internal/database"
	"os"
	"testing"
	"time"
)

func TestMediaPersistenceAccessAndRetries(t *testing.T) {
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
	service := auth.New(db)
	var users []auth.User
	var tokens []string
	for i := 0; i < 3; i++ {
		email := fmt.Sprintf("media-%d-%d@example.com", time.Now().UnixNano(), i)
		u, e := service.Register(ctx, "Media", email, "password-12345")
		if e != nil {
			t.Fatal(e)
		}
		users = append(users, u)
		_, token, _, e := service.Login(ctx, email, "password-12345")
		if e != nil {
			t.Fatal(e)
		}
		tokens = append(tokens, token)
	}
	defer func() {
		for _, u := range users {
			db.Exec(ctx, "DELETE FROM users WHERE id=$1", u.ID)
		}
	}()
	store := chat.New(db)
	c, err := store.CreateConversation(ctx, users[0].ID, users[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := store.CreateConversation(ctx, users[0].ID, users[2].ID)
	if err != nil {
		t.Fatal(err)
	}
	pub := &testPublisher{}
	server := httptest.NewServer(newRouter(db, pub))
	defer server.Close()
	request := func(actor int, method, path, field, typ string, b []byte, fields map[string]string, status int, rangeHeader string) ([]byte, http.Header) {
		t.Helper()
		var body bytes.Buffer
		contentType := ""
		if field != "" {
			w := multipart.NewWriter(&body)
			h := textproto.MIMEHeader{}
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="media"`, field))
			h.Set("Content-Type", typ)
			part, _ := w.CreatePart(h)
			part.Write(b)
			for k, v := range fields {
				w.WriteField(k, v)
			}
			w.Close()
			contentType = w.FormDataContentType()
		}
		req, _ := http.NewRequest(method, server.URL+path, &body)
		if actor >= 0 {
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tokens[actor]})
		}
		req.Header.Set("Content-Type", contentType)
		if rangeHeader != "" {
			req.Header.Set("Range", rangeHeader)
		}
		res, e := server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode != status {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, res.StatusCode, status, data)
		}
		return data, res.Header
	}
	audio := audioFixture(t, "silence.webm")
	fields := map[string]string{"duration_ms": "1000", "client_message_id": "a1b2c3d4-1234-4234-8234-123456789abc"}
	path := "/api/conversations/" + c.ID + "/voice"
	data, _ := request(0, "POST", path, "audio", "audio/webm;codecs=opus", audio, fields, 201, "")
	var result struct {
		Message chat.Message `json:"message"`
	}
	json.Unmarshal(data, &result)
	for _, column := range []string{"audio", "audio_mime", "duration_ms"} {
		if _, err := db.Exec(ctx, "UPDATE messages SET "+column+"=NULL WHERE id=$1", result.Message.ID); err == nil {
			t.Fatalf("database accepted NULL voice %s", column)
		}
	}
	if result.Message.Kind != "voice" || result.Message.DurationMS != 1000 {
		t.Fatalf("bad voice %+v", result)
	}
	request(0, "POST", path, "audio", "audio/webm", audio, fields, 200, "")
	fields["duration_ms"] = "2000"
	request(0, "POST", path, "audio", "audio/webm", audio, fields, 409, "")
	fields["duration_ms"] = "1000"
	alternateAudio := append([]byte(nil), audio...)
	alternateAudio[len(alternateAudio)-1] ^= 1
	request(0, "POST", path, "audio", "audio/webm", alternateAudio, fields, 409, "")
	request(0, "POST", "/api/conversations/"+c2.ID+"/voice", "audio", "audio/webm", audio, fields, 409, "")
	request(2, "GET", result.Message.AudioURL, "", "", nil, nil, 404, "")
	request(-1, "GET", result.Message.AudioURL, "", "", nil, nil, 401, "")
	b, h := request(1, "GET", result.Message.AudioURL, "", "", nil, nil, 206, "bytes=1-3")
	if !bytes.Equal(b, audio[1:4]) || h.Get("Content-Type") != "audio/webm" {
		t.Fatal("range/mime mismatch")
	}
	list, e := store.Messages(ctx, c.ID, users[1].ID, 0, 20)
	if e != nil || len(list) != 1 || list[0].AudioURL == "" {
		t.Fatalf("persisted list: %+v %v", list, e)
	}
	if _, _, e := store.Send(ctx, c.ID, users[0].ID, fields["client_message_id"], "text"); e != chat.ErrConflict {
		t.Fatalf("voice UUID accepted as text: %v", e)
	}
	textID := "c1b2c3d4-1234-4234-8234-123456789abc"
	if _, _, e := store.Send(ctx, c.ID, users[0].ID, textID, "text"); e != nil {
		t.Fatal(e)
	}
	if _, _, e := store.SendVoice(ctx, c.ID, users[0].ID, textID, "audio/webm", audio, 1000); e != chat.ErrConflict {
		t.Fatalf("text UUID accepted as voice: %v", e)
	}
	fields["client_message_id"] = "b1b2c3d4-1234-4234-8234-123456789abc"
	request(0, "POST", path, "audio", "audio/webm", []byte("bad"), fields, 400, "")
	fields["duration_ms"] = "120001"
	request(0, "POST", path, "audio", "audio/webm", audio, fields, 400, "")
	fields["duration_ms"] = "1000"
	request(0, "POST", path, "audio", "audio/webm", make([]byte, maxVoice+1), fields, 400, "")
	var photo bytes.Buffer
	photoImage := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	photoRandom := rand.New(rand.NewSource(42))
	for i := range photoImage.Pix {
		photoImage.Pix[i] = byte(photoRandom.Intn(256))
	}
	if err := png.Encode(&photo, photoImage); err != nil {
		t.Fatal(err)
	}
	photoURL := "/api/profiles/" + users[0].ID + "/photo"
	uploaded, _ := request(0, "POST", "/api/profile/photo", "photo", "image/png", photo.Bytes(), nil, 200, "")
	var avatar struct {
		URL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(uploaded, &avatar); err != nil {
		t.Fatal(err)
	}
	photoURL = avatar.URL
	started := time.Now()
	original, originalHeaders := request(1, "GET", photoURL, "", "", nil, nil, 200, "")
	originalTime := time.Since(started)
	started = time.Now()
	thumbnail, thumbnailHeaders := request(1, "GET", photoURL+"&size=128", "", "", nil, nil, 200, "")
	thumbnailTime := time.Since(started)
	if len(thumbnail) >= len(original)/2 {
		t.Fatalf("thumbnail too large: original=%d thumbnail=%d", len(original), len(thumbnail))
	}
	if thumbnailHeaders.Get("Cache-Control") != "private, max-age=31536000, immutable" || thumbnailHeaders.Get("ETag") != originalHeaders.Get("ETag") {
		t.Fatalf("avatar cache headers original=%v thumbnail=%v", originalHeaders, thumbnailHeaders)
	}
	conditional, _ := http.NewRequest("GET", server.URL+photoURL+"&size=128", nil)
	conditional.AddCookie(&http.Cookie{Name: sessionCookie, Value: tokens[1]})
	conditional.Header.Set("If-None-Match", thumbnailHeaders.Get("ETag"))
	conditionalResponse, err := server.Client().Do(conditional)
	if err != nil {
		t.Fatal(err)
	}
	conditionalResponse.Body.Close()
	if conditionalResponse.StatusCode != http.StatusNotModified {
		t.Fatalf("conditional avatar request status=%d", conditionalResponse.StatusCode)
	}
	t.Logf("avatar network baseline: %d bytes, %s; cached thumbnail: %d bytes, %s (%.1f%% smaller); validator: 304", len(original), originalTime, len(thumbnail), thumbnailTime, 100*(1-float64(len(thumbnail))/float64(len(original))))
	request(-1, "GET", photoURL, "", "", nil, nil, 401, "")
	current, e := service.CurrentUser(ctx, tokens[0])
	if e != nil || current.AvatarURL != photoURL {
		t.Fatalf("current avatar %v %v", current, e)
	}
	found, e := store.Conversations(ctx, users[1].ID, 0, 20)
	if e != nil || found[0].Participant.AvatarURL != photoURL {
		t.Fatalf("peer avatar %v %v", found, e)
	}
	replaced, _ := request(0, "POST", "/api/profile/photo", "photo", "image/png", photo.Bytes(), nil, 200, "")
	var replacement struct {
		URL string `json:"avatar_url"`
	}
	json.Unmarshal(replaced, &replacement)
	if replacement.URL == photoURL {
		t.Fatal("replacement avatar URL must change")
	}
	photoURL = replacement.URL
	current, e = service.CurrentUser(ctx, tokens[0])
	if e != nil || current.AvatarURL != photoURL {
		t.Fatalf("replacement current avatar %v %v", current, e)
	}
	searched, e := store.SearchUsers(ctx, users[1].ID, users[0].Email)
	if e != nil || len(searched) != 1 || searched[0].AvatarURL != photoURL {
		t.Fatalf("replacement search avatar %v %v", searched, e)
	}
	peer, e := store.CreateConversation(ctx, users[1].ID, users[0].ID)
	if e != nil || peer.Participant.AvatarURL != photoURL {
		t.Fatalf("replacement created avatar %v %v", peer, e)
	}
	logged, _, _, e := service.Login(ctx, users[0].Email, "password-12345")
	if e != nil || logged.AvatarURL != photoURL {
		t.Fatalf("replacement login avatar %v %v", logged, e)
	}

	request(0, "POST", "/api/profile/photo", "photo", "image/svg+xml", []byte("<svg/>"), nil, 400, "")
	request(1, "DELETE", "/api/profile/photo", "", "", nil, nil, 200, "")
	request(1, "GET", photoURL, "", "", nil, nil, 200, "")
	request(0, "DELETE", "/api/profile/photo", "", "", nil, nil, 200, "")
	request(1, "GET", photoURL, "", "", nil, nil, 404, "")
	current, e = service.CurrentUser(ctx, tokens[0])
	if e != nil || current.AvatarURL != "" {
		t.Fatalf("deleted avatar %v %v", current, e)
	}
}
