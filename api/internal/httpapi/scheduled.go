package httpapi

import (
	"errors"
	"net/http"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
	"strings"
	"time"
	"unicode/utf8"
)

type scheduleInput struct {
	Scope    string    `json:"scope"`
	ChatID   string    `json:"chat_id"`
	ClientID string    `json:"client_message_id"`
	Body     string    `json:"body"`
	SendAt   time.Time `json:"send_at"`
}

func validScheduleBody(body string, at time.Time) bool {
	return utf8.ValidString(body) && !strings.ContainsRune(body, 0) && utf8.RuneCountInString(strings.TrimSpace(body)) >= 1 && utf8.RuneCountInString(body) <= 1000 && at.After(time.Now().Add(5*time.Second)) && at.Before(time.Now().Add(366*24*time.Hour))
}
func scheduleFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, chat.ErrScheduleState) {
		writeJSON(w, 409, map[string]string{"error": "schedule_state"})
		return
	}
	if errors.Is(err, chat.ErrScheduleLimit) {
		writeJSON(w, 429, map[string]string{"error": "schedule_limit"})
		return
	}
	chatFailure(w, err)
}
func (h *chatHandlers) createScheduled(w http.ResponseWriter, r *http.Request, u auth.User) {
	var input scheduleInput
	if !readAuthJSON(w, r, &input) {
		return
	}
	input.Body = strings.TrimSpace(input.Body)
	if (input.Scope != "personal" && input.Scope != "group") || !uuidPattern.MatchString(input.ChatID) || !uuidPattern.MatchString(input.ClientID) || !validScheduleBody(input.Body, input.SendAt) {
		writeJSON(w, 400, map[string]string{"error": "invalid_schedule"})
		return
	}
	item, err := h.store.Schedule(r.Context(), u.ID, input.Scope, input.ChatID, input.ClientID, input.Body, input.SendAt)
	if err != nil {
		scheduleFailure(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"scheduled": item})
}
func (h *chatHandlers) listScheduled(w http.ResponseWriter, r *http.Request, u auth.User) {
	scope, id := r.URL.Query().Get("scope"), r.URL.Query().Get("chat_id")
	if (scope != "personal" && scope != "group") || !uuidPattern.MatchString(id) {
		writeJSON(w, 400, map[string]string{"error": "invalid_schedule"})
		return
	}
	items, err := h.store.Scheduled(r.Context(), u.ID, scope, id)
	if err != nil {
		scheduleFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"scheduled": items})
}
func (h *chatHandlers) changeScheduled(w http.ResponseWriter, r *http.Request, u auth.User) {
	id := r.PathValue("scheduleID")
	if !uuidPattern.MatchString(id) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	cancel := r.Method == http.MethodDelete
	var input struct {
		Body   string    `json:"body"`
		SendAt time.Time `json:"send_at"`
	}
	if !cancel {
		if !readAuthJSON(w, r, &input) {
			return
		}
		input.Body = strings.TrimSpace(input.Body)
		if !validScheduleBody(input.Body, input.SendAt) {
			writeJSON(w, 400, map[string]string{"error": "invalid_schedule"})
			return
		}
	}
	if err := h.store.ChangeScheduled(r.Context(), u.ID, id, input.Body, input.SendAt, cancel); err != nil {
		scheduleFailure(w, err)
		return
	}
	w.WriteHeader(204)
}
