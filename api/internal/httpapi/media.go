package httpapi

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io"
	"mime"
	"net/http"
	"newsChat/api/internal/auth"
	"strconv"
	"strings"
	"time"
)

const maxVoice = 3 * 1024 * 1024
const maxPhoto = 2 * 1024 * 1024

func validateAudio(contentType string, b []byte) error {
	m, _, err := mime.ParseMediaType(contentType)
	if err != nil || len(b) == 0 || len(b) > maxVoice {
		return errors.New("invalid audio")
	}
	switch m {
	case "audio/webm":
		return validateWebM(b)
	case "audio/ogg":
		return validateOgg(b)
	case "audio/mp4":
		return validateMP4(b)
	}
	return errAudioContainer
}
func normalizePhoto(b []byte) ([]byte, string, error) {
	if len(b) > maxPhoto {
		return nil, "", errors.New("photo too large")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 || int64(cfg.Width)*int64(cfg.Height) > 16000000 {
		return nil, "", errors.New("invalid photo")
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, "", err
	}
	if bounds := img.Bounds(); bounds.Dx() != bounds.Dy() {
		side := min(bounds.Dx(), bounds.Dy())
		origin := bounds.Min.Add(image.Pt((bounds.Dx()-side)/2, (bounds.Dy()-side)/2))
		square := image.NewNRGBA(image.Rect(0, 0, side, side))
		draw.Draw(square, square.Bounds(), img, origin, draw.Src)
		img = square
	}
	var out bytes.Buffer
	if err = png.Encode(&out, img); err != nil {
		return nil, "", err
	}
	if out.Len() > maxPhoto {
		return nil, "", errors.New("normalized photo too large")
	}
	return out.Bytes(), "image/png", nil
}

// avatarRendition is generated only when a profile photo changes. Its box
// filter averages source pixels, preserving transparency while avoiding work
// on every later image request.
func avatarRendition(b []byte, maxDimension int) ([]byte, error) {
	source, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	bounds := source.Bounds()
	w, h := min(maxDimension, bounds.Dx()), min(maxDimension, bounds.Dy())
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0, y1 := bounds.Min.Y+y*bounds.Dy()/h, bounds.Min.Y+(y+1)*bounds.Dy()/h
		for x := 0; x < w; x++ {
			x0, x1 := bounds.Min.X+x*bounds.Dx()/w, bounds.Min.X+(x+1)*bounds.Dx()/w
			var r, g, bl, a uint64
			count := uint64((x1 - x0) * (y1 - y0))
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					c := color.RGBAModel.Convert(source.At(sx, sy)).(color.RGBA)
					r += uint64(c.R)
					g += uint64(c.G)
					bl += uint64(c.B)
					a += uint64(c.A)
				}
			}
			dst.SetRGBA(x, y, color.RGBA{R: uint8(r / count), G: uint8(g / count), B: uint8(bl / count), A: uint8(a / count)})
		}
	}
	var out bytes.Buffer
	if err = png.Encode(&out, dst); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func mediaUpload(w http.ResponseWriter, r *http.Request, field string, limit int) ([]byte, string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(limit+64*1024))
	if err := r.ParseMultipartForm(int64(limit + 64*1024)); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_media"})
		return nil, "", false
	}
	defer r.MultipartForm.RemoveAll()
	if len(r.MultipartForm.File[field]) != 1 {
		writeJSON(w, 400, map[string]string{"error": "invalid_media"})
		return nil, "", false
	}
	f, h, err := r.FormFile(field)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_media"})
		return nil, "", false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, int64(limit+1)))
	if err != nil || len(b) == 0 || len(b) > limit {
		writeJSON(w, 400, map[string]string{"error": "invalid_media"})
		return nil, "", false
	}
	return b, h.Header.Get("Content-Type"), true
}
func (h *chatHandlers) sendVoice(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := conversationID(w, r)
	if !ok {
		return
	}
	b, m, ok := mediaUpload(w, r, "audio", maxVoice)
	if !ok {
		return
	}
	duration, err := strconv.Atoi(r.FormValue("duration_ms"))
	clientID := strings.ToLower(r.FormValue("client_message_id"))
	if err != nil || duration < 1 || duration > 120000 || !uuidPattern.MatchString(clientID) || validateAudio(m, b) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_voice"})
		return
	}
	m, _, _ = mime.ParseMediaType(m)
	// duration_ms is recorder-supplied metadata, bounded but not a verified media duration.
	message, created, err := h.store.SendVoice(r.Context(), id, u.ID, clientID, m, b, duration)
	if err != nil {
		chatFailure(w, err)
		return
	}
	delivery := "disabled"
	if h.publisher.Enabled() {
		delivery = "published"
		if h.publisher.Publish("private-conversation-"+id, "message.created", map[string]string{"conversation_id": id}) != nil {
			delivery = "failed"
		}
	}
	if members, err := h.store.Members(r.Context(), id); err == nil {
		h.publishConversationUpdate("conversation.message", id, members...)
	}
	status := 200
	if created {
		status = 201
	}
	writeJSON(w, status, map[string]any{"message": message, "realtime": delivery})
}
func serveMedia(w http.ResponseWriter, r *http.Request, b []byte, m string) {
	w.Header().Set("Content-Type", m)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(b))
}
func (h *chatHandlers) audio(w http.ResponseWriter, r *http.Request, u auth.User) {
	id := r.PathValue("messageID")
	n, e := strconv.ParseInt(id, 10, 64)
	if e != nil || n < 1 {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	b, m, err := h.store.Audio(r.Context(), id, u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	serveMedia(w, r, b, m)
}
func (h *chatHandlers) setPhoto(w http.ResponseWriter, r *http.Request, u auth.User) {
	b, _, ok := mediaUpload(w, r, "photo", maxPhoto)
	if !ok {
		return
	}
	b, m, err := normalizePhoto(b)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_photo"})
		return
	}
	profile, err := avatarRendition(b, 512)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_photo"})
		return
	}
	thumbnail, err := avatarRendition(b, 128)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_photo"})
		return
	}
	avatarURL, err := h.store.SetPhoto(r.Context(), u.ID, b, m, profile, thumbnail)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"avatar_url": avatarURL})
}
func (h *chatHandlers) deletePhoto(w http.ResponseWriter, r *http.Request, u auth.User) {
	if err := h.store.DeletePhoto(r.Context(), u.ID); err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"avatar_url": ""})
}
func (h *chatHandlers) photo(w http.ResponseWriter, r *http.Request, u auth.User) {
	id := strings.ToLower(r.PathValue("userID"))
	if !uuidPattern.MatchString(id) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	size := r.URL.Query().Get("size")
	if size != "" && size != "128" && size != "512" {
		writeJSON(w, 400, map[string]string{"error": "invalid_photo_size"})
		return
	}
	b, m, revision, err := h.store.Photo(r.Context(), id, size)
	if err != nil {
		chatFailure(w, err)
		return
	}
	serveAvatar(w, r, b, m, revision)
}

func serveAvatar(w http.ResponseWriter, r *http.Request, b []byte, mime, revision string) {
	etag := `"` + revision + `"`
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("ETag", etag)
	if r.URL.Query().Get("v") == revision {
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "private, no-cache")
	}
	for _, candidate := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag || candidate == "W/"+etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	http.ServeContent(w, r, "avatar", time.Time{}, bytes.NewReader(b))
}
