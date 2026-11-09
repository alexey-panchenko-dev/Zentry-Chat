package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"newsChat/api/internal/auth"
	"newsChat/api/internal/calls"
	"newsChat/api/internal/realtime"
)

type callHandlers struct {
	store     *calls.Store
	publisher realtime.Publisher
}

func (h *callHandlers) create(w http.ResponseWriter, r *http.Request, user auth.User) {
	convID, ok := conversationID(w, r)
	if !ok {
		return
	}
	var input struct {
		ClientCallID string `json:"client_call_id"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	input.ClientCallID = strings.ToLower(input.ClientCallID)
	if !uuidPattern.MatchString(input.ClientCallID) {
		writeJSON(w, 400, map[string]string{"error": "invalid_client_call_id"})
		return
	}
	call, created, err := h.store.Create(r.Context(), convID, user.ID, input.ClientCallID)
	if err != nil {
		callFailure(w, err)
		return
	}
	status := 200
	if created {
		status = 201
	}
	writeJSON(w, status, map[string]any{"call": call, "realtime": h.notify(call)})
}

func (h *callHandlers) get(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := callID(w, r)
	if !ok {
		return
	}
	call, err := h.store.Get(r.Context(), id, user.ID)
	if err != nil {
		callFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"call": call})
}

func (h *callHandlers) active(w http.ResponseWriter, r *http.Request, user auth.User) {
	list, err := h.store.Active(r.Context(), user.ID)
	if err != nil {
		callFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"calls": list})
}

func (h *callHandlers) history(w http.ResponseWriter, r *http.Request, user auth.User) {
	list, err := h.store.History(r.Context(), user.ID)
	if err != nil {
		callFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"calls": list})
}

func (h *callHandlers) transition(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := callID(w, r)
	if !ok {
		return
	}
	var input struct {
		Action string `json:"action"`
	}
	if !readAuthJSON(w, r, &input) {
		return
	}
	switch input.Action {
	case "accept", "reject", "cancel", "end":
	default:
		writeJSON(w, 400, map[string]string{"error": "invalid_call_action"})
		return
	}
	call, err := h.store.Transition(r.Context(), id, user.ID, input.Action)
	if err != nil {
		callFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"call": call, "realtime": h.notify(call)})
}

func (h *callHandlers) sendSignal(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := callID(w, r)
	if !ok {
		return
	}
	var input struct {
		ClientSignalID string          `json:"client_signal_id"`
		Type           string          `json:"type"`
		Payload        json.RawMessage `json:"payload"`
	}
	if !readJSONLimit(w, r, &input, 64*1024) {
		return
	}
	input.ClientSignalID = strings.ToLower(input.ClientSignalID)
	if !uuidPattern.MatchString(input.ClientSignalID) || !validSignal(input.Type, input.Payload) {
		writeJSON(w, 400, map[string]string{"error": "invalid_signal"})
		return
	}
	signal, created, err := h.store.SendSignal(r.Context(), id, user.ID, input.ClientSignalID, input.Type, input.Payload)
	if err != nil {
		callFailure(w, err)
		return
	}
	delivery := "disabled"
	if h.publisher.Enabled() {
		delivery = "published"
		err := h.publisher.Publish("private-user-"+signal.RecipientID, "call.signal", map[string]any{"call_id": id, "signal_id": signal.ID, "type": signal.Type})
		if err != nil {
			delivery = "failed"
			log.Println("Pusher: сигнал сохранён, уведомление не отправлено")
		}
	}
	status := 200
	if created {
		status = 201
	}
	writeJSON(w, status, map[string]any{"signal_id": signal.ID, "realtime": delivery})
}

func (h *callHandlers) signals(w http.ResponseWriter, r *http.Request, user auth.User) {
	id, ok := callID(w, r)
	if !ok {
		return
	}
	var after int64
	if value := r.URL.Query().Get("after"); value != "" {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n < 0 {
			writeJSON(w, 400, map[string]string{"error": "invalid_cursor"})
			return
		}
		after = n
	}
	list, err := h.store.Signals(r.Context(), id, user.ID, after)
	if err != nil {
		callFailure(w, err)
		return
	}
	cursor := strconv.FormatInt(after, 10)
	if len(list) > 0 {
		cursor = list[len(list)-1].ID
	}
	writeJSON(w, 200, map[string]any{"signals": list, "next_cursor": cursor, "has_more": len(list) == 100})
}

func (h *callHandlers) notify(call calls.Call) string {
	if !h.publisher.Enabled() {
		return "disabled"
	}
	result := "published"
	for _, id := range []string{call.CallerID, call.CalleeID} {
		if err := h.publisher.Publish("private-user-"+id, "call.updated", map[string]any{"call": call}); err != nil {
			result = "failed"
			log.Println("Pusher: состояние звонка сохранено, уведомление не отправлено")
		}
	}
	return result
}

func validSignal(kind string, payload json.RawMessage) bool {
	if len(payload) == 0 || len(payload) > 60*1024 {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(payload, &fields) != nil || len(fields) == 0 {
		return false
	}
	for key, value := range fields {
		allowed := (kind == "offer" || kind == "answer") && (key == "type" || key == "sdp") || kind == "ice" && (key == "candidate" || key == "sdpMid" || key == "sdpMLineIndex" || key == "usernameFragment")
		if !allowed {
			return false
		}
		var text string
		if json.Unmarshal(value, &text) == nil && strings.ContainsRune(text, 0) {
			return false
		}
	}
	switch kind {
	case "offer", "answer":
		var value struct {
			Type string `json:"type"`
			SDP  string `json:"sdp"`
		}
		return json.Unmarshal(payload, &value) == nil && value.Type == kind && strings.HasPrefix(value.SDP, "v=0") && len(value.SDP) > 3
	case "ice":
		var value struct {
			Candidate     *string `json:"candidate"`
			SDPMid        *string `json:"sdpMid"`
			SDPMLineIndex *uint16 `json:"sdpMLineIndex"`
		}
		if json.Unmarshal(payload, &value) != nil || value.Candidate == nil {
			return false
		}
		return len(*value.Candidate) <= 4096 && (*value.Candidate == "" || strings.HasPrefix(*value.Candidate, "candidate:"))
	}
	return false
}

func callID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := strings.ToLower(r.PathValue("callID"))
	if !uuidPattern.MatchString(id) {
		writeJSON(w, 400, map[string]string{"error": "invalid_call_id"})
		return "", false
	}
	return id, true
}

func callFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, calls.ErrBlocked):
		writeJSON(w, 403, map[string]string{"error": "contact_blocked"})
	case errors.Is(err, calls.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(err, calls.ErrBusy):
		writeJSON(w, 409, map[string]string{"error": "participant_busy"})
	case errors.Is(err, calls.ErrState):
		writeJSON(w, 409, map[string]string{"error": "invalid_call_state"})
	case errors.Is(err, calls.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "idempotency_conflict"})
	default:
		log.Printf("Ошибка звонка: %v", err)
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
}
