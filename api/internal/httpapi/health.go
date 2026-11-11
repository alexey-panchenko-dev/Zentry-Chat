package httpapi

import "net/http"

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": "5.0.0", "capabilities": map[string]bool{"birth_dates": true, "short_passwords": true}})
}
