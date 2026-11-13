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

func pollFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, chat.ErrPollClosed) {
		writeJSON(w, 409, map[string]string{"error": "poll_closed"})
		return
	}
	if errors.Is(err, chat.ErrPollInput) {
		writeJSON(w, 400, map[string]string{"error": "invalid_poll"})
		return
	}
	chatFailure(w, err)
}
func pollTarget(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	group, ok := groupID(w, r)
	if !ok {
		return "", "", false
	}
	id := r.PathValue("pollID")
	if !uuidPattern.MatchString(id) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return "", "", false
	}
	return group, id, true
}
func (h *chatHandlers) notifyPoll(r *http.Request, group, user string) {
	members, err := h.store.GroupMembers(r.Context(), group, user)
	if err != nil {
		return
	}
	users := make([]string, 0, len(members))
	for _, member := range members {
		users = append(users, member.ID)
	}
	h.publishGroupUpdate(group, users...)
}
func (h *chatHandlers) createPoll(w http.ResponseWriter, r *http.Request, u auth.User) {
	group, ok := groupID(w, r)
	if !ok {
		return
	}
	var input struct {
		ClientID string     `json:"client_message_id"`
		Question string     `json:"question"`
		Options  []string   `json:"options"`
		Multiple bool       `json:"multiple"`
		ClosesAt *time.Time `json:"closes_at"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	input.Question = strings.TrimSpace(input.Question)
	valid := uuidPattern.MatchString(input.ClientID) && utf8.ValidString(input.Question) && !strings.ContainsRune(input.Question, 0) && utf8.RuneCountInString(input.Question) >= 1 && utf8.RuneCountInString(input.Question) <= 300 && len(input.Options) >= 2 && len(input.Options) <= 10
	if input.ClosesAt != nil && (!input.ClosesAt.After(time.Now()) || input.ClosesAt.After(time.Now().Add(366*24*time.Hour))) {
		valid = false
	}
	seen := map[string]bool{}
	for i, option := range input.Options {
		option = strings.TrimSpace(option)
		input.Options[i] = option
		if !utf8.ValidString(option) || strings.ContainsRune(option, 0) || utf8.RuneCountInString(option) < 1 || utf8.RuneCountInString(option) > 100 || seen[strings.ToLower(option)] {
			valid = false
		}
		seen[strings.ToLower(option)] = true
	}
	if !valid {
		writeJSON(w, 400, map[string]string{"error": "invalid_poll"})
		return
	}
	poll, err := h.store.CreatePoll(r.Context(), group, u.ID, input.ClientID, input.Question, input.Options, input.Multiple, input.ClosesAt)
	if err != nil {
		pollFailure(w, err)
		return
	}
	h.notifyPoll(r, group, u.ID)
	writeJSON(w, 201, map[string]any{"poll": poll})
}
func (h *chatHandlers) getPoll(w http.ResponseWriter, r *http.Request, u auth.User) {
	group, id, ok := pollTarget(w, r)
	if !ok {
		return
	}
	poll, err := h.store.Poll(r.Context(), group, u.ID, id)
	if err != nil {
		pollFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"poll": poll})
}
func (h *chatHandlers) votePoll(w http.ResponseWriter, r *http.Request, u auth.User) {
	group, id, ok := pollTarget(w, r)
	if !ok {
		return
	}
	var input struct {
		Options []string `json:"option_ids"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	for _, option := range input.Options {
		if !uuidPattern.MatchString(option) {
			writeJSON(w, 400, map[string]string{"error": "invalid_poll"})
			return
		}
	}
	if err := h.store.VotePoll(r.Context(), group, u.ID, id, input.Options); err != nil {
		pollFailure(w, err)
		return
	}
	h.notifyPoll(r, group, u.ID)
	poll, err := h.store.Poll(r.Context(), group, u.ID, id)
	if err != nil {
		pollFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"poll": poll})
}
func (h *chatHandlers) closePoll(w http.ResponseWriter, r *http.Request, u auth.User) {
	group, id, ok := pollTarget(w, r)
	if !ok {
		return
	}
	if err := h.store.ClosePoll(r.Context(), group, u.ID, id); err != nil {
		pollFailure(w, err)
		return
	}
	h.notifyPoll(r, group, u.ID)
	w.WriteHeader(204)
}
