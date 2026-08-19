package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// stagedFile writes a scratch file and registers it, returning its path and URL.
func stagedFile(t *testing.T, p *previewCache, token, contentType string, body []byte) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), token+".bin")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, p.stage(token, path, contentType)
}

// passthrough stands in for the embedded frontend assets.
func passthrough() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
}

func TestPreviewMiddlewareServesStagedFileOnce(t *testing.T) {
	p := newPreviewCache()
	path, url := stagedFile(t, p, "tok1", "image/png", []byte("PNGDATA"))
	handler := p.middleware(passthrough())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "PNGDATA" {
		t.Errorf("body = %q, want PNGDATA", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("content type = %q, want image/png", got)
	}
	// Object data must not sit in the webview's HTTP cache.
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("cache-control = %q, want no-store", got)
	}

	// The token is consumed and the scratch file removed, so object data stops
	// being reachable from the webview the moment it has been rendered.
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, url, nil))
	if second.Code != http.StatusNotFound {
		t.Errorf("second fetch status = %d, want 404", second.Code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("the staged file should be deleted after it is served")
	}
}

func TestPreviewMiddlewarePassesOtherPathsThrough(t *testing.T) {
	handler := newPreviewCache().middleware(passthrough())
	for _, path := range []string{"/", "/index.html", "/assets/app.js", "/_previews/x"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusTeapot {
			t.Errorf("%s was intercepted (status %d) instead of passed through", path, rec.Code)
		}
	}
}

func TestPreviewMiddlewareRejectsUnknownAndTraversalTokens(t *testing.T) {
	p := newPreviewCache()
	handler := p.middleware(passthrough())

	for _, path := range []string{
		previewRoute + "nope",
		previewRoute + "..%2F..%2Fetc%2Fpasswd",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s returned %d, want 404", path, rec.Code)
		}
	}
}

func TestPreviewExpiryDropsTheFile(t *testing.T) {
	p := newPreviewCache()
	path, url := stagedFile(t, p, "tok2", "application/pdf", []byte("%PDF"))

	// Expire it by hand rather than waiting out the real TTL.
	p.mu.Lock()
	item := p.items["tok2"]
	item.expires = time.Now().Add(-time.Minute)
	p.items["tok2"] = item
	p.mu.Unlock()

	rec := httptest.NewRecorder()
	p.middleware(passthrough()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("an expired preview returned %d, want 404", rec.Code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("an expired preview should have its file removed")
	}
}

func TestDiscardAllRemovesEveryStagedFile(t *testing.T) {
	p := newPreviewCache()
	pathA, _ := stagedFile(t, p, "a", "image/png", []byte("a"))
	pathB, _ := stagedFile(t, p, "b", "image/png", []byte("b"))

	p.discardAll()

	for _, path := range []string{pathA, pathB} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s survived shutdown cleanup", path)
		}
	}
	if len(p.items) != 0 {
		t.Errorf("%d previews still registered after discardAll", len(p.items))
	}
}

func TestStagingEvictsExpiredNeighbours(t *testing.T) {
	p := newPreviewCache()
	stale, _ := stagedFile(t, p, "stale", "image/png", []byte("x"))
	p.mu.Lock()
	item := p.items["stale"]
	item.expires = time.Now().Add(-time.Hour)
	p.items["stale"] = item
	p.mu.Unlock()

	// Staging anything new sweeps previews nobody ever fetched.
	stagedFile(t, p, "fresh", "image/png", []byte("y"))

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("an abandoned preview should be swept when the next one is staged")
	}
	if _, ok := p.items["stale"]; ok {
		t.Error("the expired entry should be gone from the cache")
	}
}
