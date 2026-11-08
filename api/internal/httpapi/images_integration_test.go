package httpapi

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"newsChat/api/internal/chat"
	"testing"
)

func TestMessageImagesPermissionsAndControls(t *testing.T) {
	f := newControlFixture(t)
	img := image.NewRGBA(image.Rect(0, 0, 20, 16))
	img.Set(0, 0, color.RGBA{255, 100, 20, 255})
	var jpegData bytes.Buffer
	if err := jpeg.Encode(&jpegData, img, nil); err != nil {
		t.Fatal(err)
	}
	upload := func(actor int, c, id string, data []byte, want int) chat.Message {
		t.Helper()
		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		part, _ := mw.CreateFormFile("photo", "gallery.jpg")
		_, _ = part.Write(data)
		_ = mw.WriteField("client_message_id", id)
		_ = mw.Close()
		req, _ := http.NewRequest("POST", f.server.URL+"/api/conversations/"+c+"/images", &b)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		if actor >= 0 {
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.tokens[actor]})
		}
		res, err := f.server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("upload actor%d status%d want%d %s", actor, res.StatusCode, want, raw)
		}
		var result struct {
			Message chat.Message `json:"message"`
		}
		_ = json.Unmarshal(raw, &result)
		return result.Message
	}
	get := func(actor int, url string, want int) {
		t.Helper()
		req, _ := http.NewRequest("GET", f.server.URL+url, nil)
		if actor >= 0 {
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.tokens[actor]})
		}
		res, err := f.server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("get actor%d status%d want%d %s", actor, res.StatusCode, want, raw)
		}
		if want == 200 {
			if res.Header.Get("Cache-Control") != "private, no-store" || res.Header.Get("Content-Type") != "image/jpeg" {
				t.Fatal(res.Header)
			}
			if _, err = jpeg.Decode(bytes.NewReader(raw)); err != nil {
				t.Fatal(err)
			}
		}
	}
	listImages := func(actor int, c, before string, want int) []chat.Message {
		t.Helper()
		path := "/api/conversations/" + c + "/images?limit=12"
		if before != "" {
			path += "&before=" + before
		}
		req, _ := http.NewRequest("GET", f.server.URL+path, nil)
		if actor >= 0 {
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.tokens[actor]})
		}
		res, err := f.server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("image list actor%d status%d want%d: %s", actor, res.StatusCode, want, raw)
		}
		var result struct {
			Images []chat.Message `json:"images"`
		}
		if want == http.StatusOK {
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
		}
		return result.Images
	}
	const id = "ec667c9a-1c80-40f3-8287-ef304d5c3451"
	upload(-1, f.conv.ID, id, jpegData.Bytes(), 401)
	upload(2, f.conv.ID, id, jpegData.Bytes(), 404)
	m := upload(0, f.conv.ID, id, jpegData.Bytes(), 201)
	if m.Kind != "image" || m.ImageURL == "" {
		t.Fatalf("image message %+v", m)
	}
	if got := listImages(0, f.conv.ID, "", 200); len(got) != 1 || got[0].ImageURL != m.ImageURL {
		t.Fatalf("photo gallery for sender: %+v", got)
	}
	if got := listImages(1, f.conv.ID, "", 200); len(got) != 1 || got[0].ImageURL != m.ImageURL {
		t.Fatalf("photo gallery for peer: %+v", got)
	}
	listImages(-1, f.conv.ID, "", 401)
	listImages(2, f.conv.ID, "", 404)
	retry := upload(0, f.conv.ID, id, jpegData.Bytes(), 200)
	if retry.ID != m.ID {
		t.Fatal("retry created duplicate")
	}
	if _, _, err := f.store.Send(f.ctx, f.conv.ID, f.users[0].ID, id, ""); err == nil {
		t.Fatalf("text reused image UUID: %v", err)
	}
	get(-1, m.ImageURL, 401)
	get(2, m.ImageURL, 404)
	get(0, m.ImageURL, 200)
	get(1, m.ImageURL, 200)
	messages, err := f.store.Messages(f.ctx, f.conv.ID, f.users[1].ID, 0, 10)
	if err != nil || len(messages) != 1 || messages[0].ImageURL != m.ImageURL {
		t.Fatalf("messages %+v %v", messages, err)
	}
	changed := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var other bytes.Buffer
	_ = jpeg.Encode(&other, changed, nil)
	upload(0, f.conv.ID, id, other.Bytes(), 409)
	if _, _, err = f.store.Send(f.ctx, f.conv.ID, f.users[0].ID, "ec667c9a-1c80-40f3-8287-ef304d5c3452", "text"); err != nil {
		t.Fatal(err)
	}
	upload(0, f.conv.ID, "ec667c9a-1c80-40f3-8287-ef304d5c3452", jpegData.Bytes(), 409)
	c, err := f.store.CreateConversation(f.ctx, f.users[0].ID, f.users[2].ID)
	if err != nil {
		t.Fatal(err)
	}
	upload(0, c.ID, id, jpegData.Bytes(), 409)
	if _, err = f.store.ChangeControls(f.ctx, f.conv.ID, f.users[0].ID, "clear"); err != nil {
		t.Fatal(err)
	}
	if got := listImages(0, f.conv.ID, "", 200); len(got) != 0 {
		t.Fatalf("cleared photo remained in gallery: %+v", got)
	}
	if got := listImages(1, f.conv.ID, "", 200); len(got) != 1 || got[0].ImageURL != m.ImageURL {
		t.Fatalf("other participant should retain photo: %+v", got)
	}
	get(0, m.ImageURL, 404)
	get(1, m.ImageURL, 200)
	upload(0, f.conv.ID, id, jpegData.Bytes(), 404)
	if _, err = f.store.ChangeControls(f.ctx, f.conv.ID, f.users[1].ID, "block"); err != nil {
		t.Fatal(err)
	}
	upload(0, f.conv.ID, "ec667c9a-1c80-40f3-8287-ef304d5c3453", jpegData.Bytes(), 403)
	upload(1, f.conv.ID, "ec667c9a-1c80-40f3-8287-ef304d5c3453", jpegData.Bytes(), 403)
	if _, err = f.store.ChangeControls(f.ctx, f.conv.ID, f.users[1].ID, "unblock"); err != nil {
		t.Fatal(err)
	}
	upload(0, f.conv.ID, "ec667c9a-1c80-40f3-8287-ef304d5c3453", jpegData.Bytes(), 201)
	upload(0, f.conv.ID, "ec667c9a-1c80-40f3-8287-ef304d5c3454", []byte("not a photo"), 400)
	upload(0, f.conv.ID, "ec667c9a-1c80-40f3-8287-ef304d5c3454", make([]byte, maxMessageImage+1), 400)
}
