package httpapi

import (
	"net/http"

	"newsChat/api/internal/auth"
	"newsChat/api/internal/calls"
	"newsChat/api/internal/chat"
	"newsChat/api/internal/realtime"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newRouter(db *pgxpool.Pool, publishers ...realtime.Publisher) http.Handler {
	mux := http.NewServeMux()
	registerDocs(mux)
	h := &authHandlers{service: auth.New(db)}
	var publisher realtime.Publisher = &realtime.Client{}
	if len(publishers) > 0 {
		publisher = publishers[0]
	}
	ch := &chatHandlers{store: chat.New(db), auth: h.service, publisher: publisher}
	chatMux := http.NewServeMux()
	chatMux.HandleFunc("GET /api/profile", ch.signedIn(ch.getProfile))
	chatMux.HandleFunc("PUT /api/profile", ch.signedIn(ch.updateProfile))
	chatMux.HandleFunc("GET /api/discovery", ch.signedIn(ch.discovery))
	chatMux.HandleFunc("GET /api/discovery/likes", ch.signedIn(ch.discovery))
	chatMux.HandleFunc("POST /api/discovery/votes", ch.signedIn(ch.vote))
	chatMux.HandleFunc("GET /api/unread", ch.signedIn(ch.unread))
	chatMux.HandleFunc("GET /api/conversations/{id}/read", ch.signedIn(ch.readState))
	chatMux.HandleFunc("POST /api/conversations/{id}/read", ch.signedIn(ch.markRead))
	chatMux.HandleFunc("POST /api/conversations/{id}/voice", ch.signedIn(ch.sendVoice))
	chatMux.HandleFunc("GET /api/messages/{messageID}/audio", ch.signedIn(ch.audio))
	chatMux.HandleFunc("POST /api/profile/photo", ch.signedIn(ch.setPhoto))
	chatMux.HandleFunc("DELETE /api/profile/photo", ch.signedIn(ch.deletePhoto))
	chatMux.HandleFunc("GET /api/profiles/{userID}/photo", ch.signedIn(ch.photo))
	callHandlers := &callHandlers{store: calls.New(db), publisher: publisher}
	chatMux.HandleFunc("POST /api/conversations/{id}/calls", ch.signedIn(callHandlers.create))
	chatMux.HandleFunc("GET /api/calls", ch.signedIn(callHandlers.active))
	chatMux.HandleFunc("GET /api/calls/{callID}", ch.signedIn(callHandlers.get))
	chatMux.HandleFunc("POST /api/calls/{callID}/actions", ch.signedIn(callHandlers.transition))
	chatMux.HandleFunc("POST /api/calls/{callID}/signals", ch.signedIn(callHandlers.sendSignal))
	chatMux.HandleFunc("GET /api/calls/{callID}/signals", ch.signedIn(callHandlers.signals))
	chatMux.HandleFunc("GET /api/users", ch.signedIn(ch.searchUsers))
	chatMux.HandleFunc("POST /api/conversations", ch.signedIn(ch.createConversation))
	chatMux.HandleFunc("GET /api/conversations", ch.signedIn(ch.listConversations))
	chatMux.HandleFunc("GET /api/conversations/{id}/messages", ch.signedIn(ch.listMessages))
	chatMux.HandleFunc("POST /api/conversations/{id}/messages", ch.signedIn(ch.sendMessage))
	chatMux.HandleFunc("GET /api/realtime/config", ch.signedIn(ch.realtimeConfig))
	chatMux.HandleFunc("POST /api/pusher/auth", ch.signedIn(ch.pusherAuth))
	protectedChat := protectAPI(chatMux, 120)
	for _, pattern := range []string{"/api/profile", "/api/discovery", "/api/discovery/", "/api/unread", "/api/profile/", "/api/profiles/", "/api/messages/", "/api/users", "/api/conversations", "/api/conversations/", "/api/realtime/", "/api/pusher/", "/api/calls", "/api/calls/"} {
		mux.Handle(pattern, protectedChat)
	}
	authMux := http.NewServeMux()
	authMux.HandleFunc("POST /api/auth/register", h.register)
	authMux.HandleFunc("POST /api/auth/login", h.login)
	authMux.HandleFunc("POST /api/auth/logout", h.logout)
	authMux.HandleFunc("GET /api/auth/me", h.me)
	mux.Handle("/api/auth/", protectAuth(authMux))
	mux.HandleFunc("GET /api/ready", readinessHandler(db))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "route_not_found"})
	})

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
			return
		}
		healthHandler(w, r)
	})

	return mux
}
