package httpapi

import (
	"errors"
	"net/http"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
	"strings"
	"unicode/utf8"
)

func (h *chatHandlers) groupContacts(w http.ResponseWriter, r *http.Request, u auth.User) {
	v, e := h.store.GroupContacts(r.Context(), u.ID)
	if e != nil {
		chatFailure(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"contacts": v})
}
func (h *chatHandlers) createGroup(w http.ResponseWriter, r *http.Request, u auth.User) {
	var in struct {
		Name      string   `json:"name"`
		MemberIDs []string `json:"member_ids"`
	}
	if !readAuthJSON(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if !utf8.ValidString(in.Name) || strings.ContainsRune(in.Name, 0) || utf8.RuneCountInString(in.Name) < 1 || utf8.RuneCountInString(in.Name) > 80 || len(in.MemberIDs) < 2 || len(in.MemberIDs) > 49 {
		writeJSON(w, 400, map[string]string{"error": "invalid_group"})
		return
	}
	seen := map[string]bool{}
	for i, id := range in.MemberIDs {
		id = strings.ToLower(id)
		if !uuidPattern.MatchString(id) || id == u.ID || seen[id] {
			writeJSON(w, 400, map[string]string{"error": "invalid_members"})
			return
		}
		seen[id] = true
		in.MemberIDs[i] = id
	}
	g, members, e := h.store.CreateGroup(r.Context(), u.ID, in.Name, in.MemberIDs)
	if errors.Is(e, chat.ErrGroupMembers) {
		writeJSON(w, 400, map[string]string{"error": "contacts_required"})
		return
	}
	if e != nil {
		chatFailure(w, e)
		return
	}
	h.publishGroupUpdate(g.ID, members...)
	writeJSON(w, 201, map[string]any{"group": g})
}
func (h *chatHandlers) listGroups(w http.ResponseWriter, r *http.Request, u auth.User) {
	v, e := h.store.Groups(r.Context(), u.ID)
	if e != nil {
		chatFailure(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"groups": v})
}
func (h *chatHandlers) groupMembers(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}
	v, e := h.store.GroupMembers(r.Context(), id, u.ID)
	if e != nil {
		chatFailure(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"members": v})
}
func (h *chatHandlers) groupMessages(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}
	before, limit, ok := pagination(w, r)
	if !ok {
		return
	}
	v, e := h.store.GroupMessages(r.Context(), id, u.ID, before, limit+1)
	if e != nil {
		chatFailure(w, e)
		return
	}
	cursor := ""
	if len(v) > limit {
		v = v[:limit]
		cursor = v[len(v)-1].ID
	}
	writeJSON(w, 200, map[string]any{"messages": v, "next_cursor": cursor})
}
func (h *chatHandlers) sendGroupMessage(w http.ResponseWriter, r *http.Request, u auth.User) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}
	var in struct {
		ClientMessageID string `json:"client_message_id"`
		Body            string `json:"body"`
	}
	if !readAuthJSON(w, r, &in) {
		return
	}
	in.ClientMessageID = strings.ToLower(in.ClientMessageID)
	in.Body = strings.TrimSpace(in.Body)
	if !uuidPattern.MatchString(in.ClientMessageID) || !utf8.ValidString(in.Body) || strings.ContainsRune(in.Body, 0) || utf8.RuneCountInString(in.Body) < 1 || utf8.RuneCountInString(in.Body) > 1000 {
		writeJSON(w, 400, map[string]string{"error": "invalid_message"})
		return
	}
	m, created, members, e := h.store.SendGroupMessage(r.Context(), id, u.ID, in.ClientMessageID, in.Body)
	if e != nil {
		chatFailure(w, e)
		return
	}
	h.publishGroupUpdate(id, members...)
	status := 200
	if created {
		status = 201
	}
	writeJSON(w, status, map[string]any{"message": m})
}
func groupID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := strings.ToLower(r.PathValue("id"))
	if !uuidPattern.MatchString(id) {
		writeJSON(w, 400, map[string]string{"error": "invalid_group_id"})
		return "", false
	}
	return id, true
}
func (h *chatHandlers) publishGroupUpdate(id string, users ...string) {
	if !h.publisher.Enabled() {
		return
	}
	for _, user := range users {
		_ = h.publisher.Publish("private-user-"+user, "group.updated", map[string]string{"group_id": id})
	}
}
