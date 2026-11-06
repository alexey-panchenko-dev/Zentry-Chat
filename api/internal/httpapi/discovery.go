package httpapi

import (
	"net/http"
	"newsChat/api/internal/auth"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func (h *chatHandlers) getProfile(w http.ResponseWriter, r *http.Request, u auth.User) {
	p, err := h.store.OwnProfile(r.Context(), u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"profile": p})
}
func (h *chatHandlers) updateProfile(w http.ResponseWriter, r *http.Request, u auth.User) {
	var input struct {
		BirthDate *string `json:"birth_date"`
		Name      string  `json:"name"`
		Age       *int    `json:"age"`
		Bio       string  `json:"bio"`
		City      string  `json:"city"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Bio = strings.TrimSpace(input.Bio)
	input.City = strings.TrimSpace(input.City)
	for _, value := range []string{input.Name, input.Bio, input.City} {
		if !utf8.ValidString(value) || strings.ContainsRune(value, 0) || utf8.RuneCountInString(value) > 100 {
			writeJSON(w, 400, map[string]string{"error": "invalid_profile"})
			return
		}
	}
	if input.Name == "" || input.Age != nil && (*input.Age < 0 || int64(*input.Age) > 2147483647) {
		writeJSON(w, 400, map[string]string{"error": "invalid_profile"})
		return
	}
	if input.BirthDate != nil {
		if _, err := auth.ParseBirthDate(*input.BirthDate, time.Now()); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid_birth_date"})
			return
		}
	}
	_, err := h.store.UpdateProfileWithBirthDate(r.Context(), u.ID, input.Name, input.Age, input.Bio, input.City, input.BirthDate)
	if err != nil {
		chatFailure(w, err)
		return
	}
	p, err := h.store.OwnProfile(r.Context(), u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"profile": p})
}
func (h *chatHandlers) discovery(w http.ResponseWriter, r *http.Request, u auth.User) {
	profiles, err := h.store.Discovery(r.Context(), u.ID, r.URL.Path == "/api/discovery/likes")
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"profiles": profiles})
}
func (h *chatHandlers) vote(w http.ResponseWriter, r *http.Request, u auth.User) {
	var input struct {
		UserID string `json:"user_id"`
		Liked  *bool  `json:"liked"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	input.UserID = strings.ToLower(input.UserID)
	if input.Liked == nil || !uuidPattern.MatchString(input.UserID) || input.UserID == u.ID {
		writeJSON(w, 400, map[string]string{"error": "invalid_vote"})
		return
	}
	id, err := h.store.Vote(r.Context(), u.ID, input.UserID, *input.Liked)
	if err != nil {
		chatFailure(w, err)
		return
	}
	for _, user := range []string{u.ID, input.UserID} {
		if h.publisher.Enabled() {
			_ = h.publisher.Publish("private-user-"+user, "discovery.updated", map[string]any{})
		}
	}
	if id != "" {
		h.notifyConversation(id, u.ID, input.UserID)
	}
	writeJSON(w, 200, map[string]any{"matched": id != "", "conversation_id": id})
}
func (h *chatHandlers) unread(w http.ResponseWriter, r *http.Request, u auth.User) {
	count, err := h.store.UnreadChats(r.Context(), u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]int{"unread_chats": count})
}
func (h *chatHandlers) readState(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := conversationID(w, r)
	if !ok {
		return
	}
	state, err := h.store.ReadState(r.Context(), id, u.ID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, state)
}
func (h *chatHandlers) markRead(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := conversationID(w, r)
	if !ok {
		return
	}
	var input struct {
		LastMessageID string `json:"last_message_id"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	through, err := strconv.ParseInt(input.LastMessageID, 10, 64)
	if err != nil || through < 1 {
		writeJSON(w, 400, map[string]string{"error": "invalid_message_id"})
		return
	}
	cursor, users, err := h.store.MarkRead(r.Context(), id, u.ID, through)
	if err != nil {
		chatFailure(w, err)
		return
	}
	if h.publisher.Enabled() {
		for _, user := range users {
			_ = h.publisher.Publish("private-user-"+user, "conversation.read", map[string]string{"conversation_id": id, "user_id": u.ID, "last_read_id": cursor})
		}
	}
	writeJSON(w, 200, map[string]string{"last_read_id": cursor})
}
