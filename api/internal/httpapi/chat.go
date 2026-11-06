package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
	"newsChat/api/internal/realtime"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var socketPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)

type chatHandlers struct {
	store     *chat.Store
	auth      *auth.Service
	publisher realtime.Publisher
}

type signedInHandler func(http.ResponseWriter, *http.Request, auth.User)

func (h *chatHandlers) signedIn(next signedInHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		user, err := h.auth.CurrentUser(r.Context(), cookie.Value)
		if errors.Is(err, auth.ErrSession) {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		if err != nil {
			chatFailure(w, err)
			return
		}
		next(w, r, user)
	}
}

func (h *chatHandlers) searchUsers(w http.ResponseWriter, r *http.Request, user auth.User) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if !utf8.ValidString(query) || strings.ContainsRune(query, 0) || utf8.RuneCountInString(query) < 2 || utf8.RuneCountInString(query) > 100 {
		writeJSON(w, 400, map[string]string{"error": "invalid_query", "message": "query: от 2 до 100 символов"})
		return
	}
	users, err := h.store.SearchUsers(r.Context(), user.ID, query)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"users": users})
}

func (h *chatHandlers) createConversation(w http.ResponseWriter, r *http.Request, user auth.User) {
	var input struct {
		UserID string `json:"user_id"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	input.UserID = strings.ToLower(input.UserID)
	if !uuidPattern.MatchString(input.UserID) || input.UserID == user.ID {
		writeJSON(w, 400, map[string]string{"error": "invalid_user_id"})
		return
	}
	c, err := h.store.CreateConversation(r.Context(), user.ID, input.UserID)
	if err != nil {
		chatFailure(w, err)
		return
	}
	// Creation is idempotent: an existing pair returns the same dialog.
	h.notifyConversation(c.ID, user.ID, input.UserID)
	writeJSON(w, 200, map[string]any{"conversation": c})
}

// Personal channels let clients discover dialogs they have not opened yet.
// Only IDs are published; PostgreSQL remains the source of conversation data.
func (h *chatHandlers) notifyConversation(id string, users ...string) {
	h.publishConversationUpdate("conversation.updated", id, users...)
}

func (h *chatHandlers) publishConversationUpdate(event, id string, users ...string) {
	if !h.publisher.Enabled() {
		return
	}
	for _, userID := range users {
		if err := h.publisher.Publish("private-user-"+userID, event, map[string]string{"conversation_id": id}); err != nil {
			log.Println("Pusher: не удалось обновить список переписок; данные сохранены в PostgreSQL")
		}
	}
}

func (h *chatHandlers) listConversations(w http.ResponseWriter, r *http.Request, user auth.User) {
	before, limit, ok := pagination(w, r)
	if !ok {
		return
	}
	list, err := h.store.Conversations(r.Context(), user.ID, before, limit+1)
	if err != nil {
		chatFailure(w, err)
		return
	}
	cursor := ""
	if len(list) > limit {
		list = list[:limit]
		cursor = list[len(list)-1].Position
	}
	writeJSON(w, 200, map[string]any{"conversations": list, "next_cursor": cursor})
}

func (h *chatHandlers) listMessages(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := conversationID(w, r)
	if !ok {
		return
	}
	before, limit, ok := pagination(w, r)
	if !ok {
		return
	}
	list, err := h.store.Messages(r.Context(), id, user.ID, before, limit+1)
	if err != nil {
		chatFailure(w, err)
		return
	}
	cursor := ""
	if len(list) > limit {
		list = list[:limit]
		cursor = list[len(list)-1].ID
	}
	writeJSON(w, 200, map[string]any{"messages": list, "next_cursor": cursor})
}

func (h *chatHandlers) sendMessage(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := conversationID(w, r)
	if !ok {
		return
	}
	var input struct {
		ClientMessageID string `json:"client_message_id"`
		Body            string `json:"body"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	input.ClientMessageID = strings.ToLower(input.ClientMessageID)
	input.Body = strings.TrimSpace(input.Body)
	if !uuidPattern.MatchString(input.ClientMessageID) || !utf8.ValidString(input.Body) || utf8.RuneCountInString(input.Body) < 1 || utf8.RuneCountInString(input.Body) > 1000 || strings.ContainsRune(input.Body, 0) {
		writeJSON(w, 400, map[string]string{"error": "invalid_message", "message": "Нужны client_message_id (UUID) и body (1–1000 символов)"})
		return
	}
	message, created, err := h.store.Send(r.Context(), id, user.ID, input.ClientMessageID, input.Body)
	if err != nil {
		chatFailure(w, err)
		return
	}
	delivery := "disabled"
	if h.publisher.Enabled() {
		// The message is committed before contacting Pusher. Replays re-publish the
		// same ID, so clients must deduplicate HTTP responses and realtime events.
		delivery = "published"
		if err := h.publisher.Publish("private-conversation-"+id, "message.created", map[string]any{"message": message}); err != nil {
			log.Println("Pusher: не удалось отправить событие; сообщение сохранено в PostgreSQL")
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

func (h *chatHandlers) realtimeConfig(w http.ResponseWriter, r *http.Request, user auth.User) {
	config := make(map[string]any)
	for key, value := range h.publisher.PublicConfig() {
		config[key] = value
	}
	config["media"] = map[string]bool{"voice": true, "profile_photos": true}
	writeJSON(w, 200, config)
}

func (h *chatHandlers) pusherAuth(w http.ResponseWriter, r *http.Request, user auth.User) {
	var input struct {
		SocketID    string `json:"socket_id"`
		ChannelName string `json:"channel_name"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	const prefix = "private-conversation-"
	id := strings.TrimPrefix(input.ChannelName, prefix)
	personal := input.ChannelName == "private-user-"+user.ID
	if (!personal && (!strings.HasPrefix(input.ChannelName, prefix) || !uuidPattern.MatchString(id))) || len(input.SocketID) > 100 || !socketPattern.MatchString(input.SocketID) {
		writeJSON(w, 400, map[string]string{"error": "invalid_channel"})
		return
	}
	if !personal {
		if err := h.store.CheckMember(r.Context(), id, user.ID); err != nil {
			chatFailure(w, err)
			return
		}
	}
	if !h.publisher.Enabled() {
		writeJSON(w, 503, map[string]string{"error": "pusher_not_configured"})
		return
	}
	data, err := h.publisher.Authorize(input.SocketID, input.ChannelName)
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, json.RawMessage(data))
}

func conversationID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := strings.ToLower(r.PathValue("id"))
	if !uuidPattern.MatchString(id) {
		writeJSON(w, 400, map[string]string{"error": "invalid_conversation_id"})
		return "", false
	}
	return id, true
}

func pagination(w http.ResponseWriter, r *http.Request) (int64, int, bool) {
	limit := 20
	if value := r.URL.Query().Get("limit"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 100 {
			writeJSON(w, 400, map[string]string{"error": "invalid_limit"})
			return 0, 0, false
		}
		limit = n
	}
	var before int64
	if value := r.URL.Query().Get("before"); value != "" {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n < 1 {
			writeJSON(w, 400, map[string]string{"error": "invalid_cursor"})
			return 0, 0, false
		}
		before = n
	}
	return before, limit, true
}

func chatFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, chat.ErrProfileRequired) {
		writeJSON(w, 403, map[string]string{"error": "profile_required"})
		return
	}
	if errors.Is(err, chat.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if errors.Is(err, chat.ErrConflict) {
		writeJSON(w, 409, map[string]string{"error": "client_message_id_conflict"})
		return
	}
	log.Printf("Ошибка чата: %v", err)
	writeJSON(w, 500, map[string]string{"error": "internal_error"})
}
