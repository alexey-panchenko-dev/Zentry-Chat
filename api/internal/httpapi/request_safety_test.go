package httpapi

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestReadAndWriteLimitsRejectSpoofedProxyHeaders(t *testing.T) {
	h := protectAPI(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), 2)
	for _, method := range []string{"PUT", "DELETE", "GET"} {
		limit := 2
		if method == "GET" {
			limit = 10
		}
		for i := 0; i <= limit; i++ {
			r := httptest.NewRequest(method, "/api/test", nil)
			r.RemoteAddr = "192.0.2.1:1234"
			r.Header.Set("X-Forwarded-For", string(rune('a'+i)))
			if method == "DELETE" {
				r.RemoteAddr = "192.0.2.2:1234"
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			want := 204
			if i == limit {
				want = 429
			}
			if w.Code != want {
				t.Fatalf("%s attempt %d: %d want %d", method, i, w.Code, want)
			}
		}
	}
}

func TestMediaConcurrencyIsBoundedBeforeReadingBodies(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	h := boundedRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { entered <- struct{}{}; <-release; w.WriteHeader(204) }))
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			r := httptest.NewRequest("POST", "/api/upload", nil)
			r.Header.Set("Content-Type", "multipart/form-data; boundary=test")
			h.ServeHTTP(httptest.NewRecorder(), r)
		}()
		<-entered
	}
	for _, contentType := range []string{"multipart/form-data; boundary=test", "Multipart/Form-Data; boundary=test"} {
		r := httptest.NewRequest("POST", "/api/upload", nil)
		r.Header.Set("Content-Type", contentType)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 503 || w.Header().Get("Retry-After") != "1" {
			close(release)
			workers.Wait()
			t.Fatal("media overload was not rejected", contentType, w.Code)
		}
	}
	close(release)
	workers.Wait()
}

func TestImagePreviewsQueueWithinSharedMediaBudget(t *testing.T) {
	entered := make(chan struct{}, 3)
	release := make(chan struct{})
	h := boundedRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
		w.WriteHeader(204)
	}))
	var workers sync.WaitGroup
	for _, path := range []string{"/api/messages/1/file", "/api/profiles/1/photo"} {
		workers.Add(1)
		go func() {
			defer workers.Done()
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
		}()
		<-entered
	}
	done := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/messages/2/image?preview=1", nil))
		done <- w.Code
	}()
	select {
	case <-entered:
		close(release)
		workers.Wait()
		t.Fatal("third image decoder bypassed media budget")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	workers.Wait()
	select {
	case code := <-done:
		if code != 204 {
			t.Fatal("normal preview queue failed", code)
		}
	case <-time.After(time.Second):
		t.Fatal("preview did not resume after a slot was released")
	}
}

func TestAPIConcurrencyAndRequestDeadline(t *testing.T) {
	entered := make(chan struct{}, 64)
	release := make(chan struct{})
	h := boundedRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deadline, ok := r.Context().Deadline(); !ok || time.Until(deadline) > 15*time.Second {
			t.Error("missing bounded request lifetime")
		}
		entered <- struct{}{}
		<-release
		w.WriteHeader(204)
	}))
	var workers sync.WaitGroup
	for range 64 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/test", nil))
		}()
		<-entered
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/test", nil))
	close(release)
	workers.Wait()
	if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("direct API omitted security headers")
	}
	if w.Code != 503 {
		t.Fatal("unbounded concurrent API work", w.Code)
	}
}

func TestAccountLimitsAreIndependentAndSeparateReadsFromWrites(t *testing.T) {
	var b accountBudget
	for i := 0; i < 120; i++ {
		if !b.allow("one", "POST") {
			t.Fatal("early write limit")
		}
	}
	if b.allow("one", "PUT") {
		t.Fatal("mutation methods bypassed the account limit")
	}
	if !b.allow("two", "POST") || !b.allow("one", "GET") {
		t.Fatal("one account blocked a peer or reads")
	}
	for i := 1; i < 300; i++ {
		if !b.allow("one", "GET") {
			t.Fatal("early read limit")
		}
	}
	if b.allow("one", "GET") {
		t.Fatal("unbounded reads")
	}
}
