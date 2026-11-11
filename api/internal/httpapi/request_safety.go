package httpapi

import (
	"context"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Reject excess work before reading a body or acquiring a database connection.
// The media gate bounds expensive decoding independently of small API requests.
func boundedRequests(next http.Handler) http.Handler {
	active := make(chan struct{}, 64)
	media := make(chan struct{}, 2)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		select {
		case active <- struct{}{}:
			defer func() { <-active }()
		default:
			w.Header().Set("Retry-After", "1")
			writeJSON(w, 503, map[string]string{"error": "server_busy"})
			return
		}
		contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		mediaRead := (r.Method == http.MethodGet || r.Method == http.MethodHead) &&
			(strings.HasSuffix(r.URL.Path, "/image") || strings.HasSuffix(r.URL.Path, "/photo") ||
				strings.HasSuffix(r.URL.Path, "/audio") || strings.HasSuffix(r.URL.Path, "/file"))
		if contentType == "multipart/form-data" || mediaRead {
			// Media is stored as database bytes, so original downloads also need
			// this budget. Normal page loads queue briefly; uploads fail fast.
			if mediaRead {
				timer := time.NewTimer(2 * time.Second)
				defer timer.Stop()
				select {
				case media <- struct{}{}:
				case <-r.Context().Done():
					return
				case <-timer.C:
					w.Header().Set("Retry-After", "1")
					writeJSON(w, 503, map[string]string{"error": "server_busy"})
					return
				}
			} else {
				select {
				case media <- struct{}{}:
				default:
					w.Header().Set("Retry-After", "1")
					writeJSON(w, 503, map[string]string{"error": "server_busy"})
					return
				}
			}
			defer func() { <-media }()
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type accountBudget struct {
	mu         sync.Mutex
	entries    map[string]authAttempt
	sweepAfter time.Time
}

func (b *accountBudget) allow(user, method string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.entries == nil {
		b.entries = make(map[string]authAttempt)
	}
	now := time.Now()
	if !now.Before(b.sweepAfter) {
		for key, value := range b.entries {
			if !now.Before(value.until) {
				delete(b.entries, key)
			}
		}
		b.sweepAfter = now.Add(5 * time.Second)
	}
	limit := 120
	if method == http.MethodGet || method == http.MethodHead {
		user += ":read"
		limit = 300
	}
	a := b.entries[user]
	if !now.Before(a.until) {
		a = authAttempt{until: now.Add(time.Minute)}
	}
	if a.count >= limit || (a.count == 0 && len(b.entries) >= 10000) {
		return false
	}
	a.count++
	b.entries[user] = a
	return true
}
