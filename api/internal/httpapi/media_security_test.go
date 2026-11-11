package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMediaOriginGuardAndPreflight(t *testing.T) {
	for _, path := range []string{"/api/profile/photo", "/api/conversations/a/voice", "/api/messages/1/audio", "/api/profiles/a/photo"} {
		r := httptest.NewRequest(http.MethodPost, path, nil)
		r.Header.Set("Origin", "https://evil.example")
		w := httptest.NewRecorder()
		newRouter(nil).ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("%s cross origin: %d", path, w.Code)
		}
	}
	r := httptest.NewRequest(http.MethodOptions, "/api/profile/photo", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	newRouter(nil).ServeHTTP(w, r)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Methods") != "GET, POST, PUT, DELETE, OPTIONS" {
		t.Fatalf("delete preflight %d %v", w.Code, w.Header())
	}
}
