package httpapi

import (
	"encoding/hex"
	"net/http"
	"os"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	status := map[string]any{"status": "ok", "version": "5.0.0", "capabilities": map[string]bool{"birth_dates": true, "short_passwords": true}}
	if revision := os.Getenv("RENDER_GIT_COMMIT"); len(revision) == 40 {
		if _, err := hex.DecodeString(revision); err == nil {
			status["revision"] = revision
		}
	}
	writeJSON(w, http.StatusOK, status)
}
