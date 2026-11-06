package httpapi

import (
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type authAttempt struct {
	count int
	until time.Time
}

// A bounded, per-process limiter. Proxy headers are deliberately not trusted.
func protectAuth(next http.Handler) http.Handler {
	return protectAPI(next, 20)
}

func protectAPI(next http.Handler, maxAttempts int) http.Handler {
	var mu sync.Mutex
	attempts := make(map[string]authAttempt)
	origins := os.Getenv("ALLOWED_ORIGINS")
	if origins == "" {
		origins = "http://localhost:5173,http://localhost:3000,http://localhost:9090"
	}
	allowed := make(map[string]bool)
	for _, origin := range strings.Split(origins, ",") {
		allowed[strings.TrimSpace(origin)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !allowed[origin] {
				writeJSON(w, 403, map[string]string{"error": "origin_not_allowed"})
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")
			w.WriteHeader(204)
			return
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" && origin == "" {
			writeJSON(w, 403, map[string]string{"error": "origin_required"})
			return
		}
		if r.Method == http.MethodPost {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			now := time.Now()
			mu.Lock()
			for key, value := range attempts {
				if !now.Before(value.until) {
					delete(attempts, key)
				}
			}
			a := attempts[ip]
			blocked := a.count >= maxAttempts || (a.count == 0 && len(attempts) >= 10000)
			if !blocked {
				if a.count == 0 {
					a.until = now.Add(time.Minute)
				}
				a.count++
				attempts[ip] = a
			}
			mu.Unlock()
			if blocked {
				w.Header().Set("Retry-After", "60")
				writeJSON(w, 429, map[string]string{"error": "too_many_requests"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
