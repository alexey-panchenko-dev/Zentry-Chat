package httpapi

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"net/http"
	"newsChat/api/internal/auth"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxMessageImage = 4 * 1024 * 1024
const maxImageCaption = 1000

func normalizeImageCaption(value string) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > maxImageCaption {
		return "", errors.New("caption too long")
	}
	return value, nil
}

func normalizeMessageImage(b []byte) ([]byte, error) {
	if len(b) == 0 || len(b) > maxMessageImage {
		return nil, errors.New("invalid image size")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 || int64(cfg.Width)*int64(cfg.Height) > 16000000 {
		return nil, errors.New("invalid image")
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	if out.Len() > maxMessageImage {
		return nil, errors.New("normalized image too large")
	}
	return out.Bytes(), nil
}
func (h *chatHandlers) sendImage(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := conversationID(w, r)
	if !ok {
		return
	}
	b, _, ok := mediaUpload(w, r, "photo", maxMessageImage)
	if !ok {
		return
	}
	clientID := strings.ToLower(r.FormValue("client_message_id"))
	caption, captionErr := normalizeImageCaption(r.FormValue("caption"))
	if !uuidPattern.MatchString(clientID) || captionErr != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_image"})
		return
	}
	b, err := normalizeMessageImage(b)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_image"})
		return
	}
	message, created, err := h.store.SendImage(r.Context(), id, u.ID, clientID, caption, b)
	if err != nil {
		chatFailure(w, err)
		return
	}
	delivery := "disabled"
	if h.publisher.Enabled() {
		delivery = "published"
		if h.publisher.Publish("private-conversation-"+id, "message.created", map[string]any{"message": message}) != nil {
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
func (h *chatHandlers) messageImage(w http.ResponseWriter, r *http.Request, u auth.User) {
	id := r.PathValue("messageID")
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n < 1 {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	b, m, err := h.store.Image(r.Context(), id, u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	serveMedia(w, r, b, m)
}
