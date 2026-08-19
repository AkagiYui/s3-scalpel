package main

import (
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

// previewRoute is the URL prefix the webview fetches staged previews from.
const previewRoute = "/_preview/"

// previewTTL bounds how long a staged preview stays fetchable. The dialog loads
// it immediately, so this only has to cover the round trip plus a reload.
const previewTTL = 10 * time.Minute

// stagedPreview is a file downloaded for preview, waiting to be fetched by the
// webview exactly once (or expire).
type stagedPreview struct {
	path        string
	contentType string
	expires     time.Time
}

// previewCache stages preview files on disk and serves them over the app's own
// asset server.
//
// The alternative — inlining each preview as a base64 data: URL — costs roughly
// 2.3x the object's size in live memory (the file bytes, the base64 string, and
// the copy the webview parses) and blocks until the whole thing is encoded. A
// local URL streams instead, and gives the webview range requests for free,
// which is what makes an embedded PDF viewer usable.
type previewCache struct {
	mu    sync.Mutex
	items map[string]stagedPreview
}

func newPreviewCache() *previewCache {
	return &previewCache{items: map[string]stagedPreview{}}
}

// stage registers a downloaded file under a fresh token and returns the URL the
// frontend should load.
func (p *previewCache) stage(token, filePath, contentType string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.evictExpiredLocked()
	p.items[token] = stagedPreview{
		path:        filePath,
		contentType: contentType,
		expires:     time.Now().Add(previewTTL),
	}
	return previewRoute + token
}

// take resolves a token, consuming it: a preview is fetched once, and leaving it
// resolvable afterwards would keep object data reachable from the webview for
// longer than it is needed.
func (p *previewCache) take(token string) (stagedPreview, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	item, ok := p.items[token]
	if !ok || time.Now().After(item.expires) {
		if ok {
			delete(p.items, token)
			_ = os.Remove(item.path)
		}
		return stagedPreview{}, false
	}
	delete(p.items, token)
	return item, true
}

// evictExpiredLocked drops and deletes previews nobody fetched. Callers hold mu.
func (p *previewCache) evictExpiredLocked() {
	now := time.Now()
	for token, item := range p.items {
		if now.After(item.expires) {
			delete(p.items, token)
			_ = os.Remove(item.path)
		}
	}
}

// discardAll removes every staged file (called on shutdown).
func (p *previewCache) discardAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for token, item := range p.items {
		delete(p.items, token)
		_ = os.Remove(item.path)
	}
}

// middleware serves staged previews and passes everything else through to the
// embedded frontend assets.
func (p *previewCache) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hasPrefix(r.URL.Path, previewRoute) {
			next.ServeHTTP(w, r)
			return
		}
		// path.Base defends against a token containing traversal segments; the
		// token is only ever a map key, never joined onto a filesystem path.
		token := path.Base(r.URL.Path)
		item, ok := p.take(token)
		if !ok {
			http.NotFound(w, r)
			return
		}
		defer func() { _ = os.Remove(item.path) }()

		f, err := os.Open(item.path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer func() { _ = f.Close() }()
		info, err := f.Stat()
		if err != nil {
			http.Error(w, "preview unavailable", http.StatusInternalServerError)
			return
		}

		if item.contentType != "" {
			w.Header().Set("Content-Type", item.contentType)
		}
		// Object data must not linger in any HTTP cache the webview keeps.
		w.Header().Set("Cache-Control", "no-store")
		// ServeContent handles range requests, which is what lets the embedded
		// PDF viewer jump to a page without re-reading the whole file.
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	})
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
