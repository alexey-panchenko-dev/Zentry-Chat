package httpapi

import (
	"errors"
	"net/http"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
	"strings"
	"unicode/utf8"
)

func (h *chatHandlers) reportAbuse(w http.ResponseWriter, r *http.Request, u auth.User) {
	var v chat.Report
	if !readAuthJSON(w, r, &v) {
		return
	}
	v.Details = strings.TrimSpace(v.Details)
	if !uuidPattern.MatchString(v.UserID) || v.UserID == u.ID || !utf8.ValidString(v.Details) || utf8.RuneCountInString(v.Details) > 1000 || strings.ContainsRune(v.Details, 0) {
		writeJSON(w, 400, map[string]string{"error": "invalid_report"})
		return
	}
	switch v.Scope {
	case "user", "discovery":
		if v.MessageID != 0 {
			writeJSON(w, 400, map[string]string{"error": "invalid_report"})
			return
		}
	case "message", "group_message":
		if v.MessageID < 1 {
			writeJSON(w, 400, map[string]string{"error": "invalid_report"})
			return
		}
	default:
		writeJSON(w, 400, map[string]string{"error": "invalid_report"})
		return
	}
	switch v.Category {
	case "spam", "harassment", "fraud", "danger", "other":
	default:
		writeJSON(w, 400, map[string]string{"error": "invalid_report"})
		return
	}
	err := h.store.Report(r.Context(), u.ID, v)
	if errors.Is(err, chat.ErrReportLimit) {
		w.Header().Set("Retry-After", "86400")
		writeJSON(w, 429, map[string]string{"error": "report_limit"})
		return
	}
	if err != nil {
		chatFailure(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"received": true})
}
