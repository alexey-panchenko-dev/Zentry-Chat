package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSwaggerRoutes(t *testing.T) {
	mux := http.NewServeMux()
	registerDocs(mux)
	for _, path := range []string{"/swagger/", "/swagger/openapi.json"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: status %d", path, w.Code)
		}
		if path == "/swagger/openapi.json" {
			var spec struct {
				OpenAPI string         `json:"openapi"`
				Paths   map[string]any `json:"paths"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
				t.Fatal(err)
			}
			if spec.OpenAPI != "3.0.3" {
				t.Fatal("invalid API specification")
			}
			for _, path := range []string{"/api/health", "/api/ready", "/api/auth/register", "/api/auth/login", "/api/auth/logout", "/api/auth/me", "/api/users", "/api/conversations", "/api/conversations/{id}/messages", "/api/pusher/auth", "/api/realtime/config"} {
				if _, ok := spec.Paths[path]; !ok {
					t.Fatalf("undocumented route: %s", path)
				}
			}
			for _, path := range []string{"/api/conversations/{id}/calls", "/api/calls", "/api/calls/{callID}", "/api/calls/{callID}/actions", "/api/calls/{callID}/signals"} {
				if _, ok := spec.Paths[path]; !ok {
					t.Fatalf("undocumented call route: %s", path)
				}
			}
			for _, path := range []string{"/api/profile", "/api/discovery", "/api/discovery/likes", "/api/discovery/votes", "/api/unread", "/api/conversations/{id}/read"} {
				if _, ok := spec.Paths[path]; !ok {
					t.Fatalf("undocumented discovery/read route: %s", path)
				}
			}
		}
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/swagger", nil))
	if w.Code != 307 || w.Header().Get("Location") != "/swagger/" {
		t.Fatal("missing Swagger redirect")
	}
}
